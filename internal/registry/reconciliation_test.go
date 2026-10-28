package registry

import (
	"strings"
	"testing"
	"time"
)

func TestImportQuarantinesPartiallyUnmappedAffected(t *testing.T) {
	for _, malformed := range []string{
		`{"package":{"name":"example"},"versions":["1.0.0"]}`,
		`{"package":{"ecosystem":"npm"},"versions":["1.0.0"]}`,
	} {
		server, client := testStore(t, "server"), testStore(t, "client")
		importRecords(t, server, "npm", fixture(t, "npm", "example", "GHSA-review", false))
		activate(t, server, client, []Identity{{"npm", "example"}})
		checkFinding(t, client, packageFor("npm", "example", "1.0.0"), "known_affected")
		raw := []byte(`{"id":"GHSA-review","modified":"2026-01-01T00:00:00Z","affected":[{"package":{"ecosystem":"npm","name":"other"},"versions":["1.0.0"]},` + malformed + `]}`)
		importRecords(t, server, "npm", raw)
		activate(t, server, client, []Identity{{"npm", "example"}})
		result, err := client.Check(packageFor("npm", "example", "1.0.0"), time.Now())
		if err != nil || result.Finding != "indeterminate" || result.Coverage != "partial" {
			t.Fatalf("partly unmapped record manufactured clearance: %s/%s, %v", result.Finding, result.Coverage, err)
		}
	}
}

func TestImportPreservesValidOtherEcosystemEntries(t *testing.T) {
	server, client := testStore(t, "server"), testStore(t, "client")
	raw := []byte(`{"id":"GHSA-multi-ecosystem","modified":"2026-01-01T00:00:00Z","affected":[{"package":{"ecosystem":"npm","name":"example"},"versions":["1.0.0"]},{"package":{"ecosystem":"Maven","name":"org.example:example"},"versions":["1.0.0"]}]}`)
	importRecords(t, server, "npm", raw)
	activate(t, server, client, []Identity{{"npm", "example"}})
	result, err := client.Check(packageFor("npm", "example", "2.0.0"), time.Now())
	if err != nil || result.Finding != "no_known_match" || result.Coverage != "complete_for_declared_sources" {
		t.Fatalf("valid cross-ecosystem entry changed coverage: %s/%s, %v", result.Finding, result.Coverage, err)
	}
}

func TestConcurrentSubscribeAcrossStoreHandles(t *testing.T) {
	dir := stateDir(t)
	first, err := Open(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := first.Close(); err != nil {
			t.Error(err)
		}
	}()
	second, err := Open(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := second.Close(); err != nil {
			t.Error(err)
		}
	}()
	type outcome struct {
		name string
		err  error
	}
	for round := 0; round < 30; round++ {
		tx, err := first.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if err := put(tx, "candidates", []Identity{}); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan outcome, 2)
		for i, store := range []*Store{first, second} {
			name := []string{"first", "second"}[i]
			go func() {
				<-start
				results <- outcome{name, store.Subscribe(packageFor("npm", name, "1.0.0"))}
			}()
		}
		close(start)
		acknowledged := map[string]bool{}
		for i := 0; i < 2; i++ {
			result := <-results
			if result.err == nil {
				acknowledged[result.name] = true
			} else if !strings.Contains(result.err.Error(), "SQLITE_BUSY") && !strings.Contains(result.err.Error(), "SQLITE_LOCKED") {
				t.Fatalf("unexpected subscription error: %v", result.err)
			}
		}
		if len(acknowledged) == 0 {
			t.Fatal("neither concurrent operation completed")
		}
		ids, err := first.Subscriptions()
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			delete(acknowledged, id.Name)
		}
		if len(acknowledged) != 0 {
			t.Fatalf("successful subscriptions lost in round %d: %v", round, acknowledged)
		}
	}
}
