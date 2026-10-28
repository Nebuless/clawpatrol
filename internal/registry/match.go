package registry

import (
	"golang.org/x/mod/semver"
	"regexp"
	"strconv"
	"strings"
)

var release = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)
var exactSemver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([+-].*)?$`)

// Python v1 supports only PEP440 release tuples. Epoch, pre, post, dev and
// local versions return unknown, including on empty scopes.
func normalizedVersion(eco, v string) (string, bool) {
	if eco == "PyPI" {
		if !release.MatchString(v) {
			return "", false
		}
		parts := strings.Split(v, ".")
		for i, s := range parts {
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return "", false
			}
			parts[i] = strconv.FormatUint(n, 10)
		}
		for len(parts) > 1 && parts[len(parts)-1] == "0" {
			parts = parts[:len(parts)-1]
		}
		return strings.Join(parts, "."), true
	}
	if eco == "Go" {
		v = strings.TrimPrefix(v, "v")
	}
	if !exactSemver.MatchString(v) || !semver.IsValid("v"+v) {
		return "", false
	}
	return "v" + v, true
}
func compare(eco, a, b string) (int, bool) {
	a, ok := normalizedVersion(eco, a)
	if !ok {
		return 0, false
	}
	b, ok = normalizedVersion(eco, b)
	if !ok {
		return 0, false
	}
	if eco != "PyPI" {
		return semver.Compare(a, b), true
	}
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aa) || i < len(bb); i++ {
		var x, y uint64
		if i < len(aa) {
			x, _ = strconv.ParseUint(aa[i], 10, 64)
		}
		if i < len(bb) {
			y, _ = strconv.ParseUint(bb[i], 10, 64)
		}
		if x < y {
			return -1, true
		}
		if x > y {
			return 1, true
		}
	}
	return 0, true
}

func membership(eco, v string, a Affected) string {
	if _, ok := normalizedVersion(eco, v); !ok {
		return "unknown"
	}
	unknown := len(a.Ranges) == 0 && len(a.Versions) == 0
	for _, x := range a.Versions {
		c, ok := compare(eco, v, x)
		if !ok {
			unknown = true
		}
		if ok && c == 0 {
			return "affected"
		}
	}
	for _, r := range a.Ranges {
		if r.Type != "SEMVER" && r.Type != "ECOSYSTEM" || eco == "PyPI" && r.Type == "SEMVER" {
			unknown = true
			continue
		}
		active := false
		valid := len(r.Events) > 0
		hit := false
		last := ""
		for _, e := range r.Events {
			count := 0
			for _, s := range []string{e.Introduced, e.Fixed, e.LastAffected, e.Limit} {
				if s != "" {
					count++
				}
			}
			if count != 1 {
				valid = false
				break
			}
			bound := e.Introduced
			if bound == "" {
				bound = e.Fixed
			}
			if bound == "" {
				bound = e.LastAffected
			}
			if bound == "" {
				bound = e.Limit
			}
			if last != "" && bound != "0" {
				c, ok := compare(eco, last, bound)
				if !ok || c >= 0 {
					valid = false
					break
				}
			}
			if bound != "0" {
				last = bound
			}
			if e.Introduced != "" {
				if active {
					valid = false
					break
				}
				active = true
				if bound == "0" {
					hit = true
				} else {
					c, ok := compare(eco, v, bound)
					if !ok {
						valid = false
						break
					}
					hit = c >= 0
				}
			} else {
				if !active {
					valid = false
					break
				}
				c, ok := compare(eco, v, bound)
				if !ok {
					valid = false
					break
				}
				if hit && (c < 0 || e.LastAffected != "" && c == 0) {
					return "affected"
				}
				active = false
				hit = false
			}
		}
		if !valid {
			unknown = true
		} else if active && hit {
			return "affected"
		}
	}
	if unknown {
		return "unknown"
	}
	return "unaffected"
}
