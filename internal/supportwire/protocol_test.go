package supportwire

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestEnvelopeReplayAndTarget(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	e := Envelope{Version: 1, Purpose: "dashboard", Connection: "fresh", Sequence: 1, RequestID: "request", OrganizationID: "org", InstallationID: "gateway", Method: "GET", Target: "/api/state", Expires: time.Now().Add(10 * time.Second).Unix(), Issued: time.Now().Unix()}
	signed, err := Sign(key, e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(pub, signed, "fresh", "org", "gateway", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(pub, signed, "fresh", "org", "gateway", 1, time.Now()); err == nil {
		t.Fatal("replayed sequence accepted")
	}
	if _, err = Verify(pub, signed, "old", "org", "gateway", 0, time.Now()); err == nil {
		t.Fatal("old connection accepted")
	}
	for _, target := range []string{"https://evil.example/", "//evil.example/", "/api/../oauth/callback", "/api/%2fcredentials", "/api/state?profile=a&profile=b", "/api/state#fragment"} {
		if err = ValidateTarget("GET", target); err == nil {
			t.Errorf("accepted %q", target)
		}
	}
}

func TestTargetEncodedQueryAndCanonicalPath(t *testing.T) {
	for _, target := range []string{
		"/api/status?profile=team%20one",
		"/api/events?agent=fd7a%3A115c%3Aa1e0%3A%3A1",
		"/api/analytics?range=1h&agent=fd7a%3A115c%3Aa1e0%3A%3A1",
		"/api/state?profile=100%25",
	} {
		if err := ValidateTarget("GET", target); err != nil {
			t.Errorf("rejected encoded query %q: %v", target, err)
		}
	}
	for _, target := range []string{
		"/api/%2e%2e/config", "/api/%252e%252e/config", "/api%2fstate",
		"/api/state?profile=%", "/api/state?profile=%GG",
		"/api/state?profile=a&pro%66ile=b", "/api/state?profile=a;profile=b",
		"//example.com/api/state", "https://example.com/api/state",
	} {
		if err := ValidateTarget("GET", target); err == nil {
			t.Errorf("accepted noncanonical target %q", target)
		}
	}
}

func TestPurposeKeysCannotBeSwapped(t *testing.T) {
	support, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registry, registryKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway, gatewayKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []ed25519.PublicKey{registry, gateway} {
		if CheckProof(wrong, Proof(key, "provider", "nonce", "org", "gateway"), "provider", "nonce", "org", "gateway") {
			t.Fatal("support proof accepted wrong key")
		}
	}
	for _, purpose := range []string{"gateway", "registry", "provider-management"} {
		if CheckProof(support, Proof(key, "provider", "nonce", "org", "gateway"), purpose, "nonce", "org", "gateway") {
			t.Fatal("proof purpose swap")
		}
	}
	now := time.Now()
	e := Envelope{Version: 1, Purpose: "dashboard", Connection: "fresh", Sequence: 1, RequestID: "request", OrganizationID: "org", InstallationID: "gateway", Method: "GET", Target: "/api/state", Issued: now.Unix(), Expires: now.Add(10 * time.Second).Unix()}
	for _, wrong := range []ed25519.PrivateKey{registryKey, gatewayKey} {
		signed, err := Sign(wrong, e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Verify(support, signed, "fresh", "org", "gateway", 0, now); err == nil {
			t.Fatal("wrong purpose signing key accepted")
		}
	}
	if CheckProof(support, Proof(gatewayKey, "gateway", "nonce", "org", "gateway"), "gateway", "nonce", "org", "gateway") {
		t.Fatal("gateway proof accepted support key")
	}
}
