package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/Nebuless/nebprx/internal/supportwire"
	"net/http"
	"net/http/httptest"
)

// Construct fresh allowlisted values. Never serialize the credential config,
// action payload, headers, facets, verifier errors or saved conversation titles.
func (w *webMux) supportIntegrations(r *http.Request) []map[string]any {
	out := []map[string]any{}
	for _, v := range w.statusList(r) {
		out = append(out, map[string]any{"id": v.ID, "name": v.Name, "type": v.Type, "connected": v.Connected, "profiles": v.Profiles, "endpoints": v.Endpoints})
	}
	return out
}
func supportEvent(e *Event) Event {
	return Event{Ts: e.Ts, ID: e.ID, Phase: e.Phase, Mode: e.Mode, AgentIP: e.AgentIP, Host: e.Host, Method: e.Method, Status: e.Status, In: e.In, Out: e.Out, Ms: e.Ms, Action: e.Action, Family: e.Family, Endpoint: e.Endpoint, Rule: e.Rule}
}
func (w *webMux) supportAnalytics(rw http.ResponseWriter, r *http.Request) {
	recorder := httptest.NewRecorder()
	w.apiAnalytics(recorder, r)
	if recorder.Code != 200 {
		http.Error(rw, "analytics unavailable", http.StatusServiceUnavailable)
		return
	}
	var data struct {
		Events     []Event `json:"events"`
		Total      int     `json:"total"`
		TotalCount int64   `json:"total_count"`
		ErrorCount int64   `json:"error_count"`
		ByDevice   any     `json:"by_device"`
		ByHost     any     `json:"by_host"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &data) != nil {
		http.Error(rw, "analytics unavailable", http.StatusServiceUnavailable)
		return
	}
	for i := range data.Events {
		data.Events[i] = supportEvent(&data.Events[i])
	}
	writeJSON(rw, data)
}
func (w *webMux) supportState(rw http.ResponseWriter, r *http.Request, s *supportStore, session supportSession) {
	agents := w.agentsList()
	for _, a := range agents {
		a.Sessions = nil
		a.UA = ""
		a.LastHost = ""
	}
	savedContext, _, err := w.loadDashboardContext(session.AccountID)
	if err != nil {
		http.Error(rw, "saved context unavailable", http.StatusConflict)
		return
	}
	if r.URL.Query().Get("profile") == "" && savedContext.Profile != "" {
		cloned := r.Clone(r.Context())
		u := *r.URL
		q := u.Query()
		q.Set("profile", savedContext.Profile)
		u.RawQuery = q.Encode()
		cloned.URL = &u
		r = cloned
	}
	writeJSON(rw, map[string]any{"account_context": savedContext, "whoami": map[string]string{"user": session.AccountName, "auth_method": "support", "public_url": session.Origin}, "integrations": w.supportIntegrations(r), "agents": agents, "dashboard_config_writes": false, "customer_dashboard_config_writes": w.g.cfg.Load().DashboardConfigWrites(), "support": map[string]any{"actor_id": session.TechnicianID, "actor_name": session.TechnicianName, "effective_id": session.AccountID, "effective_name": session.AccountName, "session_id": session.ID, "organization_id": s.org, "installation_id": s.installation, "case_id": session.CaseID, "expires": session.Expires, "saved_profile": savedContext.Profile, "saved_theme": savedContext.Theme}, "update": nil})
}
func supportEventCursor(e Event) string {
	return supportwire.Hash([]byte(fmt.Sprintf("%s\x00%s\x00%d", e.ID, e.Phase, e.Ts.UnixNano())))
}

func (w *webMux) supportEvents(rw http.ResponseWriter, r *http.Request) {
	after := r.URL.Query().Get("after")
	if after != "" {
		if b, err := hex.DecodeString(after); err != nil || len(b) != 32 {
			http.Error(rw, "invalid event cursor", 400)
			return
		}
	}
	recent, _, cancel := w.g.sink.RecentAndSubscribe()
	cancel()
	start, gap := 0, false
	if after != "" {
		found := false
		for i, e := range recent {
			if supportEventCursor(e) == after {
				start, found = i+1, true
				break
			}
		}
		gap = !found
	}
	const prefix = "event: backlog\ndata: "
	const suffix = "\n\n"
	gapFrame := ""
	if gap {
		gapFrame = "event: backlog-gap\ndata: {}\n\n"
	}
	size := len(gapFrame) + len(prefix) + len(suffix) + 2
	events := []json.RawMessage{}
	cursor, more := after, false
	for _, e := range recent[start:] {
		if ip := r.URL.Query().Get("agent"); ip != "" && e.AgentIP != ip {
			cursor = supportEventCursor(e)
			continue
		}
		b, err := json.Marshal(supportEvent(&e))
		if err != nil {
			http.Error(rw, "events unavailable", http.StatusServiceUnavailable)
			return
		}
		needed := len(b)
		if len(events) > 0 {
			needed++
		}
		if size+needed > supportwire.MaxEventBatch {
			if len(events) == 0 {
				http.Error(rw, "event summary exceeds delivery limit", http.StatusRequestEntityTooLarge)
				return
			}
			more = true
			break
		}
		events = append(events, b)
		size += needed
		cursor = supportEventCursor(e)
	}
	b, err := json.Marshal(events)
	if err != nil {
		http.Error(rw, "events unavailable", http.StatusServiceUnavailable)
		return
	}
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set(supportwire.EventCursorHeader, cursor)
	if more {
		rw.Header().Set(supportwire.EventMoreHeader, "1")
	}
	_, _ = fmt.Fprintf(rw, "%s%s%s%s", gapFrame, prefix, b, suffix)
}

func (w *webMux) supportRulePreview(rw http.ResponseWriter, r *http.Request) {
	var body struct {
		ActionID string `json:"action_id"`
		Verdict  string `json:"verdict"`
		Scope    string `json:"scope"`
	}
	if provider.DecodeStrict(r.Body, 16384, &body) != nil || body.ActionID == "" {
		http.Error(rw, "invalid preview", 400)
		return
	}
	ev, err := w.loadAction(body.ActionID)
	if err != nil {
		http.Error(rw, "action unavailable", 404)
		return
	}
	safe := supportEvent(ev)
	rule, err := GenerateRuleFromEvent(w.g.Policy(), &safe, RuleGenOptions{Verdict: body.Verdict, Scope: body.Scope})
	if err != nil {
		http.Error(rw, "preview unavailable for redacted action", 400)
		return
	}
	rule.DashboardConfigWrites = false
	b, err := json.Marshal(rule)
	if err != nil {
		http.Error(rw, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_, _ = bytes.NewReader(b).WriteTo(rw)
}
