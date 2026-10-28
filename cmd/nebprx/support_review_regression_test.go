package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nebuless/nebprx/internal/supportwire"
)

func TestSupportDeleteDoesNotInvertSnapshotLocks(t *testing.T) {
	s, a, c := supportFixture(t)
	_, session := supportPending(t, s, a, c)
	if err := s.activate(session.ID, c); err != nil {
		t.Fatal(err)
	}
	w := supportSurfaceMux(t, s)
	if _, err := s.db.Exec("INSERT INTO devices(id,profile,created_ns,last_seen_ns) VALUES('10.55.0.2','default',1,1)"); err != nil {
		t.Fatal(err)
	}
	if err := w.onboard.Load(s.db); err != nil {
		t.Fatal(err)
	}
	w.g.agents.onboard = w.onboard
	w.g.agents.mu.RLock()
	locked := true
	defer func() {
		if locked {
			w.g.agents.mu.RUnlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		done <- w.supportDeviceMutation(context.Background(), s, c, session, httptest.NewRequest("POST", "/api/agents/delete?ip=10.55.0.2", nil))
	}()
	deadline, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		var remaining int
		if err := s.db.QueryRowContext(deadline, "SELECT count(*) FROM devices WHERE id='10.55.0.2'").Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining == 0 {
			break
		}
		select {
		case <-deadline.Done():
			t.Fatal("device transaction did not commit")
		case <-time.After(time.Millisecond):
		}
	}
	profile := make(chan string, 1)
	go func() { profile <- w.onboard.ProfileForIP("10.55.0.2") }()
	select {
	case <-profile:
	case <-time.After(time.Second):
		w.g.agents.mu.RUnlock()
		locked = false
		<-done
		t.Fatal("deletion retained onboard lock while waiting for agent snapshot")
	}
	w.g.agents.mu.RUnlock()
	locked = false
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("deletion did not finish")
	}
	f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, session, "POST", "/__support/end"), nil, nil)
	if f.Status != 200 {
		t.Fatalf("subsequent revocation failed: %d", f.Status)
	}
	if _, err := s.authorize(session.ID); err == nil {
		t.Fatal("revoked session authorized")
	}
}

func TestSupportEnrollmentCredentialDestination(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []os.FileMode{0600, 0644} {
		path := filepath.Join(dir, fmt.Sprintf("existing-%o", mode))
		if err := os.WriteFile(path, []byte("untouched"), mode); err != nil {
			t.Fatal(err)
		}
		if f, err := reserveSupportCredential(path); err == nil {
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Fatal("existing credential destination accepted")
		}
		if b, err := os.ReadFile(path); err != nil || string(b) != "untouched" {
			t.Fatal("existing destination changed")
		}
	}
	target := filepath.Join(dir, "existing-600")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if f, err := reserveSupportCredential(link); err == nil {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("symlink destination accepted")
	}
	path := filepath.Join(dir, "new")
	f, err := reserveSupportCredential(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("synthetic-credential"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("new credential is not owner-only")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if f, err := reserveSupportCredential(filepath.Join(dir, "unsafe")); err == nil {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("non-private destination directory accepted")
	}
}

func TestSupportEventsBoundedContinuation(t *testing.T) {
	s, a, c := supportFixture(t)
	_, session := supportPending(t, s, a, c)
	if err := s.activate(session.ID, c); err != nil {
		t.Fatal(err)
	}
	w := supportSurfaceMux(t, s)
	w.g.sink.mu.Lock()
	for n := range 500 {
		w.g.sink.recent[n] = Event{ID: fmt.Sprintf("event-%03d", n), Ts: time.Unix(1700000000, int64(n)), Phase: "end", AgentIP: "fd7a:115c:a1e0::1", Host: strings.Repeat("a", 200) + ".example", Method: "GET", Status: "200", Endpoint: strings.Repeat("endpoint", 40)}
	}
	w.g.sink.recentLen = 500
	w.g.sink.mu.Unlock()
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 20; page++ {
		query := url.Values{"agent": {"fd7a:115c:a1e0::1"}}
		if cursor != "" {
			query.Set("after", cursor)
		}
		f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, session, "GET", "/api/events?"+query.Encode()), nil, nil)
		if f.Status != 200 || len(f.Body) > supportwire.MaxEventBatch {
			t.Fatalf("invalid batch: status=%d bytes=%d", f.Status, len(f.Body))
		}
		data := strings.TrimSuffix(strings.TrimPrefix(string(f.Body), "event: backlog\ndata: "), "\n\n")
		var events []Event
		if err := json.Unmarshal([]byte(data), &events); err != nil {
			t.Fatal(err)
		}
		for _, e := range events {
			if seen[e.ID] {
				t.Fatalf("duplicate event: %s", e.ID)
			}
			seen[e.ID] = true
		}
		if f.Headers.Get(supportwire.EventMoreHeader) != "1" {
			break
		}
		next := f.Headers.Get(supportwire.EventCursorHeader)
		if next == "" || next == cursor {
			t.Fatal("continuation did not advance")
		}
		cursor = next
	}
	if len(seen) != 500 {
		t.Fatalf("lost backlog events: delivered %d of 500", len(seen))
	}
	query := url.Values{"after": {strings.Repeat("0", 64)}}
	f := w.dispatchSupport(context.Background(), s, supportEnvelope(s, c, session, "GET", "/api/events?"+query.Encode()), nil, nil)
	if f.Status != 200 || !strings.Contains(string(f.Body), "event: backlog-gap") {
		t.Fatal("evicted cursor did not report a backlog gap")
	}
}
