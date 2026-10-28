package registry

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSupportHealthBoundedMetadata(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "registry"), "client")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx := context.Background()
	if got := s.SupportHealth(ctx); got != "local_only" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		state string
		age   time.Duration
		want  string
	}{{"verified", 0, "verified"}, {"verified", 25 * time.Hour, "stale"}, {"failed", 0, "sync_failed"}} {
		tx, err := s.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if err = put(tx, "sync_attempt", map[string]any{"state": tc.state, "at": time.Now().Add(-tc.age)}); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if got := s.SupportHealth(ctx); got != tc.want {
			t.Fatalf("health=%s want=%s", got, tc.want)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got := s.SupportHealth(ctx); got != "unavailable" {
		t.Fatal(got)
	}
}
