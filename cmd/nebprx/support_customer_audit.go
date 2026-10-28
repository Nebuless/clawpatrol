package main

import (
	"database/sql"
	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/google/uuid"
)

func (s *supportStore) customerAudit(tx *sql.Tx, a dashboardAccount, c provider.Case, session, grant, operation, reason string) error {
	_, err := tx.Exec("INSERT INTO support_audit(id,at,request_id,actor_id,actor_name,actor_issuer,effective_id,effective_name,organization_id,installation_id,case_id,session_id,grant_id,assignment_revision,operation,method,outcome,reason) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'POST','success',?)", uuid.NewString(), s.now().Unix(), uuid.NewString(), a.ID, a.Name, a.Authority, a.ID, a.Name, s.org, s.installation, c.ID, session, grant, c.Revision, operation, reason)
	return err
}
