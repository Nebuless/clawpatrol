package main

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type dashboardAccount struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Authority string `json:"-"`
	Subject   string `json:"-"`
	Revision  int64  `json:"revision"`
}

func backfillDashboardAccounts(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec("INSERT OR IGNORE INTO installation_identity(singleton,id) VALUES(1,?)", uuid.NewString()); err != nil {
		return err
	}
	// Idempotent, including crash/retry after upgrading an existing root.
	if _, err = tx.Exec("INSERT OR IGNORE INTO dashboard_accounts(id,auth_kind,auth_authority,auth_subject,display_name) SELECT ?, 'password','local',username,username FROM dashboard_users WHERE username=?", uuid.NewString(), dashboardRootUsername); err != nil {
		return err
	}
	return tx.Commit()
}

func resolveDashboardAccount(db *sql.DB, kind, authority, subject, name string) (dashboardAccount, error) {
	var a dashboardAccount
	if db == nil || authority == "" || subject == "" || name == "" {
		return a, errors.New("account identity unavailable")
	}
	// Never create tailnet subjects from a saved login. Callers must first
	// prove current membership using the server's direct whois adapter.
	if kind != "password" && kind != "tailnet" {
		return a, errors.New("unsupported account kind")
	}
	if kind == "password" {
		if _, exists, err := lookupDashboardUser(db, dashboardRootUsername); err != nil || !exists {
			return a, errors.New("root unavailable")
		}
	}
	_, err := db.Exec("INSERT INTO dashboard_accounts(id,auth_kind,auth_authority,auth_subject,display_name) VALUES(?,?,?,?,?) ON CONFLICT(auth_kind,auth_authority,auth_subject) DO UPDATE SET display_name=excluded.display_name WHERE active=1", uuid.NewString(), kind, authority, subject, name)
	if err != nil {
		return a, err
	}
	err = db.QueryRow("SELECT id,display_name,auth_kind,auth_authority,auth_subject,permission_revision FROM dashboard_accounts WHERE auth_kind=? AND auth_authority=? AND auth_subject=? AND active=1", kind, authority, subject).Scan(&a.ID, &a.Name, &a.Kind, &a.Authority, &a.Subject, &a.Revision)
	if err != nil {
		return a, fmt.Errorf("account unavailable: %w", err)
	}
	return a, nil
}
