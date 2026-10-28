package main

import (
	"database/sql"
	"github.com/Nebuless/nebprx/internal/provider"
)

func (s *supportStore) customerRevoke(a dashboardAccount, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transaction(func(tx *sql.Tx) error {
		var grant, session, caseID string
		err := tx.QueryRow("SELECT id,'',case_id FROM support_grants WHERE id=? AND account_id=? UNION ALL SELECT grant_id,id,case_id FROM support_sessions WHERE id=? AND account_id=? LIMIT 1", id, a.ID, id, a.ID).Scan(&grant, &session, &caseID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE support_sessions SET state='revoked' WHERE grant_id=? AND account_id=?", grant, a.ID); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE support_grants SET state='revoked' WHERE id=? AND account_id=? AND state='issued'", grant, a.ID); err != nil {
			return err
		}
		return s.customerAudit(tx, a, provider.Case{ID: caseID}, session, grant, "revoke", "customer_revoke")
	})
}
