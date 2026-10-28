package registry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Register the SQLite driver used by registry stores.
)

// Store persists one registry role and its local state.
type Store struct {
	db   *sql.DB
	Mode string
	Dir  string
	mu   sync.Mutex
}

// Open initializes an owner-only registry store for the specified role.
func Open(dir, mode string) (*Store, error) {
	if mode != "server" && mode != "client" {
		return nil, errors.New("mode must be server or client")
	}
	if dir == "" {
		return nil, errors.New("state directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("state directory must be owner-only, not a symlink")
	}
	path := filepath.Join(dir, "registry.db")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		err = f.Close()
	} else if errors.Is(err, os.ErrExist) {
		err = secureFile(path)
	}
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Mode: mode, Dir: dir}
	fail := func(e error) (*Store, error) { return nil, errors.Join(e, db.Close()) }
	// DELETE journal avoids long-lived WAL sidecars. FULL keeps published revisions durable.
	if _, err = db.Exec("PRAGMA busy_timeout=5000; PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL;"); err != nil {
		return fail(err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fail(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY,value BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS records (ecosystem TEXT NOT NULL,id TEXT NOT NULL,body BLOB NOT NULL,PRIMARY KEY(ecosystem,id));
 CREATE TABLE IF NOT EXISTS bindings (ecosystem TEXT NOT NULL,name TEXT NOT NULL,id TEXT NOT NULL,PRIMARY KEY(ecosystem,name,id));
 CREATE TABLE IF NOT EXISTS scopes (key TEXT PRIMARY KEY,body BLOB NOT NULL,expires TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS credentials (installation TEXT PRIMARY KEY,hash BLOB NOT NULL);`); err != nil {
		return fail(err)
	}
	var role string
	err = tx.QueryRow("SELECT value FROM meta WHERE key='role'").Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.Exec("INSERT INTO meta(key,value) VALUES('role',?),('schema','1'),('revision','0'),('sequence','0')", mode)
		role = mode
	}
	if err != nil {
		return fail(err)
	}
	if role != mode {
		return fail(errors.New("registry role mismatch"))
	}
	var schema string
	if err = tx.QueryRow("SELECT value FROM meta WHERE key='schema'").Scan(&schema); err != nil {
		return fail(err)
	}
	if schema != "1" {
		return fail(errors.New("unsupported registry schema"))
	}
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	if mode == "server" {
		if _, err = s.signingKey(); err != nil {
			return fail(err)
		}
	}
	if _, err = s.LocalToken(); err != nil {
		return fail(err)
	}
	return s, nil
}

// Close releases the registry database.
func (s *Store) Close() error { return s.db.Close() }
func secureFile(path string) error {
	i, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 {
		return errors.New("credential/state file must be owner-only regular file")
	}
	return nil
}

// ReadSecret reads a bounded owner-only regular file.
func ReadSecret(path string) ([]byte, error) {
	if err := secureFile(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, MaxRecord+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxRecord {
		return nil, errors.New("private file size limit")
	}
	return b, nil
}

// WriteSecret exclusively creates and syncs an owner-only secret file.
func WriteSecret(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}
func (s *Store) signingKey() (ed25519.PrivateKey, error) {
	path := filepath.Join(s.Dir, "signing.key")
	b, err := ReadSecret(path)
	if errors.Is(err, os.ErrNotExist) {
		_, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return nil, e
		}
		if e = WriteSecret(path, key); e != nil {
			return nil, e
		}
		b = key
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid signing key")
	}
	return ed25519.PrivateKey(b), nil
}

// PublicKey returns the server signing public key in base64.
func (s *Store) PublicKey() (string, error) {
	if err := requireMode(s, "server"); err != nil {
		return "", err
	}
	k, err := s.signingKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey)), nil
}
func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// LocalToken loads or initializes the local API bearer token.
func (s *Store) LocalToken() (string, error) {
	path := filepath.Join(s.Dir, "local.token")
	b, err := ReadSecret(path)
	if errors.Is(err, os.ErrNotExist) {
		t, e := token()
		if e != nil {
			return "", e
		}
		if e = WriteSecret(path, []byte(t)); e != nil {
			return "", e
		}
		return t, nil
	}
	if err != nil {
		return "", err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(b))
	if err != nil || len(decoded) != 32 {
		return "", errors.New("invalid local API credential")
	}
	return string(b), nil
}

// Enroll replaces an installation's previous credential. Plaintext exists only in output file.
func (s *Store) Enroll(installation, out string) error {
	if s.Mode != "server" || !validInstallation(installation) {
		return errors.New("server and valid installation ID required")
	}
	t, err := token()
	if err != nil {
		return err
	}
	if err = WriteSecret(out, []byte(t)); err != nil {
		return err
	}
	h := sha256.Sum256([]byte(t))
	_, err = s.db.Exec("INSERT INTO credentials(installation,hash) VALUES(?,?) ON CONFLICT(installation) DO UPDATE SET hash=excluded.hash", installation, h[:])
	return err
}
func validInstallation(id string) bool {
	return len(id) > 0 && len(id) <= 100 && plainName.MatchString(id) && !containsSlash(id)
}
func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}
func (s *Store) get(key string, v any) error {
	return scanMeta(s.db.QueryRow("SELECT value FROM meta WHERE key=?", key), v)
}
func scanMeta(row *sql.Row, v any) error {
	var b []byte
	if err := row.Scan(&b); err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func put(tx *sql.Tx, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, b)
	return err
}
func number(tx *sql.Tx, key string) (int64, error) {
	var n int64
	err := tx.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&n)
	return n, err
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// SetInventory validates and atomically replaces local inventory.
func (s *Store) SetInventory(in Inventory) error {
	if s.Mode != "client" || len(in.Packages) > MaxPackages {
		return errors.New("client inventory limited to 256 packages")
	}
	visibility := map[string]bool{}
	for i, p := range in.Packages {
		q, err := validatePackage(p)
		if err != nil {
			return err
		}
		in.Packages[i] = q
		if prior, ok := visibility[q.key()]; ok && prior != q.Public {
			return errors.New("conflicting public/private package identities")
		}
		visibility[q.key()] = q.Public
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = put(tx, "inventory", in); err != nil {
		return err
	}
	return tx.Commit()
}

// Inventory loads the current local inventory.
func (s *Store) Inventory() (Inventory, error) {
	var in Inventory
	err := s.get("inventory", &in)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return in, err
}

// Subscriptions returns public identities eligible for upstream disclosure.
func (s *Store) Subscriptions() ([]Identity, error) {
	in, err := s.Inventory()
	if err != nil {
		return nil, err
	}
	var candidates []Identity
	err = s.get("candidates", &candidates)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	ids := map[string]Identity{}
	private := map[string]bool{}
	for _, p := range in.Packages {
		if p.Public {
			ids[p.key()] = p.Identity
		} else {
			private[p.key()] = true
		}
	}
	for _, p := range candidates {
		if !private[p.key()] {
			ids[p.key()] = p
		}
	}
	if len(ids) > MaxPackages {
		return nil, errors.New("subscription limit reached")
	}
	out := make([]Identity, 0, len(ids))
	for _, id := range ids {
		out = append(out, id)
	}
	sortIdentities(out)
	return out, nil
}

// Subscribe adds a public candidate without overriding private inventory.
func (s *Store) Subscribe(p Package) error {
	if err := requireMode(s, "client"); err != nil {
		return err
	}
	p, err := validatePackage(p)
	if err != nil {
		return err
	}
	if !p.Public {
		return errors.New("private candidates never subscribe")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var in Inventory
	if err = scanMeta(tx.QueryRow("SELECT value FROM meta WHERE key='inventory'"), &in); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	for _, q := range in.Packages {
		if q.Identity == p.Identity && !q.Public {
			return errors.New("identity marked private in inventory")
		}
	}
	var ids []Identity
	err = scanMeta(tx.QueryRow("SELECT value FROM meta WHERE key='candidates'"), &ids)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if slices.Contains(ids, p.Identity) {
		return nil
	}
	if len(ids) >= 64 {
		return errors.New("candidate limit reached (64)")
	}
	ids = append(ids, p.Identity)
	if err = put(tx, "candidates", ids); err != nil {
		return err
	}
	return tx.Commit()
}

// Status reports role-specific health and local findings.
func (s *Store) Status(ctx context.Context) (map[string]any, error) {
	out := map[string]any{"mode": s.Mode, "schema": 1}
	var rev, seq int64
	if err := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='revision'").Scan(&rev); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='sequence'").Scan(&seq); err != nil {
		return nil, err
	}
	out["revision"] = rev
	out["sequence"] = seq
	if s.Mode == "server" {
		health := []Health{}
		for _, eco := range []string{"npm", "PyPI", "crates.io", "Go"} {
			h := Health{Ecosystem: eco, State: "bootstrapping"}
			err := s.get("health/"+eco, &h)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			health = append(health, h)
		}
		out["sources"] = health
	} else {
		in, err := s.Inventory()
		if err != nil {
			return nil, err
		}
		out["graph_complete"] = in.GraphComplete
		out["inventory_packages"] = len(in.Packages)
		attempt := make(map[string]any)
		if e := s.get("sync_attempt", &attempt); e == nil {
			out["sync_attempt"] = attempt
		} else if !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		last := make(map[string]any)
		if err = s.get("last_sync", &last); err == nil {
			out["last_sync"] = last
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		results := []Result{}
		for _, p := range in.Packages {
			r, e := s.Check(p, time.Now())
			if e != nil {
				return nil, e
			}
			results = append(results, r)
		}
		out["findings"] = results
	}
	return out, nil
}
func requireMode(s *Store, mode string) error {
	if s.Mode != mode {
		return fmt.Errorf("operation requires %s registry", mode)
	}
	return nil
}
