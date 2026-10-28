package provider

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMaintenanceRetentionAndTokenCleanup(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = s.AddOrganization("org", "Organization"); err != nil {
		t.Fatal(err)
	}
	if err = s.RegisterInstallation(Installation{ID: "gateway", OrganizationID: "org", Name: "Gateway", Hosting: "customer"}); err != nil {
		t.Fatal(err)
	}
	if err = s.AddTechnician("tech", "https://idp.example", "subject", "Technician", true); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old := now.Add(-91 * 24 * time.Hour).Unix()
	for _, q := range []string{
		"INSERT INTO oauth_pending VALUES('state','browser','nonce','verifier',1)",
		"INSERT INTO sessions VALUES('session','tech',X'1234',1,1)",
		"INSERT INTO rate_limits VALUES('scope',1,1)",
		"INSERT INTO cases VALUES('case','org','gateway','intent','account','summary','general','tech',1,'closed',1)",
		"INSERT INTO handoffs VALUES('handoff','pending','session','org','gateway','pending.support.example',1,'case')",
		"INSERT INTO support_routes VALUES('route','active','session','org','gateway','case','active.support.example',1)",
		"UPDATE installations SET enrollment_hash='expired',enrollment_expires=1",
	} {
		if _, err = s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.db.Exec("UPDATE provider_audit SET at=?", old); err != nil {
		t.Fatal(err)
	}
	if err = s.Maintain(now, 90); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"oauth_pending", "sessions", "rate_limits", "provider_audit", "handoffs", "support_routes", "cases"} {
		var count int
		if err = s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s: %d %v", table, count, err)
		}
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM installations WHERE enrollment_hash IS NOT NULL").Scan(&count); err != nil || count != 0 {
		t.Fatalf("enrollment material retained: %d %v", count, err)
	}
	if _, err = s.db.Exec("INSERT INTO sessions VALUES('fresh','tech',X'1234',?,?)", now.Unix(), now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if err = s.Maintain(now, 90); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil || count != 1 {
		t.Fatalf("live session lost: %d %v", count, err)
	}
	for _, days := range []int{29, 366} {
		if err = s.Maintain(now, days); err == nil {
			t.Fatalf("invalid retention accepted: %d", days)
		}
	}
}
