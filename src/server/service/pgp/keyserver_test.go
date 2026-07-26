package pgp

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// rewriteTransport routes every request to the test server regardless of the
// original host, so publish tests can use realistic keyserver URLs (including
// keys.openpgp.org, which selects the vks JSON path).
type rewriteTransport struct{ target *url.URL }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// useTestKeyserver points keyserverHTTPClient at the given server and shortens
// the retry backoff, restoring both on cleanup.
func useTestKeyserver(t *testing.T, ts *httptest.Server) {
	t.Helper()
	target, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	prevClient := keyserverHTTPClient
	prevBackoff := keyserverRetryBackoff
	keyserverHTTPClient = &http.Client{Transport: rewriteTransport{target: target}}
	keyserverRetryBackoff = time.Millisecond
	t.Cleanup(func() {
		keyserverHTTPClient = prevClient
		keyserverRetryBackoff = prevBackoff
	})
}

// TestPublishToKeyserversSuccess verifies both the vks (keys.openpgp.org) and
// HKP (/pks/add) submission paths are exercised and all states returned.
func TestPublishToKeyserversSuccess(t *testing.T) {
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	useTestKeyserver(t, ts)

	servers := []string{"https://keys.openpgp.org", "https://keyserver.ubuntu.com/"}
	states, err := PublishToKeyservers(context.Background(), []byte("PUBKEY"), servers)
	if err != nil {
		t.Fatalf("PublishToKeyservers: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("states = %d, want 2", len(states))
	}
	if states[0].URL != servers[0] || states[0].PublishedAt.IsZero() {
		t.Fatalf("unexpected state[0]: %+v", states[0])
	}
	sawVKS, sawHKP := false, false
	for _, p := range paths {
		switch p {
		case "/vks/v1/upload":
			sawVKS = true
		case "/pks/add":
			sawHKP = true
		}
	}
	if !sawVKS || !sawHKP {
		t.Fatalf("expected both vks and hkp paths, got %v", paths)
	}
}

// TestPublishToKeyserversPartialFailure verifies a server returning an error
// status yields no state for that server plus a combined error, while the other
// still succeeds.
func TestPublishToKeyserversPartialFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pks/add" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	useTestKeyserver(t, ts)

	servers := []string{"https://keys.openpgp.org", "https://keyserver.ubuntu.com"}
	states, err := PublishToKeyservers(context.Background(), []byte("PUBKEY"), servers)
	if err == nil {
		t.Fatal("expected combined error for the failing keyserver")
	}
	if len(states) != 1 || states[0].URL != servers[0] {
		t.Fatalf("expected 1 successful state for openpgp, got %+v", states)
	}
}

// TestPublishToKeyserversRetries verifies a persistently failing keyserver is
// retried keyserverMaxAttempts times before giving up.
func TestPublishToKeyserversRetries(t *testing.T) {
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	useTestKeyserver(t, ts)

	states, err := PublishToKeyservers(context.Background(), []byte("K"), []string{"https://keyserver.ubuntu.com"})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if len(states) != 0 {
		t.Fatalf("expected no states, got %+v", states)
	}
	if got := atomic.LoadInt32(&hits); got != keyserverMaxAttempts {
		t.Fatalf("server hit %d times, want %d", got, keyserverMaxAttempts)
	}
}

// TestPublishToKeyserversSkipsBlank verifies empty and whitespace-only entries
// are skipped and produce no error.
func TestPublishToKeyserversSkipsBlank(t *testing.T) {
	states, err := PublishToKeyservers(context.Background(), []byte("K"), []string{"", "   "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected no states, got %+v", states)
	}
}

// TestPublishToKeyserversContextCancelled verifies a cancelled context stops the
// retry loop during backoff.
func TestPublishToKeyserversContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	useTestKeyserver(t, ts)
	keyserverRetryBackoff = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PublishToKeyservers(ctx, []byte("K"), []string{"https://keyserver.ubuntu.com"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// TestSubmitToKeyserverBadURL verifies an unparseable keyserver URL is reported.
func TestSubmitToKeyserverBadURL(t *testing.T) {
	if err := submitToKeyserver(context.Background(), "://bad", []byte("K")); err == nil {
		t.Fatal("expected parse error for malformed url")
	}
}

// TestWriteAndLoadKeyserverState verifies the round-trip and 0600 file mode.
func TestWriteAndLoadKeyserverState(t *testing.T) {
	dir := t.TempDir()
	states := []KeyserverState{
		{URL: "https://keys.openpgp.org", PublishedAt: time.Now().UTC().Truncate(time.Second)},
		{URL: "https://keyserver.ubuntu.com", PublishedAt: time.Now().UTC().Truncate(time.Second)},
	}
	if err := WriteKeyserverState(dir, states); err != nil {
		t.Fatalf("WriteKeyserverState: %v", err)
	}
	info, err := os.Stat(filepath.Join(SecurityDir(dir), KeyserverStateFile))
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	loaded, err := LoadKeyserverState(dir)
	if err != nil {
		t.Fatalf("LoadKeyserverState: %v", err)
	}
	if len(loaded) != 2 || loaded[0].URL != states[0].URL {
		t.Fatalf("loaded = %+v", loaded)
	}
}

// TestLoadKeyserverStateMissing verifies a missing file returns (nil, nil).
func TestLoadKeyserverStateMissing(t *testing.T) {
	states, err := LoadKeyserverState(t.TempDir())
	if err != nil || states != nil {
		t.Fatalf("expected (nil,nil), got (%+v,%v)", states, err)
	}
}

// TestLoadKeyserverStateEmptyAndBad verifies empty files are treated as no state
// and malformed JSON surfaces an error.
func TestLoadKeyserverStateEmptyAndBad(t *testing.T) {
	dir := t.TempDir()
	secDir := SecurityDir(dir)
	if err := os.MkdirAll(secDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	statePath := filepath.Join(secDir, KeyserverStateFile)

	if err := os.WriteFile(statePath, []byte{}, 0o600); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	if states, err := LoadKeyserverState(dir); err != nil || states != nil {
		t.Fatalf("empty file: expected (nil,nil), got (%+v,%v)", states, err)
	}

	if err := os.WriteFile(statePath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	if _, err := LoadKeyserverState(dir); err == nil {
		t.Fatal("expected decode error for malformed json")
	}
}

// TestWriteKeyserverStateMkdirFails verifies a mkdir failure is surfaced.
func TestWriteKeyserverStateMkdirFails(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	if err := WriteKeyserverState(blocker, nil); err == nil {
		t.Fatal("expected mkdir error when config dir is a file")
	}
}

// TestUpdateKeyserversPublished verifies the states are written to the live row.
func TestUpdateKeyserversPublished(t *testing.T) {
	db := newTestDB(t)
	created := time.Now().UTC().Truncate(time.Second)
	if _, err := db.Exec(
		`INSERT INTO pgp_keypair (fingerprint, created_at, expires_at) VALUES (?, ?, ?)`,
		"FFFF", created, created.Add(DefaultValidity),
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	states := []KeyserverState{{URL: "https://keys.openpgp.org", PublishedAt: created}}
	if err := UpdateKeyserversPublished(db, states); err != nil {
		t.Fatalf("UpdateKeyserversPublished: %v", err)
	}
	var raw string
	if err := db.QueryRow(`SELECT keyservers_published FROM pgp_keypair WHERE fingerprint = 'FFFF'`).Scan(&raw); err != nil {
		t.Fatalf("scan: %v", err)
	}
	var got []KeyserverState
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal stored: %v", err)
	}
	if len(got) != 1 || got[0].URL != states[0].URL {
		t.Fatalf("stored states = %+v", got)
	}
}

// TestUpdateKeyserversPublishedNoTable verifies DB errors are surfaced.
func TestUpdateKeyserversPublishedNoTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := UpdateKeyserversPublished(db, nil); err == nil {
		t.Fatal("expected error updating without pgp_keypair table")
	}
}
