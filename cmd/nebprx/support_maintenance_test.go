package main

import (
	"context"
	"testing"
	"time"
)

func TestSupportMaintenanceTerminalRowsAndUnredeemedCodes(t *testing.T) {
	s, a, c := supportFixture(t)
	now := s.now()
	_, pending := supportPending(t, s, a, c)
	s.now = func() time.Time { return now.Add(2 * 24 * time.Hour) }
	if err := s.maintain(90); err != nil {
		t.Fatal(err)
	}
	var state string
	var hashCount int
	if err := s.db.QueryRow("SELECT state FROM support_sessions WHERE id=?", pending.ID).Scan(&state); err != nil || state != "revoked" {
		t.Fatalf("expired session: %s %v", state, err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM support_grants WHERE code_hash IS NOT NULL").Scan(&hashCount); err != nil || hashCount != 0 {
		t.Fatalf("terminal code hash retained: %d %v", hashCount, err)
	}
	s.now = func() time.Time { return now.Add(91 * 24 * time.Hour) }
	if err := s.maintain(90); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"support_sessions", "support_grants", "support_audit"} {
		var count int
		if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s: %d %v", table, count, err)
		}
	}
	// Fresh unredeemed codes must also become terminal and lose their hash.
	s.now = func() time.Time { return now }
	if _, _, err := s.issue(context.Background(), c, a); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now.Add(2 * 24 * time.Hour) }
	if err := s.maintain(90); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM support_grants WHERE code_hash IS NOT NULL OR state='issued'").Scan(&hashCount); err != nil || hashCount != 0 {
		t.Fatalf("expired code retained: %d %v", hashCount, err)
	}
	if err := s.maintain(29); err == nil {
		t.Fatal("invalid retention accepted")
	}
}
