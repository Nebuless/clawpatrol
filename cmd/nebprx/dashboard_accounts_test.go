package main

import (
	"path/filepath"
	"testing"
)

func TestDashboardAccountGeneration(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := setDashboardUser(db, "root", "synthetic-password"); err != nil {
		t.Fatal(err)
	}
	a, err := resolveDashboardAccount(db, "password", "local", "root", "root")
	if err != nil {
		t.Fatal(err)
	}
	if err := backfillDashboardAccounts(db); err != nil {
		t.Fatal(err)
	}
	b, err := resolveDashboardAccount(db, "password", "local", "root", "root")
	if err != nil || a.ID != b.ID {
		t.Fatalf("unstable identity: %v", err)
	}
	if err := deleteDashboardUser(db, "root"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveDashboardAccount(db, "password", "local", "root", "root"); err == nil {
		t.Fatal("absent root accepted")
	}
	if err := setDashboardUser(db, "root", "replacement-password"); err != nil {
		t.Fatal(err)
	}
	c, err := resolveDashboardAccount(db, "password", "local", "root", "root")
	if err != nil || c.ID == a.ID {
		t.Fatalf("old generation revived: %v", err)
	}
}

func TestDashboardNewMigrationAtomic(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := applyAtomicMigration(db, 999, "CREATE TABLE atomic_probe(id TEXT); INSERT INTO absent_table VALUES(1);"); err == nil {
		t.Fatal("fault accepted")
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='atomic_probe'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial DDL: %d %v", n, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM _schema WHERE version=999").Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial marker: %d %v", n, err)
	}
}
