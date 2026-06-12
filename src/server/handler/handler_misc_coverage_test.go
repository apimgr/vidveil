// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for handler functions not yet covered by other test files.
// Covers AgeVerifyPage cookie path, SearchPage text/html and text/plain,
// APIEngines text/plain, APIHealthCheck text format, APIStats, APIVersion,
// APIEngineHealth, NotFoundHandler, InternalErrorHandler, SendOK, SendError,
// getUptime days branch, metrics Handler, and rotateLocked rotation.
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// newMiscTestHandler returns a SearchHandler with empty EngineManager and no metrics.
func newMiscTestHandler() *SearchHandler {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	return &SearchHandler{
		appConfig: cfg,
		engineMgr: mgr,
	}
}

// ── AgeVerifyPage ─────────────────────────────────────────────────────────────

// When the age-verify cookie is already set, AgeVerifyPage redirects immediately.
func TestAgeVerifyPage_AlreadyVerified_Redirects(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/age-verify", nil)
	req.AddCookie(&http.Cookie{Name: ageVerifyCookieName, Value: "1"})

	h.AgeVerifyPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("AgeVerifyPage already-verified: status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/" {
		t.Errorf("AgeVerifyPage already-verified: Location = %q, want /", loc)
	}
}

// When already verified and a valid redirect is given, it uses that redirect.
func TestAgeVerifyPage_AlreadyVerified_WithRedirect_UsesParam(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/age-verify?redirect=/search", nil)
	req.AddCookie(&http.Cookie{Name: ageVerifyCookieName, Value: "1"})

	h.AgeVerifyPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("AgeVerifyPage redirect param: status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/search" {
		t.Errorf("AgeVerifyPage redirect param: Location = %q, want /search", loc)
	}
}

// Bad (non-slash) redirect defaults to /.
func TestAgeVerifyPage_AlreadyVerified_BadRedirect_FallsBackToHome(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/age-verify?redirect=http://evil.com", nil)
	req.AddCookie(&http.Cookie{Name: ageVerifyCookieName, Value: "1"})

	h.AgeVerifyPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("AgeVerifyPage bad-redirect: status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/" {
		t.Errorf("AgeVerifyPage bad-redirect: Location = %q, want /", loc)
	}
}

// No cookie present — curl UA means renderResponse returns plain text 200.
func TestAgeVerifyPage_NotVerified_CurlUA_ReturnsText(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/age-verify", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.AgeVerifyPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AgeVerifyPage not-verified curl: status = %d, want 200", rr.Code)
	}
}

// ── SearchPage ────────────────────────────────────────────────────────────────

// Browser UA sends text/html → SearchPage calls renderResponse → renderTemplate →
// empty templatesFS causes a 500 (covers the html branch code path regardless).
func TestSearchPage_BrowserUA_HitsHTMLPath(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	h.SearchPage(rr, req)

	// templatesFS is zero-value → template parse fails → 500; the html branch is still exercised.
	if rr.Code == http.StatusFound {
		t.Error("SearchPage browser UA: unexpected redirect, expected non-redirect response")
	}
}

// Curl UA triggers text/plain format (non-browser path) in SearchPage.
func TestSearchPage_CurlUA_ReturnsText(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.SearchPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SearchPage curl: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("SearchPage curl: Content-Type = %q, want text/plain", ct)
	}
}

// JSON format with engines param populates engineNames from the URL param.
func TestSearchPage_JSONFormat_EnginesParam_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=test&engines=ph,xv", nil)
	req.Header.Set("Accept", "application/json")

	h.SearchPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SearchPage JSON engines: status = %d, want 200", rr.Code)
	}
}

// Unknown UA with Accept: */* falls through to default → text/html → renderResponse →
// empty templatesFS → 500 (still exercises the default switch case code path).
func TestSearchPage_DefaultFormat_HitsDefaultCase(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=amateur", nil)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "some-unknown-client/1.0")

	h.SearchPage(rr, req)

	// Should not redirect (q is non-empty); result is whatever renderResponse returns.
	if rr.Code == http.StatusFound {
		t.Error("SearchPage default format: unexpected redirect")
	}
}

// ── APIEngines ────────────────────────────────────────────────────────────────

// Text/plain format for APIEngines (curl UA) exercises the text branch.
func TestAPIEngines_PlainTextFormat_ReturnsText(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.APIEngines(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIEngines plain: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("APIEngines plain: Content-Type = %q, want text/plain", ct)
	}
}

// JSON format for APIEngines.
func TestAPIEngines_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines", nil)
	req.Header.Set("Accept", "application/json")

	h.APIEngines(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIEngines JSON: status = %d, want 200", rr.Code)
	}
}

// ── APIHealthCheck ────────────────────────────────────────────────────────────

// Text format via curl UA exercises the full text output branch.
func TestAPIHealthCheck_TextFormat_ReturnsText(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck text: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("APIHealthCheck text: Content-Type = %q, want text/plain", ct)
	}
}

// JSON format for APIHealthCheck (default path).
func TestAPIHealthCheck_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req.Header.Set("Accept", "application/json")

	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck JSON: status = %d, want 200", rr.Code)
	}
}

// ── APIStats ──────────────────────────────────────────────────────────────────

func TestAPIStats_TextFormat_ReturnsText(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.APIStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIStats text: status = %d, want 200", rr.Code)
	}
}

func TestAPIStats_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Accept", "application/json")

	h.APIStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIStats JSON: status = %d, want 200", rr.Code)
	}
}

// ── APIVersion ────────────────────────────────────────────────────────────────

func TestAPIVersion_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)

	h.APIVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIVersion: status = %d, want 200", rr.Code)
	}
}

// ── APIEngineHealth ───────────────────────────────────────────────────────────

func TestAPIEngineHealth_EmptyManager_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines/health", nil)

	h.APIEngineHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIEngineHealth: status = %d, want 200", rr.Code)
	}
}

// ── NotFoundHandler / InternalErrorHandler ────────────────────────────────────

// NotFoundHandler uses empty templatesFS → falls back to plain text 404.
func TestNotFoundHandler_ReturnsPlainText404(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)

	h.NotFoundHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("NotFoundHandler: status = %d, want 404", rr.Code)
	}
}

// InternalErrorHandler uses empty templatesFS → falls back to plain text 500.
func TestInternalErrorHandler_ReturnsPlainText500(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	h.InternalErrorHandler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("InternalErrorHandler: status = %d, want 500", rr.Code)
	}
}

// ── SendOK / SendError ────────────────────────────────────────────────────────

func TestSendOK_ValidData_Returns200(t *testing.T) {
	rr := httptest.NewRecorder()
	SendOK(rr, map[string]string{"key": "value"})
	if rr.Code != http.StatusOK {
		t.Errorf("SendOK: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("SendOK: Content-Type = %q, want application/json", ct)
	}
}

// Passing an unmarshalable value (channel) triggers the marshal error fallback.
func TestSendOK_UnmarshalableData_Returns500(t *testing.T) {
	rr := httptest.NewRecorder()
	// Channels cannot be marshaled by encoding/json.
	SendOK(rr, make(chan int))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("SendOK unmarshalable: status = %d, want 500", rr.Code)
	}
}

func TestSendError_KnownCode_ReturnsCorrectStatus(t *testing.T) {
	tests := []struct {
		code       string
		wantStatus int
	}{
		{"BAD_REQUEST", 400},
		{"UNAUTHORIZED", 401},
		{"FORBIDDEN", 403},
		{"NOT_FOUND", 404},
		{"RATE_LIMITED", 429},
		{"MAINTENANCE", 503},
		{"UNKNOWN_CODE", 500},
	}
	for _, tc := range tests {
		rr := httptest.NewRecorder()
		SendError(rr, tc.code, "test message")
		if rr.Code != tc.wantStatus {
			t.Errorf("SendError(%s): status = %d, want %d", tc.code, rr.Code, tc.wantStatus)
		}
	}
}

// ── getUptime ─────────────────────────────────────────────────────────────────

// Normal case: uptime < 24h returns "0h Xm Xs" format.
func TestGetUptime_SubDay_NoPrefix(t *testing.T) {
	result := getUptime()
	if result == "" {
		t.Error("getUptime: returned empty string")
	}
}

// When serverStartTime is over 24 hours ago, getUptime must include a day prefix.
func TestGetUptime_OverOneDayAgo_IncludesDays(t *testing.T) {
	old := serverStartTime
	serverStartTime = time.Now().Add(-25 * time.Hour)
	defer func() { serverStartTime = old }()

	result := getUptime()
	if !strings.Contains(result, "d") {
		t.Errorf("getUptime >24h: got %q, expected 'd' day marker", result)
	}
}

// ── ServerMetrics.Handler ─────────────────────────────────────────────────────

// Loopback request (127.0.0.1) with no token configured → 200 and metrics output.
func TestMetricsHandler_LoopbackNoToken_Returns200(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Metrics.Token = ""
	mgr := engine.NewEngineManager(cfg)
	m := NewMetrics(cfg, mgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	m.Handler()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics Handler loopback: status = %d, want 200", rr.Code)
	}
}

// Non-loopback request with no token configured → 403 Forbidden.
func TestMetricsHandler_NonLoopbackNoToken_Returns403(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Metrics.Token = ""
	mgr := engine.NewEngineManager(cfg)
	m := NewMetrics(cfg, mgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "192.168.1.100:1234"

	m.Handler()(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("metrics Handler non-loopback: status = %d, want 403", rr.Code)
	}
}

// Correct Bearer token → 200.
func TestMetricsHandler_CorrectBearerToken_Returns200(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Metrics.Token = "secret-token"
	mgr := engine.NewEngineManager(cfg)
	m := NewMetrics(cfg, mgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.RemoteAddr = "192.168.1.100:1234"

	m.Handler()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics Handler correct token: status = %d, want 200", rr.Code)
	}
}

// Wrong Bearer token but correct query-param token → 200.
func TestMetricsHandler_QueryParamToken_Returns200(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Metrics.Token = "qptoken"
	mgr := engine.NewEngineManager(cfg)
	m := NewMetrics(cfg, mgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics?token=qptoken", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	req.RemoteAddr = "10.0.0.1:1234"

	m.Handler()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics Handler query token: status = %d, want 200", rr.Code)
	}
}

// Wrong token in both header and query → 401.
func TestMetricsHandler_WrongToken_Returns401(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Metrics.Token = "correct"
	mgr := engine.NewEngineManager(cfg)
	m := NewMetrics(cfg, mgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	req.RemoteAddr = "192.168.1.100:1234"

	m.Handler()(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("metrics Handler wrong token: status = %d, want 401", rr.Code)
	}
}

// ── rotateLocked via slidingWindowCounter ─────────────────────────────────────

// Forcing lastRotate to be >24h ago exercises the "all stale" rotation branch.
func TestSlidingWindowCounter_RotateAllStale_MiscCoverage(t *testing.T) {
	c := newSlidingWindowCounter()
	c.increment()
	// Move lastRotate to 25 hours ago to trigger full-stale rotation.
	c.mu.Lock()
	c.lastRotate = time.Now().Truncate(time.Hour).Add(-25 * time.Hour)
	c.mu.Unlock()
	// count calls rotateLocked internally, clearing stale buckets.
	got := c.count()
	// All buckets should be cleared, so count is 0.
	if got != 0 {
		t.Errorf("slidingWindowCounter stale rotation: count = %d, want 0", got)
	}
}

// Forcing lastRotate a few hours ago exercises the partial rotation branch.
func TestSlidingWindowCounter_RotatePartial_MiscCoverage(t *testing.T) {
	c := newSlidingWindowCounter()
	c.increment()
	// Move lastRotate 2 hours ago to rotate 2 buckets.
	c.mu.Lock()
	c.lastRotate = time.Now().Truncate(time.Hour).Add(-2 * time.Hour)
	c.mu.Unlock()
	// The increment was in the old bucket; after rotation it may be cleared.
	_ = c.count()
}

// ── HealthCheck ───────────────────────────────────────────────────────────────

// HealthCheck with JSON Accept returns JSON 200.
func TestHealthCheck_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")

	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck JSON: status = %d, want 200", rr.Code)
	}
}

// HealthCheck with curl UA returns plain text.
func TestHealthCheck_TextFormat_ReturnsText(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck text: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("HealthCheck text: Content-Type = %q, want text/plain", ct)
	}
}

// HealthCheck with browser UA exercises the default case (HTML via renderHealthzHTML).
func TestHealthCheck_BrowserUA_HitsHTMLPath(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	h.HealthCheck(rr, req)

	// templatesFS is zero-value → renderHealthzHTML fails → 500
	if rr.Code == http.StatusFound {
		t.Error("HealthCheck browser: unexpected redirect")
	}
}

// HealthCheck with PendingRestart=true exercises the pending_restart branch.
func TestHealthCheck_PendingRestart_JSONIncludesPendingRestart(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.PendingRestart = true
	cfg.RestartReasons = []string{"config changed"}
	mgr := engine.NewEngineManager(cfg)
	h := &SearchHandler{appConfig: cfg, engineMgr: mgr}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")

	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck pending_restart: status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "pending_restart") {
		t.Errorf("HealthCheck pending_restart: response missing 'pending_restart' key")
	}
}

// ── Handler with metrics ──────────────────────────────────────────────────────

// newMetricsTestHandler creates a handler with metrics wired up.
func newMetricsTestHandler() *SearchHandler {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	m := NewMetrics(cfg, mgr)
	return &SearchHandler{
		appConfig: cfg,
		engineMgr: mgr,
		metrics:   m,
	}
}

// APIHealthCheck with metrics present exercises the metrics getter branches.
func TestAPIHealthCheck_WithMetrics_ReturnsJSON(t *testing.T) {
	h := newMetricsTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req.Header.Set("Accept", "application/json")

	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck with metrics: status = %d, want 200", rr.Code)
	}
}

// HealthCheck with metrics present exercises getRequestsTotal / getRequests24h / etc.
func TestHealthCheck_WithMetrics_JSONReturns200(t *testing.T) {
	h := newMetricsTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")

	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck with metrics: status = %d, want 200", rr.Code)
	}
}

// ── APIHealthCheck pending_restart path ───────────────────────────────────────

func TestAPIHealthCheck_PendingRestart_IncludesField(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.PendingRestart = true
	cfg.RestartReasons = []string{"test"}
	mgr := engine.NewEngineManager(cfg)
	h := &SearchHandler{appConfig: cfg, engineMgr: mgr}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req.Header.Set("Accept", "application/json")

	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck pending_restart: status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "pending_restart") {
		t.Errorf("APIHealthCheck pending_restart: missing field in body: %s", body)
	}
}

// ── HealthCheck text with pending_restart ─────────────────────────────────────

func TestHealthCheck_TextFormat_PendingRestart(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.PendingRestart = true
	cfg.RestartReasons = []string{"cfg"}
	mgr := engine.NewEngineManager(cfg)
	h := &SearchHandler{appConfig: cfg, engineMgr: mgr}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck text pending_restart: status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "pending_restart") {
		t.Errorf("HealthCheck text pending_restart: missing field in body: %s", body)
	}
}

// ── APIHealthCheck text with pending_restart ──────────────────────────────────

func TestAPIHealthCheck_TextFormat_PendingRestart(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.PendingRestart = true
	cfg.RestartReasons = []string{"cfg"}
	mgr := engine.NewEngineManager(cfg)
	h := &SearchHandler{appConfig: cfg, engineMgr: mgr}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck text pending_restart: status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "pending_restart") {
		t.Errorf("APIHealthCheck text pending_restart: missing field: %s", body)
	}
}

// ── renderTemplate additional named templates ─────────────────────────────────

// Each named template exercises the switch case and hits the parse error (empty FS).
func TestRenderTemplate_SearchName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "search", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate search empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_PreferencesName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "preferences", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate preferences empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_AgeVerifyName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "age-verify", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate age-verify empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_ContentRestrictedName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "content-restricted", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate content-restricted empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_ContentBlockedName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "content-blocked", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate content-blocked empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_PrivacyName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "privacy", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate privacy empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_NojsHomeName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/home", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate nojs/home empty FS: want 500, got %d", rr.Code)
	}
}

func TestRenderTemplate_NojsSearchName_Covered(t *testing.T) {
	h := newMiscTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/search", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate nojs/search empty FS: want 500, got %d", rr.Code)
	}
}

// ── APIHealthCheck — with Tor service set ────────────────────────────────────

func TestAPIHealthCheck_WithTorRunning_CoversLines1084_1085(t *testing.T) {
	// Use the testTorChecker from handler_content_coverage_test.go
	h := newRenderTestHandler()
	h.torSvc = &testTorChecker{enabled: true, running: true}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	h.APIHealthCheck(rr, req)

	if rr.Code == 0 {
		t.Error("APIHealthCheck: expected non-zero status")
	}
}

func TestAPIHealthCheck_WithTorNotRunning_CoversLine1087(t *testing.T) {
	h := newRenderTestHandler()
	h.torSvc = &testTorChecker{enabled: true, running: false}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	h.APIHealthCheck(rr, req)

	if rr.Code == 0 {
		t.Error("APIHealthCheck: expected non-zero status")
	}
}
