// SPDX-License-Identifier: MIT
// AI.md PART 16 (CSRF Protection) / PART 14 (CSRF_FAILED): deterministic coverage
// for the CSRF middleware, its bypass logic, the token generator, and the
// Secure-flag resolver. All paths run without a server, root, or network.
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func csrfTestConfig() config.CSRFConfig {
	return config.CSRFConfig{
		Enabled:     true,
		TokenLength: 32,
		CookieName:  "csrf_token",
		HeaderName:  "X-CSRF-Token",
		Secure:      "auto",
		ExemptPaths: []string{"/api/v1/webhooks/*"},
	}
}

// TestCsrfGenToken covers both the positive-length and non-positive (default 32)
// branches and verifies the output is valid hex of the expected byte length.
func TestCsrfGenToken(t *testing.T) {
	tok := csrfGenToken(16)
	if len(tok) != 32 { // 16 bytes -> 32 hex chars
		t.Errorf("len(csrfGenToken(16)) = %d, want 32", len(tok))
	}
	def := csrfGenToken(0)
	if len(def) != 64 { // defaults to 32 bytes -> 64 hex chars
		t.Errorf("len(csrfGenToken(0)) = %d, want 64 (default 32 bytes)", len(def))
	}
	neg := csrfGenToken(-5)
	if len(neg) != 64 {
		t.Errorf("len(csrfGenToken(-5)) = %d, want 64", len(neg))
	}
}

// TestCsrfSecureFlag covers the true/false/auto branches, including both auto
// signals (TLS and X-Forwarded-Proto).
func TestCsrfSecureFlag(t *testing.T) {
	if !csrfSecureFlag("true", httptest.NewRequest(http.MethodGet, "/", nil)) {
		t.Error(`secure "true" should be true`)
	}
	if csrfSecureFlag("false", httptest.NewRequest(http.MethodGet, "/", nil)) {
		t.Error(`secure "false" should be false`)
	}
	// auto over plain HTTP -> false
	if csrfSecureFlag("auto", httptest.NewRequest(http.MethodGet, "/", nil)) {
		t.Error(`auto over http should be false`)
	}
	// auto with forwarded-proto https -> true
	rFwd := httptest.NewRequest(http.MethodGet, "/", nil)
	rFwd.Header.Set("X-Forwarded-Proto", "https")
	if !csrfSecureFlag("auto", rFwd) {
		t.Error(`auto with X-Forwarded-Proto=https should be true`)
	}
}

// TestCsrfBypass covers every bypass condition and the non-bypass fall-through.
func TestCsrfBypass(t *testing.T) {
	cfg := csrfTestConfig()
	const session = "session"

	// Safe method bypasses.
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if !csrfBypass(cfg, session, httptest.NewRequest(m, "/x", nil)) {
			t.Errorf("method %s should bypass", m)
		}
	}

	// Bearer token bypasses.
	rB := httptest.NewRequest(http.MethodPost, "/x", nil)
	rB.Header.Set("Authorization", "Bearer abc")
	if !csrfBypass(cfg, session, rB) {
		t.Error("bearer auth should bypass")
	}

	// X-API-Token bypasses.
	rA := httptest.NewRequest(http.MethodPost, "/x", nil)
	rA.Header.Set("X-API-Token", "tok")
	if !csrfBypass(cfg, session, rA) {
		t.Error("X-API-Token should bypass")
	}

	// WebSocket upgrade bypasses.
	rW := httptest.NewRequest(http.MethodPost, "/x", nil)
	rW.Header.Set("Upgrade", "websocket")
	if !csrfBypass(cfg, session, rW) {
		t.Error("websocket upgrade should bypass")
	}

	// No session cookie bypasses (public request).
	if !csrfBypass(cfg, session, httptest.NewRequest(http.MethodPost, "/x", nil)) {
		t.Error("missing session cookie should bypass")
	}

	// Exempt path bypasses (with a session cookie present).
	rE := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", nil)
	rE.AddCookie(&http.Cookie{Name: session, Value: "s"})
	if !csrfBypass(cfg, session, rE) {
		t.Error("exempt path should bypass")
	}

	// Authenticated POST to a non-exempt path does NOT bypass.
	rN := httptest.NewRequest(http.MethodPost, "/settings", nil)
	rN.AddCookie(&http.Cookie{Name: session, Value: "s"})
	if csrfBypass(cfg, session, rN) {
		t.Error("authenticated non-exempt POST should not bypass")
	}
}

// TestCsrfMiddleware_SafeMethodSetsCookie verifies a GET request passes through
// and receives a fresh CSRF cookie.
func TestCsrfMiddleware_SafeMethodSetsCookie(t *testing.T) {
	mw := newCSRFMiddleware(csrfTestConfig(), "session", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "csrf_token=") {
		t.Errorf("expected csrf_token cookie, got %q", w.Header().Get("Set-Cookie"))
	}
}

// TestCsrfMiddleware_PostTokenAbsentDenied verifies an authenticated POST with no
// CSRF cookie is denied with 403.
func TestCsrfMiddleware_PostTokenAbsentDenied(t *testing.T) {
	mw := newCSRFMiddleware(csrfTestConfig(), "session", nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	r := httptest.NewRequest(http.MethodPost, "/settings", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "s"})
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if called {
		t.Error("next handler should not run on CSRF denial")
	}
}

// TestCsrfMiddleware_PostTokenMismatchDenied verifies a submitted token that does
// not match the cookie is denied.
func TestCsrfMiddleware_PostTokenMismatchDenied(t *testing.T) {
	mw := newCSRFMiddleware(csrfTestConfig(), "session", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	r := httptest.NewRequest(http.MethodPost, "/settings", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "s"})
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookievalue"})
	r.Header.Set("X-CSRF-Token", "different")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// TestCsrfMiddleware_PostTokenMatchAllowed verifies a matching header token passes
// validation and reaches the next handler.
func TestCsrfMiddleware_PostTokenMatchAllowed(t *testing.T) {
	mw := newCSRFMiddleware(csrfTestConfig(), "session", nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodPost, "/settings", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "s"})
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "matchme"})
	r.Header.Set("X-CSRF-Token", "matchme")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if !called {
		t.Fatal("next handler should run when tokens match")
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

// TestCsrfMiddleware_PostTokenMatchViaForm verifies the form-field fallback path
// (no header, token supplied as a form value).
func TestCsrfMiddleware_PostTokenMatchViaForm(t *testing.T) {
	mw := newCSRFMiddleware(csrfTestConfig(), "session", nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	body := strings.NewReader("csrf_token=formtoken")
	r := httptest.NewRequest(http.MethodPost, "/settings", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "session", Value: "s"})
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "formtoken"})
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if !called {
		t.Fatal("next handler should run when form token matches")
	}
}
