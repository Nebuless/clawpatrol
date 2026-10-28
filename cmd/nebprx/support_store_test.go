package main

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
)

func TestSupportConcurrentReplayRevokeExpiry(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = setDashboardUser(db, "root", "synthetic-password"); err != nil {
		t.Fatal(err)
	}
	a, err := resolveDashboardAccount(db, "password", "local", "root", "Customer")
	if err != nil {
		t.Fatal(err)
	}
	s, err := newSupportStore(db, "org", "installation", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	s.domain = "support.example"
	c := provider.Case{ID: "case", OrganizationID: "org", InstallationID: "installation", TechnicianID: "tech", TechnicianName: "Technician", Revision: 1, State: "assigned", Expires: time.Now().Add(time.Hour).Unix()}
	c.AccountID = a.ID
	if err = s.mirrorCase(c); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.issue(context.Background(), c, a)
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	var id string
	for range 100 {
		wg.Go(func() {
			session, err := s.redeem(context.Background(), c, "tech", code, "https://session.support.example")
			if err == nil {
				wins.Add(1)
				id = session.ID
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("winners=%d", wins.Load())
	}
	if _, err = s.authorize(id); err == nil {
		t.Fatal("pending session accessed data")
	}
	if err = s.activate(id, c); err != nil {
		t.Fatal(err)
	}
	if _, err = s.authorize(id); err != nil {
		t.Fatal(err)
	}
	if err = s.revoke(id, "customer_end"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.authorize(id); err == nil {
		t.Fatal("revoked session accepted")
	}
	if _, err = s.redeem(context.Background(), c, "tech", code, "https://session.support.example"); err == nil {
		t.Fatal("consumed code replayed")
	}
}
