package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckUsesOnlyPersistedAdministrativeDecision(t *testing.T) {
	server, client := testStore(t, "server"), testStore(t, "client")
	p := packageFor("npm", "example", "1.0.0")
	importRecords(t, server, "npm", fixture(t, "npm", "example", "GHSA-synthetic", false))
	activate(t, server, client, []Identity{p.Identity})

	p.Decision = "allow"
	result, err := client.Check(p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.Finding != "known_affected" || result.Decision != "deny" {
		t.Fatalf("query supplied its own authorization: %+v", result)
	}

	if err := client.SetInventory(Inventory{Packages: []Package{p}}); err != nil {
		t.Fatal(err)
	}
	p.Decision = ""
	result, err = client.Check(p, time.Now())
	if err != nil || result.Finding != "known_affected" || result.Decision != "allow" {
		t.Fatalf("persisted administrator decision was lost: %+v, %v", result, err)
	}
}

func TestCheckEndpointRejectsRequestDecision(t *testing.T) {
	client := testStore(t, "client")
	token, err := client.LocalToken()
	if err != nil {
		t.Fatal(err)
	}
	handler := client.Handler()
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"ecosystem":"npm","name":"example","version":"1.0.0","public":true,"decision":"allow"}`, http.StatusBadRequest},
		{`{"ecosystem":"npm","name":"example","version":"1.0.0","public":true}`, http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/check", strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.want {
			t.Fatalf("status %d, want %d: %s", response.Code, tc.want, response.Body.String())
		}
		if response.Code == http.StatusOK {
			var result Result
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Decision != "review" {
				t.Fatalf("missing intelligence authorized a request: %+v", result)
			}
		}
	}
}

func TestSyncChecksPersistedTrustBeforeSendingCredentials(t *testing.T) {
	server, client := testStore(t, "server"), testStore(t, "client")
	activate(t, server, client, nil)
	received := make(chan struct{}, 1)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		http.Error(w, "unexpected request", http.StatusServiceUnavailable)
	}))
	defer other.Close()
	remote := remoteFor(t, server)
	remote.URL = other.URL
	remote.AllowHTTP = true
	if err := client.Sync(context.Background(), remote); err == nil {
		t.Fatal("changed server pin was accepted")
	}
	select {
	case <-received:
		t.Fatal("installation credential was sent before checking the persisted server pin")
	default:
	}
}

func TestImportRejectsUnmappedActiveAdvisory(t *testing.T) {
	server, client := testStore(t, "server"), testStore(t, "client")
	p := packageFor("npm", "example", "1.0.0")
	importRecords(t, server, "npm", fixture(t, "npm", "example", "GHSA-original", false))
	wrongEcosystem := fixture(t, "Go", "example.org/module", "GO-2099-1", false)
	for _, raw := range [][]byte{wrongEcosystem, []byte(`{"id":"GHSA-missing-package","modified":"2026-01-01T00:00:00Z","affected":[]}`)} {
		path := filepath.Join(t.TempDir(), "wrong-export.json")
		if err := os.WriteFile(path, append(append([]byte("["), raw...), ']'), 0600); err != nil {
			t.Fatal(err)
		}
		if err := server.Import(context.Background(), "npm", path, "wrong-export", time.Now()); err == nil {
			t.Fatal("active advisory without declared ecosystem scope was accepted")
		}
		activate(t, server, client, []Identity{p.Identity})
		result, err := client.Check(p, time.Now())
		if err != nil || result.Finding != "known_affected" || result.Freshness != "stale" {
			t.Fatalf("bad export erased previous evidence: %+v, %v", result, err)
		}
	}
}

func TestOpenRejectsCorruptLocalCredential(t *testing.T) {
	for _, value := range []string{"", "not-an-installation-secret"} {
		dir := stateDir(t)
		client, err := Open(dir, "client")
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "local.token"), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		opened, err := Open(dir, "client")
		if err == nil {
			if err := opened.Close(); err != nil {
				t.Fatal(err)
			}
			t.Fatal("corrupt local credential accepted")
		}
	}
}

func TestImportQuarantinesInvalidCoordinates(t *testing.T) {
	server, client := testStore(t, "server"), testStore(t, "client")
	good := fixture(t, "npm", "example", "GHSA-valid-coordinate", false)
	invalid := fixture(t, "npm", "https://github.com/example/repository", "GHSA-invalid-coordinate", false)
	importRecords(t, server, "npm", good, invalid)
	activate(t, server, client, []Identity{{"npm", "example"}, {"npm", "unmatched"}})
	checkFinding(t, client, packageFor("npm", "example", "1.0.0"), "known_affected")
	result, err := client.Check(packageFor("npm", "unmatched", "1.0.0"), time.Now())
	if err != nil || result.Finding != "indeterminate" || result.Coverage != "partial" {
		t.Fatalf("quarantined coordinate produced clearance: %+v, %v", result, err)
	}
	var count int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM records").Scan(&count); err != nil || count != 2 {
		t.Fatalf("quarantined raw evidence was discarded: count=%d, %v", count, err)
	}
}

func TestPublishedPackageIdentityForms(t *testing.T) {
	for _, identity := range []Identity{{"npm", "JSONStream"}, {"crates.io", "Tauri"}, {"Go", "git.sr.ht/~example/module"}} {
		got, err := Canonical(identity)
		if err != nil || got != identity {
			t.Fatalf("published spelling was rejected or changed: %+v -> %+v, %v", identity, got, err)
		}
	}
}
