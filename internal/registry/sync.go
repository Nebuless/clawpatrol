package registry

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Protocol identifies the signed registry snapshot format.
const Protocol = "prx-registry-snapshot-v1"

// SnapshotTTL bounds snapshot validity.
const SnapshotTTL = 6 * time.Hour

// SourceMaxAge bounds the age of source evidence considered fresh.
const SourceMaxAge = 48 * time.Hour

// SyncRequest declares the installation and public package scope.
type SyncRequest struct {
	Installation string     `json:"installation"`
	Packages     []Identity `json:"packages"`
}

// Remote configures the pinned upstream and installation credentials.
type Remote struct {
	URL, Installation, Token, PublicKey string
	AllowHTTP                           bool
}

func sortIdentities(ids []Identity) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].key() < ids[j].key() })
}
func validateScopes(ids []Identity) ([]Identity, error) {
	if len(ids) > MaxPackages {
		return nil, errors.New("scope count exceeds 256")
	}
	out := append([]Identity{}, ids...)
	seen := map[string]bool{}
	for i, id := range out {
		p, err := Canonical(id)
		if err != nil || p != id {
			return nil, errors.New("scope must use canonical supported identity")
		}
		if seen[p.key()] {
			return nil, errors.New("duplicate scope")
		}
		seen[p.key()] = true
		out[i] = p
	}
	sortIdentities(out)
	return out, nil
}

// Publish signs a bounded scope snapshot and advances the publication sequence.
func (s *Store) Publish(ctx context.Context, req SyncRequest, now time.Time) (Envelope, error) {
	var env Envelope
	if err := requireMode(s, "server"); err != nil {
		return env, err
	}
	ids, err := validateScopes(req.Packages)
	if err != nil {
		return env, err
	}
	if !validInstallation(req.Installation) {
		return env, errors.New("invalid installation")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return env, err
	}
	defer func() { _ = tx.Rollback() }()
	seq, err := number(tx, "sequence")
	if err != nil {
		return env, err
	}
	rev, err := number(tx, "revision")
	if err != nil {
		return env, err
	}
	snap := Snapshot{Protocol: Protocol, Installation: req.Installation, Sequence: seq + 1, Revision: rev, Issued: now.UTC(), Expires: now.Add(SnapshotTTL).UTC(), Scopes: []Scope{}}
	totalSize := 0
	for _, id := range ids {
		scope := Scope{Identity: id, Records: []Record{}, Health: Health{Ecosystem: id.Ecosystem, State: "bootstrapping"}}
		var b []byte
		err = tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key=?", "health/"+id.Ecosystem).Scan(&b)
		if err == nil {
			if err = json.Unmarshal(b, &scope.Health); err != nil {
				return env, err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return env, err
		}
		rows, e := tx.QueryContext(ctx, "SELECT r.body FROM records r JOIN bindings b ON r.ecosystem=b.ecosystem AND r.id=b.id WHERE b.ecosystem=? AND b.name=? ORDER BY r.id", id.Ecosystem, id.Name)
		if e != nil {
			return env, e
		}
		defer func() { _ = rows.Close() }()
		size := 0
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				e = errors.Join(e, rows.Close())
				return env, e
			}
			size += len(raw)
			totalSize += len(raw)
			if size > MaxSnapshot || totalSize > MaxSnapshot || len(scope.Records) >= 4096 {
				e = errors.Join(errors.New("scope snapshot too large"), rows.Close())
				return env, e
			}
			var rec Record
			if e = json.Unmarshal(raw, &rec); e != nil {
				e = errors.Join(e, rows.Close())
				return env, e
			}
			scope.Records = append(scope.Records, rec)
		}
		e = errors.Join(rows.Err(), rows.Close())
		if e != nil {
			return env, e
		}
		snap.Scopes = append(snap.Scopes, scope)
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return env, err
	}
	if len(payload) > MaxSnapshot {
		return env, errors.New("snapshot too large")
	}
	key, err := s.signingKey()
	if err != nil {
		return env, err
	}
	env = Envelope{Payload: payload, Signature: ed25519.Sign(key, payload), SHA256: digest(payload)}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='sequence'", seq+1); err != nil {
		return env, err
	}
	if err = tx.Commit(); err != nil {
		return env, err
	}
	return env, nil
}

// Verify validates snapshot integrity, identity, scope and time bounds.
func Verify(env Envelope, key ed25519.PublicKey, installation string, expected []Identity, now time.Time) (Snapshot, error) {
	var snap Snapshot
	if len(key) != ed25519.PublicKeySize || len(env.Payload) > MaxSnapshot || digest(env.Payload) != env.SHA256 || !ed25519.Verify(key, env.Payload, env.Signature) {
		return snap, errors.New("snapshot signature/hash invalid")
	}
	if err := strictJSON(env.Payload, &snap); err != nil {
		return snap, err
	}
	if snap.Protocol != Protocol || snap.Installation != installation || snap.Sequence <= 0 || snap.Revision < 0 {
		return snap, errors.New("wrong snapshot protocol, installation or revision")
	}
	if snap.Issued.IsZero() || snap.Issued.After(now.Add(time.Minute)) || !snap.Expires.After(now) || !snap.Expires.After(snap.Issued) || snap.Expires.Sub(snap.Issued) > SnapshotTTL {
		return snap, errors.New("snapshot expired or invalid time bounds")
	}
	ids, err := validateScopes(expected)
	if err != nil {
		return snap, err
	}
	if len(ids) != len(snap.Scopes) {
		return snap, errors.New("snapshot scope mismatch")
	}
	for i, scope := range snap.Scopes {
		if scope.Identity != ids[i] || scope.Health.Ecosystem != ids[i].Ecosystem {
			return snap, errors.New("snapshot scope mismatch")
		}
		if scope.Health.Checked.After(snap.Issued.Add(time.Minute)) {
			return snap, errors.New("source observation in future")
		}
		switch scope.Health.State {
		case "bootstrapping", "refreshing", "complete", "partial", "failed":
		default:
			return snap, errors.New("invalid source health")
		}
		if scope.Health.State == "complete" && (scope.Health.Checked.IsZero() || scope.Health.Rejected != 0) {
			return snap, errors.New("inconsistent complete source health")
		}
		if len(scope.Records) > 4096 {
			return snap, errors.New("scope record count limit")
		}
		seen := map[string]bool{}
		for _, rec := range scope.Records {
			a, e := decodeAdvisory(rec.Raw)
			if e != nil {
				return snap, e
			}
			if rec.Digest != digest(rec.Raw) || rec.Observed.IsZero() || rec.Observed.After(snap.Issued.Add(time.Minute)) {
				return snap, errors.New("record digest/observation invalid")
			}
			source, license, credit, ok := attribution(a, rec.Raw)
			if !ok || source != rec.Source || license != rec.License || credit != rec.Attribution {
				return snap, errors.New("record attribution mismatch")
			}
			if seen[a.ID] {
				return snap, errors.New("duplicate advisory")
			}
			seen[a.ID] = true
			relevant := false
			for _, affected := range a.Affected {
				p, e := Canonical(affected.Package)
				if e == nil && p == scope.Identity {
					relevant = true
				}
			}
			if !relevant {
				return snap, errors.New("out-of-scope advisory")
			}
		}
	}
	return snap, nil
}
func strictJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

// Activate atomically installs a verified snapshot and advances replay protection.
func (s *Store) Activate(ctx context.Context, env Envelope, remote Remote, ids []Identity, now time.Time) error {
	if err := requireMode(s, "client"); err != nil {
		return err
	}
	if err := remote.Validate(); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(remote.PublicKey)
	if err != nil {
		return errors.New("invalid pinned Ed25519 public key")
	}
	snap, err := Verify(env, ed25519.PublicKey(key), remote.Installation, ids, now)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Pin persists atomically with activation. Key/server/installation changes
	// require explicit new state and out-of-band bootstrap, never implicit TOFU.
	pin := remote.pin()
	var old []byte
	err = tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='trust'").Scan(&old)
	if err == nil && string(old) != pin {
		return errors.New("persisted trust pin mismatch")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	seq, err := number(tx, "sequence")
	if err != nil {
		return err
	}
	rev, err := number(tx, "revision")
	if err != nil {
		return err
	}
	if snap.Sequence <= seq || snap.Revision < rev {
		return errors.New("snapshot rollback/replay rejected")
	}
	for _, scope := range snap.Scopes {
		b, e := json.Marshal(scope)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO scopes(key,body,expires) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET body=excluded.body,expires=excluded.expires", scope.Identity.key(), b, snap.Expires.Format(time.RFC3339Nano)); e != nil {
			return e
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO meta(key,value) VALUES('trust',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", pin); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='sequence'", snap.Sequence); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=? WHERE key='revision'", snap.Revision); err != nil {
		return err
	}
	if err = put(tx, "last_sync", map[string]any{"state": "verified", "received": now, "issued": snap.Issued, "expires": snap.Expires, "sequence": snap.Sequence}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r Remote) pin() string { return r.URL + "\n" + r.Installation + "\n" + r.PublicKey }

// Validate requires a trusted origin, installation credentials and signing key.
func (r Remote) Validate() error {
	u, err := url.Parse(r.URL)
	if err != nil || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return errors.New("server URL must be origin without credentials, query or path")
	}
	if u.Scheme != "https" && (!r.AllowHTTP || u.Scheme != "http" || !loopbackHost(u.Hostname())) {
		return errors.New("verified HTTPS required (explicit loopback HTTP development exception only)")
	}
	key, err := base64.StdEncoding.DecodeString(r.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize || !validInstallation(r.Installation) || len(r.Token) < 32 || len(r.Token) > 256 {
		return errors.New("installation, credential and explicit Ed25519 trust pin required")
	}
	return nil
}

// Sync fetches and activates a scoped snapshot while recording attempt health.
func (s *Store) Sync(ctx context.Context, remote Remote) (err error) {
	if err := requireMode(s, "client"); err != nil {
		return err
	}
	defer func() {
		state := "verified"
		if err != nil {
			state = "failed"
		}
		tx, e := s.db.Begin()
		if e == nil {
			defer func() { _ = tx.Rollback() }()
			e = put(tx, "sync_attempt", map[string]any{"state": state, "at": time.Now().UTC()})
			if e == nil {
				e = tx.Commit()
			}
		}
		err = errors.Join(err, e)
	}()
	if err := remote.Validate(); err != nil {
		return err
	}
	// Check persisted trust before sending credentials or subscription names.
	// Activation rechecks inside its transaction to protect concurrent bootstrap.
	var pin []byte
	if e := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='trust'").Scan(&pin); e == nil {
		if string(pin) != remote.pin() {
			return errors.New("persisted trust pin mismatch")
		}
	} else if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	ids, err := s.Subscriptions()
	if err != nil {
		return err
	}
	body, err := json.Marshal(SyncRequest{Installation: remote.Installation, Packages: ids})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(remote.URL, "/")+"/v1/snapshot", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+remote.Token)
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") }}
	resp, err := hc.Do(req)
	if err != nil {
		return errors.New("registry transport failure (last verified data retained)")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry sync HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2*MaxSnapshot+1))
	if err != nil {
		return err
	}
	if len(b) > 2*MaxSnapshot {
		return errors.New("snapshot envelope too large")
	}
	var env Envelope
	if err = strictJSON(b, &env); err != nil {
		return err
	}
	return s.Activate(ctx, env, remote, ids, time.Now().UTC())
}
