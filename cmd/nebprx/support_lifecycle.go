package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/user"

	"github.com/google/uuid"
)

type supportLifecycleActor struct{ id, name, issuer, method string }

var systemSupportActor = supportLifecycleActor{"system:gateway", "PRX gateway", "local-system", "SYSTEM"}

func localSupportActor() supportLifecycleActor {
	a := supportLifecycleActor{fmt.Sprintf("uid:%d", os.Geteuid()), "Local operating-system user", "local-os", "CLI"}
	if u, err := user.Current(); err == nil {
		a.name = u.Username
	}
	return a
}

// Call with s.mu held. Audit and revocation normally commit together. If audit
// storage fails, deny this process and attempt durable revocation plus disable
// without audit. Return the failure so operators can repair evidence storage.
func (s *supportStore) revokeAllSupportLocked(actor supportLifecycleActor, reason string, disable bool) error {
	err := s.transaction(func(tx *sql.Tx) error {
		if err := s.auditSupportTermination(tx, actor, reason); err != nil {
			return err
		}
		if disable {
			if err := s.auditSupportControl(tx, actor, "support_disable", reason); err != nil {
				return err
			}
		}
		return revokeSupportRows(tx, disable)
	})
	if err == nil {
		return nil
	}
	s.blocked.Store(true)
	recovery := s.transaction(func(tx *sql.Tx) error { return revokeSupportRows(tx, true) })
	return fmt.Errorf("support lifecycle persistence failed; disabling recovery attempted: %w", errors.Join(err, recovery))
}

func (s *supportStore) revokeAllSupport(actor supportLifecycleActor, reason string, disable bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revokeAllSupportLocked(actor, reason, disable)
}

func revokeSupportRows(tx *sql.Tx, disable bool) error {
	for _, q := range []string{
		"UPDATE support_sessions SET state='revoked' WHERE state!='revoked'",
		"UPDATE support_grants SET state='revoked' WHERE state='issued'",
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	if disable {
		_, err := tx.Exec("UPDATE support_control SET enabled=0 WHERE singleton=1")
		return err
	}
	return nil
}

func (s *supportStore) auditSupportTermination(tx *sql.Tx, actor supportLifecycleActor, reason string) error {
	// INSERT SELECT keeps Go memory bounded and records each affected binding.
	// SQLite produces a fresh UUID-shaped identifier for every selected row.
	const rowID = "lower(hex(randomblob(4)))||'-'||lower(hex(randomblob(2)))||'-'||lower(hex(randomblob(2)))||'-'||lower(hex(randomblob(2)))||'-'||lower(hex(randomblob(6)))"
	const prefix = "INSERT INTO support_audit(id,at,request_id,actor_id,actor_name,actor_issuer,effective_id,effective_name,organization_id,installation_id,case_id,grant_id,session_id,assignment_revision,operation,method,outcome,reason) SELECT " + rowID + ",?,?,?,?,?,r.account_id,a.display_name,?,?,r.case_id,"
	for _, source := range []struct{ table, grant, session, operation, predicate string }{
		{"support_sessions", "r.grant_id", "r.id", "session_revoke", "r.state!='revoked'"},
		{"support_grants", "r.id", "NULL", "grant_revoke", "r.state='issued'"},
	} {
		q := prefix + source.grant + "," + source.session + ",r.assignment_revision,?,?,'success',? FROM " + source.table + " r JOIN dashboard_accounts a ON a.id=r.account_id WHERE " + source.predicate
		if _, err := tx.Exec(q, s.now().Unix(), uuid.NewString(), actor.id, actor.name, actor.issuer, s.org, s.installation, source.operation, actor.method, reason); err != nil {
			return err
		}
	}
	return nil
}

func (s *supportStore) auditSupportControl(tx *sql.Tx, actor supportLifecycleActor, operation, reason string) error {
	_, err := tx.Exec("INSERT INTO support_audit(id,at,request_id,actor_id,actor_name,actor_issuer,organization_id,installation_id,operation,method,outcome,reason) VALUES(?,?,?,?,?,?,?,?,?,?,'success',?)", uuid.NewString(), s.now().Unix(), uuid.NewString(), actor.id, actor.name, actor.issuer, s.org, s.installation, operation, actor.method, reason)
	return err
}
