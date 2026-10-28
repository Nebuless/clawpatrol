package provider

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
)

const providerCookie = "__Host-prx_provider"
const supportCookie = "__Host-prx_support"

func setCookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
func (s *Server) providerHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__provider/oidc/login" && r.Method == "GET" {
		location, browser, err := s.Auth.Begin(r.Context())
		if err != nil {
			http.Error(w, "login unavailable", http.StatusServiceUnavailable)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "__Host-prx_login", Value: browser, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 300})
		http.Redirect(w, r, location, http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/__provider/oidc/callback" && r.Method == "GET" {
		token, err := s.Auth.Callback(r.Context(), r.URL.Query().Get("state"), cookieValue(r, "__Host-prx_login"), r.URL.Query().Get("code"))
		setCookie(w, "__Host-prx_login", "", -1)
		if err != nil {
			http.Error(w, "login unavailable", http.StatusForbidden)
			return
		}
		setCookie(w, providerCookie, token, 1800)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.Method != "GET" && r.Header.Get("Origin") != s.PublicURL {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	identity, err := s.Auth.Authenticate(r.Context(), cookieValue(r, providerCookie))
	if err != nil {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<!doctype html><title>PRX Provider</title><h1>PRX Provider</h1><a href='/__provider/oidc/login'>Sign in with technician MFA</a>")
			return
		}
		http.Error(w, "provider authentication required", http.StatusUnauthorized)
		return
	}
	if r.URL.Path == "/__provider/logout" && r.Method == "POST" {
		if s.Auth.Logout(identity.SessionHash) != nil {
			http.Error(w, "logout unavailable", http.StatusServiceUnavailable)
			return
		}
		setCookie(w, providerCookie, "", -1)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/" && r.Method == "GET" {
		http.Redirect(w, r, "/__provider/ui", http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/__provider/session" && r.Method == "GET" {
		jsonResponse(w, identity.Technician)
		return
	}
	if r.URL.Path == "/__provider/technicians" && r.Method == "GET" {
		rows, e := s.Store.db.Query("SELECT id,name,administrator FROM technicians WHERE enabled=1 AND id IN (SELECT technician_id FROM organization_assignments WHERE organization_id IN (SELECT organization_id FROM organization_assignments WHERE technician_id=?))", identity.ID)
		if e != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() { _ = rows.Close() }()
		out := []Technician{}
		for rows.Next() {
			var t Technician
			if rows.Scan(&t.ID, &t.Name, &t.Administrator) != nil {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			out = append(out, t)
		}
		if rows.Err() != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		jsonResponse(w, out)
		return
	}
	if r.URL.Path == "/__provider/register" && r.Method == "POST" {
		var v Installation
		if DecodeStrict(r.Body, 16384, &v) != nil || !identity.Administrator {
			http.Error(w, "registration denied", http.StatusForbidden)
			return
		}
		orgs, e := s.Store.Organizations(identity.Technician)
		allowed := false
		for _, org := range orgs {
			if org.ID == v.OrganizationID && org.Enabled {
				allowed = true
			}
		}
		if e != nil || !allowed || s.Store.register(identity.Technician, v) != nil {
			http.Error(w, "registration unavailable", http.StatusForbidden)
			return
		}
		jsonResponse(w, map[string]bool{"ok": true})
		return
	}
	if r.URL.Path == "/__provider/ui" && r.Method == "GET" {
		s.managementPage(w, r, identity)
		return
	}
	if r.Method == "GET" && (strings.HasPrefix(r.URL.Path, "/_a/") || strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/fonts/") || r.URL.Path == "/prx-icon.svg") {
		s.asset(w, r)
		return
	}
	org, id := r.URL.Query().Get("organization"), r.URL.Query().Get("installation")
	if r.URL.Path == "/__provider/organization" {
		if r.Method == "GET" {
			var result Organization
			err := s.Store.db.QueryRow("SELECT o.id,o.name,o.enabled FROM organizations o JOIN organization_assignments a ON a.organization_id=o.id WHERE o.id=? AND a.technician_id=?", org, identity.ID).Scan(&result.ID, &result.Name, &result.Enabled)
			if err != nil {
				http.Error(w, "organization unavailable", http.StatusForbidden)
				return
			}
			jsonResponse(w, result)
			return
		}
		if r.Method == "POST" || r.Method == "PATCH" {
			var v Organization
			if DecodeStrict(r.Body, 4096, &v) != nil || r.Method == "PATCH" && v.ID != org || s.Store.manageOrganization(identity.Technician, v.ID, v.Name, v.Enabled, r.Method == "POST") != nil {
				http.Error(w, "organization update unavailable", http.StatusForbidden)
				return
			}
			jsonResponse(w, map[string]bool{"ok": true})
			return
		}
	}
	if r.URL.Path == "/__provider/organization-assign" && r.Method == "POST" {
		var v struct{ Technician string }
		if DecodeStrict(r.Body, 4096, &v) != nil || s.Store.assignOrganization(identity.Technician, org, v.Technician) != nil {
			http.Error(w, "assignment unavailable", http.StatusForbidden)
			return
		}
		jsonResponse(w, map[string]bool{"ok": true})
		return
	}
	if r.URL.Path == "/__provider/organizations" && r.Method == "GET" {
		v, e := s.Store.Organizations(identity.Technician)
		if e != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		jsonResponse(w, v)
		return
	}
	if r.URL.Path == "/__provider/installations" && r.Method == "GET" {
		v, e := s.Store.Installations(identity.Technician, org)
		if e != nil {
			http.Error(w, "unavailable", http.StatusForbidden)
			return
		}
		jsonResponse(w, v)
		return
	}
	if r.URL.Path == "/__provider/installation" && r.Method == "GET" {
		h, err := s.installationHealth(r.Context(), identity.Technician, org, id)
		if err != nil {
			http.Error(w, "installation unavailable", http.StatusForbidden)
			return
		}
		if err = s.Store.mutate(identity.Technician, org, id, "installation_health", func(*sql.Tx) error { return nil }); err != nil {
			http.Error(w, "audit unavailable", http.StatusServiceUnavailable)
			return
		}
		jsonResponse(w, h)
		return
	}
	if r.URL.Path == "/__provider/installation" && r.Method == "POST" {
		var v struct {
			Name    string
			Enabled bool
		}
		if DecodeStrict(r.Body, 16384, &v) != nil || v.Name == "" || s.Store.Manage(identity.Technician, org, id, v.Name, v.Enabled) != nil {
			http.Error(w, "update unavailable", http.StatusForbidden)
			return
		}
		jsonResponse(w, map[string]bool{"ok": true})
		return
	}
	if _, err = s.Store.InstallationFor(identity.ID, org, id); err != nil {
		http.Error(w, "installation unavailable", http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	case "/__provider/cases":
		if r.Method == "GET" {
			v, e := s.Store.Cases(identity.Technician, org, id)
			if e != nil {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			jsonResponse(w, v)
			return
		}
	case "/__provider/assign":
		if r.Method == "POST" {
			var v struct{ Case, Technician string }
			if DecodeStrict(r.Body, 16384, &v) != nil || s.Store.AssignCase(identity.Technician, org, id, v.Case, v.Technician) != nil {
				http.Error(w, "assignment unavailable", http.StatusForbidden)
				return
			}
			jsonResponse(w, map[string]bool{"ok": true})
			return
		}
	case "/__provider/redeem":
		if r.Method == "POST" {
			s.redeemHTTP(w, r, identity, org, id)
			return
		}
	case "/__provider/view-as":
		if r.Method == "POST" {
			r.Body = http.MaxBytesReader(w, r.Body, 16384)
			if r.ParseForm() != nil || len(r.PostForm["case"]) != 1 || len(r.PostForm["code"]) != 1 {
				http.Error(w, "invalid request", 400)
				return
			}
			body, _ := json.Marshal(map[string]string{"Case": r.PostForm.Get("case"), "Code": r.PostForm.Get("code")})
			r.Body = io.NopCloser(bytes.NewReader(body))
			s.redeemHTTP(w, r, identity, org, id)
			return
		}
	}
	http.Error(w, "route unavailable", 404)
}

var handoffPage = template.Must(template.New("handoff").Parse("<!doctype html><html><head><title>PRX support activation</title><meta name='referrer' content='strict-origin'></head><body><h1>Open approved support session</h1><form method='post' action='{{.Origin}}/__support/activate'><input type='hidden' name='handoff' value='{{.Token}}'><button type='submit'>Open customer dashboard</button></form></body></html>"))

func (s *Server) redeemHTTP(w http.ResponseWriter, r *http.Request, i Identity, org, id string) {
	var v struct{ Case, Code string }
	if DecodeStrict(r.Body, 16384, &v) != nil || time.Now().Unix()-i.AuthTime > 300 {
		http.Error(w, "recent MFA and phone approval required", http.StatusForbidden)
		return
	}
	c, err := s.Store.CurrentCase(org, id, v.Case)
	if err != nil || c.State != "assigned" || c.TechnicianID != i.ID {
		http.Error(w, "grant unavailable", http.StatusForbidden)
		return
	}
	host := strings.ToLower(randomToken()[:32])
	host = strings.ReplaceAll(strings.ReplaceAll(host, "_", "a"), "-", "b") + "." + s.SupportDomain
	origin := "https://" + host
	body, _ := json.Marshal(map[string]string{"Case": c.ID, "Code": v.Code, "Origin": origin})
	frame, err := s.dashboard(r.Context(), org, id, supportwire.Envelope{TechnicianID: i.ID, TechnicianName: i.Name, CaseID: c.ID, Revision: c.Revision, Method: "POST", Target: "/__support/redeem", Origin: origin}, body, nil)
	if err != nil || frame.Status != 200 {
		http.Error(w, "grant unavailable", http.StatusForbidden)
		return
	}
	var result struct {
		ID      string
		Expires int64
	}
	if json.Unmarshal(frame.Body, &result) != nil || result.ID == "" {
		http.Error(w, "grant unavailable", http.StatusServiceUnavailable)
		return
	}
	handoff := randomToken()
	err = s.Store.mutate(i.Technician, org, id, "handoff_create", func(tx *sql.Tx) error {
		_, e := tx.Exec("INSERT INTO handoffs(token_hash,session_id,provider_session_hash,organization_id,installation_id,host,expires,case_id) VALUES(?,?,?,?,?,?,?,?)", digest(handoff), result.ID, i.SessionHash, org, id, host, time.Now().Add(30*time.Second).Unix(), c.ID)
		return e
	})
	if err != nil {
		s.cancelSession(r, org, id, result.ID, i, c, origin)
		http.Error(w, "handoff unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = handoffPage.Execute(w, map[string]string{"Origin": origin, "Token": handoff})
}
func (s *Server) cancelSession(r *http.Request, org, id, session string, i Identity, c Case, origin string) {
	_, _ = s.dashboard(r.Context(), org, id, supportwire.Envelope{TechnicianID: i.ID, TechnicianName: i.Name, CaseID: c.ID, Revision: c.Revision, SessionID: session, Method: "POST", Target: "/__support/end", Origin: origin}, nil, nil)
}
func (s *Server) supportHTTP(w http.ResponseWriter, r *http.Request) {
	origin := "https://" + r.Host
	if r.URL.Path == "/__support/activate" {
		if r.Method != "POST" || r.Header.Get("Origin") != s.PublicURL {
			http.Error(w, "activation denied", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		if r.ParseForm() != nil || len(r.PostForm["handoff"]) != 1 {
			http.Error(w, "invalid activation", 400)
			return
		}
		var session, hash, org, id, caseID string
		err := s.Store.db.QueryRow("DELETE FROM handoffs WHERE token_hash=? AND host=? AND expires>? RETURNING session_id,provider_session_hash,organization_id,installation_id,case_id", digest(r.PostForm.Get("handoff")), r.Host, time.Now().Unix()).Scan(&session, &hash, &org, &id, &caseID)
		if err != nil {
			http.Error(w, "activation unavailable", http.StatusForbidden)
			return
		}
		i, err := s.Auth.authenticateHash(r.Context(), hash)
		if err != nil {
			http.Error(w, "activation unavailable", http.StatusForbidden)
			return
		}
		c, err := s.Store.CurrentCase(org, id, caseID)
		if err != nil || c.TechnicianID != i.ID || c.State != "assigned" {
			http.Error(w, "activation unavailable", http.StatusForbidden)
			return
		}
		frame, err := s.dashboard(r.Context(), org, id, supportwire.Envelope{TechnicianID: i.ID, TechnicianName: i.Name, SessionID: session, CaseID: c.ID, Revision: c.Revision, Method: "POST", Target: "/__support/activate", Origin: origin}, nil, nil)
		if err != nil || frame.Status != 200 {
			s.cancelSession(r, org, id, session, i, c, origin)
			http.Error(w, "activation unavailable", http.StatusForbidden)
			return
		}
		token := randomToken()
		err = s.Store.mutate(i.Technician, org, id, "activate", func(tx *sql.Tx) error {
			_, e := tx.Exec("INSERT INTO support_routes VALUES(?,?,?,?,?,?,?,?)", digest(token), session, hash, org, id, caseID, r.Host, min(i.Expires, time.Now().Add(15*time.Minute).Unix()))
			return e
		})
		if err != nil {
			s.cancelSession(r, org, id, session, i, c, origin)
			http.Error(w, "activation unavailable", http.StatusServiceUnavailable)
			return
		}
		setCookie(w, supportCookie, token, 900)
		// Finish on this origin before navigating again. SameSite=Strict cookies
		// are withheld on a redirect chain that began at the Provider origin.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>PRX support ready</title><h1>Support session ready</h1><a href='/'>Continue to customer dashboard</a>")
		return
	}
	if r.Method != "GET" && r.Header.Get("Origin") != origin {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	var session, hash, org, id, caseID string
	err := s.Store.db.QueryRow("SELECT session_id,provider_session_hash,organization_id,installation_id,case_id FROM support_routes WHERE token_hash=? AND host=? AND expires>?", digest(cookieValue(r, supportCookie)), r.Host, time.Now().Unix()).Scan(&session, &hash, &org, &id, &caseID)
	if err != nil {
		if cookieValue(r, supportCookie) != "" {
			setCookie(w, supportCookie, "", -1)
		}
		http.Error(w, "support session ended", http.StatusUnauthorized)
		return
	}
	i, err := s.Auth.authenticateHash(r.Context(), hash)
	if err != nil {
		_, _ = s.Store.db.Exec("DELETE FROM support_routes WHERE session_id=? AND provider_session_hash=?", session, hash)
		http.Error(w, "technician unavailable", http.StatusUnauthorized)
		return
	}
	if _, err = s.Store.InstallationFor(i.ID, org, id); err != nil {
		http.Error(w, "installation unavailable", http.StatusForbidden)
		return
	}
	c, err := s.Store.CurrentCase(org, id, caseID)
	if err != nil || c.State != "assigned" || c.TechnicianID != i.ID {
		http.Error(w, "case unavailable", http.StatusForbidden)
		return
	}
	if supportwire.ValidateTarget(r.Method, r.URL.RequestURI()) != nil {
		http.Error(w, "route denied", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, supportwire.MaxBody+1))
	if err != nil || len(body) > supportwire.MaxBody {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	if r.URL.Path == "/api/events" && r.Method == "GET" {
		s.eventStream(w, r, org, id, session, hash, caseID, origin)
		return
	}
	headers, err := supportwire.SafeHeaders(r.Header)
	if err != nil {
		http.Error(w, "invalid headers", 400)
		return
	}
	frame, err := s.dashboard(r.Context(), org, id, supportwire.Envelope{TechnicianID: i.ID, TechnicianName: i.Name, SessionID: session, CaseID: c.ID, Revision: c.Revision, Method: r.Method, Target: r.URL.RequestURI(), Origin: origin}, body, headers)
	if err != nil {
		http.Error(w, "support unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.URL.Path == "/__support/end" || frame.Status == 401 {
		setCookie(w, supportCookie, "", -1)
		_, _ = s.Store.db.Exec("DELETE FROM support_routes WHERE session_id=?", session)
	}
	if typ := frame.Headers.Get("Content-Type"); typ != "" {
		w.Header().Set("Content-Type", typ)
	}
	w.WriteHeader(frame.Status)
	_, _ = io.Copy(w, bytes.NewReader(frame.Body))
}
