// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for server debug handlers and setter methods.
// Tests handleDebugConfig, handleDebugRoutes, handleDebugCache, handleDebugMemory,
// handleDebugGoroutines, handleDebugDB, handleDebugScheduler, handleDebugEngines,
// handleDebugEngine, registerDebugRoutes (early-return path), debugLog,
// debugLogDB, debugLogCache, SetTorService, SetGeoIPService, SetBlocklistService.
package server

import (
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
)

// ── mock MigrationManager ─────────────────────────────────────────────────────

type mockMigrationMgr struct {
	db *sql.DB
}

func (m *mockMigrationMgr) GetMigrationStatus() ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *mockMigrationMgr) RunMigrations() error     { return nil }
func (m *mockMigrationMgr) RollbackMigration() error { return nil }
func (m *mockMigrationMgr) GetDB() *sql.DB           { return m.db }

// ── mock types for server setters ─────────────────────────────────────────────

type mockSrvTorChecker struct{}

func (m *mockSrvTorChecker) IsEnabled() bool                   { return false }
func (m *mockSrvTorChecker) IsRunning() bool                   { return false }
func (m *mockSrvTorChecker) IsStarting() bool                  { return false }
func (m *mockSrvTorChecker) AllowUserIPForward() bool          { return false }
func (m *mockSrvTorChecker) UseNetworkEnabled() bool           { return false }
func (m *mockSrvTorChecker) OutboundEnabled() bool             { return false }
func (m *mockSrvTorChecker) GetInfo() map[string]interface{}   { return nil }
func (m *mockSrvTorChecker) GetHTTPClient(_ bool) *http.Client { return &http.Client{} }

type mockSrvGeoIPChecker struct{}

func (m *mockSrvGeoIPChecker) IsEnabled() bool            { return false }
func (m *mockSrvGeoIPChecker) GetRestrictionMode() string { return "off" }
func (m *mockSrvGeoIPChecker) CheckContentRestriction(_ string, _ bool) *geoip.RestrictionResult {
	return nil
}

// mockSrvGeoIPBlocker implements both handler.GeoIPChecker and GeoIPBlocker.
type mockSrvGeoIPBlocker struct{}

func (m *mockSrvGeoIPBlocker) IsEnabled() bool            { return false }
func (m *mockSrvGeoIPBlocker) GetRestrictionMode() string { return "off" }
func (m *mockSrvGeoIPBlocker) CheckContentRestriction(_ string, _ bool) *geoip.RestrictionResult {
	return nil
}
func (m *mockSrvGeoIPBlocker) IsBlocked(_ string) bool { return false }

type mockSrvBlocklist struct{}

func (m *mockSrvBlocklist) IsBlocked(_ string) bool { return false }

func TestRefreshOperatorTokenHashAfterConfigReload(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Token = "first-operator-token"
	s := &Server{appConfig: cfg}
	s.RefreshOperatorTokenHash()
	first, configured := s.getOperatorTokenHash()
	if !configured {
		t.Fatal("expected configured operator token hash")
	}

	cfg.Server.Token = "second-operator-token"
	s.RefreshOperatorTokenHash()
	second, configured := s.getOperatorTokenHash()
	if !configured || first == second {
		t.Fatal("operator token hash was not refreshed after config reload")
	}
}

// ── server setters ────────────────────────────────────────────────────────────

func TestSetTorService_StoresService(t *testing.T) {
	s := &Server{appConfig: config.DefaultAppConfig()}
	svc := &mockSrvTorChecker{}
	s.SetTorService(svc)
	if s.torSvc != svc {
		t.Error("SetTorService: torSvc field not set")
	}
}

func TestSetTorService_WithSearchHandler(t *testing.T) {
	cfg := config.DefaultAppConfig()
	s := &Server{
		appConfig:     cfg,
		searchHandler: handler.NewSearchHandler(cfg, nil),
	}
	svc := &mockSrvTorChecker{}
	// Must not panic when searchHandler is non-nil.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetTorService with searchHandler panicked: %v", r)
		}
	}()
	s.SetTorService(svc)
}

func TestSetGeoIPService_GeoIPCheckerOnly(t *testing.T) {
	s := &Server{appConfig: config.DefaultAppConfig()}
	svc := &mockSrvGeoIPChecker{}
	// GeoIPChecker-only mock — type assertion to GeoIPBlocker should fail gracefully.
	s.SetGeoIPService(svc)
	// geoIPBlocker should remain nil since mock doesn't implement GeoIPBlocker.
	if s.geoIPBlocker != nil {
		t.Error("SetGeoIPService: geoIPBlocker should be nil for non-blocker")
	}
}

func TestSetGeoIPService_WithBlocker(t *testing.T) {
	s := &Server{appConfig: config.DefaultAppConfig()}
	svc := &mockSrvGeoIPBlocker{}
	s.SetGeoIPService(svc)
	// Type assertion should succeed and geoIPBlocker should be set.
	if s.geoIPBlocker == nil {
		t.Error("SetGeoIPService: geoIPBlocker should be set for blocker type")
	}
}

func TestSetBlocklistService_StoresService(t *testing.T) {
	s := &Server{appConfig: config.DefaultAppConfig()}
	svc := &mockSrvBlocklist{}
	s.SetBlocklistService(svc)
	if s.ipBlocklist != svc {
		t.Error("SetBlocklistService: ipBlocklist field not set")
	}
}

// ── registerDebugRoutes (early return when debug disabled) ────────────────────

func TestRegisterDebugRoutes_DebugDisabled_NoRoutes(t *testing.T) {
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		router:    chi.NewRouter(),
	}
	// In test environment debug mode is not enabled → function returns early.
	// Verify it does not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("registerDebugRoutes panicked: %v", r)
		}
	}()
	s.registerDebugRoutes(s.router)
}

// ── handleDebugConfig ─────────────────────────────────────────────────────────

func TestHandleDebugConfig_ReturnsJSON(t *testing.T) {
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		router:    chi.NewRouter(),
	}
	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
	rec := httptest.NewRecorder()
	s.handleDebugConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugConfig: status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("handleDebugConfig: empty body")
	}
}

// ── handleDebugRoutes ─────────────────────────────────────────────────────────

func TestHandleDebugRoutes_ReturnsJSON(t *testing.T) {
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		router:    chi.NewRouter(),
	}
	// Register a test route so the walk has something.
	s.router.Get("/test", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/debug/routes", nil)
	rec := httptest.NewRecorder()
	s.handleDebugRoutes(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugRoutes: status = %d, want 200", rec.Code)
	}
}

// ── handleDebugCache ──────────────────────────────────────────────────────────

func TestHandleDebugCache_ReturnsJSON(t *testing.T) {
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		router:    chi.NewRouter(),
	}
	req := httptest.NewRequest(http.MethodGet, "/debug/cache", nil)
	rec := httptest.NewRecorder()
	s.handleDebugCache(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugCache: status = %d, want 200", rec.Code)
	}
}

// ── handleDebugMemory ─────────────────────────────────────────────────────────

func TestHandleDebugMemory_ReturnsJSON(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/debug/memory", nil)
	rec := httptest.NewRecorder()
	s.handleDebugMemory(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugMemory: status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("handleDebugMemory: empty body")
	}
}

// ── handleDebugGoroutines ─────────────────────────────────────────────────────

func TestHandleDebugGoroutines_ReturnsJSON(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/debug/goroutines", nil)
	rec := httptest.NewRecorder()
	s.handleDebugGoroutines(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugGoroutines: status = %d, want 200", rec.Code)
	}
}

// ── debugLog (early return when debug disabled) ───────────────────────────────

func TestDebugLog_DebugDisabled_NoPanic(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("debugLog panicked: %v", r)
		}
	}()
	s.debugLog(req, 200, 50*time.Millisecond, 100)
}

// ── debugLogDB (early return when debug disabled) ─────────────────────────────

func TestDebugLogDB_DebugDisabled_NoPanic(t *testing.T) {
	s := &Server{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("debugLogDB panicked: %v", r)
		}
	}()
	s.debugLogDB("SELECT 1", nil, 10*time.Millisecond, nil)
}

// ── debugLogCache (early return when debug disabled) ──────────────────────────

func TestDebugLogCache_DebugDisabled_NoPanic(t *testing.T) {
	s := &Server{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("debugLogCache panicked: %v", r)
		}
	}()
	s.debugLogCache("get", "key:123", true, 500*time.Microsecond)
}

// ── handleDebugDB ─────────────────────────────────────────────────────────────

func TestHandleDebugDB_NilDB_ReturnsJSON(t *testing.T) {
	s := &Server{
		appConfig:    config.DefaultAppConfig(),
		migrationMgr: &mockMigrationMgr{db: nil},
	}
	req := httptest.NewRequest(http.MethodGet, "/debug/db", nil)
	rec := httptest.NewRecorder()
	s.handleDebugDB(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugDB nil DB: status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("handleDebugDB nil DB: empty body")
	}
}

// ── handleDebugScheduler ──────────────────────────────────────────────────────

func TestHandleDebugScheduler_EmptyScheduler_ReturnsJSON(t *testing.T) {
	sched := scheduler.NewScheduler()
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		scheduler: sched,
	}
	req := httptest.NewRequest(http.MethodGet, "/debug/scheduler", nil)
	rec := httptest.NewRecorder()
	s.handleDebugScheduler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugScheduler: status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("handleDebugScheduler: empty body")
	}
}

// ── handleDebugEngines ────────────────────────────────────────────────────────

// Empty engine manager (no InitializeEngines call) → DebugSearch returns immediately.
func TestHandleDebugEngines_EmptyManager_ReturnsJSON(t *testing.T) {
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		engineMgr: mgr,
	}
	req := httptest.NewRequest(http.MethodGet, "/debug/engines?q=test", nil)
	rec := httptest.NewRecorder()
	s.handleDebugEngines(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleDebugEngines: status = %d, want 200", rec.Code)
	}
}

// ── handleDebugEngine ─────────────────────────────────────────────────────────

// Non-existent engine name → 404 JSON response.
// chi.URLParam returns "" when no route context is present → GetEngine("") not found.
func TestHandleDebugEngine_NotFound_Returns404(t *testing.T) {
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	s := &Server{
		appConfig: config.DefaultAppConfig(),
		engineMgr: mgr,
	}
	// No chi route context → chi.URLParam(r, "name") returns "" → not found.
	req := httptest.NewRequest(http.MethodGet, "/debug/engine/nonexistent?q=test", nil)
	rec := httptest.NewRecorder()
	s.handleDebugEngine(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("handleDebugEngine not found: status = %d, want 404", rec.Code)
	}
}

// ── registerDebugRoutes — debug enabled path ──────────────────────────────────

func TestRegisterDebugRoutes_DebugEnabled_RegistersRoutes(t *testing.T) {
	// Import mode from src/mode package
	// We use t.Cleanup to restore debug state
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	s := &Server{
		appConfig: config.DefaultAppConfig(),
		router:    chi.NewRouter(),
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("registerDebugRoutes (enabled) panicked: %v", r)
		}
	}()
	s.registerDebugRoutes(s.router)
}

// ── debugLog — debug enabled path ─────────────────────────────────────────────

func TestDebugLog_DebugEnabled_NoPanic(t *testing.T) {
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	s := &Server{appConfig: config.DefaultAppConfig()}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	s.debugLog(req, 200, 50*time.Millisecond, 1024)
}

func TestDebugLogDB_DebugEnabled_NoPanic(t *testing.T) {
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	s := &Server{appConfig: config.DefaultAppConfig()}
	s.debugLogDB("SELECT 1", []any{"arg"}, 5*time.Millisecond, nil)
}

func TestDebugLogCache_DebugEnabled_NoPanic(t *testing.T) {
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	s := &Server{appConfig: config.DefaultAppConfig()}
	s.debugLogCache("GET", "test-key", true, 2*time.Millisecond)
}

func TestDebugRoutesRequireOperatorToken(t *testing.T) {
	wasDebugEnabled := mode.IsDebugEnabled()
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(wasDebugEnabled) })

	cfg := config.DefaultAppConfig()
	cfg.Server.Token = "operator-secret"
	s := &Server{appConfig: cfg, router: chi.NewRouter()}
	s.RefreshOperatorTokenHash()
	s.registerDebugRoutes(s.router)
	searchHandler := handler.NewSearchHandler(cfg, engine.NewEngineManager(cfg))
	s.registerDebugAPIRoutes(s.router, "/api/debug", searchHandler)
	s.registerDebugAPIRoutes(s.router, "/api/v1/debug", searchHandler)

	paths := []string{
		"/debug/config",
		"/debug/pprof/goroutine",
		"/api/debug/engines",
		"/api/v1/debug/engines",
	}
	credentials := []struct {
		name   string
		value  string
		header string
		query  string
	}{
		{name: "Authorization Bearer", value: "Bearer operator-secret", header: "Authorization"},
		{name: "Authorization Basic", value: "Basic " + base64.StdEncoding.EncodeToString([]byte("operator:operator-secret")), header: "Authorization"},
		{name: "Authorization Digest", value: "Digest operator-secret", header: "Authorization"},
		{name: "X-API-Key", value: "operator-secret", header: "X-API-Key"},
		{name: "X-Api-Key", value: "operator-secret", header: "X-Api-Key"},
		{name: "API-Key", value: "operator-secret", header: "API-Key"},
		{name: "ApiKey", value: "operator-secret", header: "ApiKey"},
		{name: "X-Auth-Token", value: "operator-secret", header: "X-Auth-Token"},
		{name: "X-Access-Token", value: "operator-secret", header: "X-Access-Token"},
		{name: "X-Token", value: "operator-secret", header: "X-Token"},
		{name: "Token", value: "operator-secret", header: "Token"},
		{name: "X-CSRF-Token", value: "operator-secret", header: "X-CSRF-Token"},
		{name: "X-XSRF-Token", value: "operator-secret", header: "X-XSRF-Token"},
		{name: "X-Session-ID", value: "operator-secret", header: "X-Session-ID"},
		{name: "X-Service-Token", value: "operator-secret", header: "X-Service-Token"},
		{name: "X-Internal-Token", value: "operator-secret", header: "X-Internal-Token"},
		{name: "query token", query: "?token=operator-secret"},
	}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			for _, authorization := range []string{"", "Bearer wrong-secret", "Basic operator-secret"} {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.Header.Set("Accept", "application/json")
				if authorization != "" {
					req.Header.Set("Authorization", authorization)
				}
				rec := httptest.NewRecorder()
				s.router.ServeHTTP(rec, req)
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("Authorization %q: status = %d, want 401", authorization, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), `"error": "UNAUTHORIZED"`) {
					t.Errorf("Authorization %q: error body = %q, want generic UNAUTHORIZED", authorization, rec.Body.String())
				}
			}

			for _, credential := range credentials {
				req := httptest.NewRequest(http.MethodGet, path+credential.query, nil)
				if credential.header != "" {
					req.Header.Set(credential.header, credential.value)
				}
				rec := httptest.NewRecorder()
				s.router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Errorf("valid %s: status = %d, want 200", credential.name, rec.Code)
				}
			}
		})
	}

	t.Run("credential priority is first found wins", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/debug/engines?token=operator-secret", nil)
		req.Header.Set("Authorization", "Bearer wrong-secret")
		req.Header.Set("X-API-Key", "operator-secret")
		rec := httptest.NewRecorder()
		s.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("invalid higher-priority credential status = %d, want 401", rec.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/debug/engines?token=wrong-secret", nil)
		req.Header.Set("Authorization", "Bearer operator-secret")
		req.Header.Set("X-API-Key", "wrong-secret")
		rec = httptest.NewRecorder()
		s.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("valid higher-priority credential status = %d, want 200", rec.Code)
		}
	})

	t.Run("invalid token sources are rejected", func(t *testing.T) {
		for _, credential := range []struct {
			name   string
			header string
			value  string
			query  string
		}{
			{name: "API key", header: "X-API-Key", value: "wrong-secret"},
			{name: "service token", header: "X-Service-Token", value: "wrong-secret"},
			{name: "digest credential", header: "Authorization", value: "Digest wrong-secret"},
			{name: "query token", query: "?token=wrong-secret"},
		} {
			req := httptest.NewRequest(http.MethodGet, "/api/debug/engines"+credential.query, nil)
			if credential.header != "" {
				req.Header.Set(credential.header, credential.value)
			}
			rec := httptest.NewRecorder()
			s.router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("invalid %s: status = %d, want 401", credential.name, rec.Code)
			}
		}
	})
}

func TestDebugRoutesFailClosedWithoutConfiguredOperatorToken(t *testing.T) {
	wasDebugEnabled := mode.IsDebugEnabled()
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(wasDebugEnabled) })

	cfg := config.DefaultAppConfig()
	cfg.Server.Token = ""
	s := &Server{appConfig: cfg, router: chi.NewRouter()}
	s.registerDebugRoutes(s.router)

	req := httptest.NewRequest(http.MethodGet, "/debug/config", nil)
	req.Header.Set("X-API-Key", "any-token")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unconfigured operator token: status = %d, want 401", rec.Code)
	}
}

func TestAuthorizationTokenRejectsMalformedAuthorization(t *testing.T) {
	for _, value := range []string{"", "Bearer", "Bearer ", "Bearer  token", "Bearer token extra", "Basic token"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", value)
		if token, ok := extractRequestToken(req); ok {
			t.Errorf("extractRequestToken(%q) = %q, true; want rejection", value, token)
		}
	}
	if token, ok := bearerToken("bEaReR token"); !ok || token != "token" {
		t.Errorf("bearerToken case-insensitive scheme = (%q, %t), want (token, true)", token, ok)
	}
}
