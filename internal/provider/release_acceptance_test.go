package provider

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nebuless/nebprx/internal/registry"
)

// Exercise production cookie, enrollment and connector admission, not just token hashes.
func TestReleaseCredentialPurposeBoundaries(t *testing.T) {
	a, _ := oidcFixture(t)
	state, browser, code := oidcCode(t, a)
	providerToken, err := a.Callback(context.Background(), state, browser, code)
	if err != nil {
		t.Fatal(err)
	}
	s := a.store
	if err = s.AddOrganization("org", "Organization"); err != nil {
		t.Fatal(err)
	}
	if err = s.AssignOrganization("tech", "org"); err != nil {
		t.Fatal(err)
	}
	if err = s.RegisterInstallation(Installation{ID: "gateway", OrganizationID: "org", Name: "Gateway", Hosting: "customer"}); err != nil {
		t.Fatal(err)
	}
	enrollment, err := s.Enrollment("org", "gateway")
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	connector, err := s.Enroll("org", "gateway", enrollment, gatewayPublic)
	if err != nil {
		t.Fatal(err)
	}
	_, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r, err := registry.Open(filepath.Join(t.TempDir(), "registry"), "client")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	registryToken, err := r.LocalToken()
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Store: s, Auth: a, Key: signingKey, PublicURL: "https://provider.example", SupportDomain: "support.example"}
	// This is an actual routing-cookie credential bound to the authenticated session.
	c, err := s.SubmitCase("org", "gateway", "intent", "account", "Synthetic case", "connection")
	if err != nil {
		t.Fatal(err)
	}
	supportToken := randomToken()
	if _, err = s.db.Exec("INSERT INTO support_routes VALUES(?,?,?,?,?,?,?,?)", digest(supportToken), "local-session", digest(providerToken), "org", "gateway", c.ID, "session.support.example", a.now().Unix()+60); err != nil {
		t.Fatal(err)
	}
	tokens := map[string]string{"registry": registryToken, "connector": connector, "signing-key": base64.StdEncoding.EncodeToString(signingKey), "provider-cookie": providerToken, "support-cookie": supportToken, "spent-enrollment": enrollment}
	for purpose, token := range tokens {
		t.Run(purpose, func(t *testing.T) {
			req := httptest.NewRequest("GET", "https://provider.example/__provider/session", nil)
			req.AddCookie(&http.Cookie{Name: providerCookie, Value: token})
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			want := 401
			if purpose == "provider-cookie" {
				want = 200
			}
			if rec.Code != want {
				t.Fatalf("provider cookie boundary: %d", rec.Code)
			}
			req = httptest.NewRequest("GET", "https://provider.example/__connector/connect?organization=org&installation=gateway", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec = httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			// Valid connector advances to WebSocket validation. Others never upgrade.
			want = 403
			if purpose == "connector" {
				want = 426
			}
			if rec.Code != want {
				t.Fatalf("connector boundary: %d", rec.Code)
			}
			req = httptest.NewRequest("GET", "https://session.support.example/api/state", nil)
			req.AddCookie(&http.Cookie{Name: supportCookie, Value: token})
			req.AddCookie(&http.Cookie{Name: providerCookie, Value: providerToken})
			rec = httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			want = 401
			// Valid support cookie reaches current-case check. Submitted case has no approval.
			if purpose == "support-cookie" {
				want = 403
			}
			if rec.Code != want {
				t.Fatalf("support cookie boundary: %d", rec.Code)
			}
			req = httptest.NewRequest("GET", "https://registry.example/v1/status", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec = httptest.NewRecorder()
			r.Handler().ServeHTTP(rec, req)
			want = 401
			if purpose == "registry" {
				want = 200
			}
			if rec.Code != want {
				t.Fatalf("registry boundary: %d", rec.Code)
			}
			payload, _ := json.Marshal(map[string]any{"Organization": "org", "Installation": "gateway", "Token": token, "Key": gatewayPublic})
			req = httptest.NewRequest("POST", "https://provider.example/__connector/enroll", bytes.NewReader(payload))
			rec = httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != 403 {
				t.Fatalf("spent enrollment or wrong-purpose credential accepted: %d", rec.Code)
			}
		})
	}
	// Cookie name is part of purpose, even when its value is a valid Provider token.
	req := httptest.NewRequest("GET", "https://provider.example/__provider/session", nil)
	req.AddCookie(&http.Cookie{Name: supportCookie, Value: providerToken})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatal("support cookie became Provider session")
	}
}

func TestReleaseForgedActorIgnoredByManagementAudit(t *testing.T) {
	a, _ := oidcFixture(t)
	state, browser, code := oidcCode(t, a)
	token, err := a.Callback(context.Background(), state, browser, code)
	if err != nil {
		t.Fatal(err)
	}
	s := a.store
	if err = s.AddOrganization("org", "Organization"); err != nil {
		t.Fatal(err)
	}
	if err = s.AssignOrganization("tech", "org"); err != nil {
		t.Fatal(err)
	}
	if err = s.RegisterInstallation(Installation{ID: "gateway", OrganizationID: "org", Name: "Gateway", Hosting: "customer"}); err != nil {
		t.Fatal(err)
	}
	server := &Server{Store: s, Auth: a, PublicURL: "https://provider.example", SupportDomain: "support.example"}
	req := httptest.NewRequest("POST", "https://provider.example/__provider/installation?organization=org&installation=gateway", strings.NewReader(`{"Name":"Updated","Enabled":true}`))
	req.Header.Set("Origin", server.PublicURL)
	req.Header.Set("X-PRX-Actor-ID", "FORGED_ACTOR_SENTINEL")
	req.Header.Set("X-Forwarded-User", "FORGED_ACTOR_SENTINEL")
	req.Header.Set("Tailscale-User-Login", "FORGED_ACTOR_SENTINEL")
	req.AddCookie(&http.Cookie{Name: providerCookie, Value: token})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("verified request: %d", rec.Code)
	}
	var id, name, issuer string
	var effectiveID, effectiveName any
	if err = s.db.QueryRow("SELECT actor_id,actor_name,actor_issuer,effective_id,effective_name FROM provider_audit WHERE operation='installation_update' ORDER BY rowid DESC LIMIT 1").Scan(&id, &name, &issuer, &effectiveID, &effectiveName); err != nil {
		t.Fatal(err)
	}
	if id != "tech" || name != "Alex Technician" || issuer != a.config.Issuer || effectiveID != nil || effectiveName != nil {
		t.Fatal("management audit lost actual actor or fabricated customer")
	}
	if strings.Contains(rec.Body.String(), "FORGED_ACTOR_SENTINEL") {
		t.Fatal("forged identity reflected")
	}
}
