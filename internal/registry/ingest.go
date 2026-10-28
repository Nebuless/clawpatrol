package registry

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// MaxExport bounds compressed or raw export bytes.
const MaxExport = 512 << 20

// MaxExpanded bounds expanded export bytes.
const MaxExpanded = 1 << 30

// MaxRecords bounds the number of records per import.
const MaxRecords = 250000

// Source identifies a configured ecosystem export URL.
type Source struct {
	Ecosystem string `json:"ecosystem"`
	URL       string `json:"url"`
}

// ValidateSources checks ecosystem uniqueness and credential-free HTTPS URLs.
func ValidateSources(sources []Source) error {
	if len(sources) > 4 {
		return errors.New("at most four ecosystem exports")
	}
	seen := map[string]bool{}
	for _, s := range sources {
		if _, err := Canonical(Identity{s.Ecosystem, "example"}); err != nil {
			return err
		}
		if seen[s.Ecosystem] {
			return errors.New("duplicate ecosystem export")
		}
		seen[s.Ecosystem] = true
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
			return errors.New("export URL must be credential-free HTTPS URL")
		}
	}
	return nil
}

// Source rights are independent of OSV's code license. MAL and unknown sources
// remain excluded until report-specific redistribution rights are reviewed.
func attribution(a Advisory, raw []byte) (source, license, credit string, ok bool) {
	switch {
	case strings.HasPrefix(a.ID, "GHSA-"):
		return "https://github.com/github/advisory-database", "CC-BY-4.0", "GitHub Advisory Database; OSV aggregation; unmodified OSV record", true
	case strings.HasPrefix(a.ID, "PYSEC-"):
		return "https://github.com/pypa/advisory-database", "CC-BY-4.0", "PyPA advisory database; OSV aggregation; unmodified OSV record", true
	case strings.HasPrefix(a.ID, "GO-"):
		return "https://vuln.go.dev", "CC-BY-4.0", "Go vulnerability database; OSV aggregation; unmodified OSV record", true
	case strings.HasPrefix(a.ID, "RUSTSEC-"):
		license = "CC0-1.0"
		if bytes.Contains(raw, []byte("GHSA-")) || bytes.Contains(raw, []byte("CC-BY-4.0")) {
			license = "CC-BY-4.0"
		}
		// Unknown explicit license statements must not be silently relicensed.
		var extra struct {
			DatabaseSpecific struct {
				License string `json:"license"`
			} `json:"database_specific"`
		}
		_ = json.Unmarshal(raw, &extra)
		if extra.DatabaseSpecific.License != "" && extra.DatabaseSpecific.License != "CC0-1.0" && extra.DatabaseSpecific.License != "CC-BY-4.0" {
			return "", "", "", false
		}
		return "https://github.com/RustSec/advisory-db", license, "RustSec advisory database (including credited GHSA-derived content); OSV aggregation; unmodified OSV record", true
	}
	return "", "", "", false
}

func decodeAdvisory(raw []byte) (Advisory, error) {
	var a Advisory
	if len(raw) > MaxRecord || len(raw) == 0 {
		return a, errors.New("advisory size limit")
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return a, errors.New("invalid OSV JSON")
	}
	if a.ID == "" || len(a.ID) > 200 || a.Modified.IsZero() || len(a.Affected) > 1024 || len(a.Aliases) > 1024 {
		return a, errors.New("invalid OSV identity, modified timestamp or affected entries")
	}
	for _, entry := range a.Affected {
		if len(entry.Versions) > 100000 || len(entry.Ranges) > 1024 {
			return a, errors.New("OSV matching work limit")
		}
		for _, r := range entry.Ranges {
			if len(r.Events) > 4096 {
				return a, errors.New("OSV range event limit")
			}
		}
	}
	return a, nil
}

func (s *Store) sourceState(eco, state string) error {
	var h Health
	err := s.get("health/"+eco, &h)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	h.Ecosystem = eco
	h.State = state
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = put(tx, "health/"+eco, h); err != nil {
		return err
	}
	return tx.Commit()
}

// Import performs a full ecosystem reconciliation. JSON arrays and OSV ZIP
// exports are accepted. Every record is validated before the transaction commits.
// A failed import retains old records and old checked time, with failed health.
func (s *Store) Import(ctx context.Context, eco, path, origin string, checked time.Time) (err error) {
	if err = requireMode(s, "server"); err != nil {
		return err
	}
	if _, err = Canonical(Identity{eco, "example"}); err != nil {
		return err
	}
	if checked.IsZero() || checked.After(time.Now().Add(time.Minute)) {
		return errors.New("valid source observation time required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.sourceState(eco, "refreshing"); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.sourceState(eco, "failed"))
		}
	}()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > MaxExport {
		return errors.New("export exceeds 512 MiB")
	}
	hash := shaReader(f)
	if hash.err != nil {
		return hash.err
	}
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, "DELETE FROM records WHERE ecosystem=?", eco); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM bindings WHERE ecosystem=?", eco); err != nil {
		return err
	}
	count, rejected, total := 0, 0, 0
	insert := func(raw []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		total += len(raw)
		if count > MaxRecords || total > MaxExpanded {
			return errors.New("export expansion/work limit")
		}
		a, e := decodeAdvisory(raw)
		if e != nil {
			return e
		}
		source, license, credit, ok := attribution(a, raw)
		if !ok {
			rejected++
			return nil
		}
		relevant, quarantined := false, false
		for _, affected := range a.Affected {
			if affected.Package.Ecosystem == "" || affected.Package.Name == "" {
				quarantined = true
				continue
			}
			if affected.Package.Ecosystem == eco {
				relevant = true
				id, ce := Canonical(affected.Package)
				if ce != nil {
					quarantined = true
					continue
				}
				if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO bindings(ecosystem,name,id) VALUES(?,?,?)", eco, id.Name, a.ID); e != nil {
					return e
				}
			}
		}
		if !relevant && a.Withdrawn == nil {
			return errors.New("active advisory does not identify the declared ecosystem")
		}
		if quarantined {
			rejected++
		}
		rec := Record{Raw: raw, Source: source, License: license, Attribution: credit, Observed: checked, Digest: digest(raw)}
		body, e := json.Marshal(rec)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO records(ecosystem,id,body) VALUES(?,?,?)", eco, a.ID, body)
		return e
	}
	magic := make([]byte, 4)
	_, _ = f.ReadAt(magic, 0)
	if bytes.Equal(magic, []byte{'P', 'K', 3, 4}) || bytes.Equal(magic, []byte{'P', 'K', 5, 6}) {
		z, e := zip.NewReader(f, info.Size())
		if e != nil {
			return e
		}
		if len(z.File) > MaxRecords {
			return errors.New("ZIP entry limit")
		}
		var expanded uint64
		for _, entry := range z.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name, ".json") {
				return errors.New("unexpected ZIP entry")
			}
			expanded += entry.UncompressedSize64
			if entry.UncompressedSize64 > MaxRecord || expanded > MaxExpanded {
				return errors.New("ZIP expansion limit")
			}
			r, e := entry.Open()
			if e != nil {
				return e
			}
			raw, e := io.ReadAll(io.LimitReader(r, MaxRecord+1))
			ce := r.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
			if e = insert(raw); e != nil {
				return e
			}
		}
	} else {
		// Arrays are convenient for bounded offline fixtures. Large corpora must
		// use ZIP so an individual JSON token cannot allocate the whole export.
		if info.Size() > MaxRecord {
			return errors.New("JSON array exceeds 2 MiB; use OSV ZIP for large exports")
		}
		d := json.NewDecoder(io.LimitReader(f, MaxExport+1))
		t, e := d.Token()
		if e != nil || t != json.Delim('[') {
			return errors.New("import requires OSV JSON array or ZIP")
		}
		for d.More() {
			var raw json.RawMessage
			if e = d.Decode(&raw); e != nil {
				return e
			}
			if e = insert(raw); e != nil {
				return e
			}
		}
		if _, e = d.Token(); e != nil {
			return e
		}
		var tail any
		if d.Decode(&tail) != io.EOF {
			return errors.New("trailing import data")
		}
	}
	h := Health{Ecosystem: eco, State: "complete", Checked: checked, Origin: origin, Digest: hash.sum, Rejected: rejected}
	if rejected > 0 {
		h.State = "partial"
	}
	if err = put(tx, "health/"+eco, h); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE meta SET value=CAST(value AS INTEGER)+1 WHERE key='revision'"); err != nil {
		return err
	}
	return tx.Commit()
}

type hashed struct {
	sum string
	err error
}

func shaReader(r io.Reader) hashed {
	h := sha256.New()
	_, err := io.Copy(h, r)
	return hashed{hex.EncodeToString(h.Sum(nil)), err}
}

// Refresh fetches configured exports while retaining prior data on failure.
func (s *Store) Refresh(ctx context.Context, sources []Source) error {
	if err := requireMode(s, "server"); err != nil {
		return err
	}
	if err := ValidateSources(sources); err != nil {
		return err
	}
	var result error
	for _, source := range sources {
		err := s.fetchExport(ctx, source)
		if err != nil {
			result = errors.Join(result, fmt.Errorf("%s export refresh failed: %w", source.Ecosystem, err))
		}
	}
	return result
}
func (s *Store) fetchExport(ctx context.Context, source Source) error {
	// No credentials, no redirects, no URLs chosen by registry clients.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return err
	}
	hc := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") }}
	resp, err := hc.Do(req)
	if err != nil {
		_ = s.sourceState(source.Ecosystem, "failed")
		return errors.New("upstream transport failure")
	}
	defer func() { _ = resp.Body.Close() }()
	fail := func(e error) error { return errors.Join(e, s.sourceState(source.Ecosystem, "failed")) }
	if resp.StatusCode != 200 {
		return fail(fmt.Errorf("upstream HTTP %d", resp.StatusCode))
	}
	if resp.ContentLength > MaxExport {
		return fail(errors.New("upstream export too large"))
	}
	f, err := os.CreateTemp(s.Dir, "export-*.tmp")
	if err != nil {
		return fail(err)
	}
	path := f.Name()
	defer func() { _ = os.Remove(path) }()
	n, err := io.Copy(f, io.LimitReader(resp.Body, MaxExport+1))
	ce := f.Close()
	if err != nil {
		return fail(err)
	}
	if ce != nil {
		return fail(ce)
	}
	if n > MaxExport {
		return fail(errors.New("upstream export too large"))
	}
	return s.Import(ctx, source.Ecosystem, path, source.URL, time.Now().UTC())
}
