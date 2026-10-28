package provider

import (
	"database/sql"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // Register the SQLite driver used by the provider store.
)

//go:embed migrations/0001_provider.sql
var schema string

// Store persists provider identities, sessions, assignments and audit records.
type Store struct{ db *sql.DB }

// Installation identifies a gateway and its owning organization.
type Installation struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Hosting        string `json:"hosting"`
	Enabled        bool   `json:"enabled"`
}

// Organization describes an enabled or disabled customer organization.
type Organization struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Technician is a named identity in the configured issuer roster.
type Technician struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Issuer        string `json:"-"`
	Subject       string `json:"-"`
	Administrator bool   `json:"administrator"`
}

// Case binds a support assignment to a gateway, account and revision.
type Case struct {
	AccountID      string `json:"account_id"`
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	InstallationID string `json:"installation_id"`
	TechnicianID   string `json:"technician_id"`
	TechnicianName string `json:"technician_name"`
	Revision       int64  `json:"revision"`
	State          string `json:"state"`
	Expires        int64  `json:"expires"`
}

// Open opens the provider database and applies its schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if err := os.Chmod(path, 0600); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return s, nil
}

// Close releases the provider database.
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec("CREATE TABLE IF NOT EXISTS _schema(version INTEGER PRIMARY KEY)"); err != nil {
		return err
	}
	var n int
	if err = tx.QueryRow("SELECT count(*) FROM _schema WHERE version=1").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err = tx.Exec(schema); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO _schema VALUES(1)"); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) mutate(actor Technician, org, installation, operation string, fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = fn(tx); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO provider_audit(id,at,actor_id,actor_name,actor_issuer,organization_id,installation_id,operation,outcome) VALUES(?,?,?,?,?,?,?,?,?)", uuid.NewString(), time.Now().Unix(), actor.ID, actor.Name, actor.Issuer, org, installation, operation, "success"); err != nil {
		return err
	}
	return tx.Commit()
}

// Local CLI management is authorized by the state-directory OS owner, not OIDC.
var localOperator = Technician{ID: "os-uid:" + strconv.Itoa(os.Getuid()), Name: "OS UID " + strconv.Itoa(os.Getuid()), Issuer: "local-os"}

// AddOrganization creates an organization using the local OS operator authority.
func (s *Store) AddOrganization(id, name string) error {
	if id == "" || name == "" {
		return errors.New("organization ID and name required")
	}
	return s.mutate(localOperator, id, "", "organization_create", func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO organizations(id,name) VALUES(?,?)", id, name)
		return err
	})
}

// AddTechnician adds a named issuer identity using local OS authority.
func (s *Store) AddTechnician(id, issuer, subject, name string, admin bool) error {
	if id == "" || issuer == "" || subject == "" || name == "" {
		return errors.New("complete named technician identity required")
	}
	return s.mutate(localOperator, "", "", "technician_create", func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO technicians(id,issuer,subject,name,administrator) VALUES(?,?,?,?,?)", id, issuer, subject, name, admin)
		return err
	})
}

// AssignOrganization assigns a technician to an organization using local OS authority.
func (s *Store) AssignOrganization(tech, org string) error {
	return s.mutate(localOperator, org, "", "organization_assign", func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT OR IGNORE INTO organization_assignments VALUES(?,?)", tech, org)
		return err
	})
}

// RegisterInstallation registers an installation using local OS authority.
func (s *Store) RegisterInstallation(i Installation) error { return s.register(localOperator, i) }
func (s *Store) register(actor Technician, i Installation) error {
	if i.ID == "" || i.OrganizationID == "" || i.Name == "" || (i.Hosting != "customer" && i.Hosting != "nebuless") {
		return errors.New("invalid installation")
	}
	return s.mutate(actor, i.OrganizationID, i.ID, "installation_create", func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO installations(id,organization_id,name,hosting) VALUES(?,?,?,?)", i.ID, i.OrganizationID, i.Name, i.Hosting)
		return err
	})
}

// InstallationFor loads an enabled installation assigned to the technician.
func (s *Store) InstallationFor(tech, org, id string) (Installation, error) {
	var i Installation
	err := s.db.QueryRow("SELECT i.id,i.organization_id,i.name,i.hosting,i.enabled FROM installations i JOIN organizations o ON o.id=i.organization_id JOIN organization_assignments a ON a.organization_id=i.organization_id JOIN technicians t ON t.id=a.technician_id WHERE t.id=? AND i.organization_id=? AND i.id=? AND t.enabled=1 AND o.enabled=1 AND i.enabled=1", tech, org, id).Scan(&i.ID, &i.OrganizationID, &i.Name, &i.Hosting, &i.Enabled)
	return i, err
}

// Technician loads an enabled technician by issuer and subject.
func (s *Store) Technician(issuer, subject string) (Technician, error) {
	var t Technician
	err := s.db.QueryRow("SELECT id,name,issuer,subject,administrator FROM technicians WHERE issuer=? AND subject=? AND enabled=1", issuer, subject).Scan(&t.ID, &t.Name, &t.Issuer, &t.Subject, &t.Administrator)
	return t, err
}

// CurrentCase loads the current assignment revision for a support case.
func (s *Store) CurrentCase(org, installation, id string) (Case, error) {
	var c Case
	err := s.db.QueryRow("SELECT c.id,c.organization_id,c.installation_id,coalesce(c.technician_id,''),coalesce(t.name,''),c.revision,c.state,c.expires,c.account_id FROM cases c JOIN installations i ON i.id=c.installation_id AND i.organization_id=c.organization_id JOIN organizations o ON o.id=c.organization_id LEFT JOIN technicians t ON t.id=c.technician_id WHERE c.organization_id=? AND c.installation_id=? AND c.id=? AND i.enabled=1 AND o.enabled=1 AND (t.enabled=1 OR c.technician_id IS NULL) AND c.expires>?", org, installation, id, time.Now().Unix()).Scan(&c.ID, &c.OrganizationID, &c.InstallationID, &c.TechnicianID, &c.TechnicianName, &c.Revision, &c.State, &c.Expires, &c.AccountID)
	return c, err
}

// SubmitCase records a customer case submission idempotently by intent.
func (s *Store) SubmitCase(org, installation, intent, account, summary, category string) (Case, error) {
	if len(summary) > 240 || summary == "" || intent == "" || account == "" || category == "" {
		return Case{}, errors.New("invalid case")
	}
	id := uuid.NewString()
	err := s.mutate(Technician{ID: installation, Issuer: "connector"}, org, installation, "case_submit", func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO cases(id,organization_id,installation_id,intent_id,account_id,summary,category,expires) SELECT ?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM installations i JOIN organizations o ON o.id=i.organization_id WHERE i.id=? AND i.organization_id=? AND i.enabled=1 AND o.enabled=1) ON CONFLICT(installation_id,intent_id) DO NOTHING", id, org, installation, intent, account, summary, category, time.Now().Add(7*24*time.Hour).Unix(), installation, org)
		if err != nil {
			return err
		}
		return tx.QueryRow("SELECT id FROM cases WHERE organization_id=? AND installation_id=? AND intent_id=? AND account_id=? AND summary=? AND category=?", org, installation, intent, account, summary, category).Scan(&id)
	})
	if err != nil {
		return Case{}, err
	}
	return s.CurrentCase(org, installation, id)
}

// AssignCase assigns a case with administrator authority and advances its revision.
func (s *Store) AssignCase(actor Technician, org, installation, id, tech string) error {
	if _, err := s.InstallationFor(actor.ID, org, installation); err != nil {
		return err
	}
	if !actor.Administrator {
		return errors.New("administrator required")
	}
	return s.mutate(actor, org, installation, "case_assign", func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE cases SET technician_id=?,revision=revision+1,state='assigned' WHERE id=? AND organization_id=? AND installation_id=? AND state!='closed' AND EXISTS(SELECT 1 FROM organization_assignments a JOIN technicians t ON t.id=a.technician_id WHERE a.organization_id=? AND a.technician_id=? AND t.enabled=1)", tech, id, org, installation, org, tech)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err == nil && n != 1 {
			return errors.New("case unavailable")
		}
		return err
	})
}
