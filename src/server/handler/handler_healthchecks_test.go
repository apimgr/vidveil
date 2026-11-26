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
	"time"

	"github.com/apimgr/vidveil/src/server/service/health"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
)

// fakeScheduler is a minimal SchedulerHealth for exercising the healthz
// scheduler check without spinning up the real scheduler.
type fakeScheduler struct{ running bool }

func (f fakeScheduler) IsRunning() bool { return f.running }

type fakeI2PChecker struct {
	enabled bool
	address string
	status  string
	state   string
	err     error
}

func (f fakeI2PChecker) Enabled() bool   { return f.enabled }
func (f fakeI2PChecker) IsRunning() bool { return f.enabled && f.err == nil }
func (f fakeI2PChecker) State() string {
	if f.state != "" {
		return f.state
	}
	if !f.enabled {
		return "disabled"
	}
	if f.err != nil {
		return "error"
	}
	return "running"
}
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

func TestHealthEndpoints_ReportMaintenanceStatus(t *testing.T) {
	for _, route := range []struct {
		name    string
		handler func(*SearchHandler, http.ResponseWriter, *http.Request)
		path    string
	}{{"frontend", (*SearchHandler).HealthCheck, "/server/healthz"}, {"api", (*SearchHandler).APIHealthCheck, "/api/v1/server/healthz"}} {
		t.Run(route.name, func(t *testing.T) {
			h := NewSearchHandler(nil, nil)
			now := time.Now().UTC()
			next := now.Add(maintenance.MaintenanceRetryInterval)
			h.SetMaintenanceStatus(&maintenance.MaintenanceStatus{
				Reason: "database_connection", Message: "Database connection recovery is in progress.", Diagnostic: "connection refused at private-db:5432", Since: now,
				SelfHealing: maintenance.SelfHealingStatus{Enabled: true, Attempts: 2, LastAttempt: &now, NextAttempt: &next},
			})
			req := httptest.NewRequest(http.MethodGet, route.path, nil)
			req.Header.Set("Accept", "application/json")
			recorder := httptest.NewRecorder()
			route.handler(h, recorder, req)
			var response HealthResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if recorder.Code != http.StatusServiceUnavailable || response.Status != "maintenance" {
				t.Fatalf("status = %d/%q, want 503/maintenance", recorder.Code, response.Status)
			}
			if response.Maintenance == nil || response.Maintenance.Reason != "database_connection" || !response.Maintenance.SelfHealing.Enabled || response.Maintenance.SelfHealing.Attempts != 2 {
				t.Errorf("maintenance status = %#v", response.Maintenance)
			}
			if strings.Contains(recorder.Body.String(), "private-db") {
				t.Error("public health response leaked the operator diagnostic")
			}
		})
	}
}

func TestOperatorStatusIncludesMaintenanceDiagnosticAndFixInstructions(t *testing.T) {
	h := NewSearchHandler(nil, nil)
	h.SetMaintenanceStatus(&maintenance.MaintenanceStatus{
		Reason: "database_connection", Diagnostic: "connection refused at private-db:5432", Since: time.Now().UTC(),
		SelfHealing: maintenance.SelfHealingStatus{Enabled: true, Attempts: 1},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Accept", "application/json")
	recorder := httptest.NewRecorder()
	h.OperatorStatus(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("operator status code = %d, want 200", recorder.Code)
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Status          maintenance.MaintenanceStatus `json:"status"`
			FixInstructions []string                      `json:"fix_instructions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode operator status: %v", err)
	}
	if !response.OK || response.Data.Status.Diagnostic != "connection refused at private-db:5432" || len(response.Data.FixInstructions) == 0 {
		t.Fatalf("operator diagnostics missing: %#v", response)
	}
}

func TestHealthCheck_EnabledI2PFailureIsDegraded(t *testing.T) {
	h := NewSearchHandler(nil, nil)
	h.SetI2PService(fakeI2PChecker{enabled: true, err: errors.New("probe failed")})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("HealthCheck(I2P failure): status = %d, want 200", rr.Code)
	}
	var response struct {
		Status   string       `json:"status"`
		Features FeaturesInfo `json:"features"`
		Checks   ChecksInfo   `json:"checks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("HealthCheck(I2P failure): invalid JSON: %v", err)
	}
	if response.Checks.I2P != "degraded" {
		t.Errorf("HealthCheck(I2P failure): checks.i2p = %q, want degraded", response.Checks.I2P)
	}
	if response.Status != "degraded" {
		t.Errorf("HealthCheck(I2P failure): status = %q, want degraded", response.Status)
	}
	if response.Features.I2P.Status != "unavailable" {
		t.Errorf("HealthCheck(I2P failure): features.i2p.status = %q, want unavailable", response.Features.I2P.Status)
	}
	if response.Features.I2P.State != "error" {
		t.Errorf("HealthCheck(I2P failure): features.i2p.state = %q, want error", response.Features.I2P.State)
	}
}

func TestHealthCheck_StartingI2PIsHealthy(t *testing.T) {
	h := NewSearchHandler(nil, nil)
	h.SetI2PService(fakeI2PChecker{enabled: true, state: "starting", status: "starting", err: errors.New("not ready")})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	h.HealthCheck(rr, req)
	var response struct {
		Status   string       `json:"status"`
		Features FeaturesInfo `json:"features"`
		Checks   ChecksInfo   `json:"checks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("HealthCheck(I2P starting): invalid JSON: %v", err)
	}
	if rr.Code != http.StatusOK || response.Status != "healthy" || response.Features.I2P.Status != "starting" || response.Checks.I2P != "ok" {
		t.Errorf("HealthCheck(I2P starting): code=%d status=%q feature=%q check=%q", rr.Code, response.Status, response.Features.I2P.Status, response.Checks.I2P)
	}
}

func TestHealthRoutes_DatasetChecksAreCachedOrderedAndAggregated(t *testing.T) {
	for _, test := range []struct {
		name         string
		critical     bool
		wantStatus   string
		wantHTTPCode int
	}{
		{name: "optional error degrades", wantStatus: "degraded", wantHTTPCode: http.StatusOK},
		{name: "critical error is unhealthy", critical: true, wantStatus: "unhealthy", wantHTTPCode: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := health.NewRegistry()
			if err := registry.Register("geoip_country", test.critical, func() string { return health.StatusError }); err != nil {
				t.Fatal(err)
			}
			registry.Refresh()
			for _, route := range []struct {
				name   string
				handle func(*SearchHandler, http.ResponseWriter, *http.Request)
			}{{"frontend", (*SearchHandler).HealthCheck}, {"api", (*SearchHandler).APIHealthCheck}} {
				t.Run(route.name, func(t *testing.T) {
					h := NewSearchHandler(nil, nil)
					h.SetHealthRegistry(registry)
					req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
					req.Header.Set("Accept", "application/json")
					rr := httptest.NewRecorder()
					route.handle(h, rr, req)
					var response struct {
						Status string            `json:"status"`
						Checks map[string]string `json:"checks"`
					}
					if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
						t.Fatalf("invalid health JSON: %v", err)
					}
					if rr.Code != test.wantHTTPCode || response.Status != test.wantStatus || response.Checks["geoip_country"] != health.StatusError {
						t.Errorf("health = code:%d status:%q checks:%#v", rr.Code, response.Status, response.Checks)
					}
					body := rr.Body.String()
					checksIndex := strings.Index(body, `"checks":`)
					coreIndex := strings.Index(body, `"database": "ok"`)
					datasetIndex := strings.Index(body, `"geoip_country": "error"`)
					if checksIndex < 0 || coreIndex < checksIndex || datasetIndex < coreIndex {
						t.Errorf("health check order is wrong: %s", body)
					}
				})
			}
		})
	}
}

func TestHealthRoutes_TorLifecycleContract(t *testing.T) {
	tests := []struct {
		name     string
		running  bool
		starting bool
		enabled  bool
		status   string
		state    string
		check    string
	}{
		{name: "disabled", status: "disabled", state: "disabled"},
		{name: "starting", enabled: true, starting: true, status: "starting", state: "starting", check: "ok"},
		{name: "healthy", enabled: true, running: true, status: "healthy", state: "running", check: "ok"},
		{name: "unavailable", enabled: true, status: "unavailable", state: "error", check: "degraded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, route := range []struct {
				name   string
				handle func(*SearchHandler, http.ResponseWriter, *http.Request)
			}{{"frontend", (*SearchHandler).HealthCheck}, {"api", (*SearchHandler).APIHealthCheck}} {
				t.Run(route.name, func(t *testing.T) {
					h := NewSearchHandler(nil, nil)
					h.SetTorService(&testTorChecker{enabled: test.enabled, running: test.running, starting: test.starting})
					req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
					req.Header.Set("Accept", "application/json")
					rr := httptest.NewRecorder()
					route.handle(h, rr, req)
					var response struct {
						Status   string `json:"status"`
						Features struct {
							Tor TorInfo `json:"tor"`
						} `json:"features"`
						Checks struct {
							Tor string `json:"tor"`
						} `json:"checks"`
					}
					if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
						t.Fatalf("invalid health JSON: %v", err)
					}
					wantStatus := "healthy"
					wantCode := http.StatusOK
					if test.status == "unavailable" {
						wantStatus = "degraded"
					}
					if rr.Code != wantCode || response.Status != wantStatus || response.Features.Tor.Enabled != test.enabled || response.Features.Tor.Running != test.running || response.Features.Tor.Status != test.status || response.Features.Tor.State != test.state || response.Checks.Tor != test.check {
						t.Errorf("health contract = code:%d status:%s enabled:%t running:%t feature:%s state:%s check:%s", rr.Code, response.Status, response.Features.Tor.Enabled, response.Features.Tor.Running, response.Features.Tor.Status, response.Features.Tor.State, response.Checks.Tor)
					}
				})
			}
		})
	}
}

func TestHealthRoutes_I2PLifecycleContract(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		state   string
		status  string
		err     error
		check   string
	}{
		{name: "disabled", state: "disabled", status: "disabled"},
		{name: "starting", enabled: true, state: "starting", status: "starting", err: errors.New("not ready"), check: "ok"},
		{name: "healthy", enabled: true, state: "running", status: "healthy", check: "ok"},
		{name: "unavailable", enabled: true, state: "error", status: "unavailable", err: errors.New("provider failed"), check: "degraded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, route := range []struct {
				name   string
				handle func(*SearchHandler, http.ResponseWriter, *http.Request)
			}{{"frontend", (*SearchHandler).HealthCheck}, {"api", (*SearchHandler).APIHealthCheck}} {
				t.Run(route.name, func(t *testing.T) {
					h := NewSearchHandler(nil, nil)
					h.SetI2PService(fakeI2PChecker{enabled: test.enabled, state: test.state, status: test.status, err: test.err})
					req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
					req.Header.Set("Accept", "application/json")
					rr := httptest.NewRecorder()
					route.handle(h, rr, req)
					var response struct {
						Status   string `json:"status"`
						Features struct {
							I2P I2PInfo `json:"i2p"`
						} `json:"features"`
						Checks struct {
							I2P string `json:"i2p"`
						} `json:"checks"`
					}
					if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
						t.Fatalf("invalid health JSON: %v", err)
					}
					wantStatus := "healthy"
					if test.status == "unavailable" {
						wantStatus = "degraded"
					}
					if rr.Code != http.StatusOK || response.Status != wantStatus || response.Features.I2P.Enabled != test.enabled || response.Features.I2P.Running != (test.state == "running") || response.Features.I2P.Status != test.status || response.Features.I2P.State != test.state || response.Checks.I2P != test.check {
						t.Errorf("health contract = code:%d status:%s enabled:%t running:%t feature:%s state:%s check:%s", rr.Code, response.Status, response.Features.I2P.Enabled, response.Features.I2P.Running, response.Features.I2P.Status, response.Features.I2P.State, response.Checks.I2P)
					}
				})
			}
		})
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
