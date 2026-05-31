// SPDX-License-Identifier: MIT
package utls

import (
	"net/http"
	"testing"
	"time"
)

// --- NewUTLSClient ---

// TestNewUTLSClient_NonNil verifies that the constructor returns a non-nil client.
func TestNewUTLSClient_NonNil(t *testing.T) {
	c := NewUTLSClient(30 * time.Second)
	if c == nil {
		t.Fatal("NewUTLSClient returned nil")
	}
}

// TestNewUTLSClient_HTTPClientNonNil verifies the embedded *http.Client is accessible.
func TestNewUTLSClient_HTTPClientNonNil(t *testing.T) {
	c := NewUTLSClient(30 * time.Second)
	if c.HTTPClient() == nil {
		t.Fatal("HTTPClient() returned nil")
	}
}

// TestNewUTLSClient_TimeoutPropagated verifies the requested timeout is set on
// the underlying http.Client.
func TestNewUTLSClient_TimeoutPropagated(t *testing.T) {
	want := 15 * time.Second
	c := NewUTLSClient(want)
	if c.HTTPClient().Timeout != want {
		t.Errorf("Timeout = %v, want %v", c.HTTPClient().Timeout, want)
	}
}

// TestNewUTLSClient_ZeroTimeout verifies zero timeout does not panic.
func TestNewUTLSClient_ZeroTimeout(t *testing.T) {
	c := NewUTLSClient(0)
	if c == nil {
		t.Fatal("NewUTLSClient(0) returned nil")
	}
	if c.HTTPClient() == nil {
		t.Fatal("HTTPClient() returned nil with zero timeout")
	}
}

// TestNewUTLSClient_TransportSet verifies the transport is not nil so that the
// uTLS dial hook is actually wired up.
func TestNewUTLSClient_TransportSet(t *testing.T) {
	c := NewUTLSClient(5 * time.Second)
	if c.HTTPClient().Transport == nil {
		t.Fatal("http.Client Transport is nil; uTLS dial hook will never be called")
	}
}

// TestNewUTLSClient_CookieJarSet verifies a cookie jar is attached so
// session cookies are preserved across redirects.
func TestNewUTLSClient_CookieJarSet(t *testing.T) {
	c := NewUTLSClient(5 * time.Second)
	if c.HTTPClient().Jar == nil {
		t.Error("http.Client Jar is nil; cookies will not be retained")
	}
}

// --- NewRoundTripper ---

// TestNewRoundTripper_NonNil verifies a non-nil http.RoundTripper is returned.
func TestNewRoundTripper_NonNil(t *testing.T) {
	rt := NewRoundTripper(10 * time.Second)
	if rt == nil {
		t.Fatal("NewRoundTripper returned nil")
	}
}

// TestNewRoundTripper_ImplementsInterface verifies the returned value satisfies
// the http.RoundTripper interface at compile and runtime.
func TestNewRoundTripper_ImplementsInterface(t *testing.T) {
	var rt http.RoundTripper = NewRoundTripper(10 * time.Second)
	if rt == nil {
		t.Fatal("NewRoundTripper does not satisfy http.RoundTripper")
	}
}

// TestNewRoundTripper_ZeroTimeout verifies zero timeout does not panic.
func TestNewRoundTripper_ZeroTimeout(t *testing.T) {
	rt := NewRoundTripper(0)
	if rt == nil {
		t.Fatal("NewRoundTripper(0) returned nil")
	}
}

// --- CreateHTTPClientWithFingerprint ---

// TestCreateHTTPClientWithFingerprint_Chrome verifies chrome fingerprint returns
// a valid *http.Client.
func TestCreateHTTPClientWithFingerprint_Chrome(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(10*time.Second, "chrome")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('chrome') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for chrome fingerprint")
	}
}

// TestCreateHTTPClientWithFingerprint_Firefox verifies firefox fingerprint.
func TestCreateHTTPClientWithFingerprint_Firefox(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(10*time.Second, "firefox")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('firefox') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for firefox fingerprint")
	}
}

// TestCreateHTTPClientWithFingerprint_Safari verifies safari fingerprint.
func TestCreateHTTPClientWithFingerprint_Safari(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(10*time.Second, "safari")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('safari') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for safari fingerprint")
	}
}

// TestCreateHTTPClientWithFingerprint_Empty verifies the empty string falls back
// without panicking and returns a usable client.
func TestCreateHTTPClientWithFingerprint_Empty(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(10*time.Second, "")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for empty fingerprint")
	}
}

// TestCreateHTTPClientWithFingerprint_Unknown verifies an unrecognised string
// falls back gracefully without panicking.
func TestCreateHTTPClientWithFingerprint_Unknown(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CreateHTTPClientWithFingerprint panicked on unknown fingerprint: %v", r)
		}
	}()
	c := CreateHTTPClientWithFingerprint(10*time.Second, "unknown-browser-xyz")
	if c == nil {
		t.Fatal("CreateHTTPClientWithFingerprint('unknown-browser-xyz') returned nil")
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil for unknown fingerprint")
	}
}

// TestCreateHTTPClientWithFingerprint_TimeoutPropagated verifies that the
// timeout argument is applied to the returned client.
func TestCreateHTTPClientWithFingerprint_TimeoutPropagated(t *testing.T) {
	want := 7 * time.Second
	c := CreateHTTPClientWithFingerprint(want, "chrome")
	if c.Timeout != want {
		t.Errorf("Timeout = %v, want %v", c.Timeout, want)
	}
}

// TestCreateHTTPClientWithFingerprint_CookieJar verifies a jar is attached.
func TestCreateHTTPClientWithFingerprint_CookieJar(t *testing.T) {
	c := CreateHTTPClientWithFingerprint(5*time.Second, "chrome")
	if c.Jar == nil {
		t.Error("Jar is nil; session cookies will not be retained")
	}
}

// --- UTLSClient.HTTPClient ---

// TestUTLSClient_HTTPClient_NonNil is a direct accessor test that ensures the
// method always returns the object constructed internally.
func TestUTLSClient_HTTPClient_NonNil(t *testing.T) {
	c := NewUTLSClient(10 * time.Second)
	hc := c.HTTPClient()
	if hc == nil {
		t.Fatal("HTTPClient() returned nil")
	}
}

// TestUTLSClient_HTTPClient_SameInstance verifies that repeated calls return
// the same pointer (no accidental reconstruction on each call).
func TestUTLSClient_HTTPClient_SameInstance(t *testing.T) {
	c := NewUTLSClient(10 * time.Second)
	if c.HTTPClient() != c.HTTPClient() {
		t.Error("HTTPClient() returned different pointers on successive calls")
	}
}
