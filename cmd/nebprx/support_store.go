package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/google/uuid"
)

var errGrantUnavailable = errors.New("grant_unavailable")

type supportStore struct {
	db                *sql.DB
	conn              *sql.Conn
	mu                sync.Mutex
	blocked           atomic.Bool
	org, installation string
	pepper            []byte
	domain            string
	membership        func(string, string, string) bool
	now               func() time.Time
}
type supportSession struct {
	ID              string
	GrantID         string
	CaseID          string
	AccountID       string
	AccountName     string
	AccountRevision int64
	TechnicianID    string
	TechnicianName  string
	Revision        int64
	State           string
	Origin          string
	Expires         int64
}

func newSupportStore(db *sql.DB, org, installation string, pepper []byte) (*supportStore, error) {
	s, err := openSupportStore(db, org, installation, pepper)
	if err != nil {
		return nil, err
	}
	if err = s.revokeAllSupport(systemSupportActor, "gateway_restart", false); err != nil {
		return nil, errors.Join(err, s.Close())
	}
	return s, nil
}

func openSupportStore(db *sql.DB, org, installation string, pepper []byte) (*supportStore, error) {
	if db == nil || org == "" || installation == "" || len(pepper) != 32 {
		return nil, errors.New("invalid support store binding")
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil, err
	}
	if _, err = conn.ExecContext(context.Background(), "PRAGMA synchronous=FULL"); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return &supportStore{db: db, conn: conn, org: org, installation: installation, pepper: append([]byte(nil), pepper...), now: time.Now}, nil
}
func (s *supportStore) Close() error { return s.conn.Close() }
func (s *supportStore) transaction(fn func(*sql.Tx) error) error {
	tx, err := s.conn.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *supportStore) audit(tx *sql.Tx, c provider.Case, account, session, grant, operation, outcome, reason string, requestMethod ...string) error {
	method := "RPC"
	if len(requestMethod) > 0 {
		method = requestMethod[0]
	}

	_, err := tx.Exec("INSERT INTO support_audit(id,at,request_id,actor_id,actor_name,actor_issuer,effective_id,effective_name,organization_id,installation_id,case_id,grant_id,session_id,assignment_revision,operation,method,outcome,reason) VALUES(?,?,?,?,?,'provider',?,(SELECT display_name FROM dashboard_accounts WHERE id=?),?,?,?,?,?,?,?,?,?,?)", uuid.NewString(), s.now().Unix(), uuid.NewString(), c.TechnicianID, c.TechnicianName, sql.NullString{String: account, Valid: account != ""}, account, s.org, s.installation, c.ID, grant, session, c.Revision, operation, method, outcome, reason)
	return err
}
func (s *supportStore) mirrorCase(c provider.Case) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.OrganizationID != s.org || c.InstallationID != s.installation || c.ID == "" || c.AccountID == "" {
		return errors.New("case binding mismatch")
	}
	return s.transaction(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE support_sessions SET state='revoked' WHERE case_id=? AND (assignment_revision!=? OR ?!='assigned')", c.ID, c.Revision, c.State); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE support_grants SET state='revoked' WHERE case_id=? AND state='issued' AND (assignment_revision!=? OR ?!='assigned')", c.ID, c.Revision, c.State); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO support_cases VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET technician_id=excluded.technician_id,technician_name=excluded.technician_name,revision=excluded.revision,state=excluded.state,expires=excluded.expires WHERE support_cases.account_id=excluded.account_id", c.ID, s.org, s.installation, c.TechnicianID, c.TechnicianName, c.Revision, c.State, c.Expires, c.AccountID)
		return err
	})
}

const supportAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func normalizeSupportCode(code string) (string, error) {
	code = strings.ToUpper(code)
	if len(code) != 22 || !strings.HasPrefix(code, "SUPPORT-") || code[12] != '-' || code[17] != '-' {
		return "", errGrantUnavailable
	}
	value := strings.ReplaceAll(code[8:], "-", "")
	if len(value) != 12 {
		return "", errGrantUnavailable
	}
	for _, ch := range value {
		if !strings.ContainsRune(supportAlphabet, ch) {
			return "", errGrantUnavailable
		}
	}
	return value, nil
}
func (s *supportStore) codeHash(grant, code string) string {
	h := hmac.New(sha256.New, s.pepper)
	h.Write([]byte(s.installation + "\x00" + grant + "\x00" + code))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *supportStore) enabled() bool {
	if s.blocked.Load() {
		return false
	}
	var enabled bool
	return s.db.QueryRow("SELECT enabled FROM support_control WHERE singleton=1").Scan(&enabled) == nil && enabled
}
func (s *supportStore) caseValid(tx *sql.Tx, c provider.Case) error {
	if s.blocked.Load() {
		return errGrantUnavailable
	}
	var enabled bool
	if err := tx.QueryRow("SELECT enabled FROM support_control WHERE singleton=1").Scan(&enabled); err != nil || !enabled {
		return errGrantUnavailable
	}
	var n int
	if c.State != "assigned" || c.TechnicianID == "" || c.OrganizationID != s.org || c.InstallationID != s.installation || c.Expires <= s.now().Unix() {
		return errGrantUnavailable
	}
	err := tx.QueryRow("SELECT count(*) FROM support_cases WHERE id=? AND organization_id=? AND installation_id=? AND technician_id=? AND revision=? AND state='assigned' AND expires>?", c.ID, s.org, s.installation, c.TechnicianID, c.Revision, s.now().Unix()).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return errGrantUnavailable
	}
	return nil
}
func (s *supportStore) issue(_ context.Context, c provider.Case, a dashboardAccount) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	for i := range b {
		b[i] = supportAlphabet[b[i]&31]
	}
	code := "SUPPORT-" + string(b[:4]) + "-" + string(b[4:8]) + "-" + string(b[8:])
	id := uuid.NewString()
	now := s.now().Unix()
	err := s.transaction(func(tx *sql.Tx) error {
		if c.AccountID != a.ID {
			return errGrantUnavailable
		}
		if err := s.caseValid(tx, c); err != nil {
			return err
		}
		var n int
		if a.Kind == "tailnet" && (s.membership == nil || !s.membership(a.Authority, a.Subject, a.Name)) {
			return errGrantUnavailable
		}
		if err := tx.QueryRow("SELECT count(*) FROM dashboard_accounts a WHERE a.id=? AND a.active=1 AND a.permission_revision=? AND EXISTS(SELECT 1 FROM dashboard_users WHERE username='root')", a.ID, a.Revision).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return errors.New("target eligibility unavailable")
		}
		if _, err := tx.Exec("UPDATE support_sessions SET state='revoked' WHERE account_id=? AND state!='revoked'", a.ID); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE support_grants SET state='revoked' WHERE account_id=? AND state='issued'", a.ID); err != nil {
			return err
		}
		normalized, _ := normalizeSupportCode(code)
		if _, err := tx.Exec("INSERT INTO support_grants VALUES(?,?,?,?,?,?,?,'issued',?,?)", id, c.ID, a.ID, a.Revision, c.TechnicianID, c.Revision, s.codeHash(id, normalized), now, now+300); err != nil {
			return err
		}
		return s.customerAudit(tx, a, c, "", id, "grant_issue", "customer_approved")
	})
	if err != nil {
		return "", "", err
	}
	return code, id, nil
}
func (s *supportStore) redeem(_ context.Context, c provider.Case, tech, code, origin string) (supportSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result supportSession
	now := s.now().Unix()
	u, err := url.Parse(origin)
	if err != nil || !validSupportOrigin(origin) || s.domain == "" || u.Port() != "" || !strings.HasSuffix(u.Hostname(), "."+s.domain) || strings.Contains(strings.TrimSuffix(u.Hostname(), "."+s.domain), ".") {
		return result, errGrantUnavailable
	}
	if c.TechnicianID != tech {
		return result, errGrantUnavailable
	}
	normalized, parseErr := normalizeSupportCode(code)
	denied := false
	err = s.transaction(func(tx *sql.Tx) error {
		if err := s.caseValid(tx, c); err != nil {
			return err
		}
		scope := "challenge:" + tech + ":" + c.ID + ":" + strconv.FormatInt(c.Revision, 10) + ":" + s.installation
		scopes := []string{scope, "technician:" + tech + ":" + s.installation, "installation:" + s.installation}
		limits := []int{5, 10, 30}
		for j, key := range scopes {
			var n int
			if err := tx.QueryRow("SELECT coalesce((SELECT failures FROM support_rate_limits WHERE scope=? AND expires>?),0)", key, now).Scan(&n); err != nil {
				return err
			}
			if n >= limits[j] {
				denied = true
				return s.audit(tx, c, "", "", "", "redeem", "denied", "throttled")
			}
		}
		rows, err := tx.Query("SELECT id,account_id,account_revision,code_hash FROM support_grants WHERE case_id=? AND technician_id=? AND assignment_revision=? AND state='issued' AND expires>?", c.ID, tech, c.Revision, now)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		var grant, account string
		var revision int64
		for rows.Next() {
			var g, a, h string
			var rev int64
			if err = rows.Scan(&g, &a, &rev, &h); err != nil {
				_ = rows.Close()
				return err
			}
			if parseErr == nil && hmac.Equal([]byte(h), []byte(s.codeHash(g, normalized))) {
				grant, account, revision = g, a, rev
			}
		}
		if err = rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
		if grant == "" {
			denied = true
			for _, key := range scopes {
				if _, err := tx.Exec("INSERT INTO support_rate_limits VALUES(?,1,?) ON CONFLICT(scope) DO UPDATE SET failures=CASE WHEN expires<=? THEN 1 ELSE failures+1 END,expires=CASE WHEN expires<=? THEN excluded.expires ELSE expires END", key, now+900, now, now); err != nil {
					return err
				}
			}
			if _, err := tx.Exec("UPDATE support_grants SET state='revoked' WHERE case_id=? AND technician_id=? AND assignment_revision=? AND state='issued' AND (SELECT failures FROM support_rate_limits WHERE scope=?)>=5", c.ID, tech, c.Revision, scope); err != nil {
				return err
			}
			return s.audit(tx, c, "", "", "", "redeem", "denied", "grant_unavailable")
		}
		var n int
		if err := tx.QueryRow("SELECT count(*) FROM dashboard_accounts WHERE id=? AND active=1 AND permission_revision=? AND EXISTS(SELECT 1 FROM dashboard_users WHERE username='root')", account, revision).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return errGrantUnavailable
		}
		if _, err := tx.Exec("UPDATE support_sessions SET state='revoked' WHERE state='pending_activation' AND pending_expires<=?", now); err != nil {
			return err
		}
		res, err := tx.Exec("UPDATE support_grants SET state='consumed' WHERE id=? AND state='issued' AND expires>?", grant, now)
		if err != nil {
			return err
		}
		n64, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n64 != 1 {
			return errGrantUnavailable
		}
		var active int
		if err := tx.QueryRow("SELECT count(*) FROM support_sessions WHERE account_id=? AND state IN ('active','pending_activation')", account).Scan(&active); err != nil {
			return err
		}
		if active != 0 {
			return errGrantUnavailable
		}
		result = supportSession{ID: uuid.NewString(), GrantID: grant, CaseID: c.ID, AccountID: account, AccountRevision: revision, TechnicianID: tech, TechnicianName: c.TechnicianName, Revision: c.Revision, State: "pending_activation", Origin: origin, Expires: now + 900}
		if _, err := tx.Exec("INSERT INTO support_sessions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", result.ID, grant, c.ID, account, revision, tech, c.TechnicianName, c.Revision, "pending_activation", origin, now, now+30, now+900); err != nil {
			return err
		}
		return s.audit(tx, c, account, result.ID, grant, "redeem", "success", "pending_activation")
	})
	if err != nil {
		return supportSession{}, err
	}
	if denied {
		return supportSession{}, errGrantUnavailable
	}
	return result, nil
}
func (s *supportStore) activate(id string, c provider.Case) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sessionMembership(id) {
		return errGrantUnavailable
	}
	return s.transaction(func(tx *sql.Tx) error {
		if err := s.caseValid(tx, c); err != nil {
			return err
		}
		res, err := tx.Exec("UPDATE support_sessions SET state='active' WHERE id=? AND case_id=? AND assignment_revision=? AND technician_id=? AND state='pending_activation' AND pending_expires>? AND expires>? AND EXISTS(SELECT 1 FROM dashboard_accounts a WHERE a.id=support_sessions.account_id AND a.active=1 AND a.permission_revision=support_sessions.account_revision) AND EXISTS(SELECT 1 FROM dashboard_users WHERE username='root')", id, c.ID, c.Revision, c.TechnicianID, s.now().Unix(), s.now().Unix())
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errGrantUnavailable
		}
		var account, grant string
		if err := tx.QueryRow("SELECT account_id,grant_id FROM support_sessions WHERE id=?", id).Scan(&account, &grant); err != nil {
			return err
		}
		return s.audit(tx, c, account, id, grant, "activate", "success", "active")
	})
}
func (s *supportStore) authorize(id string) (supportSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authorizeLocked(id)
}
func (s *supportStore) authorizeLocked(id string) (supportSession, error) {
	var r supportSession
	if s.blocked.Load() {
		return r, errGrantUnavailable
	}
	if !s.sessionMembership(id) {
		return r, errGrantUnavailable
	}
	err := s.conn.QueryRowContext(context.Background(), "SELECT s.id,s.grant_id,s.case_id,s.account_id,a.display_name,s.account_revision,s.technician_id,s.technician_name,s.assignment_revision,s.state,s.origin,s.expires FROM support_sessions s JOIN dashboard_accounts a ON a.id=s.account_id JOIN support_cases c ON c.id=s.case_id WHERE s.id=? AND (SELECT enabled FROM support_control WHERE singleton=1)=1 AND s.state='active' AND s.expires>? AND a.active=1 AND a.permission_revision=s.account_revision AND c.state='assigned' AND c.revision=s.assignment_revision AND c.technician_id=s.technician_id AND c.expires>? AND EXISTS(SELECT 1 FROM dashboard_users WHERE username='root')", id, s.now().Unix(), s.now().Unix()).Scan(&r.ID, &r.GrantID, &r.CaseID, &r.AccountID, &r.AccountName, &r.AccountRevision, &r.TechnicianID, &r.TechnicianName, &r.Revision, &r.State, &r.Origin, &r.Expires)
	return r, err
}
func (s *supportStore) sessionMembership(id string) bool {
	var kind, authority, subject, name string
	if s.conn.QueryRowContext(context.Background(), "SELECT a.auth_kind,a.auth_authority,a.auth_subject,a.display_name FROM dashboard_accounts a JOIN support_sessions s ON s.account_id=a.id WHERE s.id=?", id).Scan(&kind, &authority, &subject, &name) != nil {
		return false
	}
	return kind == "password" || (kind == "tailnet" && s.membership != nil && s.membership(authority, subject, name))
}
func (s *supportStore) revoke(id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transaction(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE support_sessions SET state='revoked' WHERE id=? OR grant_id=?", id, id); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE support_grants SET state='revoked' WHERE id=? AND state='issued'", id); err != nil {
			return err
		}
		return s.audit(tx, provider.Case{}, "", id, "", "revoke", "success", reason)
	})
}
