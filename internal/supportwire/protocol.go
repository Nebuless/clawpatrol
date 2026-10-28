package supportwire

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// MaxBody is the maximum request body size in bytes.
const MaxBody = 64 << 10

// MaxResponse is the maximum dashboard response body size in bytes.
const MaxResponse = 4 << 20

// MaxEventBatch is the maximum SSE batch size in bytes.
const MaxEventBatch = 64 << 10

// EventCursorHeader carries the last delivered event cursor.
const EventCursorHeader = "X-Prx-Event-Cursor"

// EventMoreHeader indicates that another bounded event batch is available.
const EventMoreHeader = "X-Prx-Event-More"

// Envelope binds a signed dashboard request to its identity, target and connection.
type Envelope struct {
	Version        int
	Purpose        string
	Connection     string
	Sequence       uint64
	RequestID      string
	Issued         int64
	Expires        int64
	OrganizationID string
	InstallationID string
	SessionID      string
	TechnicianID   string
	TechnicianName string
	CaseID         string
	Revision       int64
	Method         string
	Target         string
	BodyDigest     string
	Origin         string
}

// Signed carries an encoded envelope and its Ed25519 signature.
type Signed struct {
	Payload   []byte
	Signature []byte
}

// Frame carries a control request or response over the support connection.
type Frame struct {
	Kind    string
	ID      string
	Signed  Signed
	Body    []byte
	Headers http.Header
	Status  int
	Error   string
}

// Hash returns the hex-encoded SHA-256 digest of a body.
func Hash(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }

// Sign serializes and signs an envelope with an Ed25519 private key.
func Sign(key ed25519.PrivateKey, e Envelope) (Signed, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return Signed{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return Signed{}, errors.New("invalid signing key")
	}
	return Signed{Payload: b, Signature: ed25519.Sign(key, b)}, nil
}

// Verify checks signature, connection binding, freshness, sequence and request target.
func Verify(key ed25519.PublicKey, s Signed, connection, org, installation string, last uint64, now time.Time) (Envelope, error) {
	var e Envelope
	if len(s.Payload) > 16384 || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, s.Payload, s.Signature) {
		return e, errors.New("invalid envelope signature")
	}
	if err := json.Unmarshal(s.Payload, &e); err != nil {
		return e, err
	}
	if e.Version != 1 || e.Purpose != "dashboard" || e.Connection != connection || e.OrganizationID != org || e.InstallationID != installation || e.Sequence <= last || e.RequestID == "" || e.Expires <= now.Unix() || e.Issued > now.Add(5*time.Second).Unix() || e.Issued < now.Add(-20*time.Second).Unix() || e.Expires-e.Issued > 15 {
		return e, errors.New("invalid envelope binding")
	}
	if err := ValidateTarget(e.Method, e.Target); err != nil {
		return e, err
	}
	return e, nil
}

// ValidateTarget rejects unsupported methods and noncanonical relative targets.
func ValidateTarget(method, target string) error {
	if method != "GET" && method != "POST" && method != "PATCH" {
		return errors.New("unsupported method")
	}
	if len(target) > 4096 || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || strings.ContainsAny(target, "\\\r\n#") {
		return errors.New("noncanonical target")
	}
	u, err := url.ParseRequestURI(target)
	if err != nil || u.IsAbs() || u.Host != "" || path.Clean(u.Path) != u.Path || strings.Contains(u.EscapedPath(), "%") {
		return errors.New("invalid relative target")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("invalid query")
	}
	for _, v := range query {
		if len(v) != 1 {
			return errors.New("duplicate query key")
		}
	}
	return nil
}

// SafeHeaders bounds headers and returns only the permitted request fields.
func SafeHeaders(h http.Header) (http.Header, error) {
	out := http.Header{}
	n := 0
	for k, values := range h {
		for _, v := range values {
			n += len(k) + len(v)
		}
	}
	if n > 16384 {
		return nil, errors.New("headers too large")
	}
	for _, k := range []string{"Content-Type", "Accept", "Origin"} {
		if v := h.Values(k); len(v) > 1 {
			return nil, errors.New("duplicate header")
		} else if len(v) == 1 {
			out.Set(k, v[0])
		}
	}
	return out, nil
}

// Proof signs a purpose-separated connection challenge.
func Proof(key ed25519.PrivateKey, purpose, nonce, org, installation string) string {
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(purpose+"\x00"+nonce+"\x00"+org+"\x00"+installation)))
}

// CheckProof verifies a purpose-separated connection challenge.
func CheckProof(key ed25519.PublicKey, signature, purpose, nonce, org, installation string) bool {
	b, err := base64.RawURLEncoding.DecodeString(signature)
	return err == nil && len(key) == ed25519.PublicKeySize && ed25519.Verify(key, []byte(purpose+"\x00"+nonce+"\x00"+org+"\x00"+installation), b)
}
