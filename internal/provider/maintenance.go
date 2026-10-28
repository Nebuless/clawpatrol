package provider

import (
	"errors"
	"time"
)

// Maintain removes expired authentication material and retained metadata in one
// transaction. Child routes/handoffs must go before their session/case parents.
func (s *Store) Maintain(now time.Time, days int) error {
	if days < 30 || days > 365 {
		return errors.New("audit retention must be 30..365 days")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{
		"DELETE FROM support_routes WHERE expires<=? OR provider_session_hash IN (SELECT token_hash FROM sessions WHERE expires<=?)",
		"DELETE FROM handoffs WHERE expires<=? OR provider_session_hash IN (SELECT token_hash FROM sessions WHERE expires<=?)",
	} {
		if _, err = tx.Exec(q, now.Unix(), now.Unix()); err != nil {
			return err
		}
	}
	for _, q := range []string{
		"DELETE FROM sessions WHERE expires<=?",
		"DELETE FROM oauth_pending WHERE expires<=?",
		"DELETE FROM rate_limits WHERE expires<=?",
		"UPDATE installations SET enrollment_hash=NULL,enrollment_expires=NULL WHERE enrollment_expires<=?",
	} {
		if _, err = tx.Exec(q, now.Unix()); err != nil {
			return err
		}
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()
	for _, q := range []string{
		"DELETE FROM provider_audit WHERE at<?",
		"DELETE FROM cases WHERE state='closed' AND expires<? AND NOT EXISTS(SELECT 1 FROM handoffs WHERE case_id=cases.id) AND NOT EXISTS(SELECT 1 FROM support_routes WHERE case_id=cases.id)",
	} {
		if _, err = tx.Exec(q, cutoff); err != nil {
			return err
		}
	}
	return tx.Commit()
}
