// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for expvar callbacks, Cache-Control middleware branches,
// SSL HSTS header, Healthz.Root routes, and onionLocationMiddleware skip paths.
package server

import (
	"expvar"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
)

// ── mock tor service with onion address ───────────────────────────────────────

type mockTorWithOnion struct {
	addr string
}

func (m *mockTorWithOnion) IsEnabled() bool                   { return true }
func (m *mockTorWithOnion) IsRunning() bool                   { return true }
func (m *mockTorWithOnion) IsStarting() bool                  { return false }
func (m *mockTorWithOnion) AllowUserIPForward() bool          { return false }
func (m *mockTorWithOnion) UseNetworkEnabled() bool           { return false }
func (m *mockTorWithOnion) OutboundEnabled() bool             { return false }
func (m *mockTorWithOnion) GetHTTPClient(_ bool) *http.Client { return &http.Client{} }
func (m *mockTorWithOnion) GetInfo() map[string]interface{} {
	if m.addr == "" {
		return map[string]interface{}{}
	}
	return map[string]interface{}{"onion_address": m.addr}
}

// ── helper: server with custom config modifications ───────────────────────────

func newTestServerWithCfg(t *testing.T, modify func(cfg *config.AppConfig)) *Server {
	t.Helper()
	cfg := config.DefaultAppConfig()
	if modify != nil {
		modify(cfg)
	}
	base := filepath.Join(os.TempDir(), "apimgr")
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-srv-cfg-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })
	engineMgr := engine.NewEngineManager(cfg)
	sched := scheduler.NewScheduler()
	var logger *logging.AppLogger
	logger, err = logging.NewAppLogger(cfg)
	if err != nil {
		logger = nil
	}
	return NewServer(cfg, tmp, tmp, engineMgr, &stubMigrationManager{}, sched, logger)
}

// ── expvar callback coverage ──────────────────────────────────────────────────

func TestExpvar_UptimeSecondsCallback(t *testing.T) {
	v := expvar.Get("uptime_seconds")
	if v == nil {
		t.Fatal("expvar 'uptime_seconds' not registered")
	}
	fn, ok := v.(expvar.Func)
	if !ok {
		t.Fatal("uptime_seconds is not expvar.Func")
	}
	result := fn.Value()
	if _, ok := result.(float64); !ok {
		t.Errorf("uptime_seconds: want float64, got %T", result)
	}
}

func TestExpvar_GoroutinesCallback(t *testing.T) {
	v := expvar.Get("goroutines")
	if v == nil {
		t.Fatal("expvar 'goroutines' not registered")
	}
	fn, ok := v.(expvar.Func)
	if !ok {
		t.Fatal("goroutines is not expvar.Func")
	}
	result := fn.Value()
	if _, ok := result.(int); !ok {
		t.Errorf("goroutines: want int, got %T", result)
	}
}

func TestExpvar_MemoryCallback(t *testing.T) {
	v := expvar.Get("memory")
	if v == nil {
		t.Fatal("expvar 'memory' not registered")
	}
	fn, ok := v.(expvar.Func)
	if !ok {
		t.Fatal("memory is not expvar.Func")
	}
	result := fn.Value()
	m, ok := result.(map[string]uint64)
	if !ok {
		t.Fatalf("memory: want map[string]uint64, got %T", result)
	}
	for _, key := range []string{"alloc", "total_alloc", "sys", "heap_alloc", "heap_sys"} {
		if _, found := m[key]; !found {
			t.Errorf("memory map missing key %q", key)
		}
	}
}

// ── Cache-Control middleware branches ─────────────────────────────────────────

func TestMiddleware_CacheControl_StaticPath(t *testing.T) {
	s := newTestServer(t)
	// Use manifest.json which exists in the embedded static FS (a 404 would clear
	// Cache-Control in Go 1.22+ via http.Error header cleanup).
	req := httptest.NewRequest(http.MethodGet, "/static/manifest.json", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	cc := rr.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("Cache-Control for /static/ want immutable long cache, got %q", cc)
	}
}

func TestMiddleware_CacheControl_APIPath(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/status", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	cc := rr.Header().Get("Cache-Control")
	if !strings.Contains(cc, "no-cache") {
		t.Errorf("Cache-Control for /api/ want no-cache, got %q", cc)
	}
	pragma := rr.Header().Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("Pragma for /api/ want no-cache, got %q", pragma)
	}
}

// ── SSL HSTS header ───────────────────────────────────────────────────────────

func TestMiddleware_HSTS_WhenSSLEnabled(t *testing.T) {
	s := newTestServerWithCfg(t, func(cfg *config.AppConfig) {
		cfg.Server.SSL.Enabled = true
	})
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=63072000") {
		t.Errorf("HSTS header when SSL.Enabled=true: got %q", hsts)
	}
}

// ── Healthz root routes ───────────────────────────────────────────────────────

func TestServer_HealthzRoot_Returns200_WhenEnabled(t *testing.T) {
	s := newTestServerWithCfg(t, func(cfg *config.AppConfig) {
		cfg.Server.Healthz.Root.Enabled = true
	})
	for _, path := range []string{"/healthz", "/healthz.json", "/healthz.txt"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		s.router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s with Healthz.Root.Enabled=true: status=%d want 200", path, rr.Code)
		}
	}
}

func TestServer_HealthzRoot_Returns404_WhenDisabled(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("/healthz when disabled: status=%d want 404", rr.Code)
	}
}

// ── onionLocationMiddleware branches ─────────────────────────────────────────

func TestOnionMiddleware_APIPath_Skipped(t *testing.T) {
	s := newTestServer(t)
	s.SetTorService(&mockTorWithOnion{addr: "test.onion"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/status", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Header().Get("Onion-Location") != "" {
		t.Error("Onion-Location must not be set on /api/ paths")
	}
}

func TestOnionMiddleware_SSEAccept_Skipped(t *testing.T) {
	s := newTestServer(t)
	s.SetTorService(&mockTorWithOnion{addr: "test.onion"})
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("Accept", "text/event-stream")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Header().Get("Onion-Location") != "" {
		t.Error("Onion-Location must not be set on SSE (text/event-stream) requests")
	}
}

func TestOnionMiddleware_NonHTMLAccept_Skipped(t *testing.T) {
	s := newTestServer(t)
	s.SetTorService(&mockTorWithOnion{addr: "test.onion"})
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Header().Get("Onion-Location") != "" {
		t.Error("Onion-Location must not be set on non-HTML Accept requests")
	}
}

func TestOnionMiddleware_NoOnionAddress_Skipped(t *testing.T) {
	s := newTestServer(t)
	s.SetTorService(&mockTorWithOnion{addr: ""})
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Header().Get("Onion-Location") != "" {
		t.Error("Onion-Location must not be set when GetInfo has no onion_address")
	}
}

func TestOnionMiddleware_HTMLRequest_SetsOnionLocation(t *testing.T) {
	s := newTestServer(t)
	s.SetTorService(&mockTorWithOnion{addr: "abcdef1234.onion"})
	// "/" redirects with Content-Type: text/html; charset=utf-8, which triggers
	// the onionLocationWriter to set the Onion-Location header.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,*/*")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	loc := rr.Header().Get("Onion-Location")
	if !strings.Contains(loc, "abcdef1234.onion") {
		t.Errorf("Onion-Location not set on HTML response: got %q", loc)
	}
}

func TestOnionLocationWriter_SetsOnionLocation(t *testing.T) {
	// Test onionLocationWriter.WriteHeader directly — covers the text/html branch
	// that sets Onion-Location on the response writer.
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/path"}
	ow.Header().Set("Content-Type", "text/html; charset=utf-8")
	ow.WriteHeader(http.StatusOK)
	loc := rr.Header().Get("Onion-Location")
	if loc != "http://test.onion/path" {
		t.Errorf("Onion-Location: got %q, want http://test.onion/path", loc)
	}
}

func TestOnionLocationWriter_NonHTML_NoHeader(t *testing.T) {
	// When Content-Type is not text/html, Onion-Location must not be set.
	rr := httptest.NewRecorder()
	ow := &onionLocationWriter{ResponseWriter: rr, onionURL: "http://test.onion/path"}
	ow.Header().Set("Content-Type", "application/json")
	ow.WriteHeader(http.StatusOK)
	if loc := rr.Header().Get("Onion-Location"); loc != "" {
		t.Errorf("Onion-Location should not be set for non-HTML: got %q", loc)
	}
}

func TestOnionMiddleware_OnionHost_Skipped(t *testing.T) {
	s := newTestServer(t)
	s.SetTorService(&mockTorWithOnion{addr: "abcdef1234.onion"})
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Host = "abcdef1234.onion"
	req.Header.Set("Accept", "text/html,*/*")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Header().Get("Onion-Location") != "" {
		t.Error("Onion-Location must not be set when request host is .onion")
	}
}
