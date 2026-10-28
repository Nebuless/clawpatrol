// Package provider manages technician authentication and audited support access.
package provider

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
	"github.com/coder/websocket"
)

type connection struct {
	link     *supportwire.Link
	nonce    string
	sequence uint64
	mu       sync.Mutex
}

// Server serves provider management and authenticated support connections.
type Server struct {
	Store         *Store
	Auth          *Authenticator
	Key           ed25519.PrivateKey
	PublicURL     string
	SupportDomain string
	mu            sync.Mutex
	connections   map[string]*connection
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// Origin-only referrers preserve browser POST Origin for strict CSRF checks.
	w.Header().Set("Referrer-Policy", "strict-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	if strings.HasSuffix(r.Host, "."+s.SupportDomain) {
		s.supportHTTP(w, r)
		return
	}
	if "https://"+r.Host != s.PublicURL {
		http.Error(w, "invalid host", 421)
		return
	}
	switch r.URL.Path {
	case "/__connector/enroll":
		s.enrollHTTP(w, r)
	case "/__connector/connect":
		s.connectHTTP(w, r)
	default:
		s.providerHTTP(w, r)
	}
}
func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) enrollHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.Header.Get("Origin") != "" {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	var v struct {
		Organization, Installation, Token string
		Key                               []byte
	}
	if DecodeStrict(r.Body, 16384, &v) != nil {
		http.Error(w, "invalid enrollment", 400)
		return
	}
	credential, err := s.Store.Enroll(v.Organization, v.Installation, v.Token, v.Key)
	if err != nil {
		http.Error(w, "enrollment unavailable", http.StatusForbidden)
		return
	}
	jsonResponse(w, map[string]string{"credential": credential})
}
func (s *Server) connectHTTP(w http.ResponseWriter, r *http.Request) {
	org, id := r.URL.Query().Get("organization"), r.URL.Query().Get("installation")
	if r.Header.Get("Origin") != "" {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	key, err := s.Store.Connector(org, id, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if err != nil {
		http.Error(w, "connector unavailable", http.StatusForbidden)
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(16384)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	nonce := randomToken()
	handshake, cancelHandshake := context.WithTimeout(ctx, 10*time.Second)
	defer cancelHandshake()
	challenge, _ := json.Marshal(map[string]string{"nonce": nonce, "proof": supportwire.Proof(s.Key, "provider", nonce, org, id)})
	if c.Write(handshake, websocket.MessageText, challenge) != nil {
		return
	}
	_, b, err := c.Read(handshake)
	if err != nil {
		return
	}
	var proof struct{ Proof string }
	if json.Unmarshal(b, &proof) != nil || !supportwire.CheckProof(key, proof.Proof, "gateway", nonce, org, id) {
		return
	}
	link := supportwire.NewLink(c, func(_ context.Context, f supportwire.Frame) supportwire.Frame {
		return s.connectorRPC(org, id, f)
	})
	conn := &connection{link: link, nonce: nonce}
	s.mu.Lock()
	if s.connections == nil {
		s.connections = map[string]*connection{}
	}
	old := s.connections[id]
	s.connections[id] = conn
	s.mu.Unlock()
	if old != nil {
		old.link.Close()
	}
	defer func() {
		s.mu.Lock()
		if s.connections[id] == conn {
			delete(s.connections, id)
			_, _ = s.Store.db.Exec("DELETE FROM support_routes WHERE installation_id=?", id)
			_, _ = s.Store.db.Exec("DELETE FROM handoffs WHERE installation_id=?", id)
		}
		s.mu.Unlock()
	}()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ping, cancel := context.WithTimeout(ctx, 10*time.Second)
				err := c.Ping(ping)
				cancel()
				if err != nil {
					link.Close()
					return
				}
				if _, err = s.Store.Connector(org, id, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); err != nil {
					link.Close()
					return
				}
			}
		}
	}()
	_ = link.Run(ctx)
}
func (s *Server) connectorRPC(org, id string, f supportwire.Frame) supportwire.Frame {
	var v struct{ Intent, Account, Summary, Category, Case string }
	if DecodeStrict(strings.NewReader(string(f.Body)), 16384, &v) != nil {
		return supportwire.Frame{Error: "invalid control"}
	}
	var c Case
	var err error
	switch f.Kind {
	case "case_submit":
		c, err = s.Store.SubmitCase(org, id, v.Intent, v.Account, v.Summary, v.Category)
	case "case_current":
		c, err = s.Store.CurrentCase(org, id, v.Case)
	default:
		err = errors.New("unsupported control")
	}
	if err != nil {
		return supportwire.Frame{Error: "case unavailable"}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return supportwire.Frame{Error: "encoding failure"}
	}
	return supportwire.Frame{Body: b}
}
func (s *Server) dashboard(ctx context.Context, org, id string, e supportwire.Envelope, body []byte, headers http.Header) (supportwire.Frame, error) {
	s.mu.Lock()
	c := s.connections[id]
	s.mu.Unlock()
	if c == nil {
		return supportwire.Frame{}, errors.New("connector unavailable")
	}
	// Serialize signing and dispatch so received sequences cannot be reordered.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sequence++
	e.Version = 1
	e.Purpose = "dashboard"
	e.Connection = c.nonce
	e.Sequence = c.sequence
	e.RequestID = randomToken()
	e.Issued = time.Now().Unix()
	e.Expires = e.Issued + 15
	e.OrganizationID = org
	e.InstallationID = id
	e.BodyDigest = supportwire.Hash(body)
	signed, err := supportwire.Sign(s.Key, e)
	if err != nil {
		return supportwire.Frame{}, err
	}
	return c.link.Call(ctx, supportwire.Frame{Kind: "dashboard", Signed: signed, Body: body, Headers: headers})
}
