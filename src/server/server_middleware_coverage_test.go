// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for server middleware functions.
// Tests extensionStripMiddleware, onionLocationWriter, allowlistMiddleware,
// blocklistMiddleware, and geoIPMiddleware using httptest utilities.
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
)

// passThrough is a handler that records the request path and writes 200 OK.
func passThrough(t *testing.T, gotPath *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
	})
}

// ── extensionStripMiddleware ──────────────────────────────────────────────────

func TestExtensionStripMiddleware_NonAPIPassThrough(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/search.json", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	// Non-API path: path must be unchanged
	if got != "/search.json" {
		t.Errorf("extensionStripMiddleware non-API: path = %q, want '/search.json'", got)
	}
}

func TestExtensionStripMiddleware_APIStripsJSON(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/api/search.json", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "/api/search" {
		t.Errorf("extensionStripMiddleware .json: path = %q, want '/api/search'", got)
	}
}

func TestExtensionStripMiddleware_APIStripsTxt(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/api/search.txt", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "/api/search" {
		t.Errorf("extensionStripMiddleware .txt: path = %q, want '/api/search'", got)
	}
}

func TestExtensionStripMiddleware_APIStripsRSS(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/api/search.rss", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "/api/search" {
		t.Errorf("extensionStripMiddleware .rss: path = %q, want '/api/search'", got)
	}
}

func TestExtensionStripMiddleware_APIStripsAtom(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/api/search.atom", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "/api/search" {
		t.Errorf("extensionStripMiddleware .atom: path = %q, want '/api/search'", got)
	}
}

func TestExtensionStripMiddleware_APIStripsCSV(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/api/search.csv", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "/api/search" {
		t.Errorf("extensionStripMiddleware .csv: path = %q, want '/api/search'", got)
	}
}

func TestExtensionStripMiddleware_APINoExtension(t *testing.T) {
	var got string
	h := extensionStripMiddleware(passThrough(t, &got))
	req := httptest.NewRequest("GET", "/api/search", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got != "/api/search" {
		t.Errorf("extensionStripMiddleware no-ext: path = %q, want '/api/search'", got)
	}
}

// ── onionLocationWriter ───────────────────────────────────────────────────────

func TestOnionLocationWriter_HTMLResponseSetsHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/"}
	rr.Header().Set("Content-Type", "text/html; charset=utf-8")
	ow.WriteHeader(http.StatusOK)
	if rr.Header().Get("Onion-Location") != "http://test.onion/" {
		t.Errorf("onionLocationWriter: Onion-Location header not set for text/html response")
	}
}

func TestOnionLocationWriter_NonHTMLResponseNoHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/"}
	rr.Header().Set("Content-Type", "application/json")
	ow.WriteHeader(http.StatusOK)
	if rr.Header().Get("Onion-Location") != "" {
		t.Errorf("onionLocationWriter: Onion-Location header set for non-HTML response")
	}
}

func TestOnionLocationWriter_WriteTriggersHeaderCheck(t *testing.T) {
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/"}
	rr.Header().Set("Content-Type", "text/html")
	_, _ = ow.Write([]byte("hello"))
	if rr.Header().Get("Onion-Location") != "http://test.onion/" {
		t.Errorf("onionLocationWriter.Write: Onion-Location not set")
	}
}

func TestOnionLocationWriter_WriteHeaderIdempotent(t *testing.T) {
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/"}
	rr.Header().Set("Content-Type", "text/html")
	ow.WriteHeader(http.StatusOK)
	// Override content type after first WriteHeader — should not change behavior
	rr.Header().Set("Content-Type", "application/json")
	ow.WriteHeader(http.StatusOK)
	// Header was set during first WriteHeader (text/html), second call is a no-op
	if rr.Header().Get("Onion-Location") != "http://test.onion/" {
		t.Errorf("onionLocationWriter: second WriteHeader should be a no-op")
	}
}

func TestOnionLocationWriter_Unwrap(t *testing.T) {
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/"}
	if ow.Unwrap() != rr {
		t.Error("onionLocationWriter.Unwrap: returned wrong ResponseWriter")
	}
}

// ── allowlistMiddleware ───────────────────────────────────────────────────────

func newTestServerWithConfig(cfg *config.AppConfig) *Server {
	return &Server{
		appConfig: cfg,
		router:    chi.NewRouter(),
	}
}

func TestAllowlistMiddleware_EmptyAllowlist_PassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Allowlist = nil
	s := newTestServerWithConfig(cfg)

	called := false
	h := s.allowlistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("allowlistMiddleware: next handler not called with empty allowlist")
	}
}

func TestAllowlistMiddleware_IPInAllowlist_SetsContext(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Allowlist = []config.AllowlistEntry{{CIDR: "1.2.3.4"}}
	s := newTestServerWithConfig(cfg)

	var allowlisted bool
	h := s.allowlistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowlisted = isAllowlisted(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !allowlisted {
		t.Error("allowlistMiddleware: isAllowlisted should be true for IP in allowlist")
	}
}

func TestAllowlistMiddleware_IPNotInAllowlist_NotMarked(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Allowlist = []config.AllowlistEntry{{CIDR: "10.0.0.0/8"}}
	s := newTestServerWithConfig(cfg)

	var allowlisted bool
	h := s.allowlistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowlisted = isAllowlisted(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if allowlisted {
		t.Error("allowlistMiddleware: isAllowlisted should be false for IP not in allowlist")
	}
}

func TestAllowlistMiddleware_IPv6CIDR_Expanded(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Allowlist = []config.AllowlistEntry{{CIDR: "::1"}}
	s := newTestServerWithConfig(cfg)

	var allowlisted bool
	h := s.allowlistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowlisted = isAllowlisted(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !allowlisted {
		t.Error("allowlistMiddleware: IPv6 ::1 should be allowlisted")
	}
}

// ── blocklistMiddleware ───────────────────────────────────────────────────────

type mockBlocklist struct {
	blocked bool
}

func (m *mockBlocklist) IsBlocked(_ string) bool {
	return m.blocked
}

func TestBlocklistMiddleware_DisabledPassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = false
	s := newTestServerWithConfig(cfg)
	s.ipBlocklist = &mockBlocklist{blocked: true}

	called := false
	h := s.blocklistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("blocklistMiddleware: should pass through when blocklists disabled")
	}
}

func TestBlocklistMiddleware_NilBlocklist_PassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = true
	s := newTestServerWithConfig(cfg)
	s.ipBlocklist = nil

	called := false
	h := s.blocklistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("blocklistMiddleware: should pass through with nil blocklist")
	}
}

func TestBlocklistMiddleware_BlockedIP_Returns403(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = true
	s := newTestServerWithConfig(cfg)
	s.ipBlocklist = &mockBlocklist{blocked: true}

	h := s.blocklistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("blocklistMiddleware: blocked IP got %d, want 403", rr.Code)
	}
}

func TestBlocklistMiddleware_NotBlockedIP_PassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = true
	s := newTestServerWithConfig(cfg)
	s.ipBlocklist = &mockBlocklist{blocked: false}

	called := false
	h := s.blocklistMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("blocklistMiddleware: non-blocked IP should pass through")
	}
}

// ── geoIPMiddleware ───────────────────────────────────────────────────────────

type mockGeoIPBlocker struct {
	blocked bool
}

func (m *mockGeoIPBlocker) IsBlocked(_ string) bool {
	return m.blocked
}

func TestGeoIPMiddleware_DisabledPassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	s := newTestServerWithConfig(cfg)
	s.geoIPBlocker = &mockGeoIPBlocker{blocked: true}

	called := false
	h := s.geoIPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("geoIPMiddleware: should pass through when GeoIP disabled")
	}
}

func TestGeoIPMiddleware_NilBlocker_PassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	s := newTestServerWithConfig(cfg)
	s.geoIPBlocker = nil

	called := false
	h := s.geoIPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("geoIPMiddleware: should pass through with nil blocker")
	}
}

func TestGeoIPMiddleware_NoCountryLists_PassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.DenyCountries = nil
	cfg.Server.GeoIP.AllowCountries = nil
	s := newTestServerWithConfig(cfg)
	s.geoIPBlocker = &mockGeoIPBlocker{blocked: true}

	called := false
	h := s.geoIPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("geoIPMiddleware: should pass through when no country lists configured")
	}
}

func TestGeoIPMiddleware_BlockedCountry_Returns403(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.DenyCountries = []string{"US"}
	s := newTestServerWithConfig(cfg)
	s.geoIPBlocker = &mockGeoIPBlocker{blocked: true}

	h := s.geoIPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("geoIPMiddleware: blocked country got %d, want 403", rr.Code)
	}
}

func TestGeoIPMiddleware_NotBlockedCountry_PassesThrough(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.DenyCountries = []string{"CN"}
	s := newTestServerWithConfig(cfg)
	s.geoIPBlocker = &mockGeoIPBlocker{blocked: false}

	called := false
	h := s.geoIPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Error("geoIPMiddleware: non-blocked country should pass through")
	}
}
