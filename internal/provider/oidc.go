package provider

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig defines pinned issuer endpoints and required MFA assurance.
type OIDCConfig struct {
	Issuer                        string   `json:"issuer"`
	ClientID                      string   `json:"client_id"`
	ClientSecretFile              string   `json:"client_secret_file"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	JWKSEndpoint                  string   `json:"jwks_endpoint"`
	IntrospectionEndpoint         string   `json:"introspection_endpoint"`
	IntrospectionClientID         string   `json:"introspection_client_id"`
	IntrospectionClientSecretFile string   `json:"introspection_client_secret_file"`
	IntrospectionAuthMethod       string   `json:"introspection_auth_method"`
	AllowedSigningAlgorithms      []string `json:"allowed_signing_algorithms"`
	MFAACRValues                  []string `json:"mfa_acr_values"`
}

// DecodeStrict decodes one bounded JSON value and rejects unknown fields and trailing data.
func DecodeStrict(r io.Reader, limit int64, v any) error {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if int64(len(b)) > limit {
		return errors.New("payload too large")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return err
	}
	if err = d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

// ReadPrivate reads a bounded owner-only regular secret file.
func ReadPrivate(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("secret file must be owner-only regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil {
		return nil, err
	}
	if len(b) > 16384 {
		return nil, errors.New("secret file too large")
	}
	return b, nil
}

// LoadOIDCConfig loads and validates explicit HTTPS issuer configuration.
func LoadOIDCConfig(path string) (OIDCConfig, error) {
	var c OIDCConfig
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer func() { _ = f.Close() }()
	if err = DecodeStrict(f, 16384, &c); err != nil {
		return c, err
	}
	for _, raw := range []string{c.Issuer, c.AuthorizationEndpoint, c.TokenEndpoint, c.JWKSEndpoint, c.IntrospectionEndpoint} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
			return c, errors.New("OIDC endpoints require exact HTTPS URLs without credentials, query or fragment")
		}
	}
	if c.ClientID == "" || c.IntrospectionClientID == "" || c.IntrospectionAuthMethod != "client_secret_basic" || len(c.MFAACRValues) == 0 || len(c.AllowedSigningAlgorithms) == 0 {
		return c, errors.New("OIDC requires client, authenticated introspection and exact MFA assurance mapping")
	}
	for _, alg := range c.AllowedSigningAlgorithms {
		if !slices.Contains([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512", "EdDSA"}, alg) {
			return c, errors.New("unsupported signing algorithm")
		}
	}
	return c, nil
}
func randomToken() string {
	// crypto/rand.Text uses the runtime CSPRNG and terminates on entropy failure.
	return rand.Text() + rand.Text()
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// Authenticator verifies OIDC login and live technician sessions.
type Authenticator struct {
	config              OIDCConfig
	store               *Store
	client              *http.Client
	oauth               oauth2.Config
	verifier            *oidc.IDTokenVerifier
	encryption          cipher.AEAD
	introspectionSecret string
	now                 func() time.Time
}

// NewAuthenticator constructs a verifier with bounded, redirect-free issuer requests.
func NewAuthenticator(c OIDCConfig, s *Store, key []byte, publicURL string, client *http.Client) (*Authenticator, error) {
	secret, err := ReadPrivate(c.ClientSecretFile)
	if err != nil {
		return nil, err
	}
	introspectionSecret, err := ReadPrivate(c.IntrospectionClientSecretFile)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	encryption, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{}
	}
	bounded := *client
	bounded.Timeout = 10 * time.Second
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("OIDC redirect denied") }
	// Endpoints come only from owner configuration. Bound responses for both
	// oauth2 and maintained RemoteKeySet; no issuer discovery from browser input.
	transport := bounded.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	bounded.Transport = boundedTransport{base: transport}
	ctx := oidc.ClientContext(context.Background(), &bounded)
	a := &Authenticator{config: c, store: s, client: &bounded, encryption: encryption, introspectionSecret: strings.TrimSpace(string(introspectionSecret)), now: time.Now}
	a.oauth = oauth2.Config{ClientID: c.ClientID, ClientSecret: strings.TrimSpace(string(secret)), RedirectURL: publicURL + "/__provider/oidc/callback", Scopes: []string{"openid"}, Endpoint: oauth2.Endpoint{AuthURL: c.AuthorizationEndpoint, TokenURL: c.TokenEndpoint, AuthStyle: oauth2.AuthStyleInHeader}}
	a.verifier = oidc.NewVerifier(c.Issuer, oidc.NewRemoteKeySet(ctx, c.JWKSEndpoint), &oidc.Config{ClientID: c.ClientID, SupportedSigningAlgs: c.AllowedSigningAlgorithms})
	return a, nil
}

type boundedTransport struct{ base http.RoundTripper }

func (t boundedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, errors.New("IdP response too large")
	}
	response.Body = io.NopCloser(bytes.NewReader(b))
	return response, nil
}

// Begin creates a single-use PKCE login transaction and browser binding.
func (a *Authenticator) Begin(_ context.Context) (location, browser string, err error) {
	state, nonce, verifier := randomToken(), randomToken(), oauth2.GenerateVerifier()
	browser = randomToken()
	_, err = a.store.db.Exec("INSERT INTO oauth_pending VALUES(?,?,?,?,?)", digest(state), digest(browser), nonce, verifier, a.now().Add(5*time.Minute).Unix())
	if err != nil {
		return "", "", err
	}
	location = a.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce), oauth2.SetAuthURLParam("max_age", "300"), oauth2.SetAuthURLParam("acr_values", strings.Join(a.config.MFAACRValues, " ")))
	return location, browser, nil
}

// Callback consumes a login transaction and verifies identity and recent MFA.
func (a *Authenticator) Callback(ctx context.Context, state, browser, code string) (string, error) {
	var nonce, verifier string
	// Consumed before external exchange. Failed callback cannot be replayed.
	err := a.store.db.QueryRow("DELETE FROM oauth_pending WHERE state_hash=? AND browser_hash=? AND expires>? RETURNING nonce,verifier", digest(state), digest(browser), a.now().Unix()).Scan(&nonce, &verifier)
	if err != nil {
		return "", errors.New("invalid login transaction")
	}
	token, err := a.oauth.Exchange(context.WithValue(ctx, oauth2.HTTPClient, a.client), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", errors.New("OIDC exchange failed")
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return "", errors.New("ID token required")
	}
	id, err := a.verifier.Verify(ctx, raw)
	if err != nil {
		return "", errors.New("ID token verification failed")
	}
	var claims struct {
		Nonce    string `json:"nonce"`
		AZP      string `json:"azp"`
		ACR      string `json:"acr"`
		AuthTime int64  `json:"auth_time"`
	}
	if err = id.Claims(&claims); err != nil {
		return "", errors.New("invalid assurance claims")
	}
	now := a.now()
	if id.Subject == "" || claims.Nonce != nonce || claims.AZP != a.config.ClientID || !slices.Contains(a.config.MFAACRValues, claims.ACR) || claims.AuthTime <= 0 || claims.AuthTime > now.Add(5*time.Second).Unix() || now.Unix()-claims.AuthTime > 300 {
		return "", errors.New("recent configured MFA assurance required")
	}
	tech, err := a.store.Technician(a.config.Issuer, id.Subject)
	if err != nil {
		return "", errors.New("technician unavailable")
	}
	expires, err := a.introspect(ctx, token.AccessToken, id.Subject)
	if err != nil {
		return "", err
	}
	expires = min(expires, time.Unix(claims.AuthTime, 0).Add(30*time.Minute).Unix(), id.Expiry.Unix())
	cookie := randomToken()
	n := make([]byte, a.encryption.NonceSize())
	if _, err = rand.Read(n); err != nil {
		return "", err
	}
	encrypted := a.encryption.Seal(n, n, []byte(token.AccessToken), []byte(digest(cookie)))
	err = a.store.mutate(tech, "", "", "login", func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO sessions VALUES(?,?,?,?,?)", digest(cookie), tech.ID, encrypted, claims.AuthTime, expires)
		return err
	})
	if err != nil {
		return "", err
	}
	return cookie, nil
}
func (a *Authenticator) introspect(ctx context.Context, token, subject string) (int64, error) {
	form := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.IntrospectionEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(a.config.IntrospectionClientID, a.introspectionSecret)
	response, err := a.client.Do(req)
	if err != nil {
		return 0, errors.New("introspection unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 {
		return 0, errors.New("introspection authentication failed")
	}
	var c struct {
		Active   bool   `json:"active"`
		Sub      string `json:"sub"`
		ClientID string `json:"client_id"`
		Issuer   string `json:"iss"`
		Exp      int64  `json:"exp"`
	}
	// RFC 7662 allows additional fields. Required profile is checked explicitly.
	d := json.NewDecoder(io.LimitReader(response.Body, 16385))
	if err = d.Decode(&c); err != nil {
		return 0, errors.New("invalid introspection profile")
	}
	if !c.Active || c.Sub == "" || c.Sub != subject || c.ClientID != a.config.ClientID || c.Issuer != a.config.Issuer || c.Exp <= a.now().Unix() {
		return 0, errors.New("introspection requires active, matching sub/client_id/iss and future integer exp")
	}
	return c.Exp, nil
}

// Identity binds a technician to a currently authenticated provider session.
type Identity struct {
	Technician
	SessionHash string
	AuthTime    int64
	Expires     int64
}

// Authenticate validates the session cookie and checks live token introspection.
func (a *Authenticator) Authenticate(ctx context.Context, cookie string) (Identity, error) {
	return a.authenticateHash(ctx, digest(cookie))
}
func (a *Authenticator) authenticateHash(ctx context.Context, hash string) (Identity, error) {
	var i Identity
	var encrypted []byte
	err := a.store.db.QueryRow("SELECT t.id,t.name,t.issuer,t.subject,t.administrator,s.access_token,s.auth_time,s.expires FROM sessions s JOIN technicians t ON t.id=s.technician_id WHERE s.token_hash=? AND s.expires>? AND t.enabled=1", hash, a.now().Unix()).Scan(&i.ID, &i.Name, &i.Issuer, &i.Subject, &i.Administrator, &encrypted, &i.AuthTime, &i.Expires)
	if err != nil {
		return i, errors.New("provider session unavailable")
	}
	n := a.encryption.NonceSize()
	if len(encrypted) < n {
		return i, errors.New("stored token unavailable")
	}
	token, err := a.encryption.Open(nil, encrypted[:n], encrypted[n:], []byte(hash))
	if err != nil {
		return i, errors.New("stored token unavailable")
	}
	expiry, err := a.introspect(ctx, string(token), i.Subject)
	if err != nil {
		return i, err
	}
	i.Expires = min(i.Expires, expiry)
	i.SessionHash = hash
	return i, nil
}

// Logout revokes the provider session and its support routes in an audited transaction.
func (a *Authenticator) Logout(hash string) error {
	var actor Technician
	if err := a.store.db.QueryRow("SELECT t.id,t.name,t.issuer FROM sessions s JOIN technicians t ON t.id=s.technician_id WHERE s.token_hash=?", hash).Scan(&actor.ID, &actor.Name, &actor.Issuer); err != nil {
		return err
	}
	return a.store.mutate(actor, "", "", "logout", func(tx *sql.Tx) error {
		for _, q := range []string{"DELETE FROM support_routes WHERE provider_session_hash=?", "DELETE FROM handoffs WHERE provider_session_hash=?", "DELETE FROM sessions WHERE token_hash=?"} {
			if _, err := tx.Exec(q, hash); err != nil {
				return err
			}
		}
		return nil
	})
}
