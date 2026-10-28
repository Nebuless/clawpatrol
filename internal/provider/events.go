package provider

import (
	"encoding/hex"
	"github.com/Nebuless/nebprx/internal/supportwire"
	"net/http"
	"time"
)

// Each bounded delivery repeats all identity, assignment and local gateway
// admission/audit checks. No positive identity cache or connector write retries.
func (s *Server) eventStream(w http.ResponseWriter, r *http.Request, org, id, session, hash, caseID, origin string) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	end := func() {
		// Terminal browser state must not revive when an IdP/outage recovers.
		_, _ = s.Store.db.Exec("DELETE FROM support_routes WHERE session_id=? AND provider_session_hash=?", session, hash)
		_, _ = w.Write([]byte("event: session-ended\ndata: {}\n\n"))
		f.Flush()
	}
	target := r.URL.RequestURI()
	burst := 0
	for {
		var current int
		if s.Store.db.QueryRow("SELECT count(*) FROM support_routes WHERE session_id=? AND host=? AND provider_session_hash=? AND expires>?", session, r.Host, hash, time.Now().Unix()).Scan(&current) != nil || current != 1 {
			end()
			return
		}
		actor, err := s.Auth.authenticateHash(r.Context(), hash)
		if err != nil {
			end()
			return
		}
		if _, err = s.Store.InstallationFor(actor.ID, org, id); err != nil {
			end()
			return
		}
		c, err := s.Store.CurrentCase(org, id, caseID)
		if err != nil || c.TechnicianID != actor.ID || c.State != "assigned" {
			end()
			return
		}
		frame, err := s.dashboard(r.Context(), org, id, supportwire.Envelope{TechnicianID: actor.ID, TechnicianName: actor.Name, SessionID: session, CaseID: c.ID, Revision: c.Revision, Method: "GET", Target: target, Origin: origin}, nil, nil)
		if err != nil || frame.Status != 200 || len(frame.Body) > supportwire.MaxEventBatch {
			end()
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err = w.Write(frame.Body); err != nil {
			return
		}
		f.Flush()
		next := *r.URL
		query := next.Query()
		if cursor := frame.Headers.Get(supportwire.EventCursorHeader); cursor != "" {
			if b, err := hex.DecodeString(cursor); err != nil || len(b) != 32 {
				end()
				return
			}
			query.Set("after", cursor)
		}
		next.RawQuery = query.Encode()
		target = next.RequestURI()
		if frame.Headers.Get(supportwire.EventMoreHeader) == "1" && burst < 8 {
			burst++
			continue
		}
		burst = 0
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
