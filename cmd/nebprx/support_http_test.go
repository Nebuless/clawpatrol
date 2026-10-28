package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
)

func TestSupportLostHandoffAndCookieRecovery(t *testing.T) {
	for _, activated := range []bool{false, true} {
		t.Run(map[bool]string{false: "consume-before-handoff", true: "activate-before-cookie"}[activated], func(t *testing.T) {
			s, a, c := supportFixture(t)
			code, p := supportPending(t, s, a, c)
			if activated {
				if err := s.activate(p.ID, c); err != nil {
					t.Fatal(err)
				}
			}
			// Crash/lost response: the caller never receives the handoff/cookie.
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			restarted, err := newSupportStore(s.db, s.org, s.installation, s.pepper)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := restarted.Close(); err != nil {
					t.Error(err)
				}
			}()
			restarted.domain = s.domain
			if _, err = restarted.authorize(p.ID); err == nil {
				t.Fatal("restart retained orphan")
			}
			if _, err = restarted.redeem(context.Background(), c, c.TechnicianID, code, p.Origin); err == nil {
				t.Fatal("crash revived consumed code")
			}
			_, replacement := supportPending(t, restarted, a, c)
			if err = restarted.activate(replacement.ID, c); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestSupportTailnetIdentityRechecked(t *testing.T) {
	s, _, c := supportFixture(t)
	a, err := resolveDashboardAccount(s.db, "tailnet", "https://control.example", "42", "member@example.com")
	if err != nil {
		t.Fatal(err)
	}
	c.ID = "tailnet-case"
	c.AccountID = a.ID
	if err = s.mirrorCase(c); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"authority-replaced", "login-renamed", "subject-reassigned", "proof-unavailable"} {
		t.Run(change, func(t *testing.T) {
			s.membership = func(authority, subject, name string) bool {
				return authority == a.Authority && subject == "42" && name == "member@example.com"
			}
			_, p := supportPending(t, s, a, c)
			if err = s.activate(p.ID, c); err != nil {
				t.Fatal(err)
			}
			s.membership = func(authority, subject, name string) bool {
				switch change {
				case "authority-replaced":
					return authority == "https://replacement.example"
				case "login-renamed":
					return name == "renamed@example.com"
				case "subject-reassigned":
					return subject == "43"
				}
				return false
			}
			if _, err = s.authorize(p.ID); err == nil {
				t.Fatal("stale identity authorized")
			}
			if _, _, err = s.issue(context.Background(), c, a); err == nil {
				t.Fatal("stale identity issued grant")
			}
		})
	}
}
func TestSupportTypedProjectionSecretSentinel(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	base := configApplyTestMux(t, true)
	g := &Gateway{db: s.db, agents: NewAgentRegistry(), onboard: newOnboardRegistry()}
	g.cfg.Store(base.g.cfg.Load())
	g.policy.Store(base.g.Policy())
	sink, err := NewSink(s.db, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sink.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	g.sink = sink
	w := webMux{g: g, onboard: g.onboard}
	const secret = "PRX_SECRET_SENTINEL_NEVER_RELEASE"
	e := Event{ID: "event", Ts: time.Now(), Host: "public.example", Path: "/" + secret, ReqBody: secret, RespBody: secret, ReqHeaders: map[string]string{"Authorization": secret}, RespHeaders: map[string]string{"Set-Cookie": secret}, Frame: secret, Reason: secret, Credential: secret, Facets: map[string]any{"body": secret}}
	delivered, cancel := sink.Subscribe()
	defer cancel()
	sink.Emit(e)
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("sentinel action not persisted")
	}
	b := supportEvent(&e)
	rec := httptest.NewRecorder()
	writeJSON(rec, b)
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("event sentinel leaked")
	}
	for _, path := range []string{"/api/state", "/api/status", "/api/profiles", "/api/plugins", "/api/rules", "/api/support/config-view", "/api/events", "/api/actions/event", "/api/analytics"} {
		f := w.dispatchSupport(context.Background(), s, supportwire.Envelope{OrganizationID: s.org, InstallationID: s.installation, TechnicianID: c.TechnicianID, TechnicianName: c.TechnicianName, CaseID: c.ID, Revision: c.Revision, SessionID: p.ID, Origin: p.Origin, Method: "GET", Target: path}, nil, nil)
		if f.Status != 200 {
			t.Fatalf("%s: %d %s", path, f.Status, f.Body)
		}
		if strings.Contains(string(f.Body), secret) {
			t.Fatalf("%s leaked sentinel", path)
		}
	}
}
func TestSupportDeviceMutationAuditRollback(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	base := configApplyTestMux(t, true)
	g := &Gateway{db: s.db, onboard: newOnboardRegistry()}
	g.cfg.Store(base.g.cfg.Load())
	w := webMux{g: g, onboard: g.onboard}
	if _, err := s.db.Exec("INSERT INTO devices(id,profile,created_ns,last_seen_ns) VALUES('10.55.0.2','default',1,1)"); err != nil {
		t.Fatal(err)
	}
	if err := g.onboard.Load(s.db); err != nil {
		t.Fatal(err)
	}
	session, err := s.authorize(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("CREATE TRIGGER mutation_fault BEFORE INSERT ON support_audit WHEN NEW.reason='mutation_committed' BEGIN SELECT RAISE(ABORT,'audit fault'); END"); err != nil {
		t.Fatal(err)
	}
	previousDB, previousWG := globalDB, globalWG
	globalDB = s.db
	wg, err := StartWGServer(JoinConfig{WGSubnetCIDR: "10.55.0.0/24", WGListenPort: freeUDPPort(t)})
	if err != nil {
		t.Fatal(err)
	}
	globalWG = wg
	t.Cleanup(func() { wg.dev.Close(); globalDB, globalWG = previousDB, previousWG })
	key := strings.Repeat("ab", 32)
	if err = wg.AddPeer(key, "10.55.0.2"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/api/agents/delete?ip=10.55.0.2", nil)
	if err = w.supportDeviceMutation(context.Background(), s, c, session, request); err == nil {
		t.Fatal("unaudited deletion committed")
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM devices WHERE id='10.55.0.2'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("device lost: %d %v", count, err)
	}
	live, err := wg.dev.IpcGet()
	if err != nil || !strings.Contains(live, "public_key="+key) {
		t.Fatal("rollback revoked live peer")
	}
	if _, err = s.db.Exec("DROP TRIGGER mutation_fault"); err != nil {
		t.Fatal(err)
	}
	if err = w.supportDeviceMutation(context.Background(), s, c, session, request); err != nil {
		t.Fatal(err)
	}
}
func TestSupportOrdinaryConfigWritesUnaffected(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, writes := range []bool{false, true} {
			w := configApplyTestMux(t, writes)
			if enabled {
				w.g.support = &supportConnector{}
			}
			rec := httptest.NewRecorder()
			w.apiConfigApply(rec, jsonReq(map[string]string{"base_revision": "stale", "append_hcl": "rule invalid {}"}))
			expected := http.StatusForbidden
			if writes {
				expected = http.StatusConflict
			}
			if rec.Code != expected {
				t.Fatalf("support=%v writes=%v status=%d", enabled, writes, rec.Code)
			}
		}
	}
}

func TestSupportRuntimeDisableAdmission(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	// Independent connection models owner CLI changing a running gateway.
	if _, err := s.db.Exec("UPDATE support_control SET enabled=0 WHERE singleton=1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.authorize(p.ID); err == nil {
		t.Fatal("disabled support authorized")
	}
	if _, _, err := s.issue(context.Background(), c, a); err == nil {
		t.Fatal("disabled support issued code")
	}
	if _, err := s.redeem(context.Background(), c, c.TechnicianID, "SUPPORT-0000-0000-0000", p.Origin); err == nil {
		t.Fatal("disabled support redeemed code")
	}
}

func TestSupportRoutesMountOnce(t *testing.T) {
	w := newDashboardTestMux(t, nil, "synthetic-password")
	w.g.support = &supportConnector{}
	_ = w.handler()
}
