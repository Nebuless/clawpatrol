package provider

import (
	"database/sql"
	"errors"
)

func (s *Store) manageOrganization(actor Technician, id, name string, enabled, create bool) error {
	if !actor.Administrator || len(id) == 0 || len(id) > 128 || len(name) == 0 || len(name) > 240 {
		return errors.New("invalid organization")
	}
	return s.mutate(actor, id, "", "organization_update", func(tx *sql.Tx) error {
		if create {
			if _, err := tx.Exec("INSERT INTO organizations(id,name,enabled) VALUES(?,?,?)", id, name, enabled); err != nil {
				return err
			}
			_, err := tx.Exec("INSERT INTO organization_assignments VALUES(?,?)", actor.ID, id)
			return err
		}
		res, err := tx.Exec("UPDATE organizations SET name=?,enabled=? WHERE id=? AND EXISTS(SELECT 1 FROM organization_assignments WHERE technician_id=? AND organization_id=?)", name, enabled, id, actor.ID, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err == nil && n != 1 {
			return errors.New("organization unavailable")
		}
		return err
	})
}
func (s *Store) assignOrganization(actor Technician, org, tech string) error {
	if !actor.Administrator {
		return errors.New("administrator required")
	}
	return s.mutate(actor, org, "", "organization_assign", func(tx *sql.Tx) error {
		var allowed int
		if err := tx.QueryRow("SELECT count(*) FROM organization_assignments WHERE organization_id=? AND technician_id=?", org, actor.ID).Scan(&allowed); err != nil {
			return err
		}
		if allowed != 1 {
			return errors.New("organization unavailable")
		}
		// Only already-visible named colleagues can be assigned. Roster creation stays OS-owned.
		var visible int
		if err := tx.QueryRow("SELECT count(*) FROM technicians t WHERE t.id=? AND t.enabled=1 AND EXISTS(SELECT 1 FROM organization_assignments a JOIN organization_assignments b ON b.organization_id=a.organization_id WHERE a.technician_id=t.id AND b.technician_id=?)", tech, actor.ID).Scan(&visible); err != nil {
			return err
		}
		if visible != 1 {
			return errors.New("technician unavailable")
		}
		_, err := tx.Exec("INSERT OR IGNORE INTO organization_assignments VALUES(?,?)", tech, org)
		return err
	})
}
