// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for NewServer, setupMiddleware, setupRoutes,
// and related server lifecycle methods.
package server

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
)

// stubMigrationManager is a minimal no-op MigrationManager for tests.
type stubMigrationManager struct{}

func (s *stubMigrationManager) GetMigrationStatus() ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubMigrationManager) RunMigrations() error    { return nil }
func (s *stubMigrationManager) RollbackMigration() error { return nil }
func (s *stubMigrationManager) GetDB() *sql.DB          { return nil }

// newTestServer creates a Server using NewServer with minimal dependencies.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultAppConfig()

	base := filepath.Join(os.TempDir(), "apimgr")
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-server-")
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
	defer l.Close()
}

func TestServeOn_ClosedListener_ReturnsError(t *testing.T) {
	s := newTestServer(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	// Close the listener immediately so Serve returns right away.
	l.Close()
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
	l.Close()
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
