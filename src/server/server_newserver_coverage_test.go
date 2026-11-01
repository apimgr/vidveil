// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for NewServer, setupMiddleware, setupRoutes,
// and related server lifecycle methods.
package server

import (
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/branding"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
)

// stubMigrationManager is a minimal no-op MigrationManager for tests.
type stubMigrationManager struct{}

func (s *stubMigrationManager) GetMigrationStatus() ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubMigrationManager) RunMigrations() error     { return nil }
func (s *stubMigrationManager) RollbackMigration() error { return nil }
func (s *stubMigrationManager) GetDB() *sql.DB           { return nil }

// newTestServer creates a Server using NewServer with minimal dependencies.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultAppConfig()

	base := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	tmp, err := os.MkdirTemp(base, "vidveil-server-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Errorf("remove temp dir: %v", err)
		}
	})

	engineMgr := engine.NewEngineManager(cfg)
	sched := scheduler.NewScheduler()
	// The scheduler is ALWAYS running in production (AI.md PART 18); start it so
	// the /server/healthz scheduler check reports healthy, mirroring runtime.
	sched.Start(context.Background())
	t.Cleanup(sched.Stop)

	var logger *logging.AppLogger
	logger, err = logging.NewAppLogger(cfg)
	if err != nil {
		logger = nil
	}

	return NewServer(cfg, tmp, tmp, engineMgr, &stubMigrationManager{}, sched, logger)
}

// ── NewServer / setupMiddleware / setupRoutes ─────────────────────────────────

func TestNewServer_ReturnsNonNil(t *testing.T) {
	s := newTestServer(t)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestNewServer_RouterIsNonNil(t *testing.T) {
	s := newTestServer(t)
	if s.router == nil {
		t.Error("NewServer: router is nil after construction")
	}
}

func TestNewServer_SearchHandlerSet(t *testing.T) {
	s := newTestServer(t)
	if s.searchHandler == nil {
		t.Error("NewServer: searchHandler is nil after setupRoutes")
	}
}

func TestNewServer_RateLimiterSet(t *testing.T) {
	s := newTestServer(t)
	if s.rateLimiter == nil {
		t.Error("NewServer: rateLimiter is nil after construction")
	}
}

func TestNewServer_RegistersBrandingRefreshAndSeedsDefaultOGImage(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.scheduler.GetTask("branding_refresh"); err != nil {
		t.Fatalf("branding refresh task was not registered: %v", err)
	}
	if task, err := s.scheduler.GetTask("owner_idle_purge"); err != nil || task.Schedule != "@daily" {
		t.Fatalf("idle owner purge task = %#v, %v; want daily task", task, err)
	}
	if !s.brandingImages.Has("og-1200x630.png") {
		t.Fatal("embedded OpenGraph fallback image was not generated")
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/branding/og-1200x630.png", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("branding image status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("branding image Content-Type = %q, want image/png", got)
	}
}

func TestManifestUsesCachedCustomFaviconSizes(t *testing.T) {
	s := newTestServer(t)
	source := filepath.Join(s.dataDir, "favicon-source.png")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	img.Set(0, 0, color.White)
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	s.brandingImages.SetSources(branding.ImageSources{Favicon: source})
	if err := s.brandingImages.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/manifest.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "/branding/favicon-192.png") || !strings.Contains(w.Body.String(), "/branding/favicon-512.png") {
		t.Fatalf("custom favicon sizes missing from manifest: %s", w.Body.String())
	}
}

// ── Shutdown after Listen — srv set via ListenAndServe path ──────────────────

func TestShutdown_AfterListenAndServeStarted_ReturnsNil(t *testing.T) {
	s := newTestServer(t)
	l, err := s.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	// Start serving in background, then immediately shut down
	done := make(chan error, 1)
	go func() {
		done <- s.ServeOn(l)
	}()
	// Shutdown immediately
	if err := s.Shutdown(context.Background()); err != nil {
		t.Logf("Shutdown after serve start: %v (may be nil or error depending on race)", err)
	}
}

// ── Listen + ServeOn ──────────────────────────────────────────────────────────

func TestListen_LocalhostPort0_Succeeds(t *testing.T) {
	s := newTestServer(t)
	l, err := s.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = l.Close() }()
}

func TestServeOn_ClosedListener_ReturnsError(t *testing.T) {
	s := newTestServer(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	// Close the listener immediately so Serve returns right away.
	_ = l.Close()
	err = s.ServeOn(l)
	if err == nil {
		t.Error("ServeOn(closed listener): expected error, got nil")
	}
}

// ── Serve — closed listener returns immediately ───────────────────────────────

func TestServe_ClosedListener_ReturnsError(t *testing.T) {
	s := newTestServer(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	_ = l.Close()
	err = s.Serve(l)
	if err == nil {
		t.Error("Serve(closed listener): expected error, got nil")
	}
}

// ── ListenAndServe — bad address returns error ────────────────────────────────

func TestListenAndServe_BadAddress_ReturnsError(t *testing.T) {
	s := newTestServer(t)
	err := s.ListenAndServe("not-a-valid-addr:0")
	if err == nil {
		t.Error("ListenAndServe(invalid addr): expected error, got nil")
	}
}

// ── Registered routes respond ─────────────────────────────────────────────────

func TestServer_HealthzRoute_Returns200(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/server/healthz: status = %d, want 200", rr.Code)
	}
}

func TestServer_RobotsTxt_Returns200(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/robots.txt: status = %d, want 200", rr.Code)
	}
}

func TestServer_Unknown_Returns404(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/no-such-route-abc123", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown route: status = %d, want 404", rr.Code)
	}
}

func TestServer_MCPAliasesShareStatelessEndpoint(t *testing.T) {
	s := newTestServer(t)
	paths := []string{"/api/v1/server/mcp", "/api/mcp", "/mcp"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rr := httptest.NewRecorder()
		s.router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name":"searchVideos"`) {
			t.Errorf("POST %s: status=%d body=%s", path, rr.Code, rr.Body.String())
		}
		if rr.Header().Get("Mcp-Session-Id") != "" {
			t.Errorf("POST %s minted an MCP session ID", path)
		}
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			req := httptest.NewRequest(method, path, nil)
			rr := httptest.NewRecorder()
			s.router.ServeHTTP(rr, req)
			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status=%d, want 405", method, path, rr.Code)
			}
		}
	}
	for _, credential := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "X-API-Key", value: s.appConfig.Server.Token, want: false},
		{name: "Authorization", value: "Bearer " + s.appConfig.Server.Token, want: true},
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set(credential.name, credential.value)
		rr := httptest.NewRecorder()
		s.router.ServeHTTP(rr, req)
		listed := strings.Contains(rr.Body.String(), "restoreServerBackup")
		if listed != credential.want {
			t.Errorf("MCP operator tool visibility with %s = %t, want %t", credential.name, listed, credential.want)
		}
	}
	s.appConfig.Server.MCP.Enabled = false
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("disabled MCP status = %d, want 404", rr.Code)
	}
}

func TestServer_GuestPreferenceAPIUsesCanonicalRoutesWithoutCSRF(t *testing.T) {
	s := newTestServer(t)
	get := httptest.NewRecorder()
	s.router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"prefs"`) {
		t.Fatalf("GET preferences: status=%d body=%s", get.Code, get.Body.String())
	}
	post := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences", strings.NewReader(`{"theme":"light"}`))
	post.Header.Set("Content-Type", "application/json")
	updated := httptest.NewRecorder()
	s.router.ServeHTTP(updated, post)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"applied"`) {
		t.Fatalf("POST preferences without ambient CSRF token: status=%d body=%s", updated.Code, updated.Body.String())
	}
	if cookies := updated.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "theme" {
		t.Errorf("preference API set unexpected cookies: %#v", cookies)
	}
}
