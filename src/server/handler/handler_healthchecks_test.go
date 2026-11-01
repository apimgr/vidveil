// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeScheduler is a minimal SchedulerHealth for exercising the healthz
// scheduler check without spinning up the real scheduler.
type fakeScheduler struct{ running bool }

func (f fakeScheduler) IsRunning() bool { return f.running }

type fakeI2PChecker struct {
	enabled bool
	address string
	status  string
	err     error
}

func (f fakeI2PChecker) Enabled() bool                     { return f.enabled }
func (f fakeI2PChecker) IsRunning() bool                   { return f.enabled && f.err == nil }
func (f fakeI2PChecker) Status() string                    { return f.status }
func (f fakeI2PChecker) Address() string                   { return f.address }
func (f fakeI2PChecker) Provider() string                  { return "sam" }
func (f fakeI2PChecker) CheckHealth(context.Context) error { return f.err }

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

func TestHealthCheck_EnabledI2PFailureIsUnhealthy(t *testing.T) {
	h := NewSearchHandler(nil, nil)
	h.SetI2PService(fakeI2PChecker{enabled: true, err: errors.New("probe failed")})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("HealthCheck(I2P failure): status = %d, want 503", rr.Code)
	}
	var response struct {
		Features FeaturesInfo `json:"features"`
		Checks   ChecksInfo   `json:"checks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("HealthCheck(I2P failure): invalid JSON: %v", err)
	}
	if response.Checks.I2P != "error" {
		t.Errorf("HealthCheck(I2P failure): checks.i2p = %q, want error", response.Checks.I2P)
	}
	if response.Features.I2P.Status != "error:provider_unhealthy" {
		t.Errorf("HealthCheck(I2P failure): features.i2p.status = %q, want error:provider_unhealthy", response.Features.I2P.Status)
	}
}

func TestHealthCheckHTML_ShowsEnabledI2PAddressAndCopyControl(t *testing.T) {
	setRealTemplatesFS(t)
	h := NewSearchHandler(nil, nil)
	h.SetI2PService(fakeI2PChecker{enabled: true, status: "healthy", address: "example.b32.i2p"})
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "example.b32.i2p") || !strings.Contains(body, `data-copy="example.b32.i2p"`) {
		t.Fatalf("HealthCheck HTML I2P feature = %d %s", rr.Code, body)
	}
}
