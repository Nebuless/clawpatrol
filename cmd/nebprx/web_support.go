package main

import (
	"database/sql"
	"encoding/json"
	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
)

func (w *webMux) supportRoutes() []webRoute {
	routes := []webRoute{}
	if w.g.support != nil {
		for _, route := range []struct{ method, path string }{
			{"GET", "/api/support"}, {"POST", "/api/support/cases"},
			{"POST", "/api/support/grants"}, {"POST", "/api/support/revoke"},
			{"GET", "/api/support/audit"},
			// Route metadata is path-based. accountContext validates GET/PATCH.
			{"GET", "/api/account/context"},
		} {
			routes = append(routes, webRoute{Method: route.method, Path: route.path, Auth: authDashboard, Handler: w.customerSupport})
		}
	}
	return routes
}
func supportError(rw http.ResponseWriter, status int, code string) {
	rw.Header().Set("Cache-Control", "no-store")
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	writeJSON(rw, map[string]any{"error": map[string]string{"code": code, "message": "Support request unavailable"}})
}
func (w *webMux) customerSupport(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Cache-Control", "no-store")
	p, ok := principalFromContext(r.Context())
	if !ok || p.Kind == principalSupport || w.g.support == nil {
		supportError(rw, 403, "forbidden")
		return
	}
	var a dashboardAccount
	var err error
	if p.Kind == principalDashboardPassword {
		a, err = resolveDashboardAccount(w.g.db, "password", "local", dashboardRootUsername, dashboardRootUsername)
	} else if p.Kind == principalTailnet && w.tailnetMembership(p.Authority, p.Subject, p.Owner) {
		a, err = resolveDashboardAccount(w.g.db, "tailnet", p.Authority, p.Subject, p.Owner)
	} else {
		supportError(rw, 403, "membership_proof_unavailable")
		return
	}
	if err != nil {
		supportError(rw, 403, "account_unavailable")
		return
	}
	c := w.g.support
	s := c.store
	if r.URL.Path == "/api/account/context" {
		w.accountContext(rw, r, a)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/support" {
		cases := []provider.Case{}
		rows, err := w.g.db.QueryContext(r.Context(), "SELECT id FROM support_cases WHERE account_id=? ORDER BY expires DESC LIMIT 100", a.ID)
		if err != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		defer func() { _ = rows.Close() }()
		ids := []string{}
		for rows.Next() {
			var id string
			if rows.Scan(&id) != nil {
				_ = rows.Close()
				supportError(rw, 503, "unavailable")
				return
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		for _, id := range ids {
			v, e := c.currentCase(r.Context(), id)
			if e == nil && v.AccountID == a.ID {
				cases = append(cases, v)
			}
		}
		sessions := []map[string]any{}
		rows, err = w.g.db.QueryContext(r.Context(), "SELECT id,case_id,state,expires FROM support_sessions WHERE account_id=? AND state IN ('active','pending_activation') AND expires>?", a.ID, s.now().Unix())
		if err != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		for rows.Next() {
			var id, caseID, state string
			var expires int64
			if rows.Scan(&id, &caseID, &state, &expires) != nil {
				_ = rows.Close()
				supportError(rw, 503, "unavailable")
				return
			}
			sessions = append(sessions, map[string]any{"id": id, "case_id": caseID, "state": state, "expires": expires})
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		writeJSON(rw, map[string]any{"organization_id": s.org, "installation_id": s.installation, "accounts": []dashboardAccount{a}, "cases": cases, "sessions": sessions, "available": c.connected()})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/support/audit" {
		w.customerSupportAudit(rw, r, a.ID)
		return
	}
	if r.Method != "POST" {
		supportError(rw, 405, "method_not_allowed")
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		supportError(rw, 415, "content_type")
		return
	}
	switch r.URL.Path {
	case "/api/support/cases":
		var v struct {
			Intent   string `json:"intent_id"`
			Summary  string `json:"summary"`
			Category string `json:"category"`
		}
		if provider.DecodeStrict(r.Body, 4096, &v) != nil {
			supportError(rw, 400, "invalid_request")
			return
		}
		if _, err := uuid.Parse(v.Intent); err != nil {
			supportError(rw, 400, "invalid_intent")
			return
		}
		result, err := c.submitCase(r.Context(), v.Intent, a.ID, v.Summary, v.Category)
		if err != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		writeJSON(rw, result)
	case "/api/support/grants":
		var v struct {
			CaseID   string `json:"case_id"`
			Target   string `json:"target_account_id"`
			Revision int64  `json:"assignment_revision"`
		}
		if provider.DecodeStrict(r.Body, 4096, &v) != nil || v.Target != a.ID {
			supportError(rw, 400, "invalid_request")
			return
		}
		current, err := c.currentCase(r.Context(), v.CaseID)
		if err != nil || current.Revision != v.Revision || current.AccountID != a.ID {
			supportError(rw, 409, "grant_unavailable")
			return
		}
		code, id, err := s.issue(r.Context(), current, a)
		if err != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		writeJSON(rw, map[string]any{"code": code, "grant_id": id, "expires": s.now().Unix() + 300})
	case "/api/support/revoke":
		var v struct {
			ID string `json:"id"`
		}
		if provider.DecodeStrict(r.Body, 4096, &v) != nil || v.ID == "" {
			supportError(rw, 400, "invalid_request")
			return
		}
		if s.customerRevoke(a, v.ID) != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		writeJSON(rw, map[string]bool{"ok": true})
	default:
		supportError(rw, 404, "not_found")
	}
}
func (c *supportConnector) connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.link != nil && c.store.enabled()
}
func (w *webMux) accountContext(rw http.ResponseWriter, r *http.Request, a dashboardAccount) {
	if r.Method == "GET" {
		c, revision, err := w.loadDashboardContext(a.ID)
		if err != nil {
			supportError(rw, 409, "invalid_saved_context")
			return
		}
		writeJSON(rw, map[string]any{"context": c, "revision": revision, "profiles": orderedProfileNames(w.g.cfg.Load().Policy)})
		return
	}
	if r.Method != "PATCH" || r.Header.Get("Content-Type") != "application/json" {
		supportError(rw, 405, "method_not_allowed")
		return
	}
	var v struct {
		Revision int64            `json:"revision"`
		Context  dashboardContext `json:"context"`
	}
	if provider.DecodeStrict(r.Body, dashboardContextLimit, &v) != nil || v.Revision < 0 || v.Revision >= 1<<53-1 || w.validateDashboardContext(v.Context) != nil {
		supportError(rw, 400, "invalid_context")
		return
	}
	b, _ := json.Marshal(v.Context)
	res, err := w.g.db.Exec("INSERT INTO dashboard_context(account_id,context_json,revision) SELECT ?,?,1 WHERE ?=0 OR EXISTS(SELECT 1 FROM dashboard_context WHERE account_id=?) ON CONFLICT(account_id) DO UPDATE SET context_json=excluded.context_json,revision=dashboard_context.revision+1 WHERE dashboard_context.revision=?", a.ID, string(b), v.Revision, a.ID, v.Revision)
	if err != nil {
		supportError(rw, 503, "unavailable")
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		supportError(rw, 409, "stale_context")
		return
	}
	writeJSON(rw, map[string]int64{"revision": v.Revision + 1})
}
func (w *webMux) customerSupportAudit(rw http.ResponseWriter, r *http.Request, accountID string) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			supportError(rw, 400, "invalid_limit")
			return
		}
		limit = n
	}
	cursor := int64(0)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		if _, err := uuid.Parse(raw); err != nil {
			supportError(rw, 400, "invalid_cursor")
			return
		}
		if err := w.g.db.QueryRowContext(r.Context(), "SELECT rowid FROM support_audit WHERE id=? AND (effective_id=? OR actor_id=?)", raw, accountID, accountID).Scan(&cursor); err != nil {
			supportError(rw, 400, "invalid_cursor")
			return
		}
	}

	rows, err := w.g.db.QueryContext(r.Context(), "SELECT rowid,id,at,actor_id,actor_name,effective_id,effective_name,organization_id,installation_id,case_id,session_id,operation,method,outcome,reason FROM support_audit WHERE rowid>? AND (effective_id=? OR actor_id=?) ORDER BY rowid LIMIT ?", cursor, accountID, accountID, limit)
	if err != nil {
		supportError(rw, 503, "unavailable")
		return
	}
	defer func() { _ = rows.Close() }()
	defer func() { _ = rows.Close() }()
	out := []map[string]any{}
	for rows.Next() {
		var id, operation, method, outcome, reason, org, installation string
		var at, sequence int64
		var actor, name, effective, effectiveName, caseID, session sql.NullString
		if rows.Scan(&sequence, &id, &at, &actor, &name, &effective, &effectiveName, &org, &installation, &caseID, &session, &operation, &method, &outcome, &reason) != nil {
			supportError(rw, 503, "unavailable")
			return
		}
		out = append(out, map[string]any{"cursor": id, "id": id, "at": at, "actor_id": actor.String, "actor_name": name.String, "effective_id": effective.String, "effective_name": effectiveName.String, "case_id": caseID.String, "session_id": session.String, "organization_id": org, "installation_id": installation, "operation": operation, "method": method, "decision": auditDecision(operation, outcome), "outcome": outcome, "reason": reason})
	}
	if rows.Err() != nil {
		supportError(rw, 503, "unavailable")
		return
	}
	writeJSON(rw, out)
}

// Only recorded authorization decisions are labelled allow/deny. Lifecycle
// metadata is not evidence of an authorization decision.
func auditDecision(operation, outcome string) string {
	if outcome == "denied" || strings.HasPrefix(operation, "deny:") {
		return "deny"
	}
	if strings.HasPrefix(operation, "complete:") {
		return "allow"
	}
	return "not_applicable"
}
