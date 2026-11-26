// SPDX-License-Identifier: MIT
package swagger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedtheme "github.com/apimgr/vidveil/src/common/theme"
	"github.com/apimgr/vidveil/src/config"
)

// TestDetectTheme covers all theme resolution paths: default, cookie, query param,
// invalid values, and cookie-over-query precedence.
func TestDetectTheme(t *testing.T) {
	tests := []struct {
		name       string
		cookie     string
		queryParam string
		wantTheme  string
	}{
		{
			name:      "no cookie no query returns auto",
			wantTheme: "auto",
		},
		{
			name:      "cookie light returns light",
			cookie:    "light",
			wantTheme: "light",
		},
		{
			name:      "cookie dark returns dark",
			cookie:    "dark",
			wantTheme: "dark",
		},
		{
			name:      "cookie invalid returns auto",
			cookie:    "invalid",
			wantTheme: "auto",
		},
		{
			name:       "query param light returns light",
			queryParam: "light",
			wantTheme:  "light",
		},
		{
			name:       "query param dark returns dark",
			queryParam: "dark",
			wantTheme:  "dark",
		},
		{
			name:       "query param invalid returns auto",
			queryParam: "invalid",
			wantTheme:  "auto",
		},
		{
			name:       "cookie takes precedence over query param",
			cookie:     "dark",
			queryParam: "light",
			wantTheme:  "dark",
		},
		{
			name:       "invalid cookie falls through to valid query param",
			cookie:     "bogus",
			queryParam: "light",
			wantTheme:  "light",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := "/"
			if tt.queryParam != "" {
				target = "/?theme=" + tt.queryParam
			}
			r := httptest.NewRequest(http.MethodGet, target, nil)
			if tt.cookie != "" {
				r.AddCookie(&http.Cookie{Name: "theme", Value: tt.cookie})
			}

			got := DetectTheme(r)
			if got != tt.wantTheme {
				t.Errorf("DetectTheme() = %q, want %q", got, tt.wantTheme)
			}
		})
	}
}

// TestDetectThemeNilCookieField ensures DetectTheme never panics on an empty URL.
func TestDetectThemeEmptyURL(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	got := DetectTheme(r)
	if got != "auto" {
		t.Errorf("DetectTheme with no cookies or params = %q, want %q", got, "auto")
	}
}

func TestThemeClassAndPaletteUseSharedThemeTokens(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ThemeClass(request); got != "theme-auto" {
		t.Fatalf("ThemeClass without preference = %q, want theme-auto", got)
	}
	if got := Palette(DetectTheme(request)); got != sharedtheme.Dark {
		t.Errorf("auto theme palette = %+v, want the shared dark default", got)
	}

	request.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	if got := ThemeClass(request); got != "theme-light" {
		t.Errorf("ThemeClass with light cookie = %q, want theme-light", got)
	}
	if got := Palette(DetectTheme(request)); got != sharedtheme.Light {
		t.Errorf("light theme palette = %+v, want the shared light palette", got)
	}
}

// TestGenerateSpecNilConfig covers the nil-config path: must return non-empty
// valid JSON containing the "openapi" key.
func TestGenerateSpecNilConfig(t *testing.T) {
	spec := GenerateSpec(nil, httptest.NewRequest("GET", "/", nil))

	if spec == "" {
		t.Fatal("GenerateSpec(nil) returned empty string")
	}

	if !json.Valid([]byte(spec)) {
		t.Fatalf("GenerateSpec(nil) returned invalid JSON:\n%s", spec)
	}

	if !strings.Contains(spec, "openapi") {
		t.Errorf("GenerateSpec(nil) missing \"openapi\" key")
	}
}

// TestGenerateSpecWithConfig covers the default-config path: must return valid JSON
// containing VidVeil branding, the "paths" key, and the two required API paths.
func TestGenerateSpecWithConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	spec := GenerateSpec(cfg, httptest.NewRequest("GET", "/", nil))

	if spec == "" {
		t.Fatal("GenerateSpec(DefaultAppConfig()) returned empty string")
	}

	if !json.Valid([]byte(spec)) {
		t.Fatalf("GenerateSpec(DefaultAppConfig()) returned invalid JSON:\n%s", spec)
	}

	for _, want := range []string{"openapi", "VidVeil", "paths"} {
		if !strings.Contains(spec, want) {
			t.Errorf("GenerateSpec(DefaultAppConfig()) missing %q", want)
		}
	}
}

// TestGenerateSpecContainsRequiredPaths verifies the spec documents both the
// /api/v1/search and /api/v1/engines routes regardless of config.
func TestGenerateSpecContainsRequiredPaths(t *testing.T) {
	paths := []string{
		"/api/v1/search",
		"/api/v1/engines",
		"/api/v1/server/preferences/tokens",
		"/api/v1/server/preferences/tokens/primary",
		"/api/v1/server/preferences/tokens/primary/reveal",
		"/api/v1/server/preferences/tokens/{id}",
		"/api/v1/server/help/mcp",
	}

	for _, cfg := range []*config.AppConfig{nil, config.DefaultAppConfig()} {
		label := "nil config"
		if cfg != nil {
			label = "default config"
		}

		spec := GenerateSpec(cfg, httptest.NewRequest("GET", "/", nil))
		for _, p := range paths {
			if !strings.Contains(spec, p) {
				t.Errorf("GenerateSpec(%s) missing path %q", label, p)
			}
		}
	}
}

func TestGenerateSpecDocumentsImplementedAPIOperations(t *testing.T) {
	var document struct {
		Paths map[string]map[string]map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(GenerateSpec(config.DefaultAppConfig(), httptest.NewRequest(http.MethodGet, "/", nil))), &document); err != nil {
		t.Fatalf("unmarshal OpenAPI document: %v", err)
	}
	operations := map[string]string{
		"get /api/v1/server/help/mcp":                "getServerHelpMCP",
		"get /api/v1/server/graphql":                 "executeGraphQLQuery",
		"post /api/v1/server/graphql":                "executeGraphQL",
		"get /api/v1/server/dpo":                     "getServerDPO",
		"get /api/v1/bangs":                          "listBangs",
		"get /api/v1/bangs/autocomplete":             "autocompleteBangs",
		"get /api/v1/engines/{name}":                 "getEngineDetails",
		"get /api/v1/stats":                          "getServerStats",
		"get /api/v1/version":                        "getServerVersion",
		"post /api/v1/server/contact":                "submitServerContact",
		"post /api/v1/server/reports/default":        "submitDefaultReport",
		"get /api/v1/server/preferences/sync":        "exportPreferences",
		"get /api/v1/server/preferences/sync/import": "importPreferences",
		"get /api/v1/proxy/thumbnails":               "proxyThumbnail",
		"get /api/v1/proxy/videos":                   "proxyVideo",
	}
	for endpoint, operationID := range operations {
		parts := strings.SplitN(endpoint, " ", 2)
		method, path := parts[0], parts[1]
		operation, ok := document.Paths[path][method]
		if !ok {
			t.Errorf("OpenAPI missing %s %s", method, path)
			continue
		}
		if got := operation["operationId"]; got != operationID {
			t.Errorf("OpenAPI %s %s operationId = %v, want %s", method, path, got, operationID)
		}
	}
}

func TestGenerateSpecDocumentsGraphQLGETParameters(t *testing.T) {
	var document struct {
		Paths map[string]map[string]map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(GenerateSpec(config.DefaultAppConfig(), httptest.NewRequest(http.MethodGet, "/", nil))), &document); err != nil {
		t.Fatalf("unmarshal OpenAPI document: %v", err)
	}
	operation := document.Paths["/api/v1/server/graphql"]["get"]
	parameters, ok := operation["parameters"].([]interface{})
	if !ok || len(parameters) != 3 {
		t.Fatalf("GraphQL GET parameters = %#v, want query, variables, operationName", operation["parameters"])
	}
	got := make(map[string]bool, len(parameters))
	for _, raw := range parameters {
		parameter := raw.(map[string]interface{})
		got[parameter["name"].(string)] = true
	}
	for _, name := range []string{"query", "variables", "operationName"} {
		if !got[name] {
			t.Errorf("GraphQL GET parameter %q is missing", name)
		}
	}
}

func TestGenerateSpecRequiresContactCaptchaResponse(t *testing.T) {
	var document struct {
		Paths map[string]map[string]map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(GenerateSpec(config.DefaultAppConfig(), httptest.NewRequest(http.MethodGet, "/", nil))), &document); err != nil {
		t.Fatalf("unmarshal OpenAPI document: %v", err)
	}
	operation := document.Paths["/api/v1/server/contact"]["post"]
	requestBody := operation["requestBody"].(map[string]interface{})
	content := requestBody["content"].(map[string]interface{})
	media := content["application/x-www-form-urlencoded"].(map[string]interface{})
	schema := media["schema"].(map[string]interface{})
	oneOf := schema["oneOf"].([]interface{})
	if len(oneOf) != 3 {
		t.Fatalf("contact CAPTCHA alternatives = %d, want 3", len(oneOf))
	}
}

// TestGenerateSpecOmitsAdminAPIRoutes ensures the spec does not publish
// nonexistent admin route or credential contracts.
func TestGenerateSpecOmitsAdminAPIRoutes(t *testing.T) {
	spec := GenerateSpec(config.DefaultAppConfig(), httptest.NewRequest("GET", "/", nil))
	for _, forbidden := range []string{"/server/admin", "X-Admin-Token", "AdminToken"} {
		if strings.Contains(spec, forbidden) {
			t.Errorf("GenerateSpec() unexpectedly contains %q", forbidden)
		}
	}
}

// TestGenerateSpecIsIdempotent verifies that calling GenerateSpec twice with the
// same config produces identical output — important for caching correctness.
func TestGenerateSpecIsIdempotent(t *testing.T) {
	cfg := config.DefaultAppConfig()
	req := httptest.NewRequest("GET", "/", nil)
	first := GenerateSpec(cfg, req)
	second := GenerateSpec(cfg, req)
	if first != second {
		t.Error("GenerateSpec is not idempotent: two calls with the same config produced different output")
	}
}

// TestSpecHandlerReturns200 verifies that SpecHandler returns HTTP 200 with an
// application/json Content-Type.
func TestSpecHandlerReturns200(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := SpecHandler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger", nil)
	rr := httptest.NewRecorder()
	h(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("SpecHandler() status = %d, want %d", rr.Code, http.StatusOK)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("SpecHandler() Content-Type = %q, want %q", ct, "application/json")
	}
}

// TestSpecHandlerBodyIsValidJSON verifies that the SpecHandler body is valid JSON.
func TestSpecHandlerBodyIsValidJSON(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := SpecHandler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger", nil)
	rr := httptest.NewRecorder()
	h(rr, r)

	body := rr.Body.Bytes()
	if !json.Valid(body) {
		t.Errorf("SpecHandler() body is not valid JSON:\n%s", body)
	}
}

// TestSpecHandlerBodyContainsOpenAPI verifies that the JSON body includes the
// "openapi" field so a misconfigured spec generation is caught immediately.
func TestSpecHandlerBodyContainsOpenAPI(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := SpecHandler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger", nil)
	rr := httptest.NewRecorder()
	h(rr, r)

	body := rr.Body.String()
	if !strings.Contains(body, "openapi") {
		t.Error("SpecHandler() body missing \"openapi\" field")
	}
}

// TestSpecHandlerIsIdempotent verifies that repeated requests to SpecHandler
// return identical bodies (the spec is generated once at construction time).
func TestSpecHandlerIsIdempotent(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := SpecHandler(cfg)

	makeBody := func() string {
		r := httptest.NewRequest(http.MethodGet, "/api/swagger", nil)
		rr := httptest.NewRecorder()
		h(rr, r)
		return rr.Body.String()
	}

	first := makeBody()
	second := makeBody()

	if first != second {
		t.Error("SpecHandler() returned different bodies on consecutive calls")
	}
}

// TestSpecHandlerNilConfig verifies that SpecHandler(nil) does not panic and
// returns a non-empty JSON response.
func TestSpecHandlerNilConfig(t *testing.T) {
	h := SpecHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger", nil)
	rr := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("SpecHandler(nil) panicked: %v", rec)
		}
	}()

	h(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("SpecHandler(nil) status = %d, want %d", rr.Code, http.StatusOK)
	}

	if !json.Valid(rr.Body.Bytes()) {
		t.Errorf("SpecHandler(nil) body is not valid JSON:\n%s", rr.Body.String())
	}
}
