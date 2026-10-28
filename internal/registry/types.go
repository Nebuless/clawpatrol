// Package registry provides public advisory aggregation and private local inventory stores.
package registry

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// MaxPackages bounds inventory and snapshot scope counts.
const MaxPackages = 256

// MaxRecord bounds an individual record in bytes.
const MaxRecord = 2 << 20

// MaxSnapshot bounds a snapshot payload in bytes.
const MaxSnapshot = 32 << 20

// Identity identifies a package within an ecosystem.
type Identity struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

// Package binds an identity to an exact version and local disclosure policy.
type Package struct {
	Identity
	Version  string `json:"version"`
	Public   bool   `json:"public"`
	Decision string `json:"decision,omitempty"`
}

// Inventory describes installed packages and graph completeness.
type Inventory struct {
	GraphComplete bool      `json:"graph_complete"`
	Packages      []Package `json:"packages"`
}

// Event records an OSV affected-version boundary.
type Event struct {
	Introduced   string `json:"introduced,omitempty"`
	Fixed        string `json:"fixed,omitempty"`
	LastAffected string `json:"last_affected,omitempty"`
	Limit        string `json:"limit,omitempty"`
}

// Range groups OSV version events by comparison type.
type Range struct {
	Type   string  `json:"type"`
	Events []Event `json:"events"`
}

// Affected describes an advisory package and its affected versions.
type Affected struct {
	Package  Identity `json:"package"`
	Versions []string `json:"versions,omitempty"`
	Ranges   []Range  `json:"ranges,omitempty"`
}

// Advisory contains supported OSV identity and affected-package fields.
type Advisory struct {
	ID        string     `json:"id"`
	Modified  time.Time  `json:"modified"`
	Published time.Time  `json:"published"`
	Withdrawn *time.Time `json:"withdrawn,omitempty"`
	Aliases   []string   `json:"aliases,omitempty"`
	Affected  []Affected `json:"affected"`
}

// Record preserves raw advisory bytes with source attribution and integrity metadata.
type Record struct {
	Raw         []byte    `json:"raw"`
	Source      string    `json:"source"`
	License     string    `json:"license"`
	Attribution string    `json:"attribution"`
	Observed    time.Time `json:"observed"`
	Digest      string    `json:"sha256"`
}

// Health records ecosystem coverage and source observation time.
type Health struct {
	Ecosystem string    `json:"ecosystem"`
	State     string    `json:"state"`
	Checked   time.Time `json:"checked"`
	Origin    string    `json:"origin"`
	Digest    string    `json:"sha256"`
	Rejected  int       `json:"rejected"`
}

// Scope binds a package identity to records and source health.
type Scope struct {
	Identity Identity `json:"identity"`
	Records  []Record `json:"records"`
	Health   Health   `json:"health"`
}

// Snapshot is a scoped, time-bounded publication for one installation.
type Snapshot struct {
	Protocol     string    `json:"protocol"`
	Installation string    `json:"installation"`
	Sequence     int64     `json:"sequence"`
	Revision     int64     `json:"revision"`
	Issued       time.Time `json:"issued"`
	Expires      time.Time `json:"expires"`
	Scopes       []Scope   `json:"scopes"`
}

// Envelope carries a signed snapshot payload and its digest.
type Envelope struct {
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
	SHA256    string `json:"sha256"`
}

// Finding links an advisory identity to supporting evidence.
type Finding struct {
	ID       string   `json:"id"`
	Aliases  []string `json:"aliases,omitempty"`
	Evidence Record   `json:"evidence"`
}

// Result reports package findings, coverage, freshness and local policy.
type Result struct {
	Package   Package   `json:"package"`
	Finding   string    `json:"finding"`
	Decision  string    `json:"decision"`
	Coverage  string    `json:"coverage"`
	Freshness string    `json:"freshness"`
	Existence string    `json:"existence"`
	Evidence  []Finding `json:"evidence"`
}

var pyName = regexp.MustCompile(`[-_.]+`)
var plainName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
var goName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)
var npmName = regexp.MustCompile(`^(@[A-Za-z0-9._-]+/)?[A-Za-z0-9][A-Za-z0-9._-]*$`)
var crateName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var pythonName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Canonical validates an ecosystem identity and normalizes its package name.
func Canonical(p Identity) (Identity, error) {
	if len(p.Name) == 0 || len(p.Name) > 240 {
		return p, errors.New("invalid package identity")
	}
	ok := false
	switch p.Ecosystem {
	case "PyPI":
		p.Name = pyName.ReplaceAllString(strings.ToLower(p.Name), "-")
		ok = pythonName.MatchString(p.Name)
	case "npm":
		ok = npmName.MatchString(p.Name)
	case "crates.io":
		ok = crateName.MatchString(p.Name)
	case "Go":
		ok = goName.MatchString(p.Name) && !strings.Contains(p.Name, "//") && !strings.Contains(p.Name, "..")
	}
	if !ok {
		return p, errors.New("unsupported ecosystem or invalid package identity")
	}
	return p, nil
}
func (p Identity) key() string { return p.Ecosystem + "/" + p.Name }
func validatePackage(p Package) (Package, error) {
	id, err := Canonical(p.Identity)
	p.Identity = id
	if err != nil {
		return p, err
	}
	if len(p.Version) == 0 || len(p.Version) > 256 || strings.ContainsAny(p.Version, "\r\n\x00") {
		return p, errors.New("exact version required")
	}
	switch p.Decision {
	case "", "review", "allow", "deny":
	default:
		return p, errors.New("invalid local decision")
	}
	return p, nil
}
