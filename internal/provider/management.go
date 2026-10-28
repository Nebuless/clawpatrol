package provider

import (
	"crypto/ed25519"
	"database/sql"
	"errors"
	"time"
)

// RevokeTechnician disables a technician and revokes associated sessions in an audited transaction.
func (s *Store) RevokeTechnician(id string) error {
	return s.mutate(localOperator, "", "", "technician_revoke", func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE technicians SET enabled=0 WHERE id=?", id); err != nil {
			return err
		}
		for _, q := range []string{"DELETE FROM support_routes WHERE provider_session_hash IN (SELECT token_hash FROM sessions WHERE technician_id=?)", "DELETE FROM handoffs WHERE provider_session_hash IN (SELECT token_hash FROM sessions WHERE technician_id=?)", "DELETE FROM sessions WHERE technician_id=?"} {
			if _, err := tx.Exec(q, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// Organizations lists organizations assigned to the technician.
func (s *Store) Organizations(t Technician) ([]Organization, error) {
	rows, err := s.db.Query("SELECT o.id,o.name,o.enabled FROM organizations o JOIN organization_assignments a ON a.organization_id=o.id WHERE a.technician_id=? ORDER BY o.name", t.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Organization{}
	for rows.Next() {
		var o Organization
		if err = rows.Scan(&o.ID, &o.Name, &o.Enabled); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Installations lists installations in an organization assigned to the technician.
func (s *Store) Installations(t Technician, org string) ([]Installation, error) {
	rows, err := s.db.Query("SELECT i.id,i.organization_id,i.name,i.hosting,i.enabled FROM installations i JOIN organization_assignments a ON a.organization_id=i.organization_id WHERE a.technician_id=? AND i.organization_id=? ORDER BY i.name", t.ID, org)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Installation{}
	for rows.Next() {
		var i Installation
		if err = rows.Scan(&i.ID, &i.OrganizationID, &i.Name, &i.Hosting, &i.Enabled); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// Manage updates an installation after checking administrator and organization authority.
func (s *Store) Manage(actor Technician, org, id, name string, enabled bool) error {
	if !actor.Administrator {
		return errors.New("administrator required")
	}
	var allowed int
	if err := s.db.QueryRow("SELECT count(*) FROM installations i JOIN organization_assignments a ON a.organization_id=i.organization_id WHERE i.id=? AND i.organization_id=? AND a.technician_id=?", id, org, actor.ID).Scan(&allowed); err != nil {
		return err
	}
	if allowed != 1 {
		return errors.New("installation unavailable")
	}
	return s.mutate(actor, org, id, "installation_update", func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE installations SET name=?,enabled=? WHERE id=? AND organization_id=?", name, enabled, id, org)
		return err
	})
}

// Enrollment issues a short-lived enrollment token using local OS authority.
func (s *Store) Enrollment(org, id string) (string, error) {
	token := randomToken()
	err := s.mutate(localOperator, org, id, "enrollment_issue", func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE installations SET enrollment_hash=?,enrollment_expires=? WHERE id=? AND organization_id=? AND enabled=1", digest(token), time.Now().Add(5*time.Minute).Unix(), id, org)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err == nil && n != 1 {
			return errors.New("installation unavailable")
		}
		return err
	})
	return token, err
}

// Enroll consumes an enrollment token and binds an installation signing key.
func (s *Store) Enroll(org, id, token string, key []byte) (string, error) {
	if len(key) != ed25519.PublicKeySize {
		return "", errors.New("invalid installation key")
	}
	credential := randomToken()
	err := s.mutate(Technician{ID: id, Issuer: "connector"}, org, id, "enroll", func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE installations SET public_key=?,credential_hash=?,enrollment_hash=NULL,enrollment_expires=NULL WHERE id=? AND organization_id=? AND enabled=1 AND enrollment_hash=? AND enrollment_expires>?", key, digest(credential), id, org, digest(token), time.Now().Unix())
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err == nil && n != 1 {
			return errors.New("enrollment unavailable")
		}
		return err
	})
	if err != nil {
		return "", err
	}
	return credential, nil
}

// Connector authenticates an enabled connector and returns its public key.
func (s *Store) Connector(org, id, credential string) (ed25519.PublicKey, error) {
	var key []byte
	err := s.db.QueryRow("SELECT i.public_key FROM installations i JOIN organizations o ON o.id=i.organization_id WHERE i.organization_id=? AND i.id=? AND i.credential_hash=? AND i.enabled=1 AND o.enabled=1", org, id, digest(credential)).Scan(&key)
	return ed25519.PublicKey(key), err
}

// Cases lists current cases visible to the technician for an authorized installation.
func (s *Store) Cases(t Technician, org, id string) ([]Case, error) {
	if _, err := s.InstallationFor(t.ID, org, id); err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT c.id,c.organization_id,c.installation_id,coalesce(c.technician_id,''),coalesce(t.name,''),c.revision,c.state,c.expires,c.account_id FROM cases c LEFT JOIN technicians t ON t.id=c.technician_id WHERE c.organization_id=? AND c.installation_id=? AND c.expires>? AND (? OR c.technician_id=?) AND (t.enabled=1 OR c.technician_id IS NULL) ORDER BY c.id", org, id, time.Now().Unix(), t.Administrator, t.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Case{}
	for rows.Next() {
		var c Case
		if err = rows.Scan(&c.ID, &c.OrganizationID, &c.InstallationID, &c.TechnicianID, &c.TechnicianName, &c.Revision, &c.State, &c.Expires, &c.AccountID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
