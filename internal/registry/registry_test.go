package registry

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stateDir(t *testing.T) string { t.Helper(); return filepath.Join(t.TempDir(), "state") }
func testStore(t *testing.T, mode string) *Store {
	t.Helper()
	s, err := Open(stateDir(t), mode)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func fixture(t *testing.T, eco, name, id string, withdrawn bool) []byte {
	t.Helper()
	a := Advisory{ID: id, Modified: time.Now().UTC(), Affected: []Affected{{Package: Identity{eco, name}, Ranges: []Range{{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}, {Fixed: "2.0.0"}}}}}}}
	if withdrawn {
		now := time.Now().UTC()
		a.Withdrawn = &now
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func importRecords(t *testing.T, s *Store, eco string, records ...[]byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.json")
	body := bytes.Join(records, []byte(","))
	body = append(append([]byte("["), body...), ']')
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Import(context.Background(), eco, path, "synthetic", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}
func packageFor(eco, name, v string) Package {
	return Package{Identity: Identity{eco, name}, Version: v, Public: true}
}
func remoteFor(t *testing.T, s *Store) Remote {
	t.Helper()
	key, err := s.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	return Remote{URL: "https://registry.example", Installation: "test-install", PublicKey: key, Token: strings.Repeat("x", 43)}
}
func activate(t *testing.T, s, c *Store, ids []Identity) Envelope {
	t.Helper()
	r := remoteFor(t, s)
	env, err := s.Publish(context.Background(), SyncRequest{r.Installation, ids}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Activate(context.Background(), env, r, ids, time.Now()); err != nil {
		t.Fatal(err)
	}
	return env
}
func checkFinding(t *testing.T, c *Store, p Package, want string) {
	t.Helper()
	r, err := c.Check(p, time.Now())
	if err != nil || r.Finding != want {
		t.Fatalf("finding got %+v err=%v want %s", r, err, want)
	}
}

func TestLifecycleAllEcosystems(t *testing.T) {
	for _, tc := range []struct{ eco, name, id string }{{"npm", "example", "GHSA-synthetic-npm"}, {"PyPI", "example-pkg", "PYSEC-2099-1"}, {"crates.io", "example_crate", "RUSTSEC-2099-0001"}, {"Go", "example.org/module/v2", "GO-2099-0001"}} {
		t.Run(tc.eco, func(t *testing.T) {
			s, c := testStore(t, "server"), testStore(t, "client")
			v := "1.0.0"
			fixed := "2.0.0"
			if tc.eco == "Go" {
				v = "v2.0.0"
				fixed = "v2.1.0"
			}
			raw := fixture(t, tc.eco, tc.name, tc.id, false)
			if tc.eco == "Go" {
				raw = bytes.ReplaceAll(raw, []byte("2.0.0"), []byte("2.1.0"))
			}
			importRecords(t, s, tc.eco, raw)
			activate(t, s, c, []Identity{{tc.eco, tc.name}})
			checkFinding(t, c, packageFor(tc.eco, tc.name, v), "known_affected")
			checkFinding(t, c, packageFor(tc.eco, tc.name, fixed), "no_known_match")
			a := Advisory{}
			if err := json.Unmarshal(raw, &a); err != nil {
				t.Fatal(err)
			}
			a.Affected[0].Ranges = nil
			a.Affected[0].Versions = []string{v}
			raw, _ = json.Marshal(a)
			importRecords(t, s, tc.eco, raw)
			activate(t, s, c, []Identity{{tc.eco, tc.name}})
			checkFinding(t, c, packageFor(tc.eco, tc.name, v), "known_affected")
			now := time.Now().UTC()
			a.Withdrawn = &now
			raw, _ = json.Marshal(a)
			importRecords(t, s, tc.eco, raw)
			activate(t, s, c, []Identity{{tc.eco, tc.name}})
			checkFinding(t, c, packageFor(tc.eco, tc.name, v), "no_known_match")
		})
	}
}
func TestEmptyDisclosureCorrectionRemovalAndStale(t *testing.T) {
	s, c := testStore(t, "server"), testStore(t, "client")
	ids := []Identity{{"npm", "example"}}
	p := packageFor("npm", "example", "1.0.0")
	checkFinding(t, c, p, "indeterminate")
	activate(t, s, c, ids)
	checkFinding(t, c, p, "indeterminate")
	importRecords(t, s, "npm")
	activate(t, s, c, ids)
	checkFinding(t, c, p, "no_known_match")
	raw := fixture(t, "npm", "example", "GHSA-synthetic", false)
	importRecords(t, s, "npm", raw)
	activate(t, s, c, ids)
	checkFinding(t, c, p, "known_affected")
	result, err := c.Check(p, time.Now().Add(72*time.Hour))
	if err != nil || result.Finding != "known_affected" || result.Freshness != "stale" {
		t.Fatal(result, err)
	}
	raw = bytes.ReplaceAll(raw, []byte("2.0.0"), []byte("1.0.0"))
	importRecords(t, s, "npm", raw)
	activate(t, s, c, ids)
	checkFinding(t, c, p, "no_known_match")
	importRecords(t, s, "npm")
	activate(t, s, c, ids)
	checkFinding(t, c, p, "no_known_match")
	result, err = c.Check(p, time.Now().Add(72*time.Hour))
	if err != nil || result.Finding != "indeterminate" {
		t.Fatal(result, err)
	}
	// A newly signed stale feed must not refresh source observation time.
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.Import(context.Background(), "npm", path, "old-export", time.Now().Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	activate(t, s, c, ids)
	checkFinding(t, c, p, "indeterminate")
}
func TestInterruptedMalformedAndUnlicensedImports(t *testing.T) {
	s, c := testStore(t, "server"), testStore(t, "client")
	raw := fixture(t, "npm", "example", "GHSA-synthetic", false)
	importRecords(t, s, "npm", raw)
	for _, body := range [][]byte{[]byte("["), append(append([]byte("["), raw...), []byte(",null]")...)} {
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.Import(context.Background(), "npm", path, "broken", time.Now()); err == nil {
			t.Fatal("accepted broken export")
		}
		activate(t, s, c, []Identity{{"npm", "example"}})
		r, err := c.Check(packageFor("npm", "example", "1.0.0"), time.Now())
		if err != nil || r.Finding != "known_affected" || r.Freshness != "stale" {
			t.Fatal(r, err)
		}
	}
	path := filepath.Join(t.TempDir(), "cancel.json")
	if err := os.WriteFile(path, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Import(ctx, "npm", path, "cancelled", time.Now()); err == nil {
		t.Fatal("cancelled import succeeded")
	}
	unknown := fixture(t, "npm", "other", "MAL-2099-1", false)
	importRecords(t, s, "npm", raw, unknown)
	activate(t, s, c, []Identity{{"npm", "empty"}})
	checkFinding(t, c, packageFor("npm", "empty", "1.0.0"), "indeterminate")
}
func TestZIPAndRawRecordIntegrity(t *testing.T) {
	s, c := testStore(t, "server"), testStore(t, "client")
	raw := append([]byte(" \n"), fixture(t, "npm", "example", "GHSA-synthetic", false)...)
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, err := z.Create("record.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "all.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Import(context.Background(), "npm", path, "zip-test", time.Now()); err != nil {
		t.Fatal(err)
	}
	env := activate(t, s, c, []Identity{{"npm", "example"}})
	var snap Snapshot
	if err := json.Unmarshal(env.Payload, &snap); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snap.Scopes[0].Records[0].Raw, raw) {
		t.Fatal("raw bytes changed")
	}
}
func TestSnapshotVerificationAndRestartReplay(t *testing.T) {
	s := testStore(t, "server")
	dir := stateDir(t)
	c, err := Open(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	importRecords(t, s, "npm")
	ids := []Identity{{"npm", "example"}}
	remote := remoteFor(t, s)
	env := activate(t, s, c, ids)
	key, _ := base64.StdEncoding.DecodeString(remote.PublicKey)
	wrong := append([]byte{}, key...)
	wrong[0] ^= 1
	tests := []struct {
		name         string
		env          Envelope
		key          []byte
		installation string
		ids          []Identity
		now          time.Time
	}{
		{"wrong-key", env, wrong, remote.Installation, ids, time.Now()},
		{"wrong-installation", env, key, "other", ids, time.Now()},
		{"wrong-scope", env, key, remote.Installation, []Identity{{"npm", "other"}}, time.Now()},
		{"expired", env, key, remote.Installation, ids, time.Now().Add(7 * time.Hour)},
	}
	bad := env
	bad.Payload = append([]byte{}, env.Payload...)
	bad.Payload[1] ^= 1
	tests = append(tests, struct {
		name         string
		env          Envelope
		key          []byte
		installation string
		ids          []Identity
		now          time.Time
	}{"tamper", bad, key, remote.Installation, ids, time.Now()})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(tc.env, ed25519.PublicKey(tc.key), tc.installation, tc.ids, tc.now); err == nil {
				t.Fatal("accepted invalid snapshot")
			}
		})
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	c, err = Open(dir, "client")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Activate(context.Background(), env, remote, ids, time.Now()); err == nil {
		t.Fatal("restart forgot replay floor")
	}
	checkFinding(t, c, packageFor("npm", "example", "1.0.0"), "no_known_match")
	newer, err := s.Publish(context.Background(), SyncRequest{remote.Installation, ids}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	wrongRemote := remote
	wrongRemote.URL = "https://other.example"
	if err = c.Activate(context.Background(), newer, wrongRemote, ids, time.Now()); err == nil {
		t.Fatal("trust pin changed")
	}
	if err = c.Activate(context.Background(), newer, remote, ids, time.Now()); err != nil {
		t.Fatal(err)
	}
}
func TestPrivacyAuthAndFirstSeen(t *testing.T) {
	s, c := testStore(t, "server"), testStore(t, "client")
	public := packageFor("npm", "example", "1.0.0")
	private := packageFor("npm", "private-name", "9.9.9")
	private.Public = false
	if err := c.SetInventory(Inventory{Packages: []Package{public, private}}); err != nil {
		t.Fatal(err)
	}
	ids, err := c.Subscriptions()
	if err != nil || len(ids) != 1 || ids[0] != public.Identity {
		t.Fatal(ids, err)
	}
	if err = c.Subscribe(private); err == nil {
		t.Fatal("private subscribed")
	}
	if err = c.Subscribe(packageFor("npm", "private-name", "1.0.0")); err == nil {
		t.Fatal("private override")
	}
	// Explicit candidates that later become private are removed from outgoing subscriptions.
	candidate := packageFor("npm", "candidate", "1.0.0")
	if err = c.Subscribe(candidate); err != nil {
		t.Fatal(err)
	}
	candidate.Public = false
	if err = c.SetInventory(Inventory{Packages: []Package{public, private, candidate}}); err != nil {
		t.Fatal(err)
	}
	ids, _ = c.Subscriptions()
	if len(ids) != 1 {
		t.Fatal(ids)
	}
	importRecords(t, s, "npm", fixture(t, "npm", "example", "GHSA-synthetic", false))
	path := filepath.Join(t.TempDir(), "token")
	if err = s.Enroll("test-install", path); err != nil {
		t.Fatal(err)
	}
	tok, _ := ReadSecret(path)
	r := remoteFor(t, s)
	r.Token = string(tok)
	var observed []byte
	handler := s.Handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		observed = b
		req.Body = io.NopCloser(bytes.NewReader(b))
		handler.ServeHTTP(w, req)
	}))
	defer srv.Close()
	r.URL = srv.URL
	r.AllowHTTP = true
	if err = c.Sync(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"1.0.0", "private-name", "9.9.9", "candidate", "graph_complete"} {
		if bytes.Contains(observed, []byte(secret)) {
			t.Fatalf("leaked %s", secret)
		}
	}
	checkFinding(t, c, public, "known_affected")
	checkFinding(t, c, packageFor("npm", "new", "1.0.0"), "indeterminate")
	if err = c.Subscribe(packageFor("npm", "new", "1.0.0")); err != nil {
		t.Fatal(err)
	}
	if err = c.Sync(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	checkFinding(t, c, packageFor("npm", "new", "1.0.0"), "no_known_match")
	bad := r
	bad.Token = strings.Repeat("z", 43)
	if err = c.Sync(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatal(err)
	}
	bad = r
	bad.Installation = "other-install"
	if err = c.Sync(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "trust pin mismatch") {
		t.Fatal(err)
	}
	rotated := filepath.Join(t.TempDir(), "rotated")
	if err = s.Enroll("test-install", rotated); err != nil {
		t.Fatal(err)
	}
	if err = c.Sync(context.Background(), r); err == nil {
		t.Fatal("old token survived rotation")
	}
	srv.Close()
	if err = c.Sync(context.Background(), r); err == nil {
		t.Fatal("offline succeeded")
	}
	checkFinding(t, c, public, "known_affected")
}
func TestRoleConfigLocalHTTPAndShutdown(t *testing.T) {
	dir := stateDir(t)
	s, err := Open(dir, "server")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if other, err := Open(dir, "client"); err == nil {
		if err := other.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("cross-role open")
	}
	for _, c := range []Config{{Mode: "bogus", Dir: t.TempDir(), Listen: "127.0.0.1:0", AllowHTTP: true, Interval: time.Hour}, {Mode: "server", Dir: t.TempDir(), Listen: "0.0.0.0:0", AllowHTTP: true, Interval: time.Hour}, {Mode: "server", Dir: t.TempDir(), Listen: "127.0.0.1:0", Interval: time.Hour}} {
		if err = c.Validate(); err == nil {
			t.Fatal("bad config accepted", c)
		}
	}
	svc, err := Start(context.Background(), Config{Mode: "client", Dir: stateDir(t), Listen: "127.0.0.1:0", AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://" + svc.Addr.String()
	resp, err := http.Get(endpoint + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatal(resp.StatusCode)
	}
	tok, _ := svc.Store.LocalToken()
	req, _ := http.NewRequest("GET", endpoint+"/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	body := `{"ecosystem":"PyPI","name":"example","version":"1.0rc1","public":true}`
	req, _ = http.NewRequest("POST", endpoint+"/v1/check", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if result.Finding != "indeterminate" {
		t.Fatal(result)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = svc.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if resp, err = http.Get(endpoint + "/v1/status"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("listener remains")
	}
}

func TestTLSAndRedirectGuards(t *testing.T) {
	// Borrow an httptest-generated certificate, then exercise real TLS listener.
	certServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	cert := certServer.TLS.Certificates[0]
	client := certServer.Client()
	certServer.Close()
	certPath, keyPath := filepath.Join(t.TempDir(), "cert.pem"), filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	svc, err := Start(context.Background(), Config{Mode: "client", Dir: stateDir(t), Listen: "127.0.0.1:0", TLSCert: certPath, TLSKey: keyPath})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := svc.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	tok, _ := svc.Store.LocalToken()
	req, _ := http.NewRequest("GET", "https://"+svc.Addr.String()+"/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	client.Transport.(*http.Transport).TLSClientConfig.MinVersion = tls.VersionTLS13
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	// Production Remote uses normal root verification. A self-signed peer fails.
	r := Remote{URL: "https://" + svc.Addr.String(), Installation: "test", Token: strings.Repeat("x", 43), PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32))}
	if err = svc.Store.Sync(context.Background(), r); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	r.URL = redirect.URL
	r.AllowHTTP = true
	if err = svc.Store.Sync(context.Background(), r); err == nil {
		t.Fatal("redirect accepted")
	}
	if reached {
		t.Fatal("redirect target reached")
	}
}

func TestSignedInvalidPayloadsAndAtomicActivation(t *testing.T) {
	s, c := testStore(t, "server"), testStore(t, "client")
	importRecords(t, s, "npm")
	ids := []Identity{{"npm", "example"}}
	remote := remoteFor(t, s)
	activate(t, s, c, ids)
	env, err := s.Publish(context.Background(), SyncRequest{remote.Installation, ids}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	key, err := s.signingKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.Revision = -1 },
		func(s *Snapshot) { s.Revision = 0 },
		func(s *Snapshot) { s.Scopes[0].Health.Checked = time.Now().Add(time.Hour) },
		func(s *Snapshot) { s.Scopes[0].Identity.Name = "other" },
		func(s *Snapshot) { s.Scopes[0].Health.Checked = time.Time{} },
		func(s *Snapshot) { s.Expires = s.Issued.Add(24 * time.Hour) },
	} {
		var snap Snapshot
		if err = json.Unmarshal(env.Payload, &snap); err != nil {
			t.Fatal(err)
		}
		mutate(&snap)
		b, _ := json.Marshal(snap)
		bad := Envelope{Payload: b, Signature: ed25519.Sign(key, b), SHA256: digest(b)}
		if err = c.Activate(context.Background(), bad, remote, ids, time.Now()); err == nil {
			t.Fatal("invalid signed payload activated")
		}
		checkFinding(t, c, packageFor("npm", "example", "1.0.0"), "no_known_match")
	}
	// An activation write failure rolls back scope and replay floors together.
	if _, err = c.db.Exec("CREATE TRIGGER fail_scopes BEFORE INSERT ON scopes BEGIN SELECT RAISE(ABORT,'synthetic disk failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err = c.Activate(context.Background(), env, remote, ids, time.Now()); err == nil {
		t.Fatal("write failure ignored")
	}
	checkFinding(t, c, packageFor("npm", "example", "1.0.0"), "no_known_match")
	if _, err = c.db.Exec("DROP TRIGGER fail_scopes"); err != nil {
		t.Fatal(err)
	}
	if err = c.Activate(context.Background(), env, remote, ids, time.Now()); err != nil {
		t.Fatal("failed activation advanced floor", err)
	}
}

func TestMalformedRequestsAndBounds(t *testing.T) {
	s := testStore(t, "server")
	path := filepath.Join(t.TempDir(), "token")
	if err := s.Enroll("test", path); err != nil {
		t.Fatal(err)
	}
	tok, _ := ReadSecret(path)
	handler := s.Handler()
	for _, body := range []string{`{"installation":"test","packages":[{"ecosystem":"npm","name":"example","version":"1.0.0"}]}`, `{"installation":"test","packages":[{"ecosystem":"npm","name":"example"},{"ecosystem":"npm","name":"example"}]}`, `{"installation":"test","packages":[{"ecosystem":"PyPI","name":"Foo_Bar"}]}`, strings.Repeat("x", MaxRecord+1)} {
		req := httptest.NewRequest("POST", "/v1/snapshot", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+string(tok))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if _, err := Open(stateDir(t), "mirror"); err == nil {
		t.Fatal("bad role accepted")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir, "server"); err == nil {
		t.Fatal("loose state dir accepted")
	}
	file := filepath.Join(t.TempDir(), "loose")
	if err := os.WriteFile(file, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSecret(file); err == nil {
		t.Fatal("loose secret accepted")
	}
}
