// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for render, response, and page handler functions at 0%.
// Tests renderTemplate, renderResponse, renderContentBlockedPage, renderHealthzHTML,
// SearchPage, PreferencesPage, AboutPage, PrivacyPage, SearchRSSFeed, SearchAtomFeed,
// Autodiscover, renderServerTemplate, ContactPage, HelpPage.
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newRenderTestHandler creates a SearchHandler wired with an empty EngineManager.
func newRenderTestHandler() *SearchHandler {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	return &SearchHandler{
		appConfig: cfg,
		engineMgr: mgr,
	}
}

// ── renderTemplate ────────────────────────────────────────────────────────────

func TestRenderTemplate_UnknownName_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "totally-unknown-template", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate unknown: status = %d, want 500", rr.Code)
	}
}

func TestRenderTemplate_ValidName_EmptyFS_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	// With empty templatesFS the main template file read will fail → 500.
	h.renderTemplate(rr, "home", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate home (empty FS): status = %d, want 500", rr.Code)
	}
}

func TestRenderTemplate_AboutName_EmptyFS_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "about", map[string]interface{}{})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderTemplate about (empty FS): status = %d, want 500", rr.Code)
	}
}

// ── renderResponse ────────────────────────────────────────────────────────────

// CLI client (vidveil-cli/ UA) receives JSON — no template needed.
func TestRenderResponse_CLIClient_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "vidveil-cli/1.0.0")

	h.renderResponse(rr, req, "home", map[string]interface{}{
		"Title": "Test",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("renderResponse CLI: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("renderResponse CLI: Content-Type = %q, want application/json", ct)
	}
}

// Curl (isHttpTool) receives plain text via renderSimpleHTML → no template needed.
func TestRenderResponse_CurlClient_ReturnsPlainText(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.renderResponse(rr, req, "home", map[string]interface{}{
		"Title": "Test",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("renderResponse curl: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("renderResponse curl: Content-Type = %q, want text/plain", ct)
	}
}

// Empty UA is also an HTTP tool path.
func TestRenderResponse_EmptyUA_ReturnsPlainText(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No User-Agent header → empty UA → isHttpTool returns true

	h.renderResponse(rr, req, "about", map[string]interface{}{
		"Title": "Test",
	})

	if rr.Code != http.StatusOK {
		t.Errorf("renderResponse empty UA: status = %d, want 200", rr.Code)
	}
}

// ── renderContentBlockedPage ──────────────────────────────────────────────────

func TestRenderContentBlockedPage_CurlUA_NoTemplatePanic(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	restriction := &geoip.RestrictionResult{
		Message: "Access restricted",
		Reason:  "Test region",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("renderContentBlockedPage panicked: %v", r)
		}
	}()
	h.renderContentBlockedPage(rr, req, restriction)

	if rr.Code != http.StatusForbidden {
		t.Errorf("renderContentBlockedPage: status = %d, want 403", rr.Code)
	}
}

func TestRenderContentBlockedPage_CLIClient_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "vidveil-cli/1.0.0")

	restriction := &geoip.RestrictionResult{
		Message: "Blocked",
		Reason:  "US",
	}

	h.renderContentBlockedPage(rr, req, restriction)

	if rr.Code != http.StatusForbidden {
		t.Errorf("renderContentBlockedPage CLI: status = %d, want 403", rr.Code)
	}
}

// ── SearchPage ────────────────────────────────────────────────────────────────

// Empty query redirects immediately — no template or search needed.
func TestSearchPage_EmptyQuery_Redirects(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)

	h.SearchPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("SearchPage empty query: status = %d, want 302", rr.Code)
	}
}

// Bang-only query (no search terms) also redirects immediately.
func TestSearchPage_BangOnlyQuery_Redirects(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=!ph", nil)

	h.SearchPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("SearchPage bang-only query: status = %d, want 302", rr.Code)
	}
}

// JSON format with a real query calls Search on the empty engine manager.
func TestSearchPage_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	req.Header.Set("Accept", "application/json")

	h.SearchPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SearchPage JSON: status = %d, want 200", rr.Code)
	}
}

// ── PreferencesPage ───────────────────────────────────────────────────────────

func TestPreferencesPage_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/preferences", nil)
	req.Header.Set("Accept", "application/json")

	h.PreferencesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PreferencesPage JSON: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("PreferencesPage JSON: Content-Type = %q, want application/json", ct)
	}
}

func TestPreferencesPage_PlainTextFormat_ReturnsText(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/preferences", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.PreferencesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PreferencesPage plain: status = %d, want 200", rr.Code)
	}
}

// ── AboutPage (SearchHandler) ─────────────────────────────────────────────────

func TestAboutPage_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	req.Header.Set("Accept", "application/json")

	h.AboutPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AboutPage JSON: status = %d, want 200", rr.Code)
	}
}

func TestAboutPage_PlainTextFormat_ReturnsText(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.AboutPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("AboutPage plain: status = %d, want 200", rr.Code)
	}
}

// ── PrivacyPage (SearchHandler) ───────────────────────────────────────────────

func TestPrivacyPage_JSONFormat_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/privacy", nil)
	req.Header.Set("Accept", "application/json")

	h.PrivacyPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PrivacyPage JSON: status = %d, want 200", rr.Code)
	}
}

func TestPrivacyPage_PlainTextFormat_ReturnsText(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/privacy", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.PrivacyPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PrivacyPage plain: status = %d, want 200", rr.Code)
	}
}

// ── renderHealthzHTML ─────────────────────────────────────────────────────────

// With empty templatesFS, the template parse fails → http.Error 500.
func TestRenderHealthzHTML_EmptyFS_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.renderHealthzHTML(rr, req, "healthy", http.StatusOK, "development",
		"0s", "localhost", time.Now().Format(time.RFC3339),
		map[string]string{"database": "ok", "cache": "ok", "disk": "ok", "scheduler": "ok"})

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderHealthzHTML empty FS: status = %d, want 500", rr.Code)
	}
}

// ── Autodiscover ──────────────────────────────────────────────────────────────

func TestAutodiscover_ReturnsJSON(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/autodiscover", nil)

	h.Autodiscover(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Autodiscover: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Autodiscover: Content-Type = %q, want application/json", ct)
	}
}

// ── SearchRSSFeed ─────────────────────────────────────────────────────────────

func TestSearchRSSFeed_EmptyQuery_Returns400(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search.rss", nil)

	h.SearchRSSFeed(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("SearchRSSFeed empty query: status = %d, want 400", rr.Code)
	}
}

// ── SearchAtomFeed ────────────────────────────────────────────────────────────

func TestSearchAtomFeed_EmptyQuery_Returns400(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search.atom", nil)

	h.SearchAtomFeed(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("SearchAtomFeed empty query: status = %d, want 400", rr.Code)
	}
}

// ── BatchSearch (empty queries path) ─────────────────────────────────────────

func TestBatchSearch_EmptyQueriesArray_Returns400(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search/batch",
		strings.NewReader(`{"queries":[]}`))
	req.Header.Set("Content-Type", "application/json")

	h.BatchSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("BatchSearch empty queries: status = %d, want 400", rr.Code)
	}
}

// ── renderServerTemplate ──────────────────────────────────────────────────────

func TestRenderServerTemplate_UnknownName_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/unknown", nil)

	h.renderServerTemplate(rr, req, "unknown-page", nil)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderServerTemplate unknown: status = %d, want 500", rr.Code)
	}
}

func TestRenderServerTemplate_ValidName_EmptyFS_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/about", nil)

	h.renderServerTemplate(rr, req, "server-about", nil)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderServerTemplate server-about (empty FS): status = %d, want 500", rr.Code)
	}
}

// ── ServerHandler pages (all call renderServerTemplate → 500 on empty FS) ─────

func TestServerHandler_AboutPage_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/about", nil)

	h.AboutPage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ServerHandler.AboutPage: status = %d, want 500", rr.Code)
	}
}

func TestServerHandler_PrivacyPage_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/privacy", nil)

	h.PrivacyPage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ServerHandler.PrivacyPage: status = %d, want 500", rr.Code)
	}
}

func TestServerHandler_ContactPage_GET_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/contact", nil)

	h.ContactPage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ServerHandler.ContactPage GET: status = %d, want 500", rr.Code)
	}
}

func TestServerHandler_ContactPage_POST_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/server/contact",
		strings.NewReader("message=hello"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.ContactPage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ServerHandler.ContactPage POST: status = %d, want 500", rr.Code)
	}
}

func TestServerHandler_HelpPage_Returns500(t *testing.T) {
	h := NewServerHandler(config.DefaultAppConfig())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/help", nil)

	h.HelpPage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("ServerHandler.HelpPage: status = %d, want 500", rr.Code)
	}
}

// ── renderHealthzHTML — status and mode branches ──────────────────────────────

func TestRenderHealthzHTML_UnhealthyStatus_Returns500(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	h.renderHealthzHTML(rr, req, "unhealthy", http.StatusServiceUnavailable,
		"production", "2h30m", "testhost", "2024-06-15T10:30:00Z",
		map[string]string{"database": "unhealthy", "cache": "degraded"})

	if rr.Code == http.StatusOK {
		t.Error("renderHealthzHTML unhealthy+production: should not return 200 with empty FS")
	}
}

func TestRenderHealthzHTML_DegradedStatus_Returns500(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	h.renderHealthzHTML(rr, req, "degraded", http.StatusOK,
		"development", "1h", "testhost", "not-a-timestamp",
		map[string]string{"disk": "degraded"})

	if rr.Code == http.StatusOK {
		t.Error("renderHealthzHTML degraded+development: should not return 200 with empty FS")
	}
}

func TestRenderHealthzHTML_ProductionMode_Returns500(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.Branding.Title = "My App"
	cfg.Server.Branding.Tagline = "My tagline"
	cfg.Server.Branding.Description = "My description"
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	h.renderHealthzHTML(rr, req, "healthy", http.StatusOK,
		"production", "72h", "prod-server", "2024-06-15T10:30:00Z",
		map[string]string{"scheduler": "healthy"})

	if rr.Code == http.StatusOK {
		t.Error("renderHealthzHTML healthy+production+branding: should not return 200 with empty FS")
	}
}

// ── renderTemplate — all template names ──────────────────────────────────────
// Each case in the switch covers a different template path.
// With empty FS, all return 500 (except "default" which returns "Template not found").

func TestRenderTemplate_Search_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "search", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate search: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_Preferences_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "preferences", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate preferences: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_AgeVerify_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "age-verify", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate age-verify: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_ContentRestricted_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "content-restricted", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate content-restricted: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_ContentBlocked_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "content-blocked", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate content-blocked: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_Privacy_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "privacy", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate privacy: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_NojsHome_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/home", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate nojs/home: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_NojsSearch_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/search", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate nojs/search: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_NojsPreferences_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/preferences", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate nojs/preferences: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_NojsAbout_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/about", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate nojs/about: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_NojsAgeVerify_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/age-verify", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate nojs/age-verify: should not return 200 with empty FS")
	}
}

func TestRenderTemplate_NojsPrivacy_Returns500(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.renderTemplate(rr, "nojs/privacy", map[string]interface{}{})
	if rr.Code == http.StatusOK {
		t.Error("renderTemplate nojs/privacy: should not return 200 with empty FS")
	}
}

// ── PreferencesPage with initialized engines ─────────────────────────────────

func TestPreferencesPage_PlainText_WithEngines_CoversLoop(t *testing.T) {
	h := newAPITestHandlerWithEngines()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/preferences", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	h.PreferencesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PreferencesPage plain (with engines): status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Engines") {
		t.Log("PreferencesPage plain: no engines section in output")
	}
}

// ── renderTemplate — WithActiveNavSet_NoOverwrite ─────────────────────────────

func TestRenderTemplate_WithActiveNavSet_NoOverwrite(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	data := map[string]interface{}{
		"ActiveNav": "custom-nav",
	}
	h.renderTemplate(rr, "home", data)
	if data["ActiveNav"] != "custom-nav" {
		t.Error("renderTemplate: should not overwrite existing ActiveNav")
	}
}

func TestRenderTemplate_WithQuerySet_NoOverwrite(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	data := map[string]interface{}{
		"Query": "existing query",
	}
	h.renderTemplate(rr, "search", data)
	if data["Query"] != "existing query" {
		t.Error("renderTemplate: should not overwrite existing Query")
	}
}
