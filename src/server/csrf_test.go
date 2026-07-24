// SPDX-License-Identifier: MIT
// AI.md PART 16 -> CSRF Protection: unit tests for pure helper functions.
package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
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
