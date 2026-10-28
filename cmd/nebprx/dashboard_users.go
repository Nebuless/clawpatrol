package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// dashboardUsersBcryptCost is the bcrypt cost used for stored passwords.
// 12 is the current OWASP default — about ~250 ms on a 2024 laptop.
const dashboardUsersBcryptCost = 12

// dashboardRootUsername is the username of the bootstrap operator row.
// Future user-management features may add more rows; the gate today
// only consults this single row.
const dashboardRootUsername = "root"

// lookupDashboardUser returns the stored bcrypt hash for the given
// username. The bool is false when no row exists; the caller is
// expected to treat that as "no auth configured yet" and trigger the
// first-run flow.
func lookupDashboardUser(db *sql.DB, username string) (string, bool, error) {
	if db == nil {
		return "", false, fmt.Errorf("no db")
	}
	var hash string
	err := db.QueryRow(
		`SELECT password_hash FROM dashboard_users WHERE username = ?`,
		username,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return hash, true, nil
}

// setDashboardUser upserts the row for username with a bcrypt hash of
// password. Used by the first-run web flow and the
// --set-dashboard-password CLI flag.
func setDashboardUser(db *sql.DB, username, password string) error {
	if db == nil {
		return fmt.Errorf("no db")
	}
	if password == "" {
		return fmt.Errorf("password must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), dashboardUsersBcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UnixNano()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(
		`INSERT INTO dashboard_users (username, password_hash, created_ns, updated_ns)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(username) DO UPDATE SET
		   password_hash = excluded.password_hash,
		   updated_ns    = excluded.updated_ns`,
		username, string(hash), now, now,
	)
	if err != nil {
		return err
	}
	// Rotating the password invalidates every session minted under
	// the old one. Belt-and-suspenders alongside the natural TTL —
	// otherwise an operator who suspects credential leak would have
	// to wait out dashboard_session_ttl after `--set-dashboard-
	// password` before old cookies stopped working.
	if _, err = tx.Exec("DELETE FROM dashboard_sessions WHERE username=?", username); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE dashboard_accounts SET permission_revision=permission_revision+1 WHERE auth_kind='password' AND auth_subject=?", username); err != nil {
		return err
	}
	if username == dashboardRootUsername {
		if _, err = tx.Exec("UPDATE dashboard_accounts SET permission_revision=permission_revision+1 WHERE auth_kind='tailnet'"); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT OR IGNORE INTO dashboard_accounts(id,auth_kind,auth_authority,auth_subject,display_name) VALUES(?,'password','local',?,?)", uuid.NewString(), username, username); err != nil {
		return err
	}
	return tx.Commit()
}

// deleteDashboardUser removes the row for username. Used by
// --reset-dashboard-password. Also wipes the user's sessions so the
// reset is immediate. No-op if the row doesn't exist.
func deleteDashboardUser(db *sql.DB, username string) error {
	if db == nil {
		return fmt.Errorf("no db")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM dashboard_users WHERE username = ?`, username); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM dashboard_sessions WHERE username=?", username); err != nil {
		return err
	}
	// Keep historical accounts for audit, retire the credential generation.
	if _, err := tx.Exec("UPDATE dashboard_accounts SET active=0, permission_revision=permission_revision+1, auth_subject=id WHERE auth_kind='password' AND auth_subject=?", username); err != nil {
		return err
	}
	if username == dashboardRootUsername {
		if _, err = tx.Exec("UPDATE dashboard_accounts SET permission_revision=permission_revision+1 WHERE auth_kind='tailnet'"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// checkDashboardPassword constant-time compares password against the
// stored hash for username. Returns (ok, exists, err) — exists=false
// means there is no row at all (caller should trigger first-run).
func checkDashboardPassword(db *sql.DB, username, password string) (bool, bool, error) {
	hash, ok, err := lookupDashboardUser(db, username)
	if err != nil || !ok {
		return false, ok, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, true, nil
		}
		return false, true, err
	}
	return true, true, nil
}
