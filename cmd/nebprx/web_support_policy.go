package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/Nebuless/nebprx/internal/supportwire"
)

func (w *webMux) dispatchSupport(ctx context.Context, s *supportStore, e supportwire.Envelope, body []byte, headers http.Header) supportwire.Frame {
	denied := func(status int) supportwire.Frame {
		return supportwire.Frame{Status: status, Body: []byte("support unavailable"), Headers: http.Header{"Content-Type": {"text/plain"}}}
	}
	if !s.enabled() || !validSupportOrigin(e.Origin) || e.TechnicianID == "" || e.CaseID == "" {
		return denied(403)
	}
	if supportwire.ValidateTarget(e.Method, e.Target) != nil {
		return denied(403)
	}
	c := provider.Case{ID: e.CaseID, OrganizationID: e.OrganizationID, InstallationID: e.InstallationID, TechnicianID: e.TechnicianID, TechnicianName: e.TechnicianName, Revision: e.Revision, State: "assigned"}
	// Provider checked current case and live IdP before signing this request.
	// Local mirror must independently match that revision, not adopt browser fields.
	var expires int64
	s.mu.Lock()
	err := s.conn.QueryRowContext(ctx, "SELECT expires,account_id FROM support_cases WHERE id=? AND state='assigned' AND technician_id=? AND revision=?", e.CaseID, e.TechnicianID, e.Revision).Scan(&expires, &c.AccountID)
	s.mu.Unlock()
	if err != nil {
		return denied(403)
	}
	c.Expires = expires
	if e.Target == "/__support/redeem" && e.Method == "POST" {
		var v struct{ Case, Code, Origin string }
		if provider.DecodeStrict(bytes.NewReader(body), supportwire.MaxBody, &v) != nil || v.Case != e.CaseID || v.Origin != e.Origin {
			return denied(403)
		}
		session, err := s.redeem(ctx, c, e.TechnicianID, v.Code, e.Origin)
		if err != nil {
			return denied(403)
		}
		b, _ := json.Marshal(session)
		return supportwire.Frame{Status: 200, Body: b, Headers: http.Header{"Content-Type": {"application/json"}}}
	}
	if e.Target == "/__support/end" && e.Method == "POST" {
		s.mu.Lock()
		defer s.mu.Unlock()
		err := s.transaction(func(tx *sql.Tx) error {
			var account, grant string
			if err := tx.QueryRow("SELECT account_id,grant_id FROM support_sessions WHERE id=? AND case_id=? AND technician_id=? AND assignment_revision=? AND origin=?", e.SessionID, c.ID, c.TechnicianID, c.Revision, e.Origin).Scan(&account, &grant); err != nil {
				return err
			}
			if _, err := tx.Exec("UPDATE support_sessions SET state='revoked' WHERE id=?", e.SessionID); err != nil {
				return err
			}
			return s.audit(tx, c, account, e.SessionID, grant, "end", "success", "technician_end", e.Method)
		})
		if err != nil {
			return denied(503)
		}
		return supportwire.Frame{Status: 200}
	}
	if e.Target == "/__support/activate" && e.Method == "POST" {
		s.mu.Lock()
		var bound int
		err := s.conn.QueryRowContext(ctx, "SELECT count(*) FROM support_sessions WHERE id=? AND origin=?", e.SessionID, e.Origin).Scan(&bound)
		s.mu.Unlock()
		if err != nil || bound != 1 {
			return denied(403)
		}
		if s.activate(e.SessionID, c) != nil {
			return denied(403)
		}
		return supportwire.Frame{Status: 200}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, err := s.authorizeLocked(e.SessionID)
	if err != nil || session.TechnicianID != e.TechnicianID || session.Revision != e.Revision || session.CaseID != e.CaseID || session.Origin != e.Origin {
		return denied(401)
	}
	audit := func(operation, outcome string) error {
		return s.transaction(func(tx *sql.Tx) error {
			return s.audit(tx, c, session.AccountID, session.ID, session.GrantID, operation, outcome, "support_request", e.Method)
		})
	}
	if err = audit("authorize:"+supportRouteTemplate(e.Target), "intent"); err != nil {
		return denied(503)
	}

	request, err := http.NewRequestWithContext(ctx, e.Method, e.Origin+e.Target, bytes.NewReader(body))
	if err != nil {
		return denied(400)
	}
	for _, values := range request.URL.Query() {
		if len(values) != 1 {
			return denied(403)
		}
	}
	request.Header, err = supportwire.SafeHeaders(headers)
	if err != nil {
		return denied(400)
	}
	request = request.WithContext(contextWithPrincipal(ctx, principal{Kind: principalSupport, Owner: session.AccountName, ActorID: e.TechnicianID, ActorName: e.TechnicianName, EffectiveID: session.AccountID, EffectiveName: session.AccountName, OrganizationID: s.org, InstallationID: s.installation, SupportSessionID: session.ID}))
	if p := request.URL.Query().Get("profile"); p != "" {
		known := false
		for _, name := range orderedProfileNames(w.g.cfg.Load().Policy) {
			if name == p {
				known = true
			}
		}
		if !known {
			return denied(403)
		}
	}
	// Production route metadata is default-deny, independent of projection switch.
	if request.URL.Path != "/api/support/config-view" && request.URL.Path != "/" && !strings.HasPrefix(request.URL.Path, "/_a/") && !strings.HasPrefix(request.URL.Path, "/assets/") && !strings.HasPrefix(request.URL.Path, "/fonts/") && request.URL.Path != "/prx-icon.svg" && request.URL.Path != "/favicon-32.png" && request.URL.Path != "/favicon-16.png" {
		allowed := false
		for _, route := range w.routes() {
			match := request.URL.Path == route.Path || route.Path == "/api/actions/" && strings.HasPrefix(request.URL.Path, route.Path)
			if match && e.Method == route.Method && route.SupportPolicy == supportAllow {
				allowed = true
				break
			}
		}
		if !allowed {
			if audit("deny:"+supportRouteTemplate(e.Target), "denied") != nil {
				return denied(503)
			}
			return denied(403)
		}
	}
	recorder := httptest.NewRecorder()
	switch {
	case e.Method == "GET" && request.URL.Path == "/api/state":
		w.supportState(recorder, request, s, session)
	case e.Method == "GET" && request.URL.Path == "/api/status":
		writeJSON(recorder, w.supportIntegrations(request))
	case e.Method == "GET" && request.URL.Path == "/api/profiles":
		w.apiProfiles(recorder, request)
	case e.Method == "GET" && request.URL.Path == "/api/plugins":
		out := []map[string]any{}
		if w.g.pluginMgr != nil {
			for _, p := range w.g.pluginMgr.PluginInfos() {
				out = append(out, map[string]any{"name": p.Name, "version": p.Version, "blocked": p.Blocked, "sandboxMode": p.SandboxMode, "endpoints": p.Endpoints})
			}
		}
		writeJSON(recorder, out)
	case e.Method == "GET" && request.URL.Path == "/api/facets":
		w.apiFacets(recorder, request)
	case e.Method == "GET" && request.URL.Path == "/api/analytics":
		w.supportAnalytics(recorder, request)
	case e.Method == "GET" && request.URL.Path == "/api/support/config-view":
		writeJSON(recorder, map[string]any{"profiles": orderedProfileNames(w.g.cfg.Load().Policy), "dashboard_config_writes": false, "exclusion": "Arbitrary HCL changes are unavailable during support."})
	case e.Method == "GET" && request.URL.Path == "/api/rules":
		rows := w.collectRuleSummaries(request.URL.Query().Get("profile"))
		for i := range rows {
			rows[i].Condition = ""
			rows[i].Reason = ""
			rows[i].Credential = ""
			rows[i].Approve = nil
		}
		writeJSON(recorder, rows)
	case e.Method == "GET" && strings.HasPrefix(request.URL.Path, "/api/actions/"):
		id := strings.TrimPrefix(request.URL.Path, "/api/actions/")
		if strings.Contains(id, "/") || request.URL.RawQuery != "" {
			recorder.WriteHeader(403)
			break
		}
		event, err := w.loadAction(id)
		if err != nil {
			recorder.WriteHeader(404)
			break
		}
		writeJSON(recorder, supportEvent(event))
	case e.Method == "GET" && request.URL.Path == "/api/events":
		w.supportEvents(recorder, request)
	case e.Method == "POST" && request.URL.Path == "/api/actions/rule-preview":
		w.supportRulePreview(recorder, request)
	case e.Method == "POST" && (request.URL.Path == "/api/agents/profile" || request.URL.Path == "/api/agents/delete"):
		err = w.supportDeviceMutation(ctx, s, c, session, request)
		if err != nil {
			recorder.WriteHeader(503)
		} else {
			writeJSON(recorder, map[string]bool{"ok": true})
		}
	case e.Method == "GET" && (request.URL.Path == "/" || strings.HasPrefix(request.URL.Path, "/_a/") || strings.HasPrefix(request.URL.Path, "/assets/") || strings.HasPrefix(request.URL.Path, "/fonts/") || request.URL.Path == "/prx-icon.svg" || request.URL.Path == "/favicon-32.png" || request.URL.Path == "/favicon-16.png"):
		w.staticHandler().ServeHTTP(recorder, request)
	default:
		if audit("deny:"+supportRouteTemplate(e.Target), "denied") != nil {
			return denied(503)
		}
		return denied(403)
	}
	if recorder.Body.Len() > supportwire.MaxResponse {
		return denied(503)
	}
	if _, err = s.authorizeLocked(session.ID); err != nil {
		return denied(401)
	}
	outcome := "success"
	if recorder.Code >= 400 {
		outcome = "failure"
	}
	if audit("complete:"+supportRouteTemplate(e.Target), outcome) != nil {
		_, _ = s.conn.ExecContext(ctx, "UPDATE support_sessions SET state='revoked' WHERE id=?", session.ID)
		return denied(503)
	}
	return supportwire.Frame{Status: recorder.Code, Body: recorder.Body.Bytes(), Headers: http.Header{
		"Content-Type":                {recorder.Header().Get("Content-Type")},
		supportwire.EventCursorHeader: {recorder.Header().Get(supportwire.EventCursorHeader)},
		supportwire.EventMoreHeader:   {recorder.Header().Get(supportwire.EventMoreHeader)},
	}}
}
func supportRouteTemplate(target string) string {
	path := strings.SplitN(target, "?", 2)[0]
	switch path {
	case "/api/state", "/api/status", "/api/profiles", "/api/support/config-view", "/api/rules", "/api/events", "/api/agents/profile", "/api/agents/delete", "/__support/end":
		return path
	}
	if strings.HasPrefix(path, "/api/actions/") {
		return "/api/actions/:id"
	}
	if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/fonts/") {
		return "/assets/:file"
	}
	return "unclassified"
}
