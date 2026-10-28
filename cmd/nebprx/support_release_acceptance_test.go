package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type releaseOutboundTrap struct{ calls atomic.Int64 }

func (tr *releaseOutboundTrap) RoundTrip(*http.Request) (*http.Response, error) {
	tr.calls.Add(1)
	return nil, errors.New("unexpected outbound request")
}

func releaseTableNames(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name!='support_audit' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}

func releaseTableRows(t *testing.T, db *sql.DB, name string) [][]any {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %q ORDER BY rowid", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		result = append(result, values)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// Snapshot every application table except the audit rows each denial must append.
func releaseGatewaySnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	snapshot := map[string][][]any{}
	for _, name := range releaseTableNames(t, db) {
		values := releaseTableRows(t, db, name)
		if len(values) > 0 {
			snapshot[name] = values
		}
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReleaseDeniedRoutesNoOutboundOrStateChange(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	w := supportSurfaceMux(t, s)
	// Include customer control routes in the production inventory too.
	w.g.support = &supportConnector{store: s}
	const sentinel = "RELEASE_SECRET_SENTINEL"
	w.g.cfgPath = filepath.Join(t.TempDir(), "gateway.hcl")
	if err := os.WriteFile(w.g.cfgPath, []byte(sentinel), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO devices(id,profile,created_ns,last_seen_ns) VALUES('10.55.0.2','default',1,1)"); err != nil {
		t.Fatal(err)
	}
	before := releaseGatewaySnapshot(t, s.db)
	trap := &releaseOutboundTrap{}
	previous := http.DefaultTransport
	http.DefaultTransport = trap
	defer func() { http.DefaultTransport = previous }()
	count := 0
	// Unlike the earlier broad method test, assert every classified denied route's
	// declared method, committed deny audit, complete DB snapshot and outbound trap.
	for _, route := range w.routes() {
		if route.SupportPolicy != supportDeny {
			continue
		}
		count++
		path := route.Path
		if strings.HasSuffix(path, "/") {
			path += "release-operation"
		}
		target := path + "?ip=10.55.0.2&profile=default&code=" + sentinel
		headers := http.Header{"Authorization": {"Bearer " + sentinel}, "X-PRX-Actor-ID": {sentinel}, "X-Forwarded-User": {sentinel}, "Cookie": {"cp_session=" + sentinel}, "Content-Type": {"application/json"}}
		f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, route.Method, target), []byte(`{"secret":"`+sentinel+`"}`), headers)
		if f.Status != 403 || strings.Contains(string(f.Body), sentinel) {
			t.Fatalf("%s %s: status=%d", route.Method, route.Path, f.Status)
		}
		var actor, name, effective, method, outcome string
		if err := s.db.QueryRow("SELECT actor_id,actor_name,effective_id,method,outcome FROM support_audit ORDER BY rowid DESC LIMIT 1").Scan(&actor, &name, &effective, &method, &outcome); err != nil {
			t.Fatal(err)
		}
		if actor != c.TechnicianID || name != c.TechnicianName || effective != a.ID || method != route.Method || outcome != "denied" {
			t.Fatalf("%s: wrong denial audit identity/method/outcome", route.Path)
		}
		if after := releaseGatewaySnapshot(t, s.db); after != before {
			t.Fatalf("%s: denied route changed application tables", route.Path)
		}
	}
	if count < 20 {
		t.Fatal("production deny inventory unexpectedly empty", count)
	}
	if trap.calls.Load() != 0 {
		t.Fatal("denied routes made outbound HTTP requests", trap.calls.Load())
	}
	raw, err := os.ReadFile(w.g.cfgPath)
	if err != nil || string(raw) != sentinel {
		t.Fatal("denied config route altered raw file", err)
	}
	// Inspect all columns, including nullable/future audit fields and blob values.
	rows, err := s.db.Query("SELECT * FROM support_audit")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		for _, value := range values {
			if strings.Contains(fmt.Sprint(value), sentinel) {
				t.Fatal("audit retained secret sentinel")
			}
			if b, ok := value.([]byte); ok && strings.Contains(string(b), sentinel) {
				t.Fatal("audit blob retained sentinel")
			}
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseOutcomeAuditFailureWithholdsCustomerReads(t *testing.T) {
	for _, target := range []string{"/api/state", "/api/events"} {
		t.Run(target, func(t *testing.T) {
			s, a, c := supportFixture(t)
			_, p := supportPending(t, s, a, c)
			if err := s.activate(p.ID, c); err != nil {
				t.Fatal(err)
			}
			w := supportSurfaceMux(t, s)
			if _, err := s.db.Exec("CREATE TRIGGER release_audit_failure BEFORE INSERT ON support_audit WHEN NEW.operation LIKE 'complete:%' BEGIN SELECT RAISE(ABORT,'RELEASE_AUDIT_SECRET_SENTINEL'); END"); err != nil {
				t.Fatal(err)
			}
			f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", target), nil, nil)
			if f.Status != 503 || string(f.Body) != "support unavailable" {
				t.Fatalf("unaudited customer result released: %d", f.Status)
			}
			if _, err := s.authorize(p.ID); err == nil {
				t.Fatal("outcome audit failure retained support session")
			}
			var completed int
			if err := s.db.QueryRow("SELECT count(*) FROM support_audit WHERE operation LIKE 'complete:%'").Scan(&completed); err != nil || completed != 0 {
				t.Fatal("false successful completion audit", err)
			}
			// Exercise normal authentication, not merely existence of dashboard_users.
			token, err := createDashboardSession(s.db, "root", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "https://customer.example/api/state", nil)
			req.AddCookie(&http.Cookie{Name: cpSessionCookieName, Value: token})
			w.dashboardAuthGate(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				principal, ok := principalFromContext(r.Context())
				if !ok || principal.Kind != principalDashboardPassword {
					t.Error("normal customer principal lost")
				}
				rw.WriteHeader(204)
			})).ServeHTTP(rec, req)
			if rec.Code != 204 {
				t.Fatal("support audit failure broke local authentication", rec.Code)
			}
		})
	}
}

func TestReleaseRevocationBeforeFinalReadRelease(t *testing.T) {
	for _, target := range []string{"/api/state", "/api/events"} {
		t.Run(target, func(t *testing.T) {
			s, a, c := supportFixture(t)
			_, p := supportPending(t, s, a, c)
			if err := s.activate(p.ID, c); err != nil {
				t.Fatal(err)
			}
			w := supportSurfaceMux(t, s)
			// Deterministic fault injection after initial admission. Persist revocation
			// with intent audit, before the buffered read reaches its final release check.
			if _, err := s.db.Exec("CREATE TRIGGER release_revoke AFTER INSERT ON support_audit WHEN NEW.operation LIKE 'authorize:%' BEGIN UPDATE support_sessions SET state='revoked' WHERE id=NEW.session_id; END"); err != nil {
				t.Fatal(err)
			}
			f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", target), nil, nil)
			if f.Status != 401 || string(f.Body) != "support unavailable" {
				t.Fatalf("post-admission customer bytes released: %d", f.Status)
			}
			var state string
			if err := s.db.QueryRow("SELECT state FROM support_sessions WHERE id=?", p.ID).Scan(&state); err != nil || state != "revoked" {
				t.Fatal("revocation injection did not run", err)
			}
			var completed int
			if err := s.db.QueryRow("SELECT count(*) FROM support_audit WHERE operation LIKE 'complete:%'").Scan(&completed); err != nil || completed != 0 {
				t.Fatal("revoked read recorded successful release", err)
			}
		})
	}
}
