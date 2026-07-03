// SPDX-License-Identifier: MIT
package swagger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// TestGenerateSpecNilConfig covers the nil-config path: must return non-empty
// valid JSON containing the "openapi" key.
func TestGenerateSpecNilConfig(t *testing.T) {
	spec := GenerateSpec(nil)

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
	spec := GenerateSpec(cfg)

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
	}

	for _, cfg := range []*config.AppConfig{nil, config.DefaultAppConfig()} {
		label := "nil config"
		if cfg != nil {
			label = "default config"
		}

		spec := GenerateSpec(cfg)
		for _, p := range paths {
			if !strings.Contains(spec, p) {
				t.Errorf("GenerateSpec(%s) missing path %q", label, p)
			}
		}
	}
}

// TestGenerateSpecAdminPathDefault verifies that a nil config uses "server/admin"
// as the admin API path in the generated spec.
func TestGenerateSpecAdminPathDefault(t *testing.T) {
	spec := GenerateSpec(nil)
	if !strings.Contains(spec, "server/admin") {
		t.Errorf("GenerateSpec(nil) missing default admin path \"server/admin\"")
	}
}

// TestGenerateSpecAdminPathCustom verifies that a custom admin path is reflected in
// the generated spec when provided via AppConfig.
func TestGenerateSpecAdminPathCustom(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Admin.Path = "myadmin"

	spec := GenerateSpec(cfg)
	if !strings.Contains(spec, "server/myadmin") {
		t.Errorf("GenerateSpec with custom admin path: missing \"server/myadmin\" in spec")
	}
}

// TestGenerateSpecIsIdempotent verifies that calling GenerateSpec twice with the
// same config produces identical output — important for caching correctness.
func TestGenerateSpecIsIdempotent(t *testing.T) {
	cfg := config.DefaultAppConfig()
	first := GenerateSpec(cfg)
	second := GenerateSpec(cfg)
	if first != second {
		t.Error("GenerateSpec is not idempotent: two calls with the same config produced different output")
	}
}

// TestHandlerReturns200 verifies that Handler returns HTTP 200 with a text/html
// Content-Type for a basic GET request.
func TestHandlerReturns200(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := Handler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger/ui", nil)
	rr := httptest.NewRecorder()
	h(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("Handler() status = %d, want %d", rr.Code, http.StatusOK)
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Handler() Content-Type = %q, want text/html", ct)
	}
}

// TestHandlerBodyContainsVidVeil verifies the rendered HTML mentions VidVeil so
// that a broken template substitution is caught.
func TestHandlerBodyContainsVidVeil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := Handler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger/ui", nil)
	rr := httptest.NewRecorder()
	h(rr, r)

	body := rr.Body.String()
	if !strings.Contains(body, "VidVeil") {
		t.Error("Handler() body does not contain \"VidVeil\"")
	}
}

// TestHandlerThemeFromCookie verifies that the theme cookie is respected and
// the light-theme CSS variables appear in the rendered page.
func TestHandlerThemeFromCookie(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := Handler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger/ui", nil)
	r.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	rr := httptest.NewRecorder()
	h(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("Handler() with light theme cookie: status = %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	// Light theme uses #f5f5f5 as background; absence means theme was ignored.
	if !strings.Contains(body, "#f5f5f5") {
		t.Error("Handler() with light theme cookie: light-theme CSS variable not found in body")
	}
}

// TestHandlerDarkThemeDefault verifies that without a cookie the dark-theme
// CSS variables are rendered (dark is the project default).
func TestHandlerDarkThemeDefault(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := Handler(cfg)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger/ui", nil)
	rr := httptest.NewRecorder()
	h(rr, r)

	body := rr.Body.String()
	// Dark theme uses #1a1a2e as background.
	if !strings.Contains(body, "#1a1a2e") {
		t.Error("Handler() without cookie: dark-theme CSS variable not found in body")
	}
}

// TestHandlerNilConfig verifies that Handler does not panic when passed a nil
// AppConfig (GenerateSpec handles nil gracefully).
func TestHandlerNilConfig(t *testing.T) {
	h := Handler(nil)

	r := httptest.NewRequest(http.MethodGet, "/api/swagger/ui", nil)
	rr := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("Handler(nil) panicked: %v", rec)
		}
	}()

	h(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("Handler(nil) status = %d, want %d", rr.Code, http.StatusOK)
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
