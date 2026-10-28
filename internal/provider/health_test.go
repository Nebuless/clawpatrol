package provider

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
	"github.com/coder/websocket"
)

func TestManagementHealthIsolationAndRecovery(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = store.AddOrganization("org", "Organization"); err != nil {
		t.Fatal(err)
	}
	if err = store.AddTechnician("tech", "issuer", "subject", "Technician", true); err != nil {
		t.Fatal(err)
	}
	if err = store.AssignOrganization("tech", "org"); err != nil {
		t.Fatal(err)
	}
	actor, err := store.Technician("issuer", "subject")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RegisterInstallation(Installation{ID: "gateway", OrganizationID: "org", Name: "Gateway", Hosting: "nebuless"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: store, connections: map[string]*connection{}}
	check := func(want string) {
		t.Helper()
		h, err := s.installationHealth(context.Background(), actor, "org", "gateway")
		if err != nil || h.Connector != want {
			t.Fatalf("health=%+v err=%v", h, err)
		}
	}
	check("not_enrolled")
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := store.Enrollment("org", "gateway")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Enroll("org", "gateway", enrollment, key.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	check("disconnected")
	if _, err = s.installationHealth(context.Background(), Technician{ID: "foreign"}, "org", "gateway"); err == nil {
		t.Fatal("foreign management health")
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		link := supportwire.NewLink(ws, func(context.Context, supportwire.Frame) supportwire.Frame {
			b, _ := json.Marshal(supportwire.ManagementHealth{Connector: "connected", Registry: "local_only"})
			return supportwire.Frame{Body: b}
		})
		_ = link.Run(ctx)
	}))
	defer endpoint.Close()
	ws, response, err := websocket.Dial(ctx, "ws"+endpoint.URL[4:], nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	link := supportwire.NewLink(ws, nil)
	defer link.Close()
	go func() { _ = link.Run(ctx) }()
	s.connections["gateway"] = &connection{link: link}
	check("connected")
	if err = store.Manage(actor, "org", "gateway", "Gateway", false); err != nil {
		t.Fatal(err)
	}
	check("disabled")
	if err = store.Manage(actor, "org", "gateway", "Gateway", true); err != nil {
		t.Fatal(err)
	}
	check("connected")
	link.Close()
	check("unavailable")
}

func TestOrganizationManagementAuthorizationAndAudit(t *testing.T) {
	a, _ := oidcFixture(t)
	s := a.store
	actor, err := s.Technician(a.config.Issuer, "technician")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.manageOrganization(actor, "new-org", "New org", true, true); err != nil {
		t.Fatal(err)
	}
	if err = s.manageOrganization(Technician{ID: "foreign", Administrator: true}, "new-org", "Stolen", true, false); err == nil {
		t.Fatal("cross-org update")
	}
	if err = s.manageOrganization(Technician{ID: actor.ID}, "other", "Other", true, true); err == nil {
		t.Fatal("non-admin create")
	}
	if err = s.manageOrganization(actor, "new-org", "Disabled", false, false); err != nil {
		t.Fatal(err)
	}
	if err = s.manageOrganization(actor, "new-org", "Enabled", true, false); err != nil {
		t.Fatal(err)
	}
	if err = s.AddOrganization("shared", "Shared"); err != nil {
		t.Fatal(err)
	}
	if err = s.AssignOrganization(actor.ID, "shared"); err != nil {
		t.Fatal(err)
	}
	if err = s.AddTechnician("colleague", a.config.Issuer, "colleague", "Colleague", false); err != nil {
		t.Fatal(err)
	}
	if err = s.AssignOrganization("colleague", "shared"); err != nil {
		t.Fatal(err)
	}
	if err = s.assignOrganization(actor, "new-org", "colleague"); err != nil {
		t.Fatal(err)
	}
	if err = s.assignOrganization(actor, "new-org", "unknown"); err == nil {
		t.Fatal("untrusted roster identity assigned")
	}
	if err = s.assignOrganization(Technician{ID: actor.ID}, "new-org", "colleague"); err == nil {
		t.Fatal("non-admin assigned technician")
	}
	var assigned int
	if err = s.db.QueryRow("SELECT count(*) FROM organization_assignments WHERE organization_id='new-org' AND technician_id='colleague'").Scan(&assigned); err != nil || assigned != 1 {
		t.Fatal("assignment not persisted", err)
	}
	var n int
	if err = s.db.QueryRow("SELECT count(*) FROM provider_audit WHERE actor_id=? AND organization_id='new-org' AND effective_id IS NULL AND effective_name IS NULL AND at>?", actor.ID, time.Now().Add(-time.Minute).Unix()).Scan(&n); err != nil || n != 4 {
		t.Fatalf("management audit=%d err=%v", n, err)
	}
}
