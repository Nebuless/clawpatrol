package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/supporttest"
)

func oidcFixture(t *testing.T) (*Authenticator, *supporttest.IDP) {
	t.Helper()
	mock, err := supporttest.NewIDP()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(mock)
	t.Cleanup(server.Close)
	mock.Issuer = server.URL
	mock.RedirectURIs = []string{"https://provider.example/__provider/oidc/callback"}
	dir := t.TempDir()
	secret := filepath.Join(dir, "client")
	intro := filepath.Join(dir, "intro")
	if err = os.WriteFile(secret, []byte("synthetic-client-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(intro, []byte("synthetic-introspection-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(dir, "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = store.AddTechnician("tech", server.URL, "technician", "Alex Technician", true); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cfg := OIDCConfig{Issuer: server.URL, ClientID: "prx-fixture", ClientSecretFile: secret, AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token", JWKSEndpoint: server.URL + "/jwks", IntrospectionEndpoint: server.URL + "/introspect", IntrospectionClientID: "prx-introspection", IntrospectionClientSecretFile: intro, IntrospectionAuthMethod: "client_secret_basic", AllowedSigningAlgorithms: []string{"RS256"}, MFAACRValues: []string{"urn:prx:test:mfa"}}
	auth, err := NewAuthenticator(cfg, store, key, "https://provider.example", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return auth, mock
}
func oidcCode(t *testing.T, a *Authenticator) (string, string, string) {
	t.Helper()
	location, browser, err := a.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	client := *a.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.PostForm(location, url.Values{"subject": {"technician"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if resp.StatusCode != 303 {
		t.Fatalf("authorize: %d", resp.StatusCode)
	}
	callback, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return callback.Query().Get("state"), browser, callback.Query().Get("code")
}
func TestOIDCRealSignedCallbackAndTrustFailures(t *testing.T) {
	a, mock := oidcFixture(t)
	state, browser, code := oidcCode(t, a)
	if _, err := a.Callback(context.Background(), state, "wrong-browser", code); err == nil {
		t.Fatal("browser binding bypassed")
	}
	cookie, err := a.Callback(context.Background(), state, browser, code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Callback(context.Background(), state, browser, code); err == nil {
		t.Fatal("callback replayed")
	}
	actor, err := a.Authenticate(context.Background(), cookie)
	if err != nil || actor.ID != "tech" {
		t.Fatalf("identity: %#v %v", actor, err)
	}
	var encrypted []byte
	if err = a.store.db.QueryRow("SELECT access_token FROM sessions").Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	n := a.encryption.NonceSize()
	raw, err := a.encryption.Open(nil, encrypted[:n], encrypted[n:], []byte(digest(cookie)))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, raw) || strings.Contains(cookie, string(raw)) {
		t.Fatal("plaintext token persisted")
	}
	mock.SetInactive(true)
	if _, err = a.Authenticate(context.Background(), cookie); err == nil {
		t.Fatal("inactive token accepted")
	}
	mock.SetInactive(false)
	mock.SetUnavailable(true)
	if _, err = a.Authenticate(context.Background(), cookie); err == nil {
		t.Fatal("IdP outage accepted")
	}
	mock.SetUnavailable(false)
	if err = a.Logout(digest(cookie)); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Authenticate(context.Background(), cookie); err == nil {
		t.Fatal("logout retained session")
	}
}
func TestOIDCRejectsSignedAuthorizedParty(t *testing.T) {
	for _, azp := range []string{"", "other-client"} {
		t.Run("azp="+azp, func(t *testing.T) {
			a, mock := oidcFixture(t)
			mock.AZP = azp
			state, browser, code := oidcCode(t, a)
			if _, err := a.Callback(context.Background(), state, browser, code); err == nil {
				t.Fatal("missing or wrong signed authorized party accepted")
			}
		})
	}
}

func TestOIDCRejectsSignedWrongAssurance(t *testing.T) {
	for _, tc := range []struct {
		name, acr string
		age       time.Duration
	}{{"unknown-acr", "urn:unknown", 0}, {"old-auth", "urn:prx:test:mfa", 6 * time.Minute}} {
		t.Run(tc.name, func(t *testing.T) {
			a, mock := oidcFixture(t)
			mock.ACR = tc.acr
			mock.AuthAge = tc.age
			state, browser, code := oidcCode(t, a)
			if _, err := a.Callback(context.Background(), state, browser, code); err == nil {
				t.Fatal("bad signed assurance accepted")
			}
		})
	}
}

func TestRosterDisableEndsAuthenticatedTechnician(t *testing.T) {
	a, _ := oidcFixture(t)
	state, browser, code := oidcCode(t, a)
	cookie, err := a.Callback(context.Background(), state, browser, code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Authenticate(context.Background(), cookie); err != nil {
		t.Fatal(err)
	}
	if err = a.store.RevokeTechnician("tech"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Authenticate(context.Background(), cookie); err == nil {
		t.Fatal("disabled roster identity retained access")
	}
	var n int
	if err = a.store.db.QueryRow("SELECT count(*) FROM sessions WHERE technician_id='tech'").Scan(&n); err != nil || n != 0 {
		t.Fatal("disabled technician tokens retained")
	}
}
