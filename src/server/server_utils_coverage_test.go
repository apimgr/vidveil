// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for server utility functions and additional middleware.
// Tests parseBodySize extras, parseDuration extras, URLNormalizeMiddleware variants,
// extractClientIP extras, isAllowlisted positive, secFetchValidationMiddleware variants,
// Shutdown, debugMiddleware, responseWriter, onionLocationMiddleware, onionLocationWriter.
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── parseBodySize extras ──────────────────────────────────────────────────────

func TestParseBodySize_PlainNumber(t *testing.T) {
	got := parseBodySize("4096", 0)
	if got != 4096 {
		t.Errorf("parseBodySize('4096') = %d, want 4096", got)
	}
}

func TestParseBodySize_InvalidReturnsDefault(t *testing.T) {
	got := parseBodySize("nope", 999)
	if got != 999 {
		t.Errorf("parseBodySize('nope') = %d, want 999 (default)", got)
	}
}

func TestParseBodySize_LowercaseMB(t *testing.T) {
	// parseBodySize uppercases input, so "mb" → "MB"
	got := parseBodySize("5mb", 0)
	want := int64(5 * 1024 * 1024)
	if got != want {
		t.Errorf("parseBodySize('5mb') = %d, want %d", got, want)
	}
}

// ── parseDuration extras ──────────────────────────────────────────────────────

func TestParseDuration_InvalidReturnsDefault(t *testing.T) {
	got := parseDuration("not-a-duration", 10*1e9)
	if got != 10*1e9 {
		t.Errorf("parseDuration('not-a-duration') = %v, want 10s", got)
	}
}

func TestParseDuration_Seconds(t *testing.T) {
	got := parseDuration("30s", 0)
	if got != 30*1e9 {
		t.Errorf("parseDuration('30s') = %v, want 30s", got)
	}
}

// ── URLNormalizeMiddleware variants ───────────────────────────────────────────

// passHandler is a minimal handler that echoes the request path.
func passHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Path", r.URL.Path)
	w.WriteHeader(http.StatusOK)
}

func TestURLNormalizeMiddleware_RootPassThrough(t *testing.T) {
	h := URLNormalizeMiddleware(http.HandlerFunc(passHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("URLNormalize root: status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Path"); got != "/" {
		t.Errorf("URLNormalize root: X-Path = %q, want /", got)
	}
}

func TestURLNormalizeMiddleware_TrailingSlashRedirects(t *testing.T) {
	h := URLNormalizeMiddleware(http.HandlerFunc(passHandler))
	req := httptest.NewRequest(http.MethodGet, "/foo/bar/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("URLNormalize trailing slash: status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/foo/bar") {
		t.Errorf("URLNormalize trailing slash: Location = %q, want /foo/bar", loc)
	}
}

func TestURLNormalizeMiddleware_TrailingSlashWithDotPassThrough(t *testing.T) {
	// Path segment containing a dot — middleware should not panic.
	h := URLNormalizeMiddleware(http.HandlerFunc(passHandler))
	req := httptest.NewRequest(http.MethodGet, "/foo.html/", nil)
	rec := httptest.NewRecorder()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("URLNormalize dot-path panicked: %v", r)
		}
	}()
	h.ServeHTTP(rec, req)
}

func TestURLNormalizeMiddleware_NoTrailingSlashPassThrough(t *testing.T) {
	h := URLNormalizeMiddleware(http.HandlerFunc(passHandler))
	req := httptest.NewRequest(http.MethodGet, "/foo/bar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("URLNormalize no-trailing-slash: status = %d, want 200", rec.Code)
	}
}

func TestURLNormalizeMiddleware_TrailingSlashPreservesQuery(t *testing.T) {
	h := URLNormalizeMiddleware(http.HandlerFunc(passHandler))
	req := httptest.NewRequest(http.MethodGet, "/search/?q=test", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("URLNormalize with query: status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "q=test") {
		t.Errorf("URLNormalize with query: Location = %q, should contain query", loc)
	}
}

// ── extractClientIP extras ────────────────────────────────────────────────────

func TestExtractClientIP_WithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1"
	got := extractClientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("extractClientIP(host-only) = %q, want '10.0.0.1'", got)
	}
}

func TestExtractClientIP_IPv6WithPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:9000"
	got := extractClientIP(req)
	if got != "::1" {
		t.Errorf("extractClientIP([::1]:port) = %q, want '::1'", got)
	}
}

// ── isAllowlisted positive path ───────────────────────────────────────────────

func TestIsAllowlisted_TrueWhenContextSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxKeyAllowlisted, true)
	req = req.WithContext(ctx)
	if !isAllowlisted(req) {
		t.Error("isAllowlisted with context flag = false, want true")
	}
}

// ── secFetchValidationMiddleware variants ─────────────────────────────────────

func TestSecFetchValidation_CrossSitePostNoBearer_Returns403(t *testing.T) {
	h := secFetchValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("secFetch cross-site POST no bearer: status = %d, want 403", rec.Code)
	}
}

func TestSecFetchValidation_CrossSitePostWithBearer_Passes(t *testing.T) {
	h := secFetchValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Authorization", "Bearer token123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("secFetch cross-site POST with bearer: status = %d, want 200", rec.Code)
	}
}

func TestSecFetchValidation_CrossSiteGet_Passes(t *testing.T) {
	h := secFetchValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// GET is a safe method — cross-site restriction only applies to mutations.
	if rec.Code != http.StatusOK {
		t.Errorf("secFetch cross-site GET: status = %d, want 200", rec.Code)
	}
}

func TestSecFetchValidation_NavigateToAPI_Returns403(t *testing.T) {
	h := secFetchValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("secFetch navigate to /api/: status = %d, want 403", rec.Code)
	}
}

func TestSecFetchValidation_NavigateToNonAPI_Passes(t *testing.T) {
	h := secFetchValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("secFetch navigate non-API: status = %d, want 200", rec.Code)
	}
}

func TestSecFetchValidation_NoHeaders_PassesThrough(t *testing.T) {
	h := secFetchValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("secFetch no headers: status = %d, want 200", rec.Code)
	}
}

// ── Shutdown ──────────────────────────────────────────────────────────────────

func TestShutdown_NilSrv_ReturnsNil(t *testing.T) {
	s := &Server{}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown(nil srv) = %v, want nil", err)
	}
}

// ── debugMiddleware ───────────────────────────────────────────────────────────

func TestDebugMiddleware_DebugDisabled_ReturnsNextDirectly(t *testing.T) {
	// When debug is not enabled, debugMiddleware returns next unchanged (no-op path).
	// We verify that the handler is still callable and status passes through.
	s := &Server{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := s.debugMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("debugMiddleware disabled: status = %d, want 204", rec.Code)
	}
}

// ── responseWriter (middleware_debug.go) ──────────────────────────────────────

func TestResponseWriterWriteHeader_SetsStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}
	rw.WriteHeader(http.StatusCreated)

	if rw.status != http.StatusCreated {
		t.Errorf("responseWriter.WriteHeader: status = %d, want 201", rw.status)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("responseWriter.WriteHeader: recorder code = %d, want 201", rec.Code)
	}
}

func TestResponseWriterWrite_UpdatesSize(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}
	n, err := rw.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("responseWriter.Write: unexpected error: %v", err)
	}
	if n != 11 {
		t.Errorf("responseWriter.Write: n = %d, want 11", n)
	}
	if rw.size != 11 {
		t.Errorf("responseWriter.Write: size = %d, want 11", rw.size)
	}
}

func TestResponseWriterWrite_AccumulatesSize(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}
	_, _ = rw.Write([]byte("abc"))
	_, _ = rw.Write([]byte("de"))
	if rw.size != 5 {
		t.Errorf("responseWriter accumulated size = %d, want 5", rw.size)
	}
}

// ── onionLocationMiddleware (Server method) ───────────────────────────────────

func TestOnionLocationMiddleware_NilTorSvc_PassesThrough(t *testing.T) {
	s := &Server{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := s.onionLocationMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("onionLocation nil torSvc: status = %d, want 200", rec.Code)
	}
}

func TestOnionLocationMiddleware_OnionHostPassesThrough(t *testing.T) {
	// If Host ends with .onion, skip (already an onion request).
	s := &Server{}
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := s.onionLocationMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "xyz.onion"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Error("onionLocation .onion host: inner handler not called")
	}
}

// ── onionLocationWriter.Flush ─────────────────────────────────────────────────

// flusherRecorder embeds httptest.ResponseRecorder and implements http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flusherRecorder) Flush() {
	f.flushed = true
}

func TestOnionLocationWriterFlush_DelegatesToFlusher(t *testing.T) {
	fr := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	w := &onionLocationWriter{ResponseWriter: fr, onionURL: "http://test.onion/"}
	w.Flush()
	if !fr.flushed {
		t.Error("onionLocationWriter.Flush: did not call underlying Flusher")
	}
}

func TestOnionLocationWriterFlush_NonFlusherNoPanic(t *testing.T) {
	// httptest.ResponseRecorder implements Flusher in stdlib; verify no panic either way.
	rec := httptest.NewRecorder()
	w := &onionLocationWriter{ResponseWriter: rec, onionURL: "http://test.onion/"}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Flush on non-Flusher panicked: %v", r)
		}
	}()
	w.Flush()
}
