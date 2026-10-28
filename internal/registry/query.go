package registry

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/mod/module"
)

// Check evaluates a package against local evidence and its current freshness.
func (s *Store) Check(p Package, now time.Time) (Result, error) {
	p, err := validatePackage(p)
	if err != nil {
		return Result{}, err
	}
	if err = requireMode(s, "client"); err != nil {
		return Result{}, err
	}
	// A check cannot authorize itself. Only the persisted local inventory can
	// supply an administrator's decision, including when Check is called by Status.
	p.Decision = ""
	in, err := s.Inventory()
	if err != nil {
		return Result{}, err
	}
	for _, local := range in.Packages {
		if local.Identity == p.Identity {
			if !local.Public {
				p.Public = false
			}
			if local.Version == p.Version && local.Decision != "" {
				p.Decision = local.Decision
			}
		}
	}
	result := Result{Package: p, Finding: "indeterminate", Decision: "review", Coverage: "unavailable", Freshness: "unknown", Existence: "unknown", Evidence: []Finding{}}
	if p.Decision != "" {
		result.Decision = p.Decision
	}
	if !p.Public {
		result.Coverage = "private"
		return result, nil
	}
	var raw []byte
	var expiry string
	err = s.db.QueryRow("SELECT body,expires FROM scopes WHERE key=?", p.Identity.key()).Scan(&raw, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	var scope Scope
	if err = json.Unmarshal(raw, &scope); err != nil {
		return result, err
	}
	expires, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil {
		return result, err
	}
	result.Coverage = "partial"
	if scope.Health.State == "complete" {
		result.Coverage = "complete_for_declared_sources"
	}
	fresh := scope.Health.State == "complete" && !scope.Health.Checked.IsZero() && !scope.Health.Checked.After(now.Add(time.Minute)) && now.Sub(scope.Health.Checked) <= SourceMaxAge && expires.After(now)
	result.Freshness = "stale"
	if fresh {
		result.Freshness = "fresh"
	}
	_, supported := normalizedVersion(p.Ecosystem, p.Version)
	if p.Ecosystem == "Go" && p.Name != "stdlib" && p.Name != "toolchain" && module.Check(p.Name, "v"+strings.TrimPrefix(p.Version, "v")) != nil {
		supported = false
	}
	unknown := !supported
	for _, rec := range scope.Records {
		a, e := decodeAdvisory(rec.Raw)
		if e != nil {
			return result, e
		}
		if a.Withdrawn != nil {
			continue
		}
		affected := false
		for _, entry := range a.Affected {
			id, e := Canonical(entry.Package)
			if e != nil || id != p.Identity {
				continue
			}
			switch membership(p.Ecosystem, p.Version, entry) {
			case "affected":
				affected = true
			case "unknown":
				unknown = true
			}
		}
		if affected {
			result.Evidence = append(result.Evidence, Finding{ID: a.ID, Aliases: a.Aliases, Evidence: rec})
		}
	}
	if !supported {
		result.Coverage = "unsupported_version"
	}
	if len(result.Evidence) > 0 {
		result.Finding = "known_affected"
		result.Decision = "deny"
	} else if fresh && !unknown {
		result.Finding = "no_known_match"
	}
	// Local administrative decisions stay independent of findings and cannot
	// manufacture a negative or erase stale evidence.
	if p.Decision != "" {
		result.Decision = p.Decision
	}
	return result, nil
}
