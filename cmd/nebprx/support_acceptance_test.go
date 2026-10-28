package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/Nebuless/nebprx/internal/supportwire"
)

func supportFixture(t *testing.T) (*supportStore, dashboardAccount, provider.Case) {
	t.Helper()
	return supportFixtureAt(t, filepath.Join(t.TempDir(), "gateway.db"))
}

func supportFixtureAt(t *testing.T, path string) (*supportStore, dashboardAccount, provider.Case) {
	t.Helper()
	db, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = setDashboardUser(db, "root", "synthetic-password"); err != nil {
		t.Fatal(err)
	}
	a, err := resolveDashboardAccount(db, "password", "local", "root", "Customer")
	if err != nil {
		t.Fatal(err)
	}
	s, err := newSupportStore(db, "org", "installation", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			t.Error(err)
		}
	})
	s.domain = "support.example"
	c := provider.Case{ID: "case", OrganizationID: "org", InstallationID: "installation", TechnicianID: "tech", TechnicianName: "Technician", Revision: 1, State: "assigned", Expires: time.Now().Add(time.Hour).Unix()}
	c.AccountID = a.ID
	if err = s.mirrorCase(c); err != nil {
		t.Fatal(err)
	}
	return s, a, c
}
func supportPending(t *testing.T, s *supportStore, a dashboardAccount, c provider.Case) (string, supportSession) {
	t.Helper()
	code, _, err := s.issue(context.Background(), c, a)
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.redeem(context.Background(), c, c.TechnicianID, code, "https://session.support.example")
	if err != nil {
		t.Fatal(err)
	}
	return code, session
}
func TestSupportPendingExpiryReissueAndRotation(t *testing.T) {
	s, a, c := supportFixture(t)
	code, p := supportPending(t, s, a, c)
	now := s.now()
	s.now = func() time.Time { return now.Add(31 * time.Second) }
	if err := s.activate(p.ID, c); err == nil {
		t.Fatal("expired handoff activated")
	}
	if _, err := s.redeem(context.Background(), c, c.TechnicianID, code, p.Origin); err == nil {
		t.Fatal("consumed code replayed")
	}
	_, replacement := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err == nil {
		t.Fatal("orphan handoff revived")
	}
	if err := s.activate(replacement.ID, c); err != nil {
		t.Fatal(err)
	}
	if err := setDashboardUser(s.db, "root", "new-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.authorize(replacement.ID); err == nil {
		t.Fatal("password rotation retained support")
	}
}
func TestSupportFailureBucketsPersistAndScope(t *testing.T) {
	s, a, c := supportFixture(t)
	code, _, err := s.issue(context.Background(), c, a)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err = s.redeem(context.Background(), c, c.TechnicianID, "not-a-code", "https://session.support.example"); err == nil {
			t.Fatal("bad code accepted")
		}
	}
	if _, err = s.redeem(context.Background(), c, c.TechnicianID, code, "https://session.support.example"); err == nil {
		t.Fatal("locked grant accepted")
	}
	var failures int
	if err = s.db.QueryRow("SELECT failures FROM support_rate_limits WHERE scope LIKE 'challenge:%'").Scan(&failures); err != nil || failures != 5 {
		t.Fatalf("failures %d: %v", failures, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := newSupportStore(s.db, s.org, s.installation, s.pepper)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	reopened.domain = s.domain
	if err = reopened.db.QueryRow("SELECT failures FROM support_rate_limits WHERE scope LIKE 'challenge:%'").Scan(&failures); err != nil || failures != 5 {
		t.Fatalf("restart lost failures: %v", err)
	}
	c.ID = "other-case"
	if err = reopened.mirrorCase(c); err != nil {
		t.Fatal(err)
	}
	_, p := supportPending(t, reopened, a, c)
	if err = reopened.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
}
func TestSupportAuditFailureAtomic(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if _, err := s.db.Exec("CREATE TRIGGER audit_fault BEFORE INSERT ON support_audit BEGIN SELECT RAISE(ABORT,'injected audit failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := s.activate(p.ID, c); err == nil {
		t.Fatal("unaudited activation")
	}
	var state string
	if err := s.db.QueryRow("SELECT state FROM support_sessions WHERE id=?", p.ID).Scan(&state); err != nil || state != "pending_activation" {
		t.Fatalf("partial activation: %s %v", state, err)
	}
	if _, _, err := s.issue(context.Background(), c, a); err == nil {
		t.Fatal("unaudited issue")
	}
}
func TestSupportAuditAccountIsolation(t *testing.T) {
	s, a, c := supportFixture(t)
	if err := s.transaction(func(tx *sql.Tx) error {
		return s.audit(tx, c, "other-account", "", "", "foreign-operation", "success", "test")
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.issue(context.Background(), c, a); err != nil {
		t.Fatal(err)
	}
	w := webMux{g: &Gateway{db: s.db}}
	rec := httptest.NewRecorder()
	w.customerSupportAudit(rec, httptest.NewRequest("GET", "/api/support/audit", nil), a.ID)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "foreign-operation") {
		t.Fatalf("foreign audit: %s", rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0]["actor_id"] != a.ID {
		t.Fatalf("customer actor lost: %v %v", rows, err)
	}
}
func TestSupportUnknownRouteDeniedAndAuditFailClosed(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	w := webMux{g: &Gateway{db: s.db}}
	for _, target := range []string{"/api/future", "/api/config", "/api/credentials/set", "/api/actions/example/download", "/api/hitl/decide"} {
		e := supportwire.Envelope{OrganizationID: s.org, InstallationID: s.installation, TechnicianID: c.TechnicianID, TechnicianName: c.TechnicianName, CaseID: c.ID, Revision: c.Revision, SessionID: p.ID, Origin: p.Origin, Method: "GET", Target: target}
		if f := w.dispatchSupport(context.Background(), s, e, nil, nil); f.Status != 403 {
			t.Fatalf("%s: %d", target, f.Status)
		}
	}
}
func TestSupportOriginAndAssignmentBinding(t *testing.T) {
	s, a, c := supportFixture(t)
	code, _, err := s.issue(context.Background(), c, a)
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"http://session.support.example", "https://attacker.example", "https://support.example", "https://nested.session.support.example", "https://session.support.example:444", "https://session.support.example/path"} {
		if _, err = s.redeem(context.Background(), c, c.TechnicianID, code, origin); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	_, p := supportPending(t, s, a, c)
	if err = s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	c.Revision++
	c.TechnicianID = "other-tech"
	if err = s.mirrorCase(c); err != nil {
		t.Fatal(err)
	}
	if _, err = s.authorize(p.ID); err == nil {
		t.Fatal("reassignment retained support")
	}
}

func TestSupportMaintenanceRetention(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Now().Add(91 * 24 * time.Hour) }
	if err := s.maintain(90); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM support_grants WHERE code_hash IS NOT NULL").Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired grant hash retained: %d %v", count, err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM support_audit").Scan(&count); err != nil || count != 0 {
		t.Fatalf("old audit retained: %d %v", count, err)
	}
	if _, err := s.authorize(p.ID); err == nil {
		t.Fatal("expired session authorized")
	}
}

func TestSupportCaseCannotChangeCustomer(t *testing.T) {
	s, a, c := supportFixture(t)
	c.AccountID = "another-account"
	if _, _, err := s.issue(context.Background(), c, a); err == nil {
		t.Fatal("issued against another customer's case")
	}
}
