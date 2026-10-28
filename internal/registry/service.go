package registry

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config defines a registry role, transport and refresh settings.
type Config struct {
	Mode, Dir, Listen, TLSCert, TLSKey string
	AllowHTTP                          bool
	Sources                            []Source
	Remote                             Remote
	InventoryPath                      string
	Interval                           time.Duration
}

func loopbackHost(host string) bool { ip := net.ParseIP(host); return ip != nil && ip.IsLoopback() }

// Defaults fills role-specific listen and refresh settings.
func (c *Config) Defaults() {
	if c.Listen == "" {
		if c.Mode == "server" {
			c.Listen = "127.0.0.1:8091"
		} else {
			c.Listen = "127.0.0.1:8092"
		}
	}
	if c.Interval == 0 {
		c.Interval = time.Hour
	}
}

// Validate checks role isolation, TLS requirements and refresh bounds.
func (c Config) Validate() error {
	if c.Mode != "server" && c.Mode != "client" {
		return errors.New("mode must be server or client")
	}
	if c.Dir == "" {
		return errors.New("state directory required")
	}
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return errors.New("listen must be IP:port")
	}
	n, e := strconv.Atoi(port)
	if net.ParseIP(host) == nil || e != nil || n < 0 || n > 65535 {
		return errors.New("listen must be literal IP and valid port")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("TLS certificate and key both required")
	}
	if c.TLSCert == "" && (!c.AllowHTTP || !loopbackHost(host)) {
		return errors.New("TLS required except explicit loopback HTTP development")
	}
	if c.Interval < time.Minute || c.Interval > 24*time.Hour {
		return errors.New("interval must be between 1m and 24h")
	}
	if c.Mode == "server" {
		if c.Remote.URL != "" || c.Remote.Token != "" || c.Remote.PublicKey != "" || c.InventoryPath != "" {
			return errors.New("server cannot use inventory or client upstream")
		}
		return ValidateSources(c.Sources)
	}
	if len(c.Sources) > 0 {
		return errors.New("client cannot ingest global sources")
	}
	if c.Remote.URL != "" {
		return c.Remote.Validate()
	}
	if c.Remote.Token != "" || c.Remote.PublicKey != "" || c.Remote.Installation != "" {
		return errors.New("client upstream required with remote credentials/trust")
	}
	return nil
}

// Service owns the registry store, HTTP listener and refresh workers.
type Service struct {
	Store    *Store
	Addr     net.Addr
	Errors   chan error
	server   *http.Server
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	once     sync.Once
	closeErr error
}

// LoadInventory decodes an owner-only inventory file.
func LoadInventory(path string) (Inventory, error) {
	var in Inventory
	b, err := ReadSecret(path)
	if err != nil {
		return in, err
	}
	if len(b) > MaxRecord {
		return in, errors.New("inventory too large")
	}
	err = strictJSON(b, &in)
	return in, err
}

// Start validates configuration and starts the registry listener and refresh workers.
func Start(parent context.Context, c Config) (*Service, error) {
	c.Defaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var tlsConfig *tls.Config
	if c.TLSCert != "" {
		if err := secureFile(c.TLSKey); err != nil {
			return nil, err
		}
		cert, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey)
		if err != nil {
			return nil, err
		}
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}}
	}
	store, err := Open(c.Dir, c.Mode)
	if err != nil {
		return nil, err
	}
	if c.InventoryPath != "" {
		in, e := LoadInventory(c.InventoryPath)
		if e == nil {
			e = store.SetInventory(in)
		}
		if e != nil {
			return nil, errors.Join(e, store.Close())
		}
	}
	ln, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return nil, errors.Join(err, store.Close())
	}
	if tlsConfig != nil {
		ln = tls.NewListener(ln, tlsConfig)
	}
	ctx, cancel := context.WithCancel(parent)
	svc := &Service{Store: store, Addr: ln.Addr(), Errors: make(chan error, 1), cancel: cancel}
	svc.server = &http.Server{Handler: store.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: time.Minute, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	svc.wg.Add(2)
	go func() {
		defer svc.wg.Done()
		if err := svc.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case svc.Errors <- err:
			default:
			}
		}
	}()
	go func() {
		defer svc.wg.Done()
		ticker := time.NewTicker(c.Interval)
		defer ticker.Stop()
		refresh := func() {
			if c.Mode == "server" && len(c.Sources) > 0 {
				if err := store.Refresh(ctx, c.Sources); err != nil && ctx.Err() == nil {
					log.Printf("registry: upstream refresh failed; previous data retained")
				}
			}
			if c.Mode == "client" && c.Remote.URL != "" {
				if c.InventoryPath != "" {
					in, e := LoadInventory(c.InventoryPath)
					if e == nil {
						e = store.SetInventory(in)
					}
					if e != nil {
						log.Printf("registry: inventory reload failed; sync paused")
						return
					}
				}
				if err := store.Sync(ctx, c.Remote); err != nil && ctx.Err() == nil {
					log.Printf("registry: sync failed; last verified data retained")
				}
			}
		}
		refresh()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh()
			}
		}
	}()
	return svc, nil
}

// Close stops workers and closes the listener and database once.
func (s *Service) Close(ctx context.Context) error {
	s.once.Do(func() {
		s.cancel()
		s.closeErr = s.server.Shutdown(ctx)
		if s.closeErr != nil {
			s.closeErr = errors.Join(s.closeErr, s.server.Close())
		}
		s.wg.Wait()
		s.closeErr = errors.Join(s.closeErr, s.Store.Close())
	})
	return s.closeErr
}
func (s *Store) authorized(r *http.Request, installation string, local bool) bool {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") || len(value) > 300 {
		return false
	}
	hash := sha256.Sum256([]byte(strings.TrimPrefix(value, "Bearer ")))
	var expected []byte
	if local {
		t, err := s.LocalToken()
		if err != nil {
			return false
		}
		h := sha256.Sum256([]byte(t))
		expected = h[:]
	} else {
		err := s.db.QueryRowContext(r.Context(), "SELECT hash FROM credentials WHERE installation=?", installation).Scan(&expected)
		if err != nil {
			expected = make([]byte, sha256.Size)
		}
	}
	return len(expected) == sha256.Size && subtle.ConstantTimeCompare(hash[:], expected) == 1
}
func reply(w http.ResponseWriter, status int, v any) {
	// JSON-only response, HTML-escaped by encoding/json and served with nosniff.
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "response encoding failed", 500)
		return
	}
	if len(b) > 2*MaxSnapshot {
		http.Error(w, "response too large", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(append(b, '\n'))
}
func requestJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRecord)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return strictJSON(b, v)
}

// Handler serves bounded authenticated registry requests.
func (s *Store) Handler() http.Handler {
	slots := make(chan struct{}, 8)
	// Global bounded concurrency and token bucket contain unauthenticated work.
	var mu sync.Mutex
	tokens := float64(32)
	last := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		now := time.Now()
		tokens += now.Sub(last).Seconds() * 8
		if tokens > 32 {
			tokens = 32
		}
		last = now
		allowed := tokens >= 1
		if allowed {
			tokens--
		}
		mu.Unlock()
		if !allowed {
			http.Error(w, "rate limit", http.StatusTooManyRequests)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("Origin") != "" {
			http.Error(w, "browser origins not allowed", http.StatusForbidden)
			return
		}
		if s.Mode == "server" && r.URL.Path == "/v1/snapshot" && r.Method == http.MethodPost {
			var req SyncRequest
			if err := requestJSON(w, r, &req); err != nil {
				http.Error(w, "invalid scope request", 400)
				return
			}
			if !validInstallation(req.Installation) || !s.authorized(r, req.Installation, false) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if _, err := validateScopes(req.Packages); err != nil {
				http.Error(w, "invalid scopes", 400)
				return
			}
			env, err := s.Publish(r.Context(), req, time.Now().UTC())
			if err != nil {
				http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
				return
			}
			reply(w, 200, env)
			return
		}
		if !s.authorized(r, "", true) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/v1/status" && r.Method == http.MethodGet:
			status, err := s.Status(r.Context())
			if err != nil {
				http.Error(w, "status unavailable", http.StatusServiceUnavailable)
				return
			}
			reply(w, 200, status)
		case s.Mode == "client" && r.URL.Path == "/v1/check" && r.Method == http.MethodPost:
			var p struct {
				Identity
				Version string `json:"version"`
				Public  bool   `json:"public"`
			}
			if err := requestJSON(w, r, &p); err != nil {
				http.Error(w, "invalid package", 400)
				return
			}
			result, err := s.Check(Package{Identity: p.Identity, Version: p.Version, Public: p.Public}, time.Now())
			if err != nil {
				http.Error(w, "invalid package or unavailable local data", 400)
				return
			}
			reply(w, 200, result)
		default:
			http.Error(w, "not found", 404)
		}
	})
}
