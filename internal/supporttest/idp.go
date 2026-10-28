// Package supporttest supplies synthetic TLS/OIDC fixtures, never production auth.
package supporttest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"
)

type authorization struct{ Nonce, Challenge, Redirect, Subject string }

// IDP verifies PKCE and signs real RS256 ID tokens for an explicit test roster.
// Its mock login has no passwords. PRX still uses its production OIDC verifier.
type IDP struct {
	Issuer                string
	RedirectURIs          []string
	Key                   *rsa.PrivateKey
	mu                    sync.Mutex
	codes                 map[string]authorization
	tokens                map[string]string
	inactive, unavailable bool
	ACR                   string
	AZP                   string
	AuthAge               time.Duration
}

// NewIDP creates an isolated synthetic issuer with a fresh signing key.
func NewIDP() (*IDP, error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &IDP{Key: k, codes: map[string]authorization{}, tokens: map[string]string{}, ACR: "urn:prx:test:mfa", AZP: "prx-fixture"}, nil
}
func token() string       { return rand.Text() + rand.Text() }
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func (i *IDP) sign(v any) (string, error) {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture", "typ": "JWT"})
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	unsigned := b64(h) + "." + b64(b)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, i.Key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + b64(sig), nil
}

var login = template.Must(template.New("login").Parse(`<!doctype html><title>Synthetic technician IdP</title><h1>Synthetic technician MFA</h1><p>Test issuer only. No real credentials.</p><form method="post"><label>Technician <select name="subject"><option value="technician">Alex Technician</option><option value="other">Other Technician</option></select></label><button>Complete synthetic MFA</button></form>`))

func (i *IDP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.mu.Lock()
	defer i.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	if i.unavailable {
		http.Error(w, "issuer unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.URL.Path {
	case "/authorize":
		q := r.URL.Query()
		redirect, err := url.Parse(q.Get("redirect_uri"))
		if err != nil || redirect.Scheme != "https" || !slices.Contains(i.RedirectURIs, redirect.String()) || q.Get("client_id") != "prx-fixture" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("nonce") == "" {
			http.Error(w, "invalid authorization", 400)
			return
		}
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = login.Execute(w, nil)
			return
		}
		if r.Method != "POST" || r.ParseForm() != nil {
			http.Error(w, "invalid login", 400)
			return
		}
		sub := r.PostForm.Get("subject")
		if sub != "technician" && sub != "other" {
			http.Error(w, "unknown fixture subject", http.StatusForbidden)
			return
		}
		code := token()
		i.codes[code] = authorization{Nonce: q.Get("nonce"), Challenge: q.Get("code_challenge"), Redirect: redirect.String(), Subject: sub}
		v := redirect.Query()
		v.Set("state", q.Get("state"))
		v.Set("code", code)
		redirect.RawQuery = v.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusSeeOther)
	case "/jwks":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": "fixture", "n": b64(i.Key.N.Bytes()), "e": b64(big.NewInt(int64(i.Key.E)).Bytes())}}})
	case "/token":
		user, secret, ok := r.BasicAuth()
		if !ok || user != "prx-fixture" || secret != "synthetic-client-secret" || r.Method != "POST" || r.ParseForm() != nil {
			http.Error(w, "invalid client", http.StatusUnauthorized)
			return
		}
		code := r.Form.Get("code")
		a, ok := i.codes[code]
		delete(i.codes, code)
		hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || b64(hash[:]) != a.Challenge || r.Form.Get("redirect_uri") != a.Redirect || r.Form.Get("grant_type") != "authorization_code" {
			http.Error(w, "invalid code", 400)
			return
		}
		access := token()
		i.tokens[access] = a.Subject
		now := time.Now()
		id, err := i.sign(map[string]any{"iss": i.Issuer, "sub": a.Subject, "aud": "prx-fixture", "azp": i.AZP, "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(), "nonce": a.Nonce, "acr": i.ACR, "auth_time": now.Add(-i.AuthAge).Unix()})
		if err != nil {
			http.Error(w, "signing failure", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600, "id_token": id})
	case "/introspect":
		user, secret, ok := r.BasicAuth()
		if !ok || user != "prx-introspection" || secret != "synthetic-introspection-secret" || r.Method != "POST" || r.ParseForm() != nil {
			http.Error(w, "invalid client", http.StatusUnauthorized)
			return
		}
		sub, ok := i.tokens[r.Form.Get("token")]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"active": ok && !i.inactive, "sub": sub, "client_id": "prx-fixture", "iss": i.Issuer, "exp": time.Now().Add(time.Hour).Unix()})
	default:
		http.NotFound(w, r)
	}
}

// SetUnavailable controls whether the synthetic issuer rejects every request.
func (i *IDP) SetUnavailable(v bool) { i.mu.Lock(); defer i.mu.Unlock(); i.unavailable = v }

// SetInactive controls whether introspection reports tokens as inactive.
func (i *IDP) SetInactive(v bool) { i.mu.Lock(); defer i.mu.Unlock(); i.inactive = v }
