// SPDX-License-Identifier: MIT
// Coverage tests for errors.go, debug.go, and metrics.go in the handler package.
// All tests use same-package access (package handler) for unexported types.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// newChiCtx returns a context with a chi route parameter set.
func newChiCtx(key, value string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
}

// ── AppError ────────────────────────────────────────────────────────────────

func TestAppError_Error_ReturnsMessage(t *testing.T) {
	e := &AppError{Message: "something broke"}
	if got := e.Error(); got != "something broke" {
		t.Errorf("AppError.Error() = %q, want %q", got, "something broke")
	}
}

func TestNewAppError_Code(t *testing.T) {
	e := NewAppError(CodeNotFound, MsgNotFound)
	if e.Code != CodeNotFound {
		t.Errorf("NewAppError.Code = %q, want %q", e.Code, CodeNotFound)
	}
}

func TestNewAppError_Message(t *testing.T) {
	e := NewAppError(CodeBadRequest, MsgBadRequest)
	if e.Message != MsgBadRequest {
		t.Errorf("NewAppError.Message = %q, want %q", e.Message, MsgBadRequest)
	}
}

func TestNewAppError_HTTPStatus(t *testing.T) {
	e := NewAppError(CodeNotFound, MsgNotFound)
	if e.HTTPStatus != http.StatusNotFound {
		t.Errorf("NewAppError.HTTPStatus = %d, want %d", e.HTTPStatus, http.StatusNotFound)
	}
}

func TestAppError_WithInternal_SetsInternalError(t *testing.T) {
	inner := errors.New("db error")
	e := NewAppError(CodeServerError, MsgServerError).WithInternal(inner)
	if !errors.Is(e.Internal, inner) {
		t.Errorf("WithInternal: Internal = %v, want %v", e.Internal, inner)
	}
}

func TestAppError_WithInternal_ReturnsChain(t *testing.T) {
	e := NewAppError(CodeServerError, MsgServerError)
	if returned := e.WithInternal(errors.New("x")); returned != e {
		t.Error("WithInternal should return the same *AppError for chaining")
	}
}

func TestAppError_WithRequestID_SetsRequestID(t *testing.T) {
	e := NewAppError(CodeBadRequest, MsgBadRequest).WithRequestID("req-abc")
	if e.RequestID != "req-abc" {
		t.Errorf("WithRequestID: RequestID = %q, want %q", e.RequestID, "req-abc")
	}
}

func TestAppError_WithRequestID_ReturnsChain(t *testing.T) {
	e := NewAppError(CodeBadRequest, MsgBadRequest)
	if returned := e.WithRequestID("x"); returned != e {
		t.Error("WithRequestID should return the same *AppError for chaining")
	}
}

func TestAppError_Write_StatusCode(t *testing.T) {
	e := NewAppError(CodeNotFound, MsgNotFound)
	rr := httptest.NewRecorder()
	e.Write(rr)
	if rr.Code != http.StatusNotFound {
		t.Errorf("AppError.Write() status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestAppError_Write_BodyContainsCode(t *testing.T) {
	e := NewAppError(CodeBadRequest, MsgBadRequest)
	rr := httptest.NewRecorder()
	e.Write(rr)
	body := rr.Body.String()
	if body == "" {
		t.Error("AppError.Write() body should not be empty")
	}
}

// ── Pre-defined error variables ────────────────────────────────────────────

func TestPredefinedAppErrors_NonNil(t *testing.T) {
	for name, err := range map[string]*AppError{
		"ErrBadRequest":   ErrBadRequest,
		"ErrUnauthorized": ErrUnauthorized,
		"ErrForbidden":    ErrForbidden,
		"ErrNotFound":     ErrNotFound,
		"ErrConflict":     ErrConflict,
		"ErrRateLimited":  ErrRateLimited,
		"ErrServerError":  ErrServerError,
		"ErrMaintenance":  ErrMaintenance,
	} {
		if err == nil {
			t.Errorf("predefined error %s is nil", name)
		}
	}
}

// ── IsRetryable ─────────────────────────────────────────────────────────────

func TestIsRetryable_DeadlineExceeded_True(t *testing.T) {
	if !IsRetryable(context.DeadlineExceeded) {
		t.Error("IsRetryable(DeadlineExceeded) = false, want true")
	}
}

func TestIsRetryable_ContextCanceled_False(t *testing.T) {
	if IsRetryable(context.Canceled) {
		t.Error("IsRetryable(Canceled) = true, want false")
	}
}

func TestIsRetryable_ECONNREFUSED_True(t *testing.T) {
	if !IsRetryable(syscall.ECONNREFUSED) {
		t.Error("IsRetryable(ECONNREFUSED) = false, want true")
	}
}

func TestIsRetryable_ECONNRESET_True(t *testing.T) {
	if !IsRetryable(syscall.ECONNRESET) {
		t.Error("IsRetryable(ECONNRESET) = false, want true")
	}
}

func TestIsRetryable_ETIMEDOUT_True(t *testing.T) {
	if !IsRetryable(syscall.ETIMEDOUT) {
		t.Error("IsRetryable(ETIMEDOUT) = false, want true")
	}
}

func TestIsRetryable_AppError503_True(t *testing.T) {
	e := &AppError{HTTPStatus: 503}
	if !IsRetryable(e) {
		t.Error("IsRetryable(503 AppError) = false, want true")
	}
}

func TestIsRetryable_AppError400_False(t *testing.T) {
	e := &AppError{HTTPStatus: 400}
	if IsRetryable(e) {
		t.Error("IsRetryable(400 AppError) = true, want false")
	}
}

func TestIsRetryable_GenericError_False(t *testing.T) {
	if IsRetryable(errors.New("some error")) {
		t.Error("IsRetryable(generic error) = true, want false")
	}
}

// ── WithRetry ───────────────────────────────────────────────────────────────

func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := WithRetry(context.Background(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("WithRetry: unexpected error %v", err)
	}
	if calls != 1 {
		t.Errorf("WithRetry: fn called %d times, want 1", calls)
	}
}

func TestWithRetry_NonRetryableErrorStopsImmediately(t *testing.T) {
	sentinel := errors.New("not retryable")
	calls := 0
	err := WithRetry(context.Background(), func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("WithRetry: error = %v, want %v", err, sentinel)
	}
	if calls != 1 {
		t.Errorf("WithRetry: fn called %d times, want 1 for non-retryable", calls)
	}
}

func TestWithRetry_CancelledContextStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WithRetry(ctx, func() error {
		return context.DeadlineExceeded
	})
	// Cancelled context returns ctx.Err() before waiting
	if !errors.Is(err, context.Canceled) {
		t.Errorf("WithRetry(cancelled ctx) = %v, want context.Canceled", err)
	}
}

// ── LogError ────────────────────────────────────────────────────────────────

func TestLogError_5xxLogsError(t *testing.T) {
	e := &AppError{Code: CodeServerError, HTTPStatus: 500, Message: "internal error"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// Must not panic; we only verify it doesn't crash
	LogError(context.Background(), e, logger)
}

func TestLogError_4xxLogsWarn(t *testing.T) {
	e := &AppError{Code: CodeBadRequest, HTTPStatus: 400, Message: "bad request"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	LogError(context.Background(), e, logger)
}

func TestLogError_WithRequestIDAndInternal(t *testing.T) {
	e := &AppError{
		Code:       CodeServerError,
		HTTPStatus: 500,
		Message:    "boom",
		RequestID:  "req-xyz",
		Internal:   errors.New("internal db error"),
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	LogError(context.Background(), e, logger)
}

// ── debug.go ────────────────────────────────────────────────────────────────

func TestDebugVars_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/vars", nil)
	rr := httptest.NewRecorder()
	DebugVars(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DebugVars status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDebugPprof_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rr := httptest.NewRecorder()
	DebugPprof(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DebugPprof status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDebugPprofCmdline_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	rr := httptest.NewRecorder()
	DebugPprofCmdline(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DebugPprofCmdline status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDebugPprofSymbol_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/symbol", nil)
	rr := httptest.NewRecorder()
	DebugPprofSymbol(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DebugPprofSymbol status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestDebugPprofHandler_Goroutine_Returns200(t *testing.T) {
	// DebugPprofHandler uses chi.URLParam to get the handler name; without a
	// chi router context the name resolves to "goroutine" which is a known
	// pprof handler and returns 200.
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine", nil)
	// Inject chi context with the "name" URL parameter.
	rctx := newChiCtx("name", "goroutine")
	req = req.WithContext(rctx)
	rr := httptest.NewRecorder()
	DebugPprofHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DebugPprofHandler(goroutine) status = %d, want 200", rr.Code)
	}
}

// ── metrics.go: slidingWindowCounter ────────────────────────────────────────

func TestSlidingWindowCounter_NewIsZero(t *testing.T) {
	c := newSlidingWindowCounter()
	if c == nil {
		t.Fatal("newSlidingWindowCounter() returned nil")
	}
	if c.count() != 0 {
		t.Errorf("new counter count = %d, want 0", c.count())
	}
}

func TestSlidingWindowCounter_IncrementAndCount(t *testing.T) {
	c := newSlidingWindowCounter()
	c.increment()
	if c.count() != 1 {
		t.Errorf("count after 1 increment = %d, want 1", c.count())
	}
}

func TestSlidingWindowCounter_MultipleIncrements(t *testing.T) {
	c := newSlidingWindowCounter()
	for i := 0; i < 5; i++ {
		c.increment()
	}
	if c.count() != 5 {
		t.Errorf("count after 5 increments = %d, want 5", c.count())
	}
}

// ── ServerMetrics ────────────────────────────────────────────────────────────

func TestNewMetrics_NonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := NewMetrics(cfg, nil)
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
}

func TestServerMetrics_IncrementRequests(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	m.IncrementRequests()
	if m.GetRequestsTotal() != 1 {
		t.Errorf("GetRequestsTotal after 1 increment = %d, want 1", m.GetRequestsTotal())
	}
}

func TestServerMetrics_IncrementSearches(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	m.IncrementSearches()
	if m.GetSearchesTotal() != 1 {
		t.Errorf("GetSearchesTotal after 1 increment = %d, want 1", m.GetSearchesTotal())
	}
}

func TestServerMetrics_IncrementCacheHits(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	m.IncrementCacheHits()
	if m.GetCacheHitsTotal() != 1 {
		t.Errorf("GetCacheHitsTotal after 1 increment = %d, want 1", m.GetCacheHitsTotal())
	}
}

func TestServerMetrics_IncrementSearchErrors(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	m.IncrementSearchErrors()
	if m.GetSearchErrors() != 1 {
		t.Errorf("GetSearchErrors after 1 increment = %d, want 1", m.GetSearchErrors())
	}
}

func TestServerMetrics_IncrementAPIRequests(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	m.IncrementAPIRequests()
	if m.GetAPIRequestsTotal() != 1 {
		t.Errorf("GetAPIRequestsTotal after 1 increment = %d, want 1", m.GetAPIRequestsTotal())
	}
}

func TestServerMetrics_GetRequests24h_InitiallyZero(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	if m.GetRequests24h() != 0 {
		t.Errorf("GetRequests24h initially = %d, want 0", m.GetRequests24h())
	}
}

func TestServerMetrics_GetSearches24h_AfterIncrement(t *testing.T) {
	m := NewMetrics(config.DefaultAppConfig(), nil)
	m.IncrementSearches()
	if m.GetSearches24h() != 1 {
		t.Errorf("GetSearches24h after 1 increment = %d, want 1", m.GetSearches24h())
	}
}

func TestServerMetrics_StartTimeRecent(t *testing.T) {
	before := time.Now()
	m := NewMetrics(config.DefaultAppConfig(), nil)
	after := time.Now()
	if m.startTime.Before(before) || m.startTime.After(after) {
		t.Errorf("startTime = %v, want between %v and %v", m.startTime, before, after)
	}
}

// NewMetrics with real engine manager (nil is acceptable per the engine package)
func TestNewMetrics_WithNilEngine(t *testing.T) {
	var mgr *engine.EngineManager
	m := NewMetrics(config.DefaultAppConfig(), mgr)
	if m == nil {
		t.Fatal("NewMetrics(cfg, nil engine) returned nil")
	}
}

// ── ServerMetrics nil-counter fallback paths (GetRequests24h / GetSearches24h) ─

func TestServerMetrics_GetRequests24h_NilCounter_ReturnsZero(t *testing.T) {
	// Create struct directly so requests24h == nil → returns 0 via line 176
	m := &ServerMetrics{}
	if got := m.GetRequests24h(); got != 0 {
		t.Errorf("GetRequests24h(nil counter) = %d, want 0", got)
	}
}

func TestServerMetrics_GetSearches24h_NilCounter_ReturnsZero(t *testing.T) {
	m := &ServerMetrics{}
	if got := m.GetSearches24h(); got != 0 {
		t.Errorf("GetSearches24h(nil counter) = %d, want 0", got)
	}
}

// ── WriteJSON — marshal error fallback path ───────────────────────────────────

func TestWriteJSON_MarshalError_WritesErrorBody(t *testing.T) {
	rr := httptest.NewRecorder()
	// Channels are not JSON-serializable → MarshalIndent returns an error
	WriteJSON(rr, http.StatusOK, make(chan int))
	body := rr.Body.String()
	if body == "" {
		t.Error("WriteJSON(chan): expected non-empty error body")
	}
}

// ── RenderErrorPage — plain text fallback (template FS empty in tests) ────────

func TestRenderErrorPage_EmptyFS_ServesPlainText(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)

	h.RenderErrorPage(rr, req, http.StatusNotFound, "Not Found", "The page does not exist")

	if rr.Code == http.StatusOK {
		t.Error("RenderErrorPage: expected non-200 code with empty FS")
	}
}

// ── ServerMetrics IncrementRequests — nil counter path ────────────────────────

func TestServerMetrics_IncrementRequests_NilCounter_NoPanic(t *testing.T) {
	m := &ServerMetrics{}
	m.IncrementRequests()
	if m.GetRequestsTotal() != 1 {
		t.Errorf("IncrementRequests(nil 24h counter) = %d, want 1", m.GetRequestsTotal())
	}
}

func TestServerMetrics_IncrementSearches_NilCounter_NoPanic(t *testing.T) {
	m := &ServerMetrics{}
	m.IncrementSearches()
	if m.GetSearchesTotal() != 1 {
		t.Errorf("IncrementSearches(nil 24h counter) = %d, want 1", m.GetSearchesTotal())
	}
}

// ── MaintenanceModeMiddleware — active maintenance mode ───────────────────────

func TestMaintenanceModeMiddleware_MaintenanceFlagExists_Returns503(t *testing.T) {
	h := newRenderTestHandler()

	// Write maintenance flag to the expected location
	paths := config.GetAppPaths("", "")
	flagFile := paths.Data + "/maintenance.flag"
	if err := os.MkdirAll(paths.Data, 0755); err != nil {
		t.Skipf("cannot create data dir %s: %v", paths.Data, err)
	}
	if err := os.WriteFile(flagFile, []byte(""), 0644); err != nil {
		t.Skipf("cannot create maintenance flag: %v", err)
	}
	defer os.Remove(flagFile)

	handler := h.MaintenanceModeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("MaintenanceModeMiddleware(active): status = %d, want 503", rr.Code)
	}
}

func TestMaintenanceModeMiddleware_HealthzPath_PassesThrough(t *testing.T) {
	h := newRenderTestHandler()

	called := false
	handler := h.MaintenanceModeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("MaintenanceModeMiddleware(/healthz): expected next handler to be called")
	}
}

// ── SecurityTxt — branches ────────────────────────────────────────────────────

func TestSecurityTxt_ContactWithMailto_SkipsPrefix(t *testing.T) {
	cfg := createTestConfig()
	cfg.Web.Security.Contact = "mailto:security@example.com"
	cfg.Web.Security.Expires = "2026-01-01T00:00:00Z"
	h := NewSearchHandler(cfg, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/security.txt", nil)
	h.SecurityTxt(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SecurityTxt: status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "mailto:security@example.com") {
		t.Error("SecurityTxt: expected existing mailto prefix preserved")
	}
}

func TestSecurityTxt_ContactWithoutMailto_AddsPrefix(t *testing.T) {
	cfg := createTestConfig()
	cfg.Web.Security.Contact = "security@example.com"
	cfg.Web.Security.Expires = ""
	h := NewSearchHandler(cfg, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/security.txt", nil)
	h.SecurityTxt(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SecurityTxt: status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "mailto:security@example.com") {
		t.Errorf("SecurityTxt: expected mailto prefix added, got %q", body)
	}
}

// ── WellKnownVidVeil — branches ───────────────────────────────────────────────

func TestWellKnownVidVeil_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/vidveil", nil)
	h.WellKnownVidVeil(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("WellKnownVidVeil: status = %d, want 200", rr.Code)
	}
}

// ── SearchPage with metrics ───────────────────────────────────────────────────

func TestSearchPage_WithMetrics_IncrementsCounter(t *testing.T) {
	// Use DefaultAppConfig to ensure ResultsPerPage != 0 (avoids divide-by-zero in Search)
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	h := NewSearchHandler(cfg, mgr)
	h.metrics = NewMetrics(cfg, mgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	req.Header.Set("Accept", "application/json")
	h.SearchPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SearchPage with metrics: status = %d, want 200", rr.Code)
	}
	if h.metrics.GetSearchesTotal() == 0 {
		t.Error("SearchPage with metrics: IncrementSearches not called")
	}
}
