package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// SupportHealth reads bounded sync metadata, never inventory or token material.
func (s *Store) SupportHealth(ctx context.Context) string {
	if s.Mode != "client" {
		return "not_client"
	}
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='sync_attempt'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "local_only"
	}
	if err != nil {
		return "unavailable"
	}
	var attempt struct {
		State string
		At    time.Time
	}
	if json.Unmarshal([]byte(raw), &attempt) != nil {
		return "unavailable"
	}
	if attempt.State != "verified" {
		return "sync_failed"
	}
	if time.Since(attempt.At) > 24*time.Hour {
		return "stale"
	}
	return "verified"
}
