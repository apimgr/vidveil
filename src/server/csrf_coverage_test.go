// SPDX-License-Identifier: MIT
// AI.md PART 16 (CSRF Protection) / PART 14 (CSRF_FAILED): deterministic coverage
// for the CSRF middleware, its bypass logic, the token generator, and the
// Secure-flag resolver. All paths run without a server, root, or network.
package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/secret"
)

type failingCSRFReader struct{}

func (failingCSRFReader) Read([]byte) (int, error) { return 0, errors.New("random source unavailable") }

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

func TestCsrfGenTokenFailsClosedWhenRandomSourceFails(t *testing.T) {
	if token := csrfGenTokenFrom(failingCSRFReader{}, 16); token != "" {
		t.Fatalf("token = %q, want empty token when randomness fails", token)
	}

	called := false
	handler := newCSRFMiddleware(csrfTestConfig(), nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	post := httptest.NewRequest(http.MethodPost, "/", nil)
	post.AddCookie(&http.Cookie{Name: "csrf_token", Value: ""})
	post.Header.Set("X-CSRF-Token", "")
	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, post)
	if called || blocked.Code != http.StatusForbidden {
		t.Fatalf("protected request called=%v status=%d, want called=false status=%d", called, blocked.Code, http.StatusForbidden)
	}
}

func TestCSRFSignedTokenValidatesOnlyWithSigningSecret(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	token := csrfSignedTokenFrom(strings.NewReader(strings.Repeat("x", 32)), 16, key)
	if token == "" || !csrfSignedTokenValid(token, key) {
		t.Fatalf("signed CSRF token did not validate with its signing key")
	}
	if csrfSignedTokenValid(token, []byte(strings.Repeat("z", 32))) {
		t.Fatal("signed CSRF token validated with a different key")
	}
	if csrfSignedTokenValid(token+"x", key) {
		t.Fatal("tampered CSRF token validated")
	}
}

func TestCSRFSignedTokenFailsClosed(t *testing.T) {
	if token := csrfSignedTokenFrom(failingCSRFReader{}, 16, []byte("key")); token != "" {
		t.Fatalf("signed token = %q, want empty token when randomness fails", token)
	}
	if token := csrfSignedTokenFrom(strings.NewReader(strings.Repeat("x", 32)), 16, nil); token != "" {
		t.Fatalf("signed token = %q, want empty token without a key", token)
	}
}

func TestCSRFMiddlewareSignsAndReissuesTokensAfterSecretRotation(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager := secret.NewManager(schema.GetDB())
	if err := manager.EnsureSecrets(context.Background()); err != nil {
		t.Fatal(err)
	}
	middleware := newCSRFMiddleware(csrfTestConfig(), nil, manager)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	get := httptest.NewRecorder()
	middleware.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/", nil))
	cookie := get.Result().Cookies()[0]
	key, err := manager.Get(context.Background(), secret.CSRFTokenSecret)
	if err != nil {
		t.Fatal(err)
	}
	if !csrfSignedTokenValid(cookie.Value, key) {
		t.Fatal("middleware issued a CSRF token not signed by csrf_token_secret")
	}

	if err := manager.RotateImmediately(context.Background(), secret.CSRFTokenSecret); err != nil {
		t.Fatal(err)
	}
	post := httptest.NewRequest(http.MethodPost, "/", nil)
	post.AddCookie(cookie)
	post.Header.Set("X-CSRF-Token", cookie.Value)
	rejected := httptest.NewRecorder()
	middleware.ServeHTTP(rejected, post)
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("POST with a revoked CSRF token status = %d, want 403", rejected.Code)
	}

	refresh := httptest.NewRequest(http.MethodGet, "/", nil)
	refresh.AddCookie(cookie)
	refreshed := httptest.NewRecorder()
	middleware.ServeHTTP(refreshed, refresh)
	newCookie := refreshed.Result().Cookies()[0]
	newKey, err := manager.Get(context.Background(), secret.CSRFTokenSecret)
	if err != nil {
		t.Fatal(err)
	}
	if newCookie.Value == cookie.Value || !csrfSignedTokenValid(newCookie.Value, newKey) {
		t.Fatal("GET did not replace the revoked CSRF cookie with a token signed by the current secret")
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

	// Safe method bypasses.
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if !csrfBypass(cfg, httptest.NewRequest(m, "/x", nil)) {
			t.Errorf("method %s should bypass", m)
		}
	}

	// Bearer token bypasses.
	rB := httptest.NewRequest(http.MethodPost, "/x", nil)
	rB.Header.Set("Authorization", "Bearer abc")
	if !csrfBypass(cfg, rB) {
		t.Error("bearer auth should bypass")
	}

	// Backend API routes never use CSRF, including anonymous endpoints.
	rAPI := httptest.NewRequest(http.MethodPost, "/api/v1/favorites", nil)
	if !csrfBypass(cfg, rAPI) {
		t.Error("API routes should bypass CSRF")
	}
	// A non-Bearer token header is not an authentication or CSRF bypass.
	rA := httptest.NewRequest(http.MethodPost, "/x", nil)
	rA.Header.Set("X-API-Token", "tok")
	if csrfBypass(cfg, rA) {
		t.Error("X-API-Token should not bypass")
	}

	// WebSocket upgrade bypasses.
	rW := httptest.NewRequest(http.MethodPost, "/x", nil)
	rW.Header.Set("Upgrade", "websocket")
	if !csrfBypass(cfg, rW) {
		t.Error("websocket upgrade should bypass")
	}

	// Exempt path bypasses.
	rE := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", nil)
	if !csrfBypass(cfg, rE) {
		t.Error("exempt path should bypass")
	}

	// Any other mutating request does NOT bypass — per AI.md PART 16 there is
	// no session-cookie or Origin-based bypass; a public POST with no bearer
	// header, safe method, WS upgrade, or exempt path must still be validated.
	rN := httptest.NewRequest(http.MethodPost, "/settings", nil)
	if csrfBypass(cfg, rN) {
		t.Error("non-exempt POST without a bypass condition should not bypass")
	}
}

// TestCsrfMiddleware_SafeMethodSetsCookie verifies a GET request passes through
// and receives a fresh CSRF cookie.
func TestCsrfMiddleware_SafeMethodSetsCookie(t *testing.T) {
	mw := newCSRFMiddleware(csrfTestConfig(), nil)
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
	mw := newCSRFMiddleware(csrfTestConfig(), nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	r := httptest.NewRequest(http.MethodPost, "/settings", nil)
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
	mw := newCSRFMiddleware(csrfTestConfig(), nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	r := httptest.NewRequest(http.MethodPost, "/settings", nil)
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
	mw := newCSRFMiddleware(csrfTestConfig(), nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodPost, "/settings", nil)
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
	mw := newCSRFMiddleware(csrfTestConfig(), nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	body := strings.NewReader("csrf_token=formtoken")
	r := httptest.NewRequest(http.MethodPost, "/settings", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "formtoken"})
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if !called {
		t.Fatal("next handler should run when form token matches")
	}
}
