// SPDX-License-Identifier: MIT
// Tests for response.go and errors.go: AppError, ErrorCodeToHTTP, IsRetryable,
// pre-defined errors, SendOK, SendError, cookie helpers, resolveLocale,
// injectLocaleData.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/apimgr/vidveil/src/common/i18n"
)

// ---- NewAppError ----

// NewAppError must return a non-nil error with the correct fields populated.
func TestNewAppError_NonNil(t *testing.T) {
	e := NewAppError(CodeBadRequest, MsgBadRequest)
	if e == nil {
		t.Fatal("NewAppError returned nil")
	}
}

// Error() must return the message passed to NewAppError.
func TestNewAppError_ErrorMessage(t *testing.T) {
	e := NewAppError(CodeNotFound, "custom not found")
	if e.Error() != "custom not found" {
		t.Errorf("Error() = %q, want %q", e.Error(), "custom not found")
	}
}

// Code and HTTPStatus must be set from the supplied error code.
func TestNewAppError_CodeAndHTTPStatus(t *testing.T) {
	e := NewAppError(CodeForbidden, MsgForbidden)
	if e.Code != CodeForbidden {
		t.Errorf("Code = %q, want %q", e.Code, CodeForbidden)
	}
	if e.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d, want 403", e.HTTPStatus)
	}
}

// WithInternal must store the wrapped internal error.
func TestNewAppError_WithInternal(t *testing.T) {
	inner := errors.New("db gone")
	e := NewAppError(CodeServerError, MsgServerError).WithInternal(inner)
	if e.Internal != inner {
		t.Error("WithInternal did not store the supplied error")
	}
}

// WithRequestID must store the request ID string.
func TestNewAppError_WithRequestID(t *testing.T) {
	e := NewAppError(CodeUnauthorized, MsgUnauthorized).WithRequestID("req-123")
	if e.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", e.RequestID, "req-123")
	}
}

// Write must send a JSON error response with the correct status code.
func TestAppError_Write(t *testing.T) {
	e := NewAppError(CodeNotFound, MsgNotFound)
	rr := httptest.NewRecorder()
	e.Write(rr)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Write status = %d, want 404", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("Write returned invalid JSON: %v", err)
	}
	if body["error"] != CodeNotFound {
		t.Errorf("body[error] = %v, want %q", body["error"], CodeNotFound)
	}
}

// ---- ErrorCodeToHTTP ----

// Each defined error code maps to its documented HTTP status.
func TestErrorCodeToHTTP_AllBranches(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{CodeBadRequest, 400},
		{CodeValidation, 400},
		{CodeUnauthorized, 401},
		{CodeTokenExpired, 401},
		{CodeTokenInvalid, 401},
		{Code2FARequired, 401},
		{Code2FAInvalid, 401},
		{CodeForbidden, 403},
		{CodeAccountLocked, 403},
		{CodeNotFound, 404},
		{CodeMethodNotAllowed, 405},
		{CodeConflict, 409},
		{CodeRateLimited, 429},
		{CodeMaintenance, 503},
		{CodeServerError, 500},
		{"UNKNOWN_CODE", 500},
		{"", 500},
	}

	for _, c := range cases {
		got := ErrorCodeToHTTP(c.code)
		if got != c.want {
			t.Errorf("ErrorCodeToHTTP(%q) = %d, want %d", c.code, got, c.want)
		}
	}
}

// ---- IsRetryable ----

// nil is never retryable.
func TestIsRetryable_Nil(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("IsRetryable(nil) must be false")
	}
}

// context.DeadlineExceeded is retryable (timeout, may succeed on retry).
func TestIsRetryable_DeadlineExceeded(t *testing.T) {
	if !IsRetryable(context.DeadlineExceeded) {
		t.Error("IsRetryable(context.DeadlineExceeded) must be true")
	}
}

// context.Canceled is not retryable (caller explicitly cancelled).
func TestIsRetryable_Canceled(t *testing.T) {
	if IsRetryable(context.Canceled) {
		t.Error("IsRetryable(context.Canceled) must be false")
	}
}

// ECONNREFUSED is a transient network error and is retryable.
func TestIsRetryable_ECONNREFUSED(t *testing.T) {
	if !IsRetryable(syscall.ECONNREFUSED) {
		t.Error("IsRetryable(ECONNREFUSED) must be true")
	}
}

// ECONNRESET is a transient network error and is retryable.
func TestIsRetryable_ECONNRESET(t *testing.T) {
	if !IsRetryable(syscall.ECONNRESET) {
		t.Error("IsRetryable(ECONNRESET) must be true")
	}
}

// ETIMEDOUT is a transient network error and is retryable.
func TestIsRetryable_ETIMEDOUT(t *testing.T) {
	if !IsRetryable(syscall.ETIMEDOUT) {
		t.Error("IsRetryable(ETIMEDOUT) must be true")
	}
}

// An AppError with HTTP 503 (service unavailable) is retryable.
func TestIsRetryable_AppError503(t *testing.T) {
	e := NewAppError(CodeMaintenance, MsgMaintenance)
	if !IsRetryable(e) {
		t.Error("IsRetryable(AppError 503) must be true")
	}
}

// An AppError with HTTP 400 (client error) is not retryable.
func TestIsRetryable_AppError400(t *testing.T) {
	e := NewAppError(CodeBadRequest, MsgBadRequest)
	if IsRetryable(e) {
		t.Error("IsRetryable(AppError 400) must be false")
	}
}

// A generic non-network error is not retryable.
func TestIsRetryable_GenericError(t *testing.T) {
	if IsRetryable(errors.New("some random error")) {
		t.Error("IsRetryable(generic error) must be false")
	}
}

// ---- Pre-defined errors ----

// All exported error sentinels must be non-nil and carry the right code.
func TestPredefinedErrors_NonNil(t *testing.T) {
	cases := []struct {
		name string
		err  *AppError
		code string
	}{
		{"ErrBadRequest", ErrBadRequest, CodeBadRequest},
		{"ErrUnauthorized", ErrUnauthorized, CodeUnauthorized},
		{"ErrForbidden", ErrForbidden, CodeForbidden},
		{"ErrNotFound", ErrNotFound, CodeNotFound},
		{"ErrRateLimited", ErrRateLimited, CodeRateLimited},
		{"ErrServerError", ErrServerError, CodeServerError},
		{"ErrMaintenance", ErrMaintenance, CodeMaintenance},
	}

	for _, c := range cases {
		if c.err == nil {
			t.Errorf("%s must not be nil", c.name)
			continue
		}
		if c.err.Code != c.code {
			t.Errorf("%s.Code = %q, want %q", c.name, c.err.Code, c.code)
		}
	}
}

// ---- SendOK ----

// SendOK must respond with 200, application/json Content-Type, and ok:true.
func TestSendOK_Basic(t *testing.T) {
	rr := httptest.NewRecorder()
	SendOK(rr, map[string]string{"key": "val"})

	if rr.Code != http.StatusOK {
		t.Errorf("SendOK status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("SendOK Content-Type = %q, want application/json", ct)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("SendOK returned invalid JSON: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("SendOK body[ok] = %v, want true", body["ok"])
	}
}

// SendOK with nil data must still produce valid JSON.
func TestSendOK_NilData(t *testing.T) {
	rr := httptest.NewRecorder()
	SendOK(rr, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("SendOK nil status = %d, want 200", rr.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("SendOK nil returned invalid JSON: %v", err)
	}
}

// ---- SendError ----

// SendError must set the status from the error code and include the code in body.
func TestSendError_BadRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, CodeBadRequest, MsgBadRequest)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("SendError status = %d, want 400", rr.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("SendError returned invalid JSON: %v", err)
	}
	if body["ok"] != false {
		t.Errorf("SendError body[ok] = %v, want false", body["ok"])
	}
	if body["error"] != CodeBadRequest {
		t.Errorf("SendError body[error] = %v, want %q", body["error"], CodeBadRequest)
	}
}

// SendError with CodeMaintenance must produce HTTP 503.
func TestSendError_Maintenance(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, CodeMaintenance, MsgMaintenance)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("SendError maintenance status = %d, want 503", rr.Code)
	}
}

// ---- Cookie helpers ----

// NewSecureCookie must set Name, Value, Path, MaxAge, HttpOnly=true, SameSite=Lax.
// When sslEnabled=false the Secure flag must be false.
func TestNewSecureCookie_Fields(t *testing.T) {
	c := NewSecureCookie("session", "abc123", "/", 3600, false)

	if c.Name != "session" {
		t.Errorf("Name = %q, want session", c.Name)
	}
	if c.Value != "abc123" {
		t.Errorf("Value = %q, want abc123", c.Value)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
	if c.MaxAge != 3600 {
		t.Errorf("MaxAge = %d, want 3600", c.MaxAge)
	}
	if !c.HttpOnly {
		t.Error("HttpOnly must be true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want SameSiteLaxMode", c.SameSite)
	}
	if c.Secure {
		t.Error("Secure must be false when sslEnabled=false")
	}
}

// When sslEnabled=true, Secure must be true.
func TestNewSecureCookie_SecureFlag(t *testing.T) {
	c := NewSecureCookie("tok", "x", "/", 60, true)
	if !c.Secure {
		t.Error("Secure must be true when sslEnabled=true")
	}
}

// NewSecureCookieStrict must use SameSite=Strict.
func TestNewSecureCookieStrict_SameSite(t *testing.T) {
	c := NewSecureCookieStrict("tok", "x", "/admin", 300, false)

	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want SameSiteStrictMode", c.SameSite)
	}
	if !c.HttpOnly {
		t.Error("HttpOnly must be true on strict cookie")
	}
}

// DeleteCookie must produce MaxAge=-1 and an empty value.
func TestDeleteCookie_Fields(t *testing.T) {
	c := DeleteCookie("session", "/")

	if c.Name != "session" {
		t.Errorf("Name = %q, want session", c.Name)
	}
	if c.Value != "" {
		t.Errorf("Value = %q, want empty", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
}

// ---- resolveLocale ----

// ?lang= query parameter takes priority over all other signals.
func TestResolveLocale_QueryParam(t *testing.T) {
	r := httptest.NewRequest("GET", "/?lang=fr", nil)
	got := resolveLocale(r)
	if got != "fr" {
		t.Errorf("resolveLocale ?lang=fr = %q, want fr", got)
	}
}

// lang= query is normalised to lower-case.
func TestResolveLocale_QueryParamUpperCase(t *testing.T) {
	r := httptest.NewRequest("GET", "/?lang=ZH-TW", nil)
	got := resolveLocale(r)
	if got != "zh-tw" {
		t.Errorf("resolveLocale ?lang=ZH-TW = %q, want zh-tw", got)
	}
}

// The lang cookie is used when no query parameter is present.
func TestResolveLocale_Cookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "lang", Value: "DE"})
	got := resolveLocale(r)
	if got != "de" {
		t.Errorf("resolveLocale cookie DE = %q, want de", got)
	}
}

// Accept-Language header is parsed when neither query nor cookie is present.
func TestResolveLocale_AcceptLanguage(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	got := resolveLocale(r)
	if got != "en-us" {
		t.Errorf("resolveLocale Accept-Language = %q, want en-us", got)
	}
}

// When no hints are present the default locale is returned.
func TestResolveLocale_Default(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	got := resolveLocale(r)
	if got != i18n.DefaultLocale {
		t.Errorf("resolveLocale default = %q, want %q", got, i18n.DefaultLocale)
	}
}

// Query parameter wins over cookie.
func TestResolveLocale_QueryOverridesCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/?lang=ja", nil)
	r.AddCookie(&http.Cookie{Name: "lang", Value: "ko"})
	got := resolveLocale(r)
	if got != "ja" {
		t.Errorf("resolveLocale query overrides cookie = %q, want ja", got)
	}
}

// Cookie wins over Accept-Language header.
func TestResolveLocale_CookieOverridesAcceptLanguage(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "lang", Value: "pt"})
	r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	got := resolveLocale(r)
	if got != "pt" {
		t.Errorf("resolveLocale cookie overrides Accept-Language = %q, want pt", got)
	}
}

// ---- injectLocaleData ----

// nil request must not panic.
func TestInjectLocaleData_NilRequest(t *testing.T) {
	data := map[string]interface{}{"X": 1}
	injectLocaleData(nil, data)
}

// nil data map must not panic.
func TestInjectLocaleData_NilData(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	injectLocaleData(r, nil)
}

// Both nil must not panic.
func TestInjectLocaleData_BothNil(t *testing.T) {
	injectLocaleData(nil, nil)
}

// Lang and Dir keys are populated when absent.
func TestInjectLocaleData_PopulatesLangAndDir(t *testing.T) {
	r := httptest.NewRequest("GET", "/?lang=en", nil)
	data := map[string]interface{}{}
	injectLocaleData(r, data)

	if _, ok := data["Lang"]; !ok {
		t.Error("injectLocaleData must populate Lang key")
	}
	if _, ok := data["Dir"]; !ok {
		t.Error("injectLocaleData must populate Dir key")
	}
}

// An existing Lang key must not be overwritten.
func TestInjectLocaleData_DoesNotOverwriteLang(t *testing.T) {
	r := httptest.NewRequest("GET", "/?lang=fr", nil)
	data := map[string]interface{}{"Lang": "preserved"}
	injectLocaleData(r, data)

	if data["Lang"] != "preserved" {
		t.Errorf("injectLocaleData overwrote existing Lang: got %v", data["Lang"])
	}
}

// An existing Dir key must not be overwritten.
func TestInjectLocaleData_DoesNotOverwriteDir(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	data := map[string]interface{}{"Lang": "en", "Dir": "preserved-dir"}
	injectLocaleData(r, data)

	if data["Dir"] != "preserved-dir" {
		t.Errorf("injectLocaleData overwrote existing Dir: got %v", data["Dir"])
	}
}
