package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
)

type supportFile struct {
	Enabled         bool   `json:"enabled"`
	Broker          string `json:"broker_url"`
	Origin          string `json:"provider_origin"`
	Domain          string `json:"support_domain"`
	Organization    string `json:"organization_id"`
	Installation    string `json:"installation_id"`
	Hosting         string `json:"hosting"`
	CredentialFile  string `json:"connector_credential_file"`
	KeyFile         string `json:"installation_key_file"`
	ProviderKeyFile string `json:"provider_support_public_key_file"`
	PepperFile      string `json:"code_pepper_file"`
	Retention       int    `json:"audit_retention_days"`
}

func loadSupportFile(path string) (supportFile, error) {
	var c supportFile
	b, e := provider.ReadPrivate(path)
	if e != nil {
		return c, e
	}
	if e = provider.DecodeStrict(bytes.NewReader(b), 16384, &c); e != nil {
		return c, e
	}
	if c.Broker != c.Origin || !validSupportOrigin(c.Broker) || c.Organization == "" || c.Installation == "" || strings.ContainsAny(c.Domain, "/: ") || (c.Hosting != "customer" && c.Hosting != "nebuless") {
		return c, errors.New("invalid support configuration")
	}
	if c.Retention == 0 {
		c.Retention = 90
	}
	if c.Retention < 30 || c.Retention > 365 {
		return c, errors.New("audit retention must be 30..365 days")
	}
	return c, nil
}
func readSupportKey(path string, size int) ([]byte, error) {
	b, err := provider.ReadPrivate(path)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(key) != size {
		return nil, errors.New("invalid base64 key length")
	}
	return key, nil
}
func (g *Gateway) startSupport(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	f, err := loadSupportFile(path)
	if err != nil {
		return err
	}
	if !f.Enabled {
		return nil
	}
	var enabled bool
	if err = g.db.QueryRow("SELECT enabled FROM support_control WHERE singleton=1").Scan(&enabled); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	key, err := readSupportKey(f.KeyFile, ed25519.PrivateKeySize)
	if err != nil {
		return err
	}
	public, err := readSupportKey(f.ProviderKeyFile, ed25519.PublicKeySize)
	if err != nil {
		return err
	}
	pepper, err := readSupportKey(f.PepperFile, 32)
	if err != nil {
		return err
	}
	credential, err := provider.ReadPrivate(f.CredentialFile)
	if err != nil {
		return err
	}
	var installation, org string
	if err = g.db.QueryRow("SELECT id,organization_id FROM installation_identity WHERE singleton=1").Scan(&installation, &org); err != nil {
		return err
	}
	if installation != f.Installation || org != f.Organization {
		return errors.New("support enrollment binding mismatch")
	}
	store, err := newSupportStore(g.db, org, installation, pepper)
	if err != nil {
		return err
	}
	store.domain = f.Domain
	web := &webMux{g: g, onboard: g.onboard}
	store.membership = web.tailnetMembership
	c := &supportConnector{config: supportConfig{Broker: f.Broker, Organization: org, Installation: installation, Credential: strings.TrimSpace(string(credential)), PrivateKey: key, ProviderKey: public, Pepper: pepper}, store: store, gateway: g}
	g.support = c
	ctx, g.supportStop = context.WithCancel(ctx)
	g.supportDone = make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for ctx.Err() == nil {
			store.mu.Lock()
			err := store.drainTunnelRevocations()
			store.mu.Unlock()
			if err == nil {
				err = c.run(ctx)
			}
			if err != nil && ctx.Err() == nil {
				slog.Error("Support connector stopped; admission requires current authority and healthy audit storage")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := store.maintain(f.Retention); err != nil {
					slog.Error("Support retention cleanup failed")
				}
			}
		}
	}()
	go func() {
		workers.Wait()
		_ = store.Close()
		close(g.supportDone)
	}()
	return nil
}
func (s *supportStore) maintain(days int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if days < 30 || days > 365 {
		return errors.New("audit retention must be 30..365 days")
	}
	now := s.now().Unix()
	return s.transaction(func(tx *sql.Tx) error {
		for _, q := range []string{"UPDATE support_sessions SET state='revoked' WHERE expires<=? OR (state='pending_activation' AND pending_expires<=?)", "UPDATE support_grants SET state='revoked' WHERE state='issued' AND expires<=?", "UPDATE support_grants SET code_hash=NULL WHERE state!='issued' AND expires<?", "DELETE FROM support_rate_limits WHERE expires<?", "DELETE FROM support_audit WHERE at<?", "DELETE FROM support_sessions WHERE state='revoked' AND expires<? AND NOT EXISTS(SELECT 1 FROM support_tunnel_revocations WHERE session_id=support_sessions.id)", "DELETE FROM support_grants WHERE state!='issued' AND expires<? AND NOT EXISTS(SELECT 1 FROM support_sessions WHERE grant_id=support_grants.id)", "DELETE FROM support_cases WHERE state='closed' AND expires<? AND NOT EXISTS(SELECT 1 FROM support_sessions WHERE case_id=support_cases.id) AND NOT EXISTS(SELECT 1 FROM support_grants WHERE case_id=support_cases.id)"} {
			var args []any
			switch {
			case strings.HasPrefix(q, "UPDATE support_sessions"):
				args = []any{now, now}
			case strings.HasPrefix(q, "UPDATE support_grants SET state"):
				args = []any{now}
			case strings.HasPrefix(q, "DELETE FROM support_audit"), strings.HasPrefix(q, "DELETE FROM support_sessions"), strings.HasPrefix(q, "DELETE FROM support_grants"), strings.HasPrefix(q, "DELETE FROM support_cases"):
				args = []any{now - int64(days)*86400}
			default:
				args = []any{now - 86400}
			}
			if _, err := tx.Exec(q, args...); err != nil {
				return err
			}
		}
		return nil
	})
}
func runSupport(args []string) {
	if err := supportCommand(args, os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "support:", err)
		os.Exit(1)
	}
}
func supportCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("support", flag.ContinueOnError)
	action := fs.String("action", "status", "enroll|status|enable|disable|revoke-all")
	path := fs.String("config-file", "", "owner-only support configuration")
	secretPath := fs.String("enrollment-secret-file", "", "owner-only one-time enrollment secret (default stdin)")
	state := fs.String("state-dir", "", "gateway state directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *state == "" {
		return errors.New("state-dir required; no positional arguments")
	}
	f, err := loadSupportFile(*path)
	if err != nil {
		return err
	}
	db, err := OpenDB(filepath.Join(*state, "nebprx.db"))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	switch *action {
	case "status":
		var id, org string
		var enabled bool
		err = db.QueryRow("SELECT id,organization_id FROM installation_identity WHERE singleton=1").Scan(&id, &org)
		if err == nil {
			err = db.QueryRow("SELECT enabled FROM support_control WHERE singleton=1").Scan(&enabled)
		}
		if err == nil {
			err = json.NewEncoder(out).Encode(map[string]any{"installation_id": id, "organization_id": org, "configured_enabled": f.Enabled, "admission_enabled": enabled, "enabled_on_next_start": f.Enabled && enabled})
		}
		return err
	case "revoke-all", "disable", "enable":
		var id, org string
		if err = db.QueryRow("SELECT id,organization_id FROM installation_identity WHERE singleton=1").Scan(&id, &org); err != nil {
			return err
		}
		if id != f.Installation || org != f.Organization {
			return errors.New("support configuration binding mismatch")
		}
		store, err := openSupportStore(db, org, id, make([]byte, 32))
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()
		actor := localSupportActor()
		if *action == "enable" {
			if !f.Enabled {
				return errors.New("set enabled=true in the private configuration before explicitly enabling support")
			}
			return store.transaction(func(tx *sql.Tx) error {
				if err := store.auditSupportControl(tx, actor, "support_enable", "local_enable"); err != nil {
					return err
				}
				_, err := tx.Exec("UPDATE support_control SET enabled=1 WHERE singleton=1")
				return err
			})
		}
		return store.revokeAllSupport(actor, "local_"+strings.ReplaceAll(*action, "-", "_"), *action == "disable")
	case "enroll":
		var id, org string
		if err = db.QueryRow("SELECT id,organization_id FROM installation_identity WHERE singleton=1").Scan(&id, &org); err != nil {
			return err
		}
		if id != f.Installation || (org != "" && org != f.Organization) {
			return errors.New("local installation ID differs from registration")
		}
		credential, err := reserveSupportCredential(f.CredentialFile)
		if err != nil {
			return err
		}
		stored := false
		defer func() {
			_ = credential.Close()
			if !stored {
				_ = os.Remove(f.CredentialFile)
			}
		}()
		var secret []byte
		if *secretPath != "" {
			secret, err = provider.ReadPrivate(*secretPath)
		} else {
			secret, err = io.ReadAll(io.LimitReader(os.Stdin, 16385))
		}
		if err != nil || len(secret) > 16384 {
			return errors.New("enrollment secret unavailable")
		}
		key, err := readSupportKey(f.KeyFile, ed25519.PrivateKeySize)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"Organization": f.Organization, "Installation": f.Installation, "Token": strings.TrimSpace(string(secret)), "Key": ed25519.PrivateKey(key).Public()})
		client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect denied") }}
		resp, err := client.Post(f.Broker+"/__connector/enroll", "application/json", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		var result struct {
			Credential string `json:"credential"`
		}
		if resp.StatusCode != 200 || provider.DecodeStrict(resp.Body, 16384, &result) != nil || result.Credential == "" {
			return errors.New("enrollment rejected")
		}
		if _, err = credential.WriteString(result.Credential); err != nil {
			return err
		}
		if err = credential.Sync(); err != nil {
			return err
		}
		if err = credential.Close(); err != nil {
			return err
		}
		stored = true
		_, err = db.Exec("UPDATE installation_identity SET organization_id=? WHERE singleton=1 AND id=?", f.Organization, id)
		return err
	default:
		return errors.New("unknown support action")
	}
}

// Reserve before consuming enrollment. Existing files, links and non-private
// directories are rejected without writing credential bytes or using the code.
func reserveSupportCredential(path string) (*os.File, error) {
	dir, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if !dir.IsDir() || dir.Mode().Perm()&0077 != 0 {
		return nil, errors.New("credential directory must be owner-only and not a symlink")
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}
func ensureSupportSecret(path string, size int) ([]byte, error) {
	b, err := readSupportKey(path, size)
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	b = make([]byte, size)
	if size == ed25519.PrivateKeySize {
		_, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return nil, e
		}
		b = key
	} else if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	file, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, e
	}
	_, e = file.WriteString(base64.StdEncoding.EncodeToString(b))
	closeErr := file.Close()
	if e != nil {
		return nil, e
	}
	return b, closeErr
}
