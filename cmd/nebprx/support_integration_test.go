package main

// Child test processes run production HTTP gates, OIDC, SQLite and WebSockets.
// No production endpoint accepts test identities or authentication bypasses.
import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/provider"
	"github.com/Nebuless/nebprx/internal/supporttest"
	"github.com/google/uuid"
)

type processFixture struct {
	Dir, Provider                  string
	IDP                            string `json:"IdP"`
	Gateways, Backends, Registries [2]string
}

func fixtureWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func fixtureJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	fixtureWrite(t, path, b)
}
func fixtureClient(t *testing.T, f processFixture) *http.Client {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(f.Dir, "tls.crt"))
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(b) {
		t.Fatal("fixture CA unavailable")
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(addr)
		if strings.HasSuffix(host, ".support.fixture.test") {
			addr = strings.TrimPrefix(f.Provider, "https://")
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	t.Cleanup(tr.CloseIdleConnections)
	jar, e := cookiejar.New(nil)
	if e != nil {
		t.Fatal(e)
	}
	return &http.Client{Transport: tr, Jar: jar, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func fixtureCertificate(t *testing.T, dir string) {
	t.Helper()
	k, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "PRX synthetic fixture"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3"), net.ParseIP("127.0.0.4")}, DNSNames: []string{"*.support.fixture.test"}}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &k.PublicKey, k)
	if e != nil {
		t.Fatal(e)
	}
	fixtureWrite(t, filepath.Join(dir, "tls.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	fixtureWrite(t, filepath.Join(dir, "tls.key"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
}
func fixturePort(t *testing.T, host string) string {
	t.Helper()
	l, e := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Error(err)
		}
	}()
	return "https://" + l.Addr().String()
}
func fixtureStart(t *testing.T, f processFixture, role string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSupportProcessHelper$", "-test.timeout=20m")
	cmd.Env = append(os.Environ(), "PRX_SUPPORT_CHILD="+role, "PRX_SUPPORT_DIR="+f.Dir, "SSL_CERT_FILE="+filepath.Join(f.Dir, "tls.crt"), "NEBPRX_TELEMETRY=0")
	log, e := os.OpenFile(filepath.Join(f.Dir, role+".log"), os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if e = cmd.Start(); e != nil {
		if err := log.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = log.Close() })
	return cmd
}
func fixtureRequest(t *testing.T, client *http.Client, method, target, origin, kind string, body io.Reader) *http.Response {
	t.Helper()
	r, e := http.NewRequest(method, target, body)
	if e != nil {
		t.Fatal(e)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if kind != "" {
		r.Header.Set("Content-Type", kind)
	}
	resp, e := client.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}
func fixtureDo(t *testing.T, client *http.Client, method, target, origin, kind string, body io.Reader, status int, v any) []byte {
	t.Helper()
	resp := fixtureRequest(t, client, method, target, origin, kind, body)
	defer func() { _ = resp.Body.Close() }()
	return fixtureRead(t, resp, status, v)
}
func fixtureRead(t *testing.T, resp *http.Response, status int, v any) []byte {
	t.Helper()
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if e != nil {
		t.Fatal(e)
	}
	if resp.StatusCode != status {
		t.Fatalf("HTTP %s %s status=%d want=%d (body suppressed)", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, status)
	}
	if v != nil && json.Unmarshal(b, v) != nil {
		t.Fatal("invalid response JSON")
	}
	return b
}
func TestSupportProcessHelper(t *testing.T) {
	role := os.Getenv("PRX_SUPPORT_CHILD")
	if role == "" {
		t.Skip("fixture subprocess only")
	}
	dir := os.Getenv("PRX_SUPPORT_DIR")
	var f processFixture
	b, e := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if e != nil || json.Unmarshal(b, &f) != nil {
		t.Fatal("fixture config unavailable")
	}
	var handler http.Handler
	var address string
	switch role {
	case "idp":
		mock, e := supporttest.NewIDP()
		if e != nil {
			t.Fatal(e)
		}
		mock.Issuer = f.IDP
		mock.RedirectURIs = []string{f.Provider + "/__provider/oidc/callback"}
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, inactive := os.Stat(filepath.Join(dir, "idp-inactive"))
			mock.SetInactive(inactive == nil)
			mock.ServeHTTP(w, r)
		})
		address = f.IDP
	case "provider":
		if e = providerCommand([]string{"--state-dir", filepath.Join(dir, "provider"), "--public-url", f.Provider, "--support-domain", "support.fixture.test", "--listen", strings.TrimPrefix(f.Provider, "https://"), "--tls-cert", filepath.Join(dir, "tls.crt"), "--tls-key", filepath.Join(dir, "tls.key"), "--oidc-config-file", filepath.Join(dir, "oidc.json")}, io.Discard); e != nil {
			t.Fatal(e)
		}
		return
	case "gateway-a", "gateway-b":
		index := 0
		if role == "gateway-b" {
			index = 1
		}
		state := filepath.Join(dir, role)
		if e = supportCommand([]string{"--action", "enroll", "--state-dir", state, "--config-file", filepath.Join(state, "support.json"), "--enrollment-secret-file", filepath.Join(dir, role+".enroll")}, io.Discard); e != nil {
			t.Fatal(e)
		}
		runGateway([]string{"--support-config-file", filepath.Join(state, "support.json"), "--registry-mode", "client", "--registry-state-dir", filepath.Join(state, "registry"), "--registry-listen", strings.TrimPrefix(f.Registries[index], "https://"), "--registry-tls-cert", filepath.Join(dir, "tls.crt"), "--registry-tls-key", filepath.Join(dir, "tls.key"), filepath.Join(state, "gateway.hcl")})
		return
	case "proxy-a", "proxy-b":
		index := 0
		if role == "proxy-b" {
			index = 1
		}
		address = f.Gateways[index]
		target, e := url.Parse(strings.Replace(f.Backends[index], "https://", "http://", 1))
		if e != nil {
			t.Fatal(e)
		}
		handler = httputil.NewSingleHostReverseProxy(target)
	default:
		t.Fatal("unknown fixture role")
	}
	server := &http.Server{Addr: strings.TrimPrefix(address, "https://"), Handler: handler, ReadHeaderTimeout: 5 * time.Second, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13}}
	if e = server.ListenAndServeTLS(filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")); e != nil {
		t.Fatal(e)
	}
}
func TestSupportProcessAcceptance(t *testing.T) {
	dir := t.TempDir()
	if keep := os.Getenv("PRX_SUPPORT_FIXTURE_DIR"); keep != "" {
		dir = keep
		if e := os.Mkdir(dir, 0700); e != nil {
			t.Fatal(e)
		}
	}
	f := processFixture{Dir: dir, Provider: fixturePort(t, "127.0.0.1"), IDP: fixturePort(t, "127.0.0.4"), Gateways: [2]string{fixturePort(t, "127.0.0.2"), fixturePort(t, "127.0.0.3")}, Backends: [2]string{fixturePort(t, "127.0.0.1"), fixturePort(t, "127.0.0.1")}, Registries: [2]string{fixturePort(t, "127.0.0.1"), fixturePort(t, "127.0.0.1")}}
	fixtureCertificate(t, dir)
	fixtureJSON(t, filepath.Join(dir, "fixture.json"), f)
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	fixtureWrite(t, filepath.Join(dir, "provider.key"), []byte(base64.StdEncoding.EncodeToString(key)))
	fixtureWrite(t, filepath.Join(dir, "client.secret"), []byte("synthetic-client-secret"))
	fixtureWrite(t, filepath.Join(dir, "intro.secret"), []byte("synthetic-introspection-secret"))
	fixtureJSON(t, filepath.Join(dir, "oidc.json"), provider.OIDCConfig{Issuer: f.IDP, ClientID: "prx-fixture", ClientSecretFile: filepath.Join(dir, "client.secret"), AuthorizationEndpoint: f.IDP + "/authorize", TokenEndpoint: f.IDP + "/token", JWKSEndpoint: f.IDP + "/jwks", IntrospectionEndpoint: f.IDP + "/introspect", IntrospectionClientID: "prx-introspection", IntrospectionClientSecretFile: filepath.Join(dir, "intro.secret"), IntrospectionAuthMethod: "client_secret_basic", AllowedSigningAlgorithms: []string{"RS256"}, MFAACRValues: []string{"urn:prx:test:mfa"}})
	if e = os.MkdirAll(filepath.Join(dir, "provider"), 0700); e != nil {
		t.Fatal(e)
	}
	fixtureWrite(t, filepath.Join(dir, "provider", "support-signing.key"), []byte(base64.StdEncoding.EncodeToString(key)))
	store, e := provider.Open(filepath.Join(dir, "provider", "provider.db"))
	if e != nil {
		t.Fatal(e)
	}
	if e = store.AddTechnician("tech", f.IDP, "technician", "Alex Technician", true); e != nil {
		t.Fatal(e)
	}
	if e = store.AddTechnician("other", f.IDP, "other", "Other Technician", false); e != nil {
		t.Fatal(e)
	}
	for j, role := range []string{"gateway-a", "gateway-b"} {
		if e = store.AddOrganization(role, role); e != nil {
			t.Fatal(e)
		}
		if e = store.AssignOrganization("other", role); e != nil {
			t.Fatal(e)
		}
		if e = store.AssignOrganization("tech", role); e != nil {
			t.Fatal(e)
		}
		hosting := "customer"
		if j == 1 {
			hosting = "nebuless"
		}
		if e = store.RegisterInstallation(provider.Installation{ID: role, OrganizationID: role, Name: role, Hosting: hosting}); e != nil {
			t.Fatal(e)
		}
		secret, e := store.Enrollment(role, role)
		if e != nil {
			t.Fatal(e)
		}
		fixtureWrite(t, filepath.Join(dir, role+".enroll"), []byte(secret))
		state := filepath.Join(dir, role)
		if e = os.MkdirAll(state, 0700); e != nil {
			t.Fatal(e)
		}
		db, e := OpenDB(filepath.Join(state, "nebprx.db"))
		if e != nil {
			t.Fatal(e)
		}
		if e = setDashboardUser(db, "root", "synthetic-password"); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec("UPDATE installation_identity SET id=?,organization_id=? WHERE singleton=1", role, role); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec("INSERT INTO devices(id,profile,created_ns,last_seen_ns) VALUES('10.55.0.2','default',1,1),('2001:db8::2','default',1,1)"); e != nil {
			t.Fatal(e)
		}
		account, err := resolveDashboardAccount(db, "password", "local", "root", "root")
		if err != nil {
			t.Fatal(err)
		}
		auditStore, err := newSupportStore(db, role, role, make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
		for n := 0; n < 105; n++ {
			if err = auditStore.transaction(func(tx *sql.Tx) error {
				return auditStore.customerAudit(tx, account, provider.Case{ID: "fixture-case"}, "", "", "fixture_event", "fixture_metadata")
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := auditStore.Close(); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		_, installationKey, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		fixtureWrite(t, filepath.Join(state, "installation.key"), []byte(base64.StdEncoding.EncodeToString(installationKey)))
		pepper := make([]byte, 32)
		if _, e = rand.Read(pepper); e != nil {
			t.Fatal(e)
		}
		fixtureWrite(t, filepath.Join(state, "pepper.key"), []byte(base64.StdEncoding.EncodeToString(pepper)))
		fixtureWrite(t, filepath.Join(state, "provider.pub"), []byte(base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))))
		fixtureJSON(t, filepath.Join(state, "support.json"), supportFile{Enabled: true, Broker: f.Provider, Origin: f.Provider, Domain: "support.fixture.test", Organization: role, Installation: role, Hosting: hosting, CredentialFile: filepath.Join(state, "connector.secret"), KeyFile: filepath.Join(state, "installation.key"), ProviderKeyFile: filepath.Join(state, "provider.pub"), PepperFile: filepath.Join(state, "pepper.key"), Retention: 30})
		_, port, e := net.SplitHostPort(strings.TrimPrefix(fixturePort(t, "127.0.0.1"), "https://"))
		if e != nil {
			t.Fatal(e)
		}
		wgPort, e := strconv.Atoi(port)
		if e != nil {
			t.Fatal(e)
		}
		_, hostPort, e := net.SplitHostPort(strings.TrimPrefix(fixturePort(t, "127.0.0.1"), "https://"))
		if e != nil {
			t.Fatal(e)
		}
		src := fmt.Sprintf("schema_version = 1\ngateway {\n state_dir = %q\n dashboard_listen = %q\n public_url = %q\n telemetry = false\n dashboard_config_writes = true\n wireguard {\n subnet_cidr = %q\n endpoint = %q\n listen_port = %d\n host_loopback_port = %s\n }\n}\nendpoint \"https\" \"public\" { hosts = [\"public.example\"] }\ncredential \"bearer_token\" \"token\" { endpoint = https.public }\nprofile \"default\" { credentials = [bearer_token.token] }\nprofile \"limited\" { credentials = [] }\n", state, strings.TrimPrefix(f.Backends[j], "https://"), f.Gateways[j], "10.55.0.0/24", "127.0.0.1:"+port, wgPort, hostPort)
		fixtureWrite(t, filepath.Join(state, "gateway.hcl"), []byte(src))
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	idpProcess := fixtureStart(t, f, "idp")
	providerProcess := fixtureStart(t, f, "provider")
	client := fixtureClient(t, f)
	wait := func(target string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			resp, e := client.Get(target)
			if e == nil {
				if err := resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode < 500 {
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatal("fixture process did not start")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	wait(f.IDP + "/jwks")
	wait(f.Provider)
	gatewayProcess := fixtureStart(t, f, "gateway-a")
	fixtureStart(t, f, "gateway-b")
	fixtureStart(t, f, "proxy-a")
	fixtureStart(t, f, "proxy-b")
	for _, g := range f.Gateways {
		wait(g + "/__login")
	}
	resp := fixtureRequest(t, client, "GET", f.Provider+"/__provider/oidc/login", "", "", nil)
	location := resp.Header.Get("Location")
	fixtureRead(t, resp, 303, nil)
	_ = resp.Body.Close()
	resp = fixtureRequest(t, client, "POST", location, f.IDP, "application/x-www-form-urlencoded", strings.NewReader("subject=technician"))
	callback := resp.Header.Get("Location")
	fixtureRead(t, resp, 303, nil)
	_ = resp.Body.Close()
	fixtureDo(t, client, "GET", callback, "", "", nil, 303, nil)
	other := fixtureClient(t, f)
	otherResponse := fixtureRequest(t, other, "GET", f.Provider+"/__provider/oidc/login", "", "", nil)
	otherLocation := otherResponse.Header.Get("Location")
	fixtureRead(t, otherResponse, 303, nil)
	_ = otherResponse.Body.Close()
	otherResponse = fixtureRequest(t, other, "POST", otherLocation, f.IDP, "application/x-www-form-urlencoded", strings.NewReader("subject=other"))
	otherCallback := otherResponse.Header.Get("Location")
	fixtureRead(t, otherResponse, 303, nil)
	_ = otherResponse.Body.Close()
	fixtureDo(t, other, "GET", otherCallback, "", "", nil, 303, nil)
	for j, role := range []string{"gateway-a", "gateway-b"} {
		gateway := f.Gateways[j]
		fixtureDo(t, client, "POST", gateway+"/__login", gateway, "application/x-www-form-urlencoded", strings.NewReader("password=synthetic-password"), 302, nil)
		var status struct {
			Available bool
			Accounts  []dashboardAccount
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			fixtureDo(t, client, "GET", gateway+"/api/support", "", "", nil, 200, &status)
			if status.Available {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("connector unavailable")
			}
			time.Sleep(50 * time.Millisecond)
		}
		var health map[string]string
		fixtureDo(t, client, "GET", f.Provider+"/__provider/installation?organization="+role+"&installation="+role, "", "", nil, 200, &health)
		if health["connector"] != "connected" || health["registry"] != "local_only" {
			t.Fatal("management health missing")
		}
		fixtureDo(t, client, "PATCH", gateway+"/api/account/context", gateway, "application/json", strings.NewReader("{\"revision\":0,\"context\":{\"profile\":\"limited\",\"theme\":\"dark\"}}"), 200, nil)
		registryToken, err := os.ReadFile(filepath.Join(dir, role, "registry", "local.token"))
		if err != nil {
			t.Fatal(err)
		}
		connectorToken, err := os.ReadFile(filepath.Join(dir, role, "connector.secret"))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{strings.TrimSpace(string(registryToken)), strings.TrimSpace(string(connectorToken)), base64.StdEncoding.EncodeToString(key)} {
			isolated := fixtureClient(t, f)
			req, _ := http.NewRequest("GET", f.Provider+"/__provider/session", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			response, err := isolated.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			fixtureRead(t, response, 401, nil)
			_ = response.Body.Close()
			req, _ = http.NewRequest("GET", f.Provider+"/__connector/connect?organization="+role+"&installation="+role, nil)
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(registryToken)))
			response, err = isolated.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			fixtureRead(t, response, 403, nil)
			_ = response.Body.Close()
			req, _ = http.NewRequest("GET", f.Registries[j]+"/v1/status", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			response, err = isolated.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			expected := 401
			if token == strings.TrimSpace(string(registryToken)) {
				expected = 200
			}
			fixtureRead(t, response, expected, nil)
			_ = response.Body.Close()
			req, _ = http.NewRequest("GET", gateway+"/api/state", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			response, err = isolated.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			fixtureRead(t, response, 401, nil)
			_ = response.Body.Close()
		}
		payload, _ := json.Marshal(map[string]string{"intent_id": uuid.NewString(), "summary": "Synthetic case", "category": "connection"})
		var c provider.Case
		fixtureDo(t, client, "POST", gateway+"/api/support/cases", gateway, "application/json", bytes.NewReader(payload), 200, &c)
		query := "?organization=" + role + "&installation=" + role
		payload, _ = json.Marshal(map[string]string{"Case": c.ID, "Technician": "tech"})
		fixtureDo(t, client, "POST", f.Provider+"/__provider/assign"+query, f.Provider, "application/json", bytes.NewReader(payload), 200, nil)
		fixtureDo(t, client, "GET", gateway+"/api/support", "", "", nil, 200, &status)
		payload, _ = json.Marshal(map[string]any{"case_id": c.ID, "target_account_id": status.Accounts[0].ID, "assignment_revision": 2})
		var grant struct{ Code string }
		fixtureDo(t, client, "POST", gateway+"/api/support/grants", gateway, "application/json", bytes.NewReader(payload), 200, &grant)
		payload, _ = json.Marshal(map[string]string{"Case": c.ID, "Code": grant.Code})
		fixtureDo(t, other, "POST", f.Provider+"/__provider/redeem"+query, f.Provider, "application/json", bytes.NewReader(payload), 403, nil)
		forged, _ := http.NewRequest("POST", f.Provider+"/__provider/redeem"+query, bytes.NewReader(payload))
		forged.Header.Set("Origin", f.Provider)
		forged.Header.Set("X-Forwarded-User", "tech")
		forged.Header.Set("X-PRX-Actor-ID", "tech")
		forgedResponse, err := other.Do(forged)
		if err != nil {
			t.Fatal(err)
		}
		fixtureRead(t, forgedResponse, 403, nil)
		_ = forgedResponse.Body.Close()
		fixtureDo(t, client, "GET", f.Provider+"/__provider/installation?organization=foreign&installation="+role, "", "", nil, 403, nil)
		var page []byte
		var wins atomic.Int32
		var group sync.WaitGroup
		for range 100 {
			group.Go(func() {
				req, err := http.NewRequest("POST", f.Provider+"/__provider/redeem"+query, bytes.NewReader(payload))
				if err != nil {
					t.Error(err)
					return
				}
				req.Header.Set("Origin", f.Provider)
				req.Header.Set("Content-Type", "application/json")
				response, err := client.Do(req)
				if err != nil {
					t.Error("concurrent redeem transport failed")
					return
				}
				defer func() {
					if err := response.Body.Close(); err != nil {
						t.Error(err)
					}
				}()
				body, err := io.ReadAll(io.LimitReader(response.Body, 65536))
				if err != nil {
					t.Error(err)
					return
				}
				if response.StatusCode == 200 {
					wins.Add(1)
					page = body
				} else if response.StatusCode != 403 && response.StatusCode != 429 {
					t.Errorf("unexpected concurrent status=%d", response.StatusCode)
				}
			})
		}
		group.Wait()
		if wins.Load() != 1 {
			t.Fatalf("process redeem winners=%d", wins.Load())
		}
		fixtureDo(t, client, "POST", f.Provider+"/__provider/redeem"+query, f.Provider, "application/json", bytes.NewReader(payload), 403, nil)
		origins := regexp.MustCompile("action='(https://[^']+)/__support/activate'").FindSubmatch(page)
		tokens := regexp.MustCompile("name='handoff' value='([^']+)'").FindSubmatch(page)
		if len(origins) != 2 || len(tokens) != 2 {
			t.Fatal("handoff form missing")
		}
		origin := string(origins[1])
		values := url.Values{"handoff": {string(tokens[1])}}
		resp = fixtureRequest(t, client, "POST", origin+"/__support/activate", f.Provider, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
		for _, cookie := range resp.Cookies() {
			if cookie.Name == cpSessionCookieName || cookie.Domain != "" || !cookie.Secure || !cookie.HttpOnly {
				t.Fatal("cookie isolation failure")
			}
		}
		fixtureRead(t, resp, 200, nil)
		_ = resp.Body.Close()
		var data map[string]json.RawMessage
		fixtureDo(t, client, "GET", origin+"/api/state", "", "", nil, 200, &data)
		var identity map[string]any
		if json.Unmarshal(data["support"], &identity) != nil || identity["actor_id"] != "tech" || identity["effective_id"] != status.Accounts[0].ID || identity["installation_id"] != role {
			t.Fatal("dual identity mismatch")
		}
		var saved struct {
			Profile string `json:"profile"`
			Theme   string `json:"theme"`
		}
		if json.Unmarshal(data["account_context"], &saved) != nil || saved.Profile != "limited" || saved.Theme != "dark" {
			t.Fatal("saved customer context missing")
		}
		var feature bool
		if json.Unmarshal(data["customer_dashboard_config_writes"], &feature) != nil || !feature {
			t.Fatal("customer feature flag lost")
		}
		fixtureDo(t, client, "POST", origin+"/api/agents/profile?ip=10.55.0.2&profile=limited", origin, "application/json", strings.NewReader("{}"), 200, nil)
		fixtureDo(t, client, "POST", gateway+"/api/agents/profile?ip=10.55.0.2&profile=default", gateway, "application/json", strings.NewReader("{}"), 200, nil)
		for _, malicious := range []struct{ method, target, source string }{
			{"GET", origin + "/__support/activate", ""},
			{"POST", origin + "/__support/activate", "https://evil.example"},
			{"POST", origin + "/__support/end", "https://evil.example"},
			{"POST", origin + "/__support/end", f.Provider},
		} {
			fixtureDo(t, client, malicious.method, malicious.target, malicious.source, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()), 403, nil)
		}
		fixtureDo(t, client, "POST", origin+"/__support/activate", f.Provider, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()), 403, nil)
		anonymous := fixtureClient(t, f)
		fixtureDo(t, anonymous, "GET", origin+"/api/state", "", "", nil, 401, nil)
		fixtureDo(t, client, "GET", "https://sibling.support.fixture.test/api/state", "", "", nil, 401, nil)
		request, _ := http.NewRequest("GET", f.Provider+"/__provider/session", nil)
		request.Host = "evil.example"
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		fixtureRead(t, response, 421, nil)
		_ = response.Body.Close()
		fixtureDo(t, client, "GET", origin+"/api/config", "", "", nil, 403, nil)
		fixtureDo(t, client, "GET", origin+"/api/state", "", "", nil, 200, nil)
		fixtureDo(t, client, "POST", origin+"/__support/end", origin, "", nil, 200, nil)
		fixtureDo(t, client, "GET", origin+"/api/state", "", "", nil, 401, nil)
	}
	fixtureJSON(t, filepath.Join(dir, "result.json"), map[string]any{"status": "passed", "processes": 7, "hosting": []string{"customer", "nebuless"}, "credentials": "runtime-only; response bodies suppressed"})
	if script := os.Getenv("PRX_SUPPORT_BROWSER_SCRIPT"); script != "" {
		cmd := exec.Command("node", script, filepath.Join(dir, "fixture.json"))
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatal("browser acceptance failed:", err)
		}
	}
	if os.Getenv("PRX_SUPPORT_FIXTURE_HOLD") != "1" {
		localCheck := func() {
			isolated := fixtureClient(t, f)
			fixtureDo(t, isolated, "POST", f.Gateways[1]+"/__login", f.Gateways[1], "application/x-www-form-urlencoded", strings.NewReader("password=synthetic-password"), 302, nil)
			fixtureDo(t, isolated, "GET", f.Gateways[1]+"/api/state", "", "", nil, 200, nil)
			token, err := os.ReadFile(filepath.Join(dir, "gateway-b", "registry", "local.token"))
			if err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequest("GET", f.Registries[1]+"/v1/status", nil)
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
			resp, err := isolated.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			fixtureRead(t, resp, 200, nil)
			_ = resp.Body.Close()
		}
		if err := gatewayProcess.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(150 * time.Millisecond)
		localCheck()
		var health map[string]string
		fixtureDo(t, client, "GET", f.Provider+"/__provider/installation?organization=gateway-a&installation=gateway-a", "", "", nil, 200, &health)
		if health["connector"] != "disconnected" {
			t.Fatal("connector disconnect hidden")
		}
		if err := idpProcess.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		fixtureDo(t, client, "GET", f.Provider+"/__provider/session", "", "", nil, 401, nil)
		localCheck()
		if err := providerProcess.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		localCheck()
	}
	if os.Getenv("PRX_SUPPORT_FIXTURE_HOLD") == "1" {
		t.Logf("Fixture ready: %s/fixture.json (private)", dir)
		select {}
	}
}
