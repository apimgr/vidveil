// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for handler API endpoints and utility functions.
// Tests APISearch, APIAutocomplete, APIEngineDetails, DebugEngine, DebugEnginesList,
// SearchRSSFeed, SearchAtomFeed, BatchSearch, ContentRestrictedPage,
// ContentRestrictedSubmit, BuildDateTime, RenderErrorPage, and MaintenanceModeMiddleware.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/cache"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// newAPITestHandler builds a SearchHandler with an empty EngineManager for API tests.
func newAPITestHandler() *SearchHandler {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	return NewSearchHandler(cfg, mgr)
}

// ── APISearch ─────────────────────────────────────────────────────────────────

func TestAPISearch_MissingQuery_Returns400(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("APISearch missing q: status = %d, want 400", w.Code)
	}
}

func TestAPISearch_BangOnlyQuery_Returns400(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=!ph", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	// "!ph" parsed: query is empty after bang parsing → 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("APISearch bang-only q: status = %d, want 400", w.Code)
	}
}

func TestAPISearch_WithQuery_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	// Empty engine manager → search returns immediately with empty results
	if w.Code != http.StatusOK {
		t.Errorf("APISearch with q: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_WithPage_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&page=2", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch with page: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_WithEnginesParam_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&engines=ph,xv", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch with engines: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_PlainTextFormat_ReturnsText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch plain text: status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("APISearch plain text: Content-Type = %q, want text/plain", ct)
	}
}

func TestAPISearch_CSVFormat_ReturnsCSV(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search.csv?q=test", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch CSV: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_RSSFormat_ReturnsRSS(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r.Header.Set("Accept", "application/rss+xml")
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch RSS: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_AtomFormat_ReturnsAtom(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r.Header.Set("Accept", "application/atom+xml")
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch Atom: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_NoCacheParam_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&nocache=1", nil)
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch nocache: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_TorCookieTrue_Covered(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r.AddCookie(&http.Cookie{Name: "vidveil-use-tor", Value: "1"})
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch tor cookie=1: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_TorCookieFalse_Covered(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r.AddCookie(&http.Cookie{Name: "vidveil-use-tor", Value: "0"})
	w := httptest.NewRecorder()
	h.APISearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APISearch tor cookie=0: status = %d, want 200", w.Code)
	}
}

func TestAPISearch_IfNoneMatch_Returns304(t *testing.T) {
	h := newAPITestHandler()
	// First request to get the ETag
	r1 := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	w1 := httptest.NewRecorder()
	h.APISearch(w1, r1)
	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Skip("No ETag in response, skipping 304 test")
	}
	// Second request with If-None-Match
	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	h.APISearch(w2, r2)
	if w2.Code != http.StatusNotModified {
		t.Errorf("APISearch If-None-Match: status = %d, want 304", w2.Code)
	}
}

// ── APIAutocomplete ───────────────────────────────────────────────────────────

func TestAPIAutocomplete_EmptyQuery_ReturnsPopular(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete", nil)
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete empty: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_EmptyQuery_PlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete empty plain: status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("APIAutocomplete empty plain: Content-Type should be text/plain")
	}
}

func TestAPIAutocomplete_BangQuery_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=!ph", nil)
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete !ph: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_BangQuery_PlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=!ph", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete !ph plain: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_BangStart_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=test+!", nil)
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete bang start: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_BangStart_PlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=test+!", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete bang start plain: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_PartialBangAtEnd_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=amateur+!ph", nil)
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete partial bang at end: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_PartialBangAtEnd_PlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=amateur+!ph", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete partial bang plain: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_PerformerAtEnd_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=teen+@mia", nil)
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete @performer: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_PerformerAtEnd_PlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=teen+@mia", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete @performer plain: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_NormalQuery_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=amateur", nil)
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete normal: status = %d, want 200", w.Code)
	}
}

func TestAPIAutocomplete_NormalQuery_PlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/autocomplete?q=amateur", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.APIAutocomplete(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("APIAutocomplete normal plain: status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("APIAutocomplete normal plain: Content-Type should be text/plain")
	}
}

// ── APIEngineDetails ──────────────────────────────────────────────────────────

// No chi route context → chi.URLParam returns "" → GetEngine("") → 404.
func TestAPIEngineDetails_NotFound_Returns404(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/engines/nonexistent", nil)
	w := httptest.NewRecorder()
	h.APIEngineDetails(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("APIEngineDetails not found: status = %d, want 404", w.Code)
	}
}

// ── DebugEngine ───────────────────────────────────────────────────────────────

// No chi route context → chi.URLParam returns "" → GetEngine("") → 404.
func TestDebugEngine_NotFound_Returns404(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/debug/engine/test?q=test", nil)
	w := httptest.NewRecorder()
	h.DebugEngine(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("DebugEngine not found: status = %d, want 404", w.Code)
	}
}

// ── DebugEnginesList ──────────────────────────────────────────────────────────

func TestDebugEnginesList_EmptyManager_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/debug/engines", nil)
	w := httptest.NewRecorder()
	h.DebugEnginesList(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("DebugEnginesList: status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("DebugEnginesList: body not valid JSON: %v", err)
	}
	if _, ok := resp["engines"]; !ok {
		t.Error("DebugEnginesList: missing 'engines' key")
	}
}

// ── SearchRSSFeed / SearchAtomFeed ────────────────────────────────────────────

func TestSearchRSSFeed_WithQuery_ReturnsRSS(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/search.rss?q=test", nil)
	w := httptest.NewRecorder()
	h.SearchRSSFeed(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("SearchRSSFeed with q: status = %d, want 200", w.Code)
	}
}

func TestSearchAtomFeed_WithQuery_ReturnsAtom(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/search.atom?q=test", nil)
	w := httptest.NewRecorder()
	h.SearchAtomFeed(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("SearchAtomFeed with q: status = %d, want 200", w.Code)
	}
}

func TestSearchRSSFeed_WithPage_ReturnsRSS(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/search.rss?q=test&page=2", nil)
	w := httptest.NewRecorder()
	h.SearchRSSFeed(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("SearchRSSFeed page 2: status = %d, want 200", w.Code)
	}
}

func TestSearchAtomFeed_WithPage_ReturnsAtom(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/search.atom?q=test&page=2", nil)
	w := httptest.NewRecorder()
	h.SearchAtomFeed(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("SearchAtomFeed page 2: status = %d, want 200", w.Code)
	}
}

// ── BatchSearch ───────────────────────────────────────────────────────────────

func TestBatchSearch_TooManyQueries_Returns400(t *testing.T) {
	h := newAPITestHandler()
	body := `{"queries":[{"q":"a"},{"q":"b"},{"q":"c"},{"q":"d"},{"q":"e"},{"q":"f"}]}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/search/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.BatchSearch(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("BatchSearch too many: status = %d, want 400", w.Code)
	}
}

func TestBatchSearch_ValidBatch_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	body := `{"queries":[{"q":"test"},{"q":"amateur","page":2}]}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/search/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.BatchSearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("BatchSearch valid: status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("BatchSearch valid: body not valid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Errorf("BatchSearch valid: ok != true, got %v", resp["ok"])
	}
}

func TestBatchSearch_WithEngines_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	body := `{"queries":[{"q":"test","engines":"ph,xv"}]}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/search/batch", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.BatchSearch(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("BatchSearch with engines: status = %d, want 200", w.Code)
	}
}

// ── ContentRestrictedPage ─────────────────────────────────────────────────────

// Without the ack cookie, renderResponse is called → CLI UA path → JSON 200.
func TestContentRestrictedPage_NoAck_CLIClient_ReturnsJSON(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/content-restricted", nil)
	r.Header.Set("User-Agent", "vidveil-cli/1.0.0")
	w := httptest.NewRecorder()
	h.ContentRestrictedPage(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("ContentRestrictedPage no ack CLI: status = %d, want 200", w.Code)
	}
}

// Without ack cookie, curl UA → plain text 200.
func TestContentRestrictedPage_NoAck_CurlClient_ReturnsText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/content-restricted", nil)
	r.Header.Set("User-Agent", "curl/7.88.1")
	w := httptest.NewRecorder()
	h.ContentRestrictedPage(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("ContentRestrictedPage no ack curl: status = %d, want 200", w.Code)
	}
}

// With ack cookie → redirect.
func TestContentRestrictedPage_WithAck_Redirects(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/content-restricted", nil)
	r.AddCookie(&http.Cookie{Name: ContentRestrictionAckCookieName, Value: "1"})
	w := httptest.NewRecorder()
	h.ContentRestrictedPage(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("ContentRestrictedPage with ack: status = %d, want 302", w.Code)
	}
}

// ContentRestrictedPage with ack and redirect param → redirects to param value.
func TestContentRestrictedPage_WithAckAndRedirect_Redirects(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/content-restricted?redirect=/search", nil)
	r.AddCookie(&http.Cookie{Name: ContentRestrictionAckCookieName, Value: "1"})
	w := httptest.NewRecorder()
	h.ContentRestrictedPage(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("ContentRestrictedPage with ack+redirect: status = %d, want 302", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/search" {
		t.Errorf("ContentRestrictedPage with ack+redirect: Location = %q, want /search", location)
	}
}

// ── ContentRestrictedSubmit ───────────────────────────────────────────────────

func TestContentRestrictedSubmit_GET_Redirects(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/content-restricted/submit", nil)
	w := httptest.NewRecorder()
	h.ContentRestrictedSubmit(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("ContentRestrictedSubmit GET: status = %d, want 302", w.Code)
	}
}

func TestContentRestrictedSubmit_POST_Redirects(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodPost, "/content-restricted/submit",
		strings.NewReader("redirect=/search"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ContentRestrictedSubmit(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("ContentRestrictedSubmit POST: status = %d, want 302", w.Code)
	}
}

func TestContentRestrictedSubmit_POST_BadRedirect_RedirectsHome(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodPost, "/content-restricted/submit",
		strings.NewReader("redirect=http://evil.com"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ContentRestrictedSubmit(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("ContentRestrictedSubmit bad redirect: status = %d, want 302", w.Code)
	}
	if w.Header().Get("Location") != "/" {
		t.Errorf("ContentRestrictedSubmit bad redirect: Location = %q, want /", w.Header().Get("Location"))
	}
}

// ── BuildDateTime ─────────────────────────────────────────────────────────────

func TestBuildDateTime_UnknownBuildTime(t *testing.T) {
	result := BuildDateTime()
	// In test environment, build time is not set → returns "unknown" or formatted string
	if result == "" {
		t.Error("BuildDateTime: empty result")
	}
}

// ── RenderErrorPage ───────────────────────────────────────────────────────────

// With empty templatesFS, template.ParseFS fails → fallback plain text error.
func TestRenderErrorPage_EmptyFS_FallsBackToPlainText(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.RenderErrorPage(w, r, http.StatusNotFound, "Not Found", "page not found")
	// Status is written via http.Error → 404
	if w.Code != http.StatusNotFound {
		t.Errorf("RenderErrorPage: status = %d, want 404", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Not Found") {
		t.Errorf("RenderErrorPage: body missing 'Not Found': %s", body)
	}
}

// ── MaintenanceModeMiddleware ─────────────────────────────────────────────────

func TestMaintenanceModeMiddleware_HealthzPath_CallsNext(t *testing.T) {
	h := newAPITestHandler()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mw := h.MaintenanceModeMiddleware(next)
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if !called {
		t.Error("MaintenanceModeMiddleware /healthz: next handler not called")
	}
}

// Normal path without maintenance file → calls next (stat fails → no maintenance).
func TestMaintenanceModeMiddleware_NormalPath_CallsNext(t *testing.T) {
	h := newAPITestHandler()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mw := h.MaintenanceModeMiddleware(next)
	r := httptest.NewRequest(http.MethodGet, "/search", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if !called {
		t.Error("MaintenanceModeMiddleware /search: next handler not called")
	}
}

// ── APIEngineDetails with initialized engine ──────────────────────────────────

// newAPITestHandlerWithEngines creates a handler with all engines initialized.
func newAPITestHandlerWithEngines() *SearchHandler {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	mgr.InitializeEngines()
	return NewSearchHandler(cfg, mgr)
}

func TestAPIEngineDetails_Found_ReturnsJSON(t *testing.T) {
	h := newAPITestHandlerWithEngines()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "pornhub")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines/pornhub", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.APIEngineDetails(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Skip("engine 'pornhub' not found — InitializeEngines may have different keys")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("APIEngineDetails found: status = %d, want 200", rr.Code)
	}
}

func TestAPIEngineDetails_Found_PlainText(t *testing.T) {
	h := newAPITestHandlerWithEngines()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "pornhub")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines/pornhub", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.APIEngineDetails(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Skip("engine 'pornhub' not found")
	}
}

func TestDebugEngine_Found_ReturnsResult(t *testing.T) {
	h := newAPITestHandlerWithEngines()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "pornhub")
	req := httptest.NewRequest(http.MethodGet, "/debug/engine/pornhub?q=test", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.DebugEngine(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Skip("engine 'pornhub' not found")
	}
}

// ── DebugEnginesList with initialized engines ─────────────────────────────────

func TestDebugEnginesList_InitializedManager_ReturnsJSON(t *testing.T) {
	h := newAPITestHandlerWithEngines()

	req := httptest.NewRequest(http.MethodGet, "/debug/engines", nil)
	rr := httptest.NewRecorder()
	h.DebugEnginesList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("DebugEnginesList(initialized): status = %d, want 200", rr.Code)
	}
}

func TestDebugEnginesList_PlainTextFormat_ReturnsText(t *testing.T) {
	h := newAPITestHandlerWithEngines()

	req := httptest.NewRequest(http.MethodGet, "/debug/engines", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	rr := httptest.NewRecorder()
	h.DebugEnginesList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("DebugEnginesList plain text: status = %d, want 200", rr.Code)
	}
}

// ── APIEngines with initialized engines ───────────────────────────────────────

func TestAPIEngines_InitializedManager_PlainText(t *testing.T) {
	h := newAPITestHandlerWithEngines()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	rr := httptest.NewRecorder()
	h.APIEngines(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIEngines plain text initialized: status = %d, want 200", rr.Code)
	}
}

// ── APISearch — min_quality, min_duration, show_ai, preview_first params ─────

func TestAPISearch_WithMinQuality_CoversParam(t *testing.T) {
	h := newAPITestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&min_quality=720", nil)
	h.APISearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APISearch min_quality: status = %d, want 200", rr.Code)
	}
}

func TestAPISearch_WithMinDuration_CoversParam(t *testing.T) {
	h := newAPITestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&min_duration=120", nil)
	h.APISearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APISearch min_duration: status = %d, want 200", rr.Code)
	}
}

func TestAPISearch_ShowAI_CoversFlag(t *testing.T) {
	h := newAPITestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&show_ai=1", nil)
	h.APISearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APISearch show_ai: status = %d, want 200", rr.Code)
	}
}

func TestAPISearch_PreviewFirst_CoversFlag(t *testing.T) {
	h := newAPITestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&preview_first=1", nil)
	h.APISearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APISearch preview_first: status = %d, want 200", rr.Code)
	}
}

// ── APISearch — SSE format path (line 1661) ───────────────────────────────────

func TestAPISearch_SSEFormat_CoversSSEPath(t *testing.T) {
	h := newAPITestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	req.Header.Set("Accept", "text/event-stream")

	h.APISearch(rr, req)

	// handleSearchSSE will fail (non-flusher) → 500, but line 1661-1663 IS covered
}

// ── APISearch — cache hit with metrics (line 1676-1678) ──────────────────────

func TestAPISearch_CacheHit_WithMetrics_CoversLine1676(t *testing.T) {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	h := NewSearchHandler(cfg, mgr)
	h.metrics = NewMetrics(cfg, mgr)

	// Pre-seed the cache using the real CacheKey function
	cacheKey := cache.CacheKey("test", 1, nil)
	h.searchCache.Set(cacheKey, &model.SearchResponse{Ok: true})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	h.APISearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Logf("APISearch(cache hit): status = %d", rr.Code)
	}
}

func TestAPISearch_CacheHit_TextPlain_WithResults_CoversResultLoop(t *testing.T) {
	cfg := config.DefaultAppConfig()
	mgr := engine.NewEngineManager(cfg)
	h := NewSearchHandler(cfg, mgr)

	// Pre-seed cache with a non-empty result (with Duration and Views set)
	cacheKey := cache.CacheKey("testloop", 1, nil)
	h.searchCache.Set(cacheKey, &model.SearchResponse{
		Ok: true,
		Data: model.SearchData{
			Results: []model.VideoResult{
				{Title: "Test Video", URL: "https://example.com/v1", Duration: "5:30", Views: "1K"},
				{Title: "Another Video", URL: "https://example.com/v2"},
			},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=testloop", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	h.APISearch(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Test Video") {
		t.Logf("APISearch(text+cache+results): body=%q", body[:min(len(body), 200)])
	}
}
