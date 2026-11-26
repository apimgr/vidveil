// SPDX-License-Identifier: MIT
package graphql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sharedtheme "github.com/apimgr/vidveil/src/common/theme"
)

func TestHandleGETSupportsQueryVariablesAndOperationName(t *testing.T) {
	h := NewHandler(nil, nil)
	values := url.Values{
		"query":         {"query First { health { status } } query Second { engines { name } }"},
		"variables":     {"{}"},
		"operationName": {"First"},
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/server/graphql?"+values.Encode(), nil)
	w := httptest.NewRecorder()
	h.Handle(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response["data"].(map[string]any)
	if !ok || data["health"].(map[string]any)["status"] != "ok" {
		t.Fatalf("response data = %#v, want only selected health operation", response["data"])
	}
}

func TestSearchBatchQueryDelegatesValidatedInputs(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetReadOperation("searchBatch", func(request *http.Request, args map[string]any) (string, error) {
		if request.Method != http.MethodPost {
			t.Errorf("captured method = %q, want POST from batch adapter", request.Method)
		}
		queries, ok := args["queries"].([]map[string]any)
		if !ok || len(queries) != 1 || queries[0]["q"] != "fox" || queries[0]["page"] != 1 {
			t.Errorf("batch args = %#v", args)
		}
		return `{"ok":true,"data":[]}`, nil
	})
	body, _ := json.Marshal(Request{
		Query:     `query Batch($queries: [BatchQueryInput!]!) { searchBatch(queries: $queries) { json } }`,
		Variables: map[string]any{"queries": []map[string]any{{"q": "fox", "page": 1}}},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/graphql", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.Handle(w, request)
	var response struct {
		Data struct {
			SearchBatch struct {
				JSON string `json:"json"`
			} `json:"searchBatch"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode batch result: %v", err)
	}
	if w.Code != http.StatusOK || response.Data.SearchBatch.JSON != `{"ok":true,"data":[]}` {
		t.Fatalf("batch result = %d %s", w.Code, w.Body.String())
	}
}

func TestHandlePOSTAcceptsNullVariablesAsOmitted(t *testing.T) {
	h := NewHandler(nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/graphql", strings.NewReader(`{"query":"{ health { status } }","variables":null}`))
	response := httptest.NewRecorder()
	h.Handle(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status": "ok"`) {
		t.Fatalf("null variables response = %d %s", response.Code, response.Body.String())
	}
}

func TestHandleExecutesAndValidatesGraphQL(t *testing.T) {
	h := NewHandler(nil, nil)
	cases := []struct {
		name, query string
		wantError   bool
	}{
		{name: "valid introspection", query: `{ __schema { queryType { name } } }`},
		{name: "unknown field", query: `{ notAField }`, wantError: true},
		{name: "invalid syntax", query: `{ health { status `, wantError: true},
		{name: "mutation is rejected by query-only schema", query: `mutation { health { status } }`, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(Request{Query: tc.query})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/server/graphql", strings.NewReader(string(body)))
			w := httptest.NewRecorder()
			h.Handle(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want GraphQL HTTP 200: %s", w.Code, w.Body.String())
			}
			var response struct {
				Errors []struct {
					Extensions map[string]any `json:"extensions"`
				} `json:"errors"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if gotError := len(response.Errors) > 0; gotError != tc.wantError {
				t.Fatalf("has errors = %v, want %v: %s", gotError, tc.wantError, w.Body.String())
			}
			for _, graphErr := range response.Errors {
				if graphErr.Extensions["code"] == nil {
					t.Fatalf("GraphQL error missing extensions.code: %s", w.Body.String())
				}
			}
		})
	}
}

func TestHandlePreExecutionErrorsUseCanonicalEnvelope(t *testing.T) {
	h := NewHandler(nil, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/server/graphql", strings.NewReader("{"))
	w := httptest.NewRecorder()
	h.Handle(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var response apiError
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode canonical error: %v", err)
	}
	if response.OK || response.Error != "BAD_REQUEST" || response.Message == "" {
		t.Fatalf("error response = %#v", response)
	}
}

func TestHandleGETWithoutQueryShowsUsage(t *testing.T) {
	h := NewHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/server/graphql", nil)
	w := httptest.NewRecorder()
	h.Handle(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Supply a query parameter") {
		t.Fatalf("GET missing-query response = %d %q", w.Code, w.Body.String())
	}
}

func TestHandleRejectsMalformedGETVariables(t *testing.T) {
	h := NewHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/server/graphql?query=%7Bhealth%7Bstatus%7D%7D&variables=%5B%5D", nil)
	w := httptest.NewRecorder()
	h.Handle(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestThemeUsesSharedPaletteAndClasses(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ThemeClass(request); got != "theme-auto" {
		t.Fatalf("ThemeClass() = %q, want theme-auto", got)
	}
	if got := Palette(DetectTheme(request)); got != sharedtheme.Dark {
		t.Errorf("auto palette = %+v, want shared dark palette", got)
	}
	request.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	if got := ThemeClass(request); got != "theme-light" {
		t.Errorf("ThemeClass(light) = %q", got)
	}
	if got := Palette(DetectTheme(request)); got != sharedtheme.Light {
		t.Errorf("light palette = %+v, want shared light palette", got)
	}
}
