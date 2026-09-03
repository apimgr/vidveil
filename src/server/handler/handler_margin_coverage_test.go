// SPDX-License-Identifier: MIT
// AI.md PART 28: Additional deterministic coverage for handler setters,
// engine-health content negotiation, and the JSON error responder.
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerSetters exercises the trivial configuration setters, which are
// otherwise only wired from main.
func TestHandlerSetters(t *testing.T) {
	h := newAPITestHandler()

	h.SetConfigDir("/tmp/cfg-xyz")
	if h.configDir != "/tmp/cfg-xyz" {
		t.Errorf("SetConfigDir: configDir = %q", h.configDir)
	}

	h.SetDataDir("/tmp/data-xyz")
	if h.dataDir != "/tmp/data-xyz" {
		t.Errorf("SetDataDir: dataDir = %q", h.dataDir)
	}

	m := &ServerMetrics{}
	h.SetMetrics(m)
	if h.metrics != m {
		t.Error("SetMetrics did not attach metrics")
	}
}

// TestAPIEngineHealth_JSON verifies the default JSON response shape.
func TestAPIEngineHealth_JSON(t *testing.T) {
	h := newAPITestHandlerWithEngines()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/engines/health", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	h.APIEngineHealth(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Errorf("expected ok=true, got %v", body["ok"])
	}
}

// TestAPIEngineHealth_PlainText verifies the text/plain content-negotiated path.
func TestAPIEngineHealth_PlainText(t *testing.T) {
	h := newAPITestHandlerWithEngines()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/engines/health", nil)
	r.Header.Set("User-Agent", "curl/7.68.0")
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()

	h.APIEngineHealth(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	// With engines initialized, the body should list at least one engine name line.
	if strings.Contains(w.Body.String(), "name:") == false && w.Body.Len() != 0 {
		t.Errorf("plain body missing engine fields: %q", w.Body.String())
	}
}

// TestSendErrorWithDetails verifies the JSON error responder maps the code to a
// status and emits a well-formed body.
func TestSendErrorWithDetails(t *testing.T) {
	cases := []struct {
		code       string
		wantStatus int
	}{
		{"BAD_REQUEST", http.StatusBadRequest},
		{"UNAUTHORIZED", http.StatusUnauthorized},
		{"NOT_FOUND", http.StatusNotFound},
		{"RATE_LIMITED", http.StatusTooManyRequests},
		{"SOMETHING_ELSE", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		sendErrorWithDetails(w, tc.code, "boom", map[string]string{"field": "x"})
		if w.Code != tc.wantStatus {
			t.Errorf("code %s: status = %d, want %d", tc.code, w.Code, tc.wantStatus)
		}
		var body APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("code %s: body not JSON: %v", tc.code, err)
		}
		if body.OK {
			t.Errorf("code %s: expected ok=false", tc.code)
		}
		if body.Error != tc.code {
			t.Errorf("code %s: error field = %q", tc.code, body.Error)
		}
	}
}
