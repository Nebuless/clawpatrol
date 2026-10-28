package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
	"github.com/coder/websocket"
)

func TestSupportLifecycleRestartAudit(t *testing.T) {
	s, a, c := supportFixture(t)
	_, session := supportPending(t, s, a, c)
	if err := s.activate(session.ID, c); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := newSupportStore(s.db, s.org, s.installation, s.pepper)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := restarted.Close(); err != nil {
			t.Error(err)
		}
	})
	assertSupportLifecycleAudit(t, restarted, session.ID, "gateway_restart", "local-system")
	if _, err := restarted.authorize(session.ID); err == nil {
		t.Fatal("restart retained active support")
	}
}

func TestSupportLifecycleCommandsAndDurableDisable(t *testing.T) {
	for _, action := range []string{"revoke-all", "disable"} {
		t.Run(action, func(t *testing.T) {
			state := t.TempDir()
			s, a, c := supportFixtureAt(t, filepath.Join(state, "nebprx.db"))
			_, session := supportPending(t, s, a, c)
			if err := s.activate(session.ID, c); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec("UPDATE installation_identity SET id=?,organization_id=? WHERE singleton=1", s.installation, s.org); err != nil {
				t.Fatal(err)
			}
			config := supportFile{Enabled: true, Broker: "https://provider.example", Origin: "https://provider.example", Domain: "support.example", Organization: s.org, Installation: s.installation, Hosting: "customer"}
			data, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "support.json")
			if err := os.WriteFile(path, data, 0400); err != nil {
				t.Fatal(err)
			}
			args := []string{"--action", action, "--state-dir", state, "--config-file", path}
			var out bytes.Buffer
			if err := supportCommand(args, &out); err != nil {
				t.Fatal(err)
			}
			assertSupportLifecycleAudit(t, s, session.ID, "local_"+strings.ReplaceAll(action, "-", "_"), "local-os")
			if _, err := s.authorize(session.ID); err == nil {
				t.Fatal("bulk command retained active support")
			}
			if action != "disable" {
				return
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(unchanged, data) {
				t.Fatalf("disable rewrote deployed config: %v", err)
			}
			g := &Gateway{db: s.db}
			if err := g.startSupport(context.Background(), path); err != nil || g.support != nil || s.enabled() {
				t.Fatalf("stale enabled config reopened support: %v", err)
			}
			args[1] = "status"
			if err := supportCommand(args, &out); err != nil {
				t.Fatal(err)
			}
			var status struct {
				Enabled bool `json:"enabled_on_next_start"`
			}
			if err := json.Unmarshal(out.Bytes(), &status); err != nil || status.Enabled {
				t.Fatalf("misreported disabled restart: %s %v", out.Bytes(), err)
			}
			args[1] = "enable"
			if err := supportCommand(args, &out); err != nil || !s.enabled() {
				t.Fatalf("explicit re-enable failed: %v", err)
			}
			if _, err := s.authorize(session.ID); err == nil {
				t.Fatal("re-enable revived old consent")
			}
		})
	}
}

func TestSupportLifecycleAuditFailureDeniesAdmission(t *testing.T) {
	for _, reason := range []string{"connector_disconnect", "gateway_restart", "local_disable"} {
		t.Run(reason, func(t *testing.T) {
			s, a, c := supportFixture(t)
			_, session := supportPending(t, s, a, c)
			if err := s.activate(session.ID, c); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec("CREATE TRIGGER fail_lifecycle_audit BEFORE INSERT ON support_audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END"); err != nil {
				t.Fatal(err)
			}
			if err := s.revokeAllSupport(systemSupportActor, reason, reason == "local_disable"); err == nil {
				t.Fatal("audit failure hidden")
			}
			var state string
			var enabled bool
			if err := s.db.QueryRow("SELECT state FROM support_sessions WHERE id=?", session.ID).Scan(&state); err != nil || state != "revoked" {
				t.Fatalf("audit failure retained session: %s %v", state, err)
			}
			if err := s.db.QueryRow("SELECT enabled FROM support_control WHERE singleton=1").Scan(&enabled); err != nil || enabled {
				t.Fatalf("audit failure retained durable admission: %v", err)
			}
			if s.enabled() {
				t.Fatal("audit failure retained process admission")
			}
			if _, _, err := s.issue(context.Background(), c, a); err == nil {
				t.Fatal("audit failure allowed new consent")
			}
		})
	}
}

func TestSupportLifecycleConnectorDisconnect(t *testing.T) {
	for _, fault := range []bool{false, true} {
		t.Run(map[bool]string{false: "audited", true: "audit-failure"}[fault], func(t *testing.T) {
			s, a, c := supportFixture(t)
			_, session := supportPending(t, s, a, c)
			if err := s.activate(session.ID, c); err != nil {
				t.Fatal(err)
			}
			if fault {
				if _, err := s.db.Exec("CREATE TRIGGER fail_disconnect_audit BEFORE INSERT ON support_audit BEGIN SELECT RAISE(ABORT,'synthetic disconnect audit failure'); END"); err != nil {
					t.Fatal(err)
				}
			}
			public, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer func() { _ = conn.CloseNow() }()
				proof, err := json.Marshal(map[string]string{"Nonce": "fixture-nonce", "Proof": supportwire.Proof(private, "provider", "fixture-nonce", s.org, s.installation)})
				if err != nil {
					t.Error(err)
					return
				}
				if err := conn.Write(ctx, websocket.MessageText, proof); err != nil {
					t.Error(err)
					return
				}
				if _, _, err := conn.Read(ctx); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			connector := supportConnector{store: s, gateway: &Gateway{onboard: newOnboardRegistry()}, client: server.Client(), config: supportConfig{Broker: server.URL, Organization: s.org, Installation: s.installation, ProviderKey: public, PrivateKey: private}}
			err = connector.run(ctx)
			if err == nil {
				t.Fatal("disconnected connector returned success")
			}
			if fault {
				if !strings.Contains(err.Error(), "lifecycle persistence failed") || s.enabled() {
					t.Fatalf("disconnect hid audit failure or retained admission: %v", err)
				}
			} else {
				assertSupportLifecycleAudit(t, s, session.ID, "connector_disconnect", "local-system")
			}
			if _, err := s.authorize(session.ID); err == nil {
				t.Fatal("disconnect retained active support")
			}
		})
	}
}

func TestSupportLifecycleUnredeemedGrantAudit(t *testing.T) {
	s, a, c := supportFixture(t)
	_, grant, err := s.issue(context.Background(), c, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.revokeAllSupport(systemSupportActor, "gateway_restart", false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM support_audit WHERE grant_id=? AND session_id IS NULL AND operation='grant_revoke' AND reason='gateway_restart' AND actor_issuer='local-system' AND effective_id=?", grant, a.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing unredeemed grant termination: %d %v", count, err)
	}
}

func assertSupportLifecycleAudit(t *testing.T, s *supportStore, session, reason, issuer string) {
	t.Helper()
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM support_audit WHERE session_id=? AND reason=? AND actor_issuer=? AND actor_id!='tech' AND effective_id IS NOT NULL AND grant_id IS NOT NULL AND operation='session_revoke'", session, reason, issuer).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing bound termination audit: %d %v", count, err)
	}
}
