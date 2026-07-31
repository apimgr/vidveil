// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeScheduler is a minimal SchedulerHealth for exercising the healthz
// scheduler check without spinning up the real scheduler.
type fakeScheduler struct{ running bool }

func (f fakeScheduler) IsRunning() bool { return f.running }

// checkDatabase reports "ok" when no DB is wired (nothing to probe).
func TestCheckDatabase_NilHandle_OK(t *testing.T) {
	h := &SearchHandler{}
	if got := h.checkDatabase(context.Background()); got != "ok" {
		t.Errorf("checkDatabase(nil) = %q, want ok", got)
	}
}

// checkCache always reports "ok": the search cache is process-local with no
// external backend, so it cannot report unhealthy.
func TestCheckCache_AlwaysOK(t *testing.T) {
	h := &SearchHandler{}
	if got := h.checkCache(); got != "ok" {
		t.Errorf("checkCache(nil) = %q, want ok", got)
	}
	h2 := NewSearchHandler(nil, nil)
	if got := h2.checkCache(); got != "ok" {
		t.Errorf("checkCache(initialized) = %q, want ok", got)
	}
}

// checkScheduler mirrors the scheduler running state; nil is treated as ok.
func TestCheckScheduler_ReflectsRunningState(t *testing.T) {
	h := &SearchHandler{}
	if got := h.checkScheduler(); got != "ok" {
		t.Errorf("checkScheduler(nil) = %q, want ok", got)
	}
	h.sched = fakeScheduler{running: true}
	if got := h.checkScheduler(); got != "ok" {
		t.Errorf("checkScheduler(running) = %q, want ok", got)
	}
	h.sched = fakeScheduler{running: false}
	if got := h.checkScheduler(); got != "error" {
		t.Errorf("checkScheduler(stopped) = %q, want error", got)
	}
}

// checkDisk reports "ok" for a normal data directory.
func TestCheckDisk_TempDir_OK(t *testing.T) {
	h := &SearchHandler{dataDir: t.TempDir()}
	if got := h.checkDisk(); got != "ok" {
		t.Errorf("checkDisk(tempdir) = %q, want ok", got)
	}
}

// A stopped scheduler must drive the overall healthz status to unhealthy (503),
// proving the checks are no longer hardcoded to "ok".
func TestHealthCheck_StoppedScheduler_Unhealthy(t *testing.T) {
	h := NewSearchHandler(nil, nil)
	h.SetScheduler(fakeScheduler{running: false})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("HealthCheck(stopped scheduler): status = %d, want 503", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("HealthCheck: invalid JSON: %v", err)
	}
	if resp["status"] != "unhealthy" {
		t.Errorf("HealthCheck: status = %v, want unhealthy", resp["status"])
	}
	checks, _ := resp["checks"].(map[string]interface{})
	if checks["scheduler"] != "error" {
		t.Errorf("HealthCheck: checks.scheduler = %v, want error", checks["scheduler"])
	}
	if !strings.Contains(rr.Body.String(), `"scheduler"`) {
		t.Errorf("HealthCheck: missing scheduler check in body")
	}
}
