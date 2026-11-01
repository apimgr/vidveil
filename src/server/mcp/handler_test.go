// SPDX-License-Identifier: MIT
package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestHandlerServesStatelessOpenAPITools(t *testing.T) {
	cfg := config.DefaultAppConfig()
	var dispatched *http.Request
	handler := OriginMiddleware(cfg, Handler(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatched = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}), nil))

	response := serveMCP(t, handler, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, "")
	if response.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d, body: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, operation := range []string{`"name":"searchVideos"`, `"name":"submitServerContact"`, `"name":"proxyVideo"`} {
		if !strings.Contains(body, operation) {
			t.Fatalf("tools/list omitted OpenAPI operation %s: %s", operation, body)
		}
	}
	if strings.Contains(body, "restoreServerBackup") || strings.Contains(body, "OwnerToken") || strings.Contains(body, "LinkedToken") {
		t.Fatalf("tools/list exposed a protected or token-management operation: %s", body)
	}
	if response.Header().Get("Mcp-Session-Id") != "" {
		t.Fatal("stateless MCP response minted a session id")
	}
	operatorHandler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), func(*http.Request) bool { return true }))
	operatorResponse := serveMCP(t, operatorHandler, `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`, "")
	if !strings.Contains(operatorResponse.Body.String(), "restoreServerBackup") || !strings.Contains(operatorResponse.Body.String(), "base64") {
		t.Fatalf("operator tools/list omitted protected restore tool: %s", operatorResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"searchVideos","arguments":{"q":"needle"}}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `\"ok\":true`) {
		t.Fatalf("tools/call response = %d %s", response.Code, response.Body.String())
	}
	if dispatched == nil || dispatched.URL.Query().Get("q") != "needle" {
		t.Fatalf("API dispatch request = %#v", dispatched)
	}
	if dispatched.Header.Get("Authorization") != "Bearer test-token" {
		t.Fatal("tool dispatch did not forward the Bearer token")
	}
}

func TestHandlerDisabledAndOriginValidation(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.MCP.Enabled = false
	handler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), nil))
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d, want 403", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{}`))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled MCP status = %d, want 404", response.Code)
	}
}

func TestHandlerRejectsSessionAndStreamingMethods(t *testing.T) {
	cfg := config.DefaultAppConfig()
	handler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), nil))
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		request := httptest.NewRequest(method, "http://api.example.test/mcp", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Mcp-Session-Id", "legacy-session-id")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("stateless request with legacy session id status = %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Mcp-Session-Id") != "" {
		t.Fatal("stateless server echoed a legacy session id")
	}
}

func serveMCP(t *testing.T, handler http.Handler, body, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
