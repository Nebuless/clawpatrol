package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Nebuless/nebprx/internal/supportwire"
)

func TestDashboardContextSchemaAndRevision(t *testing.T) {
	w := configApplyTestMux(t, true)
	if err := setDashboardUser(w.g.db, "root", "synthetic-password"); err != nil {
		t.Fatal(err)
	}
	a, err := resolveDashboardAccount(w.g.db, "password", "local", "root", "root")
	if err != nil {
		t.Fatal(err)
	}
	w.g.support = &supportConnector{}
	call := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/account/context", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r = r.WithContext(contextWithPrincipal(r.Context(), principal{Kind: principalDashboardPassword, Owner: "root"}))
		rec := httptest.NewRecorder()
		w.customerSupport(rec, r)
		return rec
	}
	for _, body := range []string{
		`{"revision":0,"context":{"profile":"missing"}}`,
		`{"revision":0,"context":{"theme":"sepia"}}`,
		`{"revision":-1,"context":{}}`,
		`{"revision":9007199254740991,"context":{}}`,
		`{"revision":0,"context":{"token":"secret"}}`,
		`{"revision":0,"account_id":"foreign","context":{}}`,
		`{"revision":0,"context":{"route":{"page":"onboard"}}}`,
		`{"revision":0,"context":{"route":{"page":"analytics","query":"code=secret"}}}`,
		`{"revision":0,"context":{"route":{"page":"device","device_ip":"bad-ip"}}}`,
		`{"revision":0,"context":{"route":{"page":"request","request_id":"../../secret"}}}`,
		`{"revision":0,"context":{"route":{"page":"main","device_ip":"10.0.0.1"}}}`,
		`{"revision":0,"context":{"filters":{"range":"forever"}}}`,
		`{"revision":0,"context":{"filters":{"host":"https://example.com/?code=secret"}}}`,
		`{"revision":0,"context":{"profile":"` + strings.Repeat("x", 129) + `"}}`,
		`{"revision":0,"context":{}}` + strings.Repeat(" ", dashboardContextLimit),
	} {
		if rec := call("PATCH", body); rec.Code != 400 {
			t.Fatalf("invalid context accepted: %d %s", rec.Code, rec.Body)
		}
	}
	good := `{"revision":0,"context":{"profile":"default","theme":"dark","route":{"page":"analytics","device_ip":"fd7a:115c:a1e0::1"},"filters":{"range":"6h","color":"host","scale":"linear","host":"public.example","device_ip":"10.55.0.2"}}}`
	if rec := call("PATCH", good); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	if rec := call("PATCH", good); rec.Code != 409 {
		t.Fatalf("stale save: %d", rec.Code)
	}
	rec := call("GET", "")
	var saved struct {
		Context  dashboardContext
		Revision int64
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || saved.Revision != 1 || saved.Context.Profile != "default" || saved.Context.Filters.Range != "6h" || saved.Context.Route.DeviceIP != "fd7a:115c:a1e0::1" {
		t.Fatalf("roundtrip: %s %v", rec.Body, err)
	}
	b, err := resolveDashboardAccount(w.g.db, "tailnet", "https://control.example", "42", "other@example.com")
	if err != nil {
		t.Fatal(err)
	}
	other, rev, err := w.loadDashboardContext(b.ID)
	if err != nil || rev != 0 || other.Profile != "" {
		t.Fatal("preferences crossed accounts")
	}
	// Concurrent tabs cannot both commit the same revision.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- call("PATCH", strings.Replace(good, `"revision":0`, `"revision":1`, 1)).Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent revision: %v", counts)
	}
	// Existing pre-F6 minimal contexts still load, but removed profiles fail closed.
	for _, raw := range []string{`{}`, `{"profile":"default","theme":"dark"}`} {
		if _, err := w.g.db.Exec("UPDATE dashboard_context SET context_json=? WHERE account_id=?", raw, a.ID); err != nil {
			t.Fatal(err)
		}
		if rec := call("GET", ""); rec.Code != 200 {
			t.Fatalf("legacy context: %s", rec.Body)
		}
	}
	for _, raw := range []string{`{"profile":"removed"}`, `{"future":"untrusted"}`} {
		if _, err := w.g.db.Exec("UPDATE dashboard_context SET context_json=? WHERE account_id=?", raw, a.ID); err != nil {
			t.Fatal(err)
		}
		if rec := call("GET", ""); rec.Code != 409 {
			t.Fatal("invalid saved context broadened")
		}
	}
}

func TestDashboardContextSupportSnapshot(t *testing.T) {
	s, a, c := supportFixture(t)
	_, session := supportPending(t, s, a, c)
	if err := s.activate(session.ID, c); err != nil {
		t.Fatal(err)
	}
	base := configApplyTestMux(t, true)
	f, err := os.OpenFile(base.g.cfgPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`
profile "limited" { credentials = [] }
`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	cfg, policy, err := loadConfig(base.g.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	base.g.cfg.Store(cfg)
	base.g.policy.Store(policy)
	g := &Gateway{db: s.db, agents: NewAgentRegistry(), onboard: newOnboardRegistry()}
	g.cfg.Store(base.g.cfg.Load())
	g.policy.Store(base.g.Policy())
	w := &webMux{g: g, onboard: g.onboard}
	if _, err := s.db.Exec("INSERT INTO dashboard_context(account_id,context_json,revision) VALUES(?,?,1)", a.ID, `{"profile":"limited","theme":"dark","route":{"page":"analytics"},"filters":{"range":"6h","scale":"linear"}}`); err != nil {
		t.Fatal(err)
	}
	dispatch := func(method, target string) supportwire.Frame {
		return w.dispatchSupport(context.Background(), s, supportwire.Envelope{OrganizationID: s.org, InstallationID: s.installation, TechnicianID: c.TechnicianID, TechnicianName: c.TechnicianName, CaseID: c.ID, Revision: c.Revision, SessionID: session.ID, Origin: session.Origin, Method: method, Target: target}, nil, nil)
	}
	result := dispatch("GET", "/api/state")
	if result.Status != 200 || !strings.Contains(string(result.Body), `"range":"6h"`) || !strings.Contains(string(result.Body), `"saved_theme":"dark"`) {
		t.Fatalf("snapshot: %d %s", result.Status, result.Body)
	}
	var state struct{ Integrations []any }
	if err := json.Unmarshal(result.Body, &state); err != nil || len(state.Integrations) != 0 {
		t.Fatalf("saved profile not applied: %s %v", result.Body, err)
	}
	ordinary := httptest.NewRecorder()
	w.apiState(ordinary, httptest.NewRequest("GET", "/api/state?profile=limited", nil))
	if err := json.Unmarshal(ordinary.Body.Bytes(), &state); err != nil || len(state.Integrations) != 0 {
		t.Fatalf("ordinary profile not applied: %s %v", ordinary.Body, err)
	}
	if result := dispatch("PATCH", "/api/account/context"); result.Status != 403 {
		t.Fatalf("support preference write: %d", result.Status)
	}
	if _, err := s.db.Exec("UPDATE dashboard_context SET context_json=? WHERE account_id=?", `{"profile":"removed"}`, a.ID); err != nil {
		t.Fatal(err)
	}
	if result := dispatch("GET", "/api/state"); result.Status != 409 {
		t.Fatalf("removed profile broadened: %d", result.Status)
	}
}
