package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/Nebuless/nebprx/internal/registry"
	"github.com/Nebuless/nebprx/internal/supportwire"
)

func TestSupportAuditPaginationMetadata(t *testing.T) {
	s, a, c := supportFixture(t)
	for n := 0; n < 205; n++ {
		if err := s.transaction(func(tx *sql.Tx) error {
			return s.audit(tx, c, a.ID, "session", "grant", fmt.Sprintf("complete:/api/state-%03d", n), "success", "metadata")
		}); err != nil {
			t.Fatal(err)
		}
	}
	w := webMux{g: &Gateway{db: s.db}}
	cursor := ""
	seen := map[string]bool{}
	for {
		rec := httptest.NewRecorder()
		w.customerSupportAudit(rec, httptest.NewRequest("GET", "/api/support/audit?limit=50&cursor="+cursor, nil), a.ID)
		if rec.Code != 200 {
			t.Fatal(rec.Code)
		}
		var rows []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			id := row["id"].(string)
			if seen[id] {
				t.Fatal("duplicate audit event")
			}
			seen[id] = true
			for key, want := range map[string]string{"actor_id": c.TechnicianID, "actor_name": c.TechnicianName, "effective_id": a.ID, "effective_name": a.Name, "organization_id": s.org, "installation_id": s.installation, "session_id": "session", "case_id": c.ID, "decision": "allow", "outcome": "success"} {
				if row[key] != want {
					t.Fatalf("%s lost", key)
				}
			}
			cursor = row["cursor"].(string)
			if cursor != id {
				t.Fatal("cursor must be opaque audit-row ID")
			}
		}
		if len(rows) < 50 {
			break
		}
	}
	if len(seen) != 205 {
		t.Fatalf("lost events: %d", len(seen))
	}
	// Foreign account cannot resolve another account's cursor or enumerate its audit.
	foreign := httptest.NewRecorder()
	w.customerSupportAudit(foreign, httptest.NewRequest("GET", "/api/support/audit?cursor="+cursor, nil), "foreign")
	if foreign.Code != 400 {
		t.Fatal("foreign cursor resolved", foreign.Code)
	}
	foreign = httptest.NewRecorder()
	w.customerSupportAudit(foreign, httptest.NewRequest("GET", "/api/support/audit", nil), "foreign")
	var foreignRows []map[string]any
	if err := json.Unmarshal(foreign.Body.Bytes(), &foreignRows); err != nil || foreign.Code != 200 || len(foreignRows) != 0 {
		t.Fatal("foreign audit released", foreign.Code, err)
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=invalid", "cursor=-1", "cursor=invalid"} {
		rec := httptest.NewRecorder()
		w.customerSupportAudit(rec, httptest.NewRequest("GET", "/api/support/audit?"+query, nil), a.ID)
		if rec.Code != 400 {
			t.Fatal(query, rec.Code)
		}
	}
}

func supportSurfaceMux(t *testing.T, s *supportStore) *webMux {
	t.Helper()
	base := configApplyTestMux(t, true)
	g := &Gateway{db: s.db, agents: NewAgentRegistry(), onboard: newOnboardRegistry()}
	g.cfg.Store(base.g.cfg.Load())
	g.policy.Store(base.g.Policy())
	sink, err := NewSink(s.db, 64)
	if err != nil {
		t.Fatal(err)
	}
	g.sink = sink
	t.Cleanup(func() {
		if err := sink.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return &webMux{g: g, onboard: g.onboard}
}
func supportEnvelope(s *supportStore, c provider.Case, p supportSession, method, target string) supportwire.Envelope {
	return supportwire.Envelope{OrganizationID: s.org, InstallationID: s.installation, TechnicianID: c.TechnicianID, TechnicianName: c.TechnicianName, CaseID: c.ID, Revision: c.Revision, SessionID: p.ID, Origin: p.Origin, Method: method, Target: target}
}

func TestSupportAllDeniedRouteMethodsNoEffects(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	w := supportSurfaceMux(t, s)
	if _, err := s.db.Exec("INSERT INTO devices(id,profile,created_ns,last_seen_ns) VALUES('10.55.0.2','default',1,1)"); err != nil {
		t.Fatal(err)
	}
	// Exercise actual production route registry, not a copy of its deny list.
	for _, route := range w.routes() {
		if route.SupportPolicy != supportDeny {
			continue
		}
		for _, method := range []string{"GET", "POST", "PATCH", "DELETE", "PUT", "HEAD", "OPTIONS"} {
			f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, method, route.Path), []byte(`{"secret":"SENTINEL_DENIED_BODY"}`), http.Header{"X-Forwarded-User": {"root"}, "Authorization": {"Bearer SENTINEL_CREDENTIAL"}})
			if f.Status < 400 || len(f.Body) > 0 && strings.Contains(string(f.Body), "SENTINEL_") {
				t.Fatalf("denied route released data: %s %s %d", method, route.Path, f.Status)
			}
		}
	}
	for _, target := range []string{"/api/future", "/API/state", "/api/state?profile=default&profile=other", "/api/%73tate", "//api/state", "/api/../api/state"} {
		if f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", target), nil, nil); f.Status < 400 {
			t.Fatal(target)
		}
	}
	var profile string
	if err := s.db.QueryRow("SELECT profile FROM devices WHERE id='10.55.0.2'").Scan(&profile); err != nil || profile != "default" {
		t.Fatal("denied mutation changed device")
	}
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM support_sessions WHERE state='active'").Scan(&n); err != nil || n != 1 {
		t.Fatal("denied route changed session")
	}
}

func TestSupportRealTimerExpiryAndBufferedEventsRevocation(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	w := supportSurfaceMux(t, s)
	// Production dispatcher buffers SSE until final admission and outcome commit.
	ch := make(chan supportwire.Frame, 1)
	go func() {
		ch <- w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", "/api/events"), nil, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	if err := s.customerRevoke(a, p.ID); err != nil {
		t.Fatal(err)
	}
	<-ch // Any admitted batch precedes revoke's shared admission lock.
	f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", "/api/events"), nil, nil)
	if f.Status != 401 || strings.Contains(string(f.Body), "event: backlog") {
		t.Fatal("post-revoke SSE payload released")
	}
	_, p = supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE support_sessions SET expires=? WHERE id=?", time.Now().Unix()+1, p.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", "/api/state"), nil, nil); f.Status != 401 || strings.Contains(string(f.Body), "account_context") {
		t.Fatal("real expiry released payload")
	}
}

func TestSupportAuditReadOnlyAndFullKeepLocalServices(t *testing.T) {
	for _, fault := range []string{"readonly", "full"} {
		t.Run(fault, func(t *testing.T) {
			s, a, c := supportFixture(t)
			_, p := supportPending(t, s, a, c)
			if err := s.activate(p.ID, c); err != nil {
				t.Fatal(err)
			}
			w := supportSurfaceMux(t, s)
			dir := filepath.Join(t.TempDir(), "registry")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			r, err := registry.Open(dir, "client")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			}()
			if fault == "readonly" {
				_, err = s.conn.ExecContext(context.Background(), "PRAGMA query_only=ON")
			} else {
				// Exercise real SQLITE_FULL on support connection without making local state read-only.
				var pages int
				if err = s.db.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
					t.Fatal(err)
				}
				if _, err = s.db.Exec("CREATE TABLE disk_fill(payload BLOB)"); err != nil {
					t.Fatal(err)
				}
				if err = s.db.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
					t.Fatal(err)
				}
				if pages >= 256 {
					t.Fatal("fixture exceeds capacity bound")
				}
				if _, err = s.conn.ExecContext(context.Background(), "PRAGMA max_page_count=256"); err != nil {
					t.Fatal(err)
				}
				_, err = s.db.Exec("CREATE TRIGGER audit_full BEFORE INSERT ON support_audit BEGIN INSERT INTO disk_fill VALUES(zeroblob(2097152)); END")
			}
			if err != nil {
				t.Fatal(err)
			}
			f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "GET", "/api/state"), nil, nil)
			if f.Status != 503 || strings.Contains(string(f.Body), "account_context") {
				t.Fatal("audit fault released payload", f.Status)
			}
			if _, ok, err := lookupDashboardUser(s.db, "root"); err != nil || !ok {
				t.Fatal("audit fault broke ordinary login identity")
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/status", nil)
			token, err := r.LocalToken()
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			r.Handler().ServeHTTP(rec, req)
			if rec.Code != 200 {
				t.Fatal("audit fault broke local registry", rec.Code)
			}
		})
	}
}

func TestSupportAuditUsesRequestMethod(t *testing.T) {
	s, a, c := supportFixture(t)
	_, p := supportPending(t, s, a, c)
	if err := s.activate(p.ID, c); err != nil {
		t.Fatal(err)
	}
	w := supportSurfaceMux(t, s)
	f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, p, "PATCH", "/api/state"), nil, nil)
	if f.Status != 403 {
		t.Fatal(f.Status)
	}
	var method string
	if err := s.db.QueryRow("SELECT method FROM support_audit WHERE operation='authorize:/api/state' ORDER BY rowid DESC LIMIT 1").Scan(&method); err != nil || method != "PATCH" {
		t.Fatalf("request method=%s err=%v", method, err)
	}
}
