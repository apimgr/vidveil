// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for server debug handlers and setter methods.
// Tests handleDebugConfig, handleDebugRoutes, handleDebugCache, handleDebugMemory,
// handleDebugGoroutines, registerDebugRoutes (early-return path), debugLog,
// debugLogDB, debugLogCache, SetTorService, SetGeoIPService, SetBlocklistService.
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/geoip"
)

// ── mock types for server setters ─────────────────────────────────────────────

type mockSrvTorChecker struct{}

func (m *mockSrvTorChecker) IsEnabled() bool                          { return false }
func (m *mockSrvTorChecker) IsRunning() bool                          { return false }
func (m *mockSrvTorChecker) IsStarting() bool                         { return false }
func (m *mockSrvTorChecker) AllowUserIPForward() bool                  { return false }
func (m *mockSrvTorChecker) UseNetworkEnabled() bool                   { return false }
func (m *mockSrvTorChecker) OutboundEnabled() bool                     { return false }
func (m *mockSrvTorChecker) GetInfo() map[string]interface{}           { return nil }
func (m *mockSrvTorChecker) GetHTTPClient(_ bool) *http.Client        { return &http.Client{} }

type mockSrvGeoIPChecker struct{}

func (m *mockSrvGeoIPChecker) IsEnabled() bool                { return false }
func (m *mockSrvGeoIPChecker) GetRestrictionMode() string      { return "off" }
func (m *mockSrvGeoIPChecker) CheckContentRestriction(_ string, _ bool) *geoip.RestrictionResult {
	return nil
}

// mockSrvGeoIPBlocker implements both handler.GeoIPChecker and GeoIPBlocker.
type mockSrvGeoIPBlocker struct{}

func (m *mockSrvGeoIPBlocker) IsEnabled() bool                { return false }
func (m *mockSrvGeoIPBlocker) GetRestrictionMode() string      { return "off" }
func (m *mockSrvGeoIPBlocker) CheckContentRestriction(_ string, _ bool) *geoip.RestrictionResult {
	return nil
}
func (m *mockSrvGeoIPBlocker) IsBlocked(_ string) bool        { return false }

type mockSrvBlocklist struct{}

func (m *mockSrvBlocklist) IsBlocked(_ string) bool { return false }

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
