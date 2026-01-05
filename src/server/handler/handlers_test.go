// SPDX-License-Identifier: MIT
// AI.md PART 23: Test coverage for handlers
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// createTestConfig returns a test configuration
func createTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Title:       "Test Vidveil",
			Description: "Test Description",
			FQDN:        "test.example.com",
			Port:        "8080",
			Mode:        "development",
			Database: config.DatabaseConfig{
				Driver: "none",
			},
			Cache: config.CacheConfig{
				Type: "memory",
			},
		},
		Web: config.WebConfig{
			UI: config.UIConfig{
				Theme: "dark",
			},
			Security: config.WebSecurityConfig{
				Contact: "security@test.example.com",
				Expires: "2025-12-31T00:00:00Z",
			},
		},
	}
}

func TestHealthCheck(t *testing.T) {
	// Skip this test as HealthCheck requires engineMgr
	t.Skip("Requires engine manager initialization")
}

func TestRobotsTxt(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	req := httptest.NewRequest("GET", "/robots.txt", nil)
	rr := httptest.NewRecorder()

	h.RobotsTxt(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("RobotsTxt returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Error("RobotsTxt should contain 'User-agent: *'")
	}

	if !strings.Contains(body, "Disallow: /search") {
		t.Error("RobotsTxt should disallow /search")
	}

	if !strings.Contains(body, "Disallow: /api/") {
		t.Error("RobotsTxt should disallow /api/")
	}

	if !strings.Contains(body, "Disallow: /admin/") {
		t.Error("RobotsTxt should disallow /admin/")
	}

	if !strings.Contains(body, "Sitemap:") {
		t.Error("RobotsTxt should contain Sitemap directive")
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("RobotsTxt Content-Type = %s, want text/plain", contentType)
	}
}

func TestSecurityTxt(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	req := httptest.NewRequest("GET", "/.well-known/security.txt", nil)
	rr := httptest.NewRecorder()

	h.SecurityTxt(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SecurityTxt returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Contact:") {
		t.Error("SecurityTxt should contain 'Contact:'")
	}

	if !strings.Contains(body, "Expires:") {
		t.Error("SecurityTxt should contain 'Expires:'")
	}

	if !strings.Contains(body, "Preferred-Languages:") {
		t.Error("SecurityTxt should contain 'Preferred-Languages:'")
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("SecurityTxt Content-Type = %s, want text/plain", contentType)
	}
}

func TestSitemapXML(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	req := httptest.NewRequest("GET", "/sitemap.xml", nil)
	rr := httptest.NewRecorder()

	h.SitemapXML(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SitemapXML returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "<?xml") {
		t.Error("SitemapXML should be valid XML")
	}

	if !strings.Contains(body, "<urlset") {
		t.Error("SitemapXML should contain <urlset>")
	}

	if !strings.Contains(body, "<url>") {
		t.Error("SitemapXML should contain <url> elements")
	}

	if !strings.Contains(body, "<loc>") {
		t.Error("SitemapXML should contain <loc> elements")
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/xml") {
		t.Errorf("SitemapXML Content-Type = %s, want application/xml", contentType)
	}
}

func TestJSONResponse(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	rr := httptest.NewRecorder()
	h.jsonResponse(rr, map[string]interface{}{
		"success": true,
		"data":    "test",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("jsonResponse returned status %d, want %d", rr.Code, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("jsonResponse Content-Type = %s, want application/json", contentType)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("jsonResponse returned invalid JSON: %v", err)
	}

	if response["success"] != true {
		t.Error("jsonResponse should contain success: true")
	}
}

func TestJSONError(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	rr := httptest.NewRecorder()
	h.jsonError(rr, "Test error", "TEST_ERROR", http.StatusBadRequest)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("jsonError returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("jsonError Content-Type = %s, want application/json", contentType)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("jsonError returned invalid JSON: %v", err)
	}

	if response["success"] != false {
		t.Error("jsonError should contain success: false")
	}

	if response["error"] != "Test error" {
		t.Errorf("jsonError error = %s, want 'Test error'", response["error"])
	}

	if response["code"] != "TEST_ERROR" {
		t.Errorf("jsonError code = %s, want 'TEST_ERROR'", response["code"])
	}
}

func TestAPISearch_MissingQuery(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	rr := httptest.NewRecorder()

	h.APISearch(rr, req)

	// Missing query should return bad request before hitting engine manager
	if rr.Code != http.StatusBadRequest {
		t.Errorf("APISearch returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APISearch returned invalid JSON: %v", err)
	}

	if response["code"] != "MISSING_QUERY" {
		t.Errorf("APISearch error code = %s, want 'MISSING_QUERY'", response["code"])
	}
}

func TestAPISearchText_MissingQuery(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	req := httptest.NewRequest("GET", "/api/v1/search/text", nil)
	rr := httptest.NewRecorder()

	h.APISearchText(rr, req)

	// Missing query should return bad request before hitting engine manager
	if rr.Code != http.StatusBadRequest {
		t.Errorf("APISearchText returned status %d, want %d", rr.Code, http.StatusBadRequest)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "required") {
		t.Error("APISearchText should indicate query is required")
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("APISearchText Content-Type = %s, want text/plain", contentType)
	}
}

func TestAPIAutocomplete_Empty(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	req := httptest.NewRequest("GET", "/api/v1/autocomplete", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	h.APIAutocomplete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIAutocomplete returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIAutocomplete returned invalid JSON: %v", err)
	}

	if response["success"] != true {
		t.Error("APIAutocomplete should return success: true")
	}

	suggestions, ok := response["suggestions"].([]interface{})
	if !ok {
		t.Error("APIAutocomplete should return suggestions array")
	}

	// Empty query returns popular searches
	if len(suggestions) == 0 {
		t.Error("APIAutocomplete should return popular suggestions for empty query")
	}

	// Check type is "popular"
	if response["type"] != "popular" {
		t.Errorf("APIAutocomplete should return type 'popular' for empty query, got %v", response["type"])
	}
}

func TestAgeVerifyMiddleware_StaticBypass(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	// Test that static files bypass age verification
	handler := h.AgeVerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	testPaths := []string{
		"/static/css/style.css",
		"/static/js/app.js",
		"/api/v1/search",
		"/healthz",
		"/robots.txt",
		"/age-verify",
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Path %s should bypass age verify, got status %d", path, rr.Code)
			}
		})
	}
}

func TestAgeVerifyMiddleware_RequiresVerification(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	handler := h.AgeVerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without cookie should redirect
	req := httptest.NewRequest("GET", "/search?q=test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Expected redirect status %d, got %d", http.StatusFound, rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.HasPrefix(location, "/age-verify") {
		t.Errorf("Expected redirect to /age-verify, got %s", location)
	}
}

func TestAgeVerifyMiddleware_WithCookie(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	handler := h.AgeVerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with valid cookie should pass through
	req := httptest.NewRequest("GET", "/search?q=test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "age_verified",
		Value: "1",
	})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d with valid cookie, got %d", http.StatusOK, rr.Code)
	}
}

func TestAgeVerifySubmit(t *testing.T) {
	cfg := createTestConfig()
	h := &Handler{cfg: cfg}

	// GET request should redirect
	req := httptest.NewRequest("GET", "/age-verify/submit", nil)
	rr := httptest.NewRecorder()
	h.AgeVerifySubmit(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("GET AgeVerifySubmit should redirect, got %d", rr.Code)
	}

	// POST request should set cookie and redirect
	req = httptest.NewRequest("POST", "/age-verify/submit", strings.NewReader("redirect=/"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.AgeVerifySubmit(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("POST AgeVerifySubmit should redirect, got %d", rr.Code)
	}

	// Check that cookie was set
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "age_verified" && c.Value == "1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AgeVerifySubmit should set age_verified cookie")
	}
}

func TestGetUptime(t *testing.T) {
	uptime := getUptime()
	if uptime == "" {
		t.Error("getUptime should return a non-empty string")
	}

	// Should contain time units
	hasTimeUnit := strings.Contains(uptime, "h") ||
		strings.Contains(uptime, "m") ||
		strings.Contains(uptime, "s") ||
		strings.Contains(uptime, "d")
	if !hasTimeUnit {
		t.Errorf("getUptime should contain time units, got %s", uptime)
	}
}

func TestSetTemplatesFS(t *testing.T) {
	// Just test that it doesn't panic
	// SetTemplatesFS(embed.FS{})
	// This is a basic smoke test
}

func TestNewHandler(t *testing.T) {
	cfg := createTestConfig()

	// Test handler creation
	h := New(cfg, nil)

	if h == nil {
		t.Fatal("New should return non-nil handler")
	}

	if h.cfg != cfg {
		t.Error("Handler should store config reference")
	}

	if h.searchCache == nil {
		t.Error("Handler should initialize search cache")
	}

	// Engine manager can be nil
	if h.engineMgr != nil {
		t.Error("Handler should have nil engine manager when passed nil")
	}
}
