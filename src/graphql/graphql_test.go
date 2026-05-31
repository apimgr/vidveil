// SPDX-License-Identifier: MIT
package graphql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// newTestHandler builds a Handler with a real (empty) EngineManager so that
// handleHealth and handleEngines don't panic on nil receiver.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	return NewHandler(cfg, mgr)
}

// --- NewHandler ---

// TestNewHandler_NonNil verifies the constructor always returns a non-nil value.
func TestNewHandler_NonNil(t *testing.T) {
	h := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

// --- writeJSON (unexported, tested indirectly through Handle) ---

// TestWriteJSON_200 verifies a 200 response is encoded as JSON with the correct status.
func TestWriteJSON_200(t *testing.T) {
	w := httptest.NewRecorder()
	type payload struct {
		Name string `json:"name"`
	}
	writeJSON(w, http.StatusOK, payload{Name: "test"})

	if w.Code != http.StatusOK {
		t.Errorf("writeJSON 200: status = %d, want 200", w.Code)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("writeJSON 200: body not valid JSON: %v", err)
	}
	if got["name"] != "test" {
		t.Errorf("writeJSON 200: body[name] = %v, want 'test'", got["name"])
	}
}

// TestWriteJSON_400 verifies a 400 error response contains the "message" field.
func TestWriteJSON_400(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusBadRequest, Response{
		Errors: []Error{{Message: "something broke"}},
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("writeJSON 400: status = %d, want 400", w.Code)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("writeJSON 400: body not valid JSON: %v", err)
	}
	errs, ok := got["errors"].([]interface{})
	if !ok || len(errs) == 0 {
		t.Fatalf("writeJSON 400: errors field missing or empty: %v", got)
	}
	first, ok := errs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("writeJSON 400: first error not an object: %v", errs[0])
	}
	if first["message"] != "something broke" {
		t.Errorf("writeJSON 400: message = %v, want 'something broke'", first["message"])
	}
}

// --- Schema ---

// TestSchema_200 verifies Schema returns 200, correct Content-Type, and schema text.
func TestSchema_200(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql/schema", nil)
	w := httptest.NewRecorder()

	h.Schema(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Schema: status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Schema: Content-Type = %q, want text/plain", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "type Query") {
		t.Errorf("Schema: body missing 'type Query', got: %s", body)
	}
}

// TestSchema_ContainsTypes verifies the schema body lists key types.
func TestSchema_ContainsTypes(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql/schema", nil)
	w := httptest.NewRecorder()

	h.Schema(w, r)

	body := w.Body.String()
	for _, want := range []string{"SearchResult", "Health", "Engine", "Bang"} {
		if !strings.Contains(body, want) {
			t.Errorf("Schema: body missing type %q", want)
		}
	}
}

// --- GraphiQL ---

// TestGraphiQL_GET verifies the GET path returns 200, text/html, and GraphQL branding.
func TestGraphiQL_GET(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql/ui", nil)
	w := httptest.NewRecorder()

	h.GraphiQL(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("GraphiQL GET: status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("GraphiQL GET: Content-Type = %q, want text/html", ct)
	}
	body := w.Body.String()
	if !strings.Contains(strings.ToLower(body), "graphql") {
		t.Errorf("GraphiQL GET: body does not mention 'graphql': %s", body[:200])
	}
}

// TestGraphiQL_POST_EmptyQuery verifies a POST with no query still returns 200 HTML.
func TestGraphiQL_POST_EmptyQuery(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodPost, "/graphql/ui", strings.NewReader("query="))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.GraphiQL(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("GraphiQL POST empty: status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("GraphiQL POST empty: Content-Type should be text/html")
	}
}

// --- Handle: unsupported methods ---

// TestHandle_UnsupportedMethod verifies that PUT/DELETE/PATCH receive 405.
func TestHandle_UnsupportedMethod(t *testing.T) {
	h := newTestHandler(t)
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/graphql", nil)
			w := httptest.NewRecorder()

			h.Handle(w, r)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Handle %s: status = %d, want 405", method, w.Code)
			}
			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Handle %s: body not valid JSON: %v", method, err)
			}
			if len(resp.Errors) == 0 {
				t.Errorf("Handle %s: expected errors in response", method)
			}
			if resp.Errors[0].Message != "Method not allowed" {
				t.Errorf("Handle %s: error message = %q, want 'Method not allowed'", method, resp.Errors[0].Message)
			}
		})
	}
}

// --- Handle: POST with invalid JSON ---

// TestHandle_POST_InvalidJSON verifies that malformed JSON returns 400 with an error.
func TestHandle_POST_InvalidJSON(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader("{invalid}"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Handle POST invalid JSON: status = %d, want 400", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle POST invalid JSON: body not valid JSON: %v", err)
	}
	if len(resp.Errors) == 0 {
		t.Fatalf("Handle POST invalid JSON: expected at least one error")
	}
	if resp.Errors[0].Message != "Invalid request body" {
		t.Errorf("Handle POST invalid JSON: error = %q, want 'Invalid request body'", resp.Errors[0].Message)
	}
}

// --- Handle: GET with empty query ---

// TestHandle_GET_EmptyQuery verifies that an empty query returns a well-formed JSON response.
func TestHandle_GET_EmptyQuery(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle GET empty query: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle GET empty query: body not valid JSON: %v", err)
	}
	// Empty query hits the "Unknown query" branch
	if len(resp.Errors) == 0 {
		t.Errorf("Handle GET empty query: expected errors field for unknown query")
	}
	if resp.Errors[0].Message != "Unknown query" {
		t.Errorf("Handle GET empty query: error = %q, want 'Unknown query'", resp.Errors[0].Message)
	}
}

// --- handleHealth (via GET query string) ---

// TestHandle_HealthQuery verifies the health resolver returns a "status" field.
func TestHandle_HealthQuery(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql?query={health{status}}", nil)
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle health query: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle health query: body not valid JSON: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Handle health query: data is nil")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Handle health query: data not a map: %T", resp.Data)
	}
	health, ok := data["health"].(map[string]interface{})
	if !ok {
		t.Fatalf("Handle health query: data[health] not a map: %v", data["health"])
	}
	if health["status"] != "ok" {
		t.Errorf("Handle health query: status = %v, want 'ok'", health["status"])
	}
}

// TestHandleHealth_Direct calls handleHealth directly to verify its structure.
func TestHandleHealth_Direct(t *testing.T) {
	h := newTestHandler(t)
	resp := h.handleHealth()

	if resp.Data == nil {
		t.Fatal("handleHealth: data is nil")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("handleHealth: data not a map: %T", resp.Data)
	}
	health, ok := data["health"].(map[string]interface{})
	if !ok {
		t.Fatalf("handleHealth: data[health] not a map: %v", data["health"])
	}
	if health["status"] != "ok" {
		t.Errorf("handleHealth: status = %v, want 'ok'", health["status"])
	}
	if _, exists := health["enginesEnabled"]; !exists {
		t.Error("handleHealth: missing 'enginesEnabled' key")
	}
}

// --- getSchema ---

// TestGetSchema_RequiredKeys verifies getSchema returns the expected top-level keys.
func TestGetSchema_RequiredKeys(t *testing.T) {
	h := newTestHandler(t)
	schema := h.getSchema()

	if _, ok := schema["queryType"]; !ok {
		t.Error("getSchema: missing 'queryType' key")
	}
	if _, ok := schema["types"]; !ok {
		t.Error("getSchema: missing 'types' key")
	}
}

// TestGetSchema_QueryTypeName verifies the queryType name is "Query".
func TestGetSchema_QueryTypeName(t *testing.T) {
	h := newTestHandler(t)
	schema := h.getSchema()

	qt, ok := schema["queryType"].(map[string]interface{})
	if !ok {
		t.Fatalf("getSchema: queryType not a map: %T", schema["queryType"])
	}
	if qt["name"] != "Query" {
		t.Errorf("getSchema: queryType.name = %v, want 'Query'", qt["name"])
	}
}

// TestGetSchema_TypesNonEmpty verifies the types slice has entries.
func TestGetSchema_TypesNonEmpty(t *testing.T) {
	h := newTestHandler(t)
	schema := h.getSchema()

	types, ok := schema["types"].([]map[string]interface{})
	if !ok {
		t.Fatalf("getSchema: types not []map[string]interface{}: %T", schema["types"])
	}
	if len(types) == 0 {
		t.Error("getSchema: types is empty")
	}
}

// --- handleIntrospection ---

// TestHandleIntrospection_Schema verifies __schema queries return a schema object.
func TestHandleIntrospection_Schema(t *testing.T) {
	h := newTestHandler(t)
	resp := h.handleIntrospection("{ __schema { queryType { name } } }")

	if resp.Data == nil {
		t.Fatal("handleIntrospection __schema: data is nil")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("handleIntrospection __schema: data not a map: %T", resp.Data)
	}
	if _, exists := data["__schema"]; !exists {
		t.Error("handleIntrospection __schema: missing '__schema' key in data")
	}
}

// TestHandleIntrospection_Type verifies __type queries return a __type key.
func TestHandleIntrospection_Type(t *testing.T) {
	h := newTestHandler(t)
	resp := h.handleIntrospection("{ __type(name: \"Query\") { name } }")

	if resp.Data == nil {
		t.Fatal("handleIntrospection __type: data is nil")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("handleIntrospection __type: data not a map: %T", resp.Data)
	}
	if _, exists := data["__type"]; !exists {
		t.Error("handleIntrospection __type: missing '__type' key in data")
	}
}

// --- Handle: introspection via HTTP ---

// TestHandle_GET_IntrospectionSchema verifies a full round-trip for __schema via GET.
func TestHandle_GET_IntrospectionSchema(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql?query={__schema{queryType{name}}}", nil)
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle __schema: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle __schema: body not valid JSON: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Handle __schema: data is nil")
	}
	data := resp.Data.(map[string]interface{})
	if _, ok := data["__schema"]; !ok {
		t.Error("Handle __schema: '__schema' key missing in data")
	}
}

// --- Handle: engines via HTTP ---

// TestHandle_GET_EnginesQuery verifies the engines resolver returns the "engines" key.
func TestHandle_GET_EnginesQuery(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql?query={engines{name}}", nil)
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle engines: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle engines: body not valid JSON: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Handle engines: data is nil")
	}
	data := resp.Data.(map[string]interface{})
	if _, ok := data["engines"]; !ok {
		t.Error("Handle engines: 'engines' key missing in data")
	}
}

// --- Handle: bangs via HTTP ---

// TestHandle_GET_BangsQuery verifies the bangs resolver returns the "bangs" key.
func TestHandle_GET_BangsQuery(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql?query={bangs{bang}}", nil)
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle bangs: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle bangs: body not valid JSON: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Handle bangs: data is nil")
	}
	data := resp.Data.(map[string]interface{})
	if _, ok := data["bangs"]; !ok {
		t.Error("Handle bangs: 'bangs' key missing in data")
	}
}

// --- Handle: autocomplete with empty prefix ---

// TestHandle_GET_AutocompleteEmptyPrefix verifies autocomplete with no prefix returns an empty list.
func TestHandle_GET_AutocompleteEmptyPrefix(t *testing.T) {
	h := newTestHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/graphql?query={autocomplete(prefix:\"\"){bang}}", nil)
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle autocomplete empty: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle autocomplete empty: body not valid JSON: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Handle autocomplete empty: data is nil")
	}
	data := resp.Data.(map[string]interface{})
	if _, ok := data["autocomplete"]; !ok {
		t.Error("Handle autocomplete empty: 'autocomplete' key missing in data")
	}
}

// --- Handle: search missing query variable ---

// TestHandle_POST_SearchMissingQuery verifies search without a query variable returns an error.
func TestHandle_POST_SearchMissingQuery(t *testing.T) {
	h := newTestHandler(t)
	body := `{"query":"{ search { query } }"}`
	r := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Handle search no variable: status = %d, want 200", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Handle search no variable: body not valid JSON: %v", err)
	}
	if len(resp.Errors) == 0 {
		t.Error("Handle search no variable: expected errors for missing search query")
	}
	if resp.Errors[0].Message != "Missing search query" {
		t.Errorf("Handle search no variable: error = %q, want 'Missing search query'", resp.Errors[0].Message)
	}
}

// --- Content-Type on Handle ---

// TestHandle_ContentType verifies Handle always sets application/json.
func TestHandle_ContentType(t *testing.T) {
	h := newTestHandler(t)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			var r *http.Request
			if method == http.MethodPost {
				r = httptest.NewRequest(method, "/graphql", strings.NewReader(`{}`))
				r.Header.Set("Content-Type", "application/json")
			} else {
				r = httptest.NewRequest(method, "/graphql", nil)
			}
			w := httptest.NewRecorder()
			h.Handle(w, r)
			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("Handle %s: Content-Type = %q, want application/json", method, ct)
			}
		})
	}
}
