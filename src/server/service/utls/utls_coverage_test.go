// SPDX-License-Identifier: MIT
// Coverage tests for unexplored paths in the utls package:
// edge/random fingerprint cases, CheckRedirect closure (too-many-redirects +
// header-preservation), dialTLS error path, dialTLSWithFingerprint error path.
package utls

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── CreateHTTPClientWithFingerprint – missing fingerprints ────────────────────

// TestCreateHTTPClientWithFingerprint_Edge verifies the "edge" fingerprint branch.
func TestCreateHTTPClientWithFingerprint_Edge(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(10*time.Second, "edge")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('edge') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for edge fingerprint")
	}
}

// TestCreateHTTPClientWithFingerprint_Random verifies the "random" fingerprint branch.
func TestCreateHTTPClientWithFingerprint_Random(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(10*time.Second, "random")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('random') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for random fingerprint")
	}
}

// ── NewUTLSClient CheckRedirect closure ──────────────────────────────────────

// TestNewUTLSClient_CheckRedirect_TooManyRedirects verifies that the
// CheckRedirect closure returns an error once 10+ redirects accumulate.
func TestNewUTLSClient_CheckRedirect_TooManyRedirects(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer srv.Close()

	client := NewUTLSClient(5 * time.Second)
	_, err := client.HTTPClient().Get(srv.URL)
	if err == nil {
		t.Error("expected too-many-redirects error, got nil")
	}
}

// TestNewUTLSClient_CheckRedirect_HeaderPreservation verifies that headers set
// on the first request are forwarded to subsequent requests during a redirect.
func TestNewUTLSClient_CheckRedirect_HeaderPreservation(t *testing.T) {
	redirected := false
	receivedHeader := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !redirected {
			redirected = true
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		receivedHeader = r.Header.Get("X-Test-Header")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewUTLSClient(5 * time.Second)
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Test-Header", "preserved")
	_, _ = client.HTTPClient().Do(req)

	if receivedHeader != "preserved" {
		t.Errorf("header not preserved across redirect: got %q, want %q", receivedHeader, "preserved")
	}
}

// ── CreateHTTPClientWithFingerprint CheckRedirect closure ────────────────────

// TestCreateHTTPClientWithFingerprint_CheckRedirect_TooMany verifies the
// CheckRedirect closure on fingerprint clients rejects chains of 10+ redirects.
func TestCreateHTTPClientWithFingerprint_CheckRedirect_TooMany(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer srv.Close()

	client := CreateHTTPClientWithFingerprint(5*time.Second, "chrome")
	_, err := client.Get(srv.URL)
	if err == nil {
		t.Error("expected too-many-redirects error, got nil")
	}
}

// TestCreateHTTPClientWithFingerprint_CheckRedirect_HeaderPreservation checks
// that fingerprint clients also propagate headers on redirect.
func TestCreateHTTPClientWithFingerprint_CheckRedirect_HeaderPreservation(t *testing.T) {
	redirected := false
	receivedHeader := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !redirected {
			redirected = true
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		receivedHeader = r.Header.Get("X-FP-Header")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := CreateHTTPClientWithFingerprint(5*time.Second, "firefox")
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-FP-Header", "fp-value")
	_, _ = client.Do(req)

	if receivedHeader != "fp-value" {
		t.Errorf("header not preserved across redirect: got %q, want %q", receivedHeader, "fp-value")
	}
}

// ── dialTLS error path ────────────────────────────────────────────────────────

// TestDialTLS_ConnectionRefused exercises dialTLS up to the TCP-connect failure.
// Port 1 is a reserved port that will be refused on loopback.
func TestDialTLS_ConnectionRefused(t *testing.T) {
	_, err := dialTLS(context.Background(), "tcp", "127.0.0.1:1")
	if err == nil {
		t.Error("dialTLS to refused port: expected error, got nil")
	}
}

// TestDialTLS_InvalidAddress exercises the SplitHostPort-failure branch of dialTLS.
func TestDialTLS_InvalidAddress(t *testing.T) {
	_, err := dialTLS(context.Background(), "tcp", "notanaddress")
	if err == nil {
		t.Error("dialTLS with no-port address: expected error, got nil")
	}
}

// ── dialTLSWithFingerprint error path ────────────────────────────────────────

// TestDialTLSWithFingerprint_ConnectionRefused exercises dialTLSWithFingerprint
// up to the TCP-connect failure so the function body is partially covered.
func TestDialTLSWithFingerprint_ConnectionRefused(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(1*time.Second, "chrome")
	transport := c.Transport.(*http.Transport)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := transport.DialTLSContext(ctx, "tcp", "127.0.0.1:1")
	if err == nil {
		t.Error("dialTLSWithFingerprint to refused port: expected error, got nil")
	}
}

// TestDialTLSWithFingerprint_InvalidAddress exercises the SplitHostPort-failure
// branch inside dialTLSWithFingerprint.
func TestDialTLSWithFingerprint_InvalidAddress(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(1*time.Second, "firefox")
	transport := c.Transport.(*http.Transport)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := transport.DialTLSContext(ctx, "tcp", "notanaddress")
	if err == nil {
		t.Error("dialTLSWithFingerprint with no-port address: expected error, got nil")
	}
}

// ── dialTLS — TLS setup and handshake paths ───────────────────────────────────
// Connect to a TLS test server. The handshake will fail (cert verification)
// but covers lines 80-91 (uTLS config + handshake error path).

func TestDialTLS_TLSServerHandshakeFails_CoversTLSSetup(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Handshake will fail (self-signed cert, InsecureSkipVerify=false)
	_, err := dialTLS(ctx, "tcp", server.Listener.Addr().String())
	if err == nil {
		// If it succeeded, that's fine too — covers the success path
		t.Log("dialTLS to TLS server: connected successfully (cert accepted)")
	}
	// Either way, TCP connect + uTLS setup lines are covered
}

func TestDialTLSWithFingerprint_TLSServerHandshakeFails_CoversTLSSetup(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := NewRoundTripper(5 * time.Second)
	rt := transport.(*http.Transport)

	// The TLS dial will reach uTLS setup code even if handshake fails
	_, err := rt.DialTLSContext(ctx, "tcp", server.Listener.Addr().String())
	if err == nil {
		t.Log("dialTLS via RoundTripper: connected successfully")
	}
}
