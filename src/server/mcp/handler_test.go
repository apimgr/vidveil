// SPDX-License-Identifier: MIT
package mcp

import (
	"encoding/json"
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
	}), nil, validTestBearer))

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
	for _, operation := range []string{"getOperatorStatus", "debugEngines", "debugEngine", "getPreferences", "savePreferences", "exportPreferences", "importPreferences", "listOwnerTokens", "createLinkedToken", "executeMCP", "OwnerToken", "LinkedToken"} {
		if strings.Contains(body, operation) {
			t.Fatalf("tools/list exposed a protected or cookie-bound preferences operation %q: %s", operation, body)
		}
	}
	if response.Header().Get("Mcp-Session-Id") != "" {
		t.Fatal("stateless MCP response minted a session id")
	}
	operatorHandler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer operator-token" }, validTestBearer))
	operatorRequest := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`))
	setMCPHeaders(operatorRequest, `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	operatorRequest.Header.Set("Authorization", "Bearer operator-token")
	operatorResponse := httptest.NewRecorder()
	operatorHandler.ServeHTTP(operatorResponse, operatorRequest)
	if !strings.Contains(operatorResponse.Body.String(), "getOperatorStatus") || strings.Contains(operatorResponse.Body.String(), "restoreServerBackup") || strings.Contains(operatorResponse.Body.String(), "debugEngines") || strings.Contains(operatorResponse.Body.String(), "debugEngine") {
		t.Fatalf("operator tools/list has the wrong operator-only tools: %s", operatorResponse.Body.String())
	}

	debugCfg := config.DefaultAppConfig()
	debugCfg.Server.Mode = "debug"
	debugOwnerHandler := OriginMiddleware(debugCfg, Handler(debugCfg, http.NotFoundHandler(), nil, validTestBearer))
	debugOwnerResponse := serveMCP(t, debugOwnerHandler, `{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}`, "")
	for _, operation := range []string{"getOperatorStatus", "debugEngines", "debugEngine"} {
		if strings.Contains(debugOwnerResponse.Body.String(), operation) {
			t.Fatalf("owner tools/list exposed operator-only operation %q: %s", operation, debugOwnerResponse.Body.String())
		}
	}
	debugOperatorHandler := OriginMiddleware(debugCfg, Handler(debugCfg, http.NotFoundHandler(), func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer operator-token" }, validTestBearer))
	debugOperatorRequest := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{}}`))
	setMCPHeaders(debugOperatorRequest, `{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{}}`)
	debugOperatorRequest.Header.Set("Authorization", "Bearer operator-token")
	debugOperatorResponse := httptest.NewRecorder()
	debugOperatorHandler.ServeHTTP(debugOperatorResponse, debugOperatorRequest)
	if !strings.Contains(debugOperatorResponse.Body.String(), "getOperatorStatus") || strings.Contains(debugOperatorResponse.Body.String(), "debugEngines") || strings.Contains(debugOperatorResponse.Body.String(), "debugEngine") {
		t.Fatalf("debug routes were exposed in MCP or operator status was missing: %s", debugOperatorResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"searchVideos","arguments":{"q":"needle"}}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"searchVideos","arguments":{"q":"needle"}}}`)
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

func TestHandlerFiltersAndRejectsToolsOutsideTokenScope(t *testing.T) {
	cfg := config.DefaultAppConfig()
	handler := Handler(cfg, http.NotFoundHandler(), nil, validTestBearer, func(_ *http.Request, method string) bool {
		return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
	})
	listBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	listed := serveMCP(t, handler, listBody, "")
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), `"name":"submitServerContact"`) || !strings.Contains(listed.Body.String(), `"name":"searchVideos"`) {
		t.Fatalf("read-scoped tools/list did not filter mutation tools: %d %s", listed.Code, listed.Body.String())
	}
	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submitServerContact","arguments":{}}}`
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(callBody))
	setMCPHeaders(request, callBody)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || response.Header().Get("WWW-Authenticate") != `Bearer error="insufficient_scope"` {
		t.Fatalf("read-scoped mutating tool response = %d %s", response.Code, response.Body.String())
	}
}

func TestHandlerOriginValidation(t *testing.T) {
	cfg := config.DefaultAppConfig()
	handler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), nil, validTestBearer))
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d, want 403", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	request.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("MCP without a foreign origin status = %d, want 200: %s", response.Code, response.Body.String())
	}
}

func TestHandlerRejectsSessionAndStreamingMethods(t *testing.T) {
	cfg := config.DefaultAppConfig()
	handler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), nil, validTestBearer))
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		request := httptest.NewRequest(method, "http://api.example.test/mcp", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	request.Header.Set("Authorization", "Bearer test-token")
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

func TestHandlerMCPValidationContract(t *testing.T) {
	cfg := config.DefaultAppConfig()
	handler := OriginMiddleware(cfg, Handler(cfg, http.NotFoundHandler(), nil, validTestBearer))
	legacy := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"initialize"}`))
	legacy.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, legacy)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "UnsupportedProtocolVersionError") || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("legacy initialize response = %d %s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/list","params":{}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":8,"method":"tools/list","params":{}}`)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Del("MCP-Protocol-Version")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `-32020`) {
		t.Fatalf("missing protocol header response = %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":12,"method":"tools/list","params":{}}`))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Mcp-Method", "tools/list")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `-32020`) {
		t.Fatalf("missing protocol header and body metadata response = %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"tools/list","params":{}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":9,"method":"tools/list","params":{}}`)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" || !strings.Contains(response.Body.String(), `"error"`) {
		t.Fatalf("unauthenticated response = %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":10,"method":"tools/list","params":{}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":10,"method":"tools/list","params":{}}`)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Mcp-Method", "ping")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `-32020`) {
		t.Fatalf("mismatched method header response = %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":11,"method":"made/up","params":{}}`))
	setMCPHeaders(request, `{"jsonrpc":"2.0","id":11,"method":"made/up","params":{}}`)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Mcp-Method", "made/up")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `-32601`) {
		t.Fatalf("unknown method response = %d %s", response.Code, response.Body.String())
	}
}

func TestToolAnnotationsFollowHTTPMethod(t *testing.T) {
	get := toolAnnotations(http.MethodGet)
	if !get.ReadOnlyHint || get.IdempotentHint || get.DestructiveHint != nil {
		t.Errorf("GET annotations = %#v", get)
	}
	post := toolAnnotations(http.MethodPost)
	if post.ReadOnlyHint || post.IdempotentHint || post.DestructiveHint != nil {
		t.Errorf("POST annotations = %#v", post)
	}
	put := toolAnnotations(http.MethodPut)
	if put.ReadOnlyHint || !put.IdempotentHint || put.DestructiveHint != nil {
		t.Errorf("PUT annotations = %#v", put)
	}
	delete := toolAnnotations(http.MethodDelete)
	if delete.ReadOnlyHint || !delete.IdempotentHint || delete.DestructiveHint == nil || !*delete.DestructiveHint {
		t.Errorf("DELETE annotations = %#v", delete)
	}
}

func serveMCP(t *testing.T, handler http.Handler, body, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/mcp", strings.NewReader(body))
	setMCPHeaders(request, body)
	request.Header.Set("Authorization", "Bearer test-token")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func validTestBearer(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer test-token" || r.Header.Get("Authorization") == "Bearer operator-token"
}

func setMCPHeaders(request *http.Request, body string) {
	var envelope map[string]any
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		request.Body = io.NopCloser(strings.NewReader(body))
		return
	}
	params, ok := envelope["params"].(map[string]any)
	if !ok {
		params = make(map[string]any)
		envelope["params"] = params
	}
	params["_meta"] = map[string]any{
		"io.modelcontextprotocol/clientCapabilities": map[string]any{"sampling": map[string]any{}},
		"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
	}
	encodedBody, err := json.Marshal(envelope)
	if err == nil {
		body = string(encodedBody)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2026-07-28")
	method, _ := envelope["method"].(string)
	request.Header.Set("Mcp-Method", method)
	if method == "tools/call" {
		if name, ok := params["name"].(string); ok {
			request.Header.Set("Mcp-Name", name)
		}
	}
	request.Body = io.NopCloser(strings.NewReader(body))
}
