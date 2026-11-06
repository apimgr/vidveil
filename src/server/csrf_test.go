// SPDX-License-Identifier: MIT
// AI.md PART 16 -> CSRF Protection: unit tests for pure helper functions.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	mcpserver "github.com/apimgr/vidveil/src/server/mcp"
)

func TestCsrfPathMatches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact match", "/api/v1/health", "/api/v1/health", true},
		{"wildcard match", "/api/v1/*", "/api/v1/health", true},
		{"wildcard no match across segments", "/api/v1/*", "/api/v1/foo/bar", false},
		{"no match", "/api/v1/health", "/api/v1/version", false},
		{"invalid pattern returns false", "[", "/api/v1/health", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := csrfPathMatches(tt.pattern, tt.path); got != tt.want {
				t.Errorf("csrfPathMatches(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestCsrfBypassMCPAliases(t *testing.T) {
	for _, path := range []string{"/mcp", "/api/mcp", "/api/v2/server/mcp"} {
		req := httptest.NewRequest("POST", path, nil)
		if !csrfBypass(config.CSRFConfig{}, req) {
			t.Errorf("CSRF should be bypassed for anonymous MCP POST %s", path)
		}
	}
	for _, path := range []string{"/not-api/server/mcp", "/custom/server/mcp"} {
		req := httptest.NewRequest("POST", path, nil)
		if csrfBypass(config.CSRFConfig{}, req) {
			t.Errorf("CSRF unexpectedly bypassed for non-MCP route %s", path)
		}
	}
}

func TestCsrfBypassBackendAPI(t *testing.T) {
	for _, path := range []string{"/api/v1/server/preferences", "/api/v2/server/preferences", "/api/v1/server/preferences/import", "/api/custom/server/preferences", "/api/v1/anything"} {
		req := httptest.NewRequest("POST", path, nil)
		if !csrfBypass(config.CSRFConfig{}, req) {
			t.Errorf("CSRF should be bypassed for API route %s", path)
		}
	}
	for _, path := range []string{"/server/preferences", "/server/preferences/import"} {
		req := httptest.NewRequest("POST", path, nil)
		if csrfBypass(config.CSRFConfig{}, req) {
			t.Errorf("CSRF unexpectedly bypassed for %s", path)
		}
	}
}

func TestCSRFMiddlewareLeavesAPIOutsideCSRF(t *testing.T) {
	h := newCSRFMiddleware(config.CSRFConfig{Enabled: true, CookieName: "csrf_token", HeaderName: "X-CSRF-Token"}, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest("POST", "/api/v1/server/preferences", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("API POST status = %d, want 204", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("API POST received CSRF cookies: %#v", rec.Result().Cookies())
	}
}

func TestCsrfBypassInternalMCPAPIDispatch(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/search/batch", nil)
	req = req.WithContext(mcpserver.WithInternalDispatch(req.Context()))
	if !csrfBypass(config.CSRFConfig{}, req) {
		t.Fatal("CSRF should be bypassed for an internal MCP API dispatch")
	}
}

func TestCsrfDeny(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/resource", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	rec := httptest.NewRecorder()

	csrfDeny(rec, req, "token_mismatch", "/api/v1/resource", nil)

	if rec.Code != 403 {
		t.Errorf("csrfDeny: status = %d, want 403", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("csrfDeny: Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("csrfDeny: response body is not valid JSON: %v", err)
	}
	if ok, _ := body["ok"].(bool); ok {
		t.Errorf("csrfDeny: body[\"ok\"] = %v, want false", body["ok"])
	}
	if body["error"] != "CSRF_FAILED" {
		t.Errorf("csrfDeny: body[\"error\"] = %v, want CSRF_FAILED", body["error"])
	}
}
