// SPDX-License-Identifier: MIT
// Additional coverage tests for handler functions that do not require templates.
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/model"
)

// ── analyzeResultFields ───────────────────────────────────────────────────────

func TestAnalyzeResultFields_Empty(t *testing.T) {
	stats := analyzeResultFields(nil)
	if stats["total_results"].(int) != 0 {
		t.Error("analyzeResultFields: expected total_results=0 for nil slice")
	}
}

func TestAnalyzeResultFields_AllFields(t *testing.T) {
	pub := time.Now()
	results := []model.VideoResult{
		{
			Title:           "Test Video",
			URL:             "https://example.com/video",
			Thumbnail:       "https://example.com/thumb.jpg",
			PreviewURL:      "https://example.com/preview.mp4",
			DownloadURL:     "https://example.com/dl.mp4",
			Duration:        "5:00",
			DurationSeconds: 300,
			Views:           "1000",
			ViewsCount:      1000,
			Rating:          4.5,
			Quality:         "720p",
			Published:       pub,
		},
	}
	stats := analyzeResultFields(results)
	if stats["total_results"].(int) != 1 {
		t.Errorf("analyzeResultFields: total_results = %v, want 1", stats["total_results"])
	}
	fields := stats["fields"].(map[string]int)
	if fields["has_title"] != 1 {
		t.Error("analyzeResultFields: expected has_title=1")
	}
	if fields["has_url"] != 1 {
		t.Error("analyzeResultFields: expected has_url=1")
	}
	if fields["has_thumbnail"] != 1 {
		t.Error("analyzeResultFields: expected has_thumbnail=1")
	}
	if fields["has_preview_url"] != 1 {
		t.Error("analyzeResultFields: expected has_preview_url=1")
	}
	if fields["has_download_url"] != 1 {
		t.Error("analyzeResultFields: expected has_download_url=1")
	}
	if fields["has_duration"] != 1 {
		t.Error("analyzeResultFields: expected has_duration=1")
	}
	if fields["has_views"] != 1 {
		t.Error("analyzeResultFields: expected has_views=1")
	}
	if fields["has_rating"] != 1 {
		t.Error("analyzeResultFields: expected has_rating=1")
	}
	if fields["has_quality"] != 1 {
		t.Error("analyzeResultFields: expected has_quality=1")
	}
	if fields["has_published"] != 1 {
		t.Error("analyzeResultFields: expected has_published=1")
	}
}

func TestAnalyzeResultFields_NoFields(t *testing.T) {
	results := []model.VideoResult{{}}
	stats := analyzeResultFields(results)
	fields := stats["fields"].(map[string]int)
	if fields["has_title"] != 0 {
		t.Error("analyzeResultFields: expected has_title=0 for empty result")
	}
	if fields["has_url"] != 0 {
		t.Error("analyzeResultFields: expected has_url=0 for empty result")
	}
}

func TestAnalyzeResultFields_DurationSecondsOnly(t *testing.T) {
	results := []model.VideoResult{{DurationSeconds: 60}}
	stats := analyzeResultFields(results)
	fields := stats["fields"].(map[string]int)
	if fields["has_duration"] != 1 {
		t.Error("analyzeResultFields: DurationSeconds alone should count as has_duration")
	}
}

func TestAnalyzeResultFields_ViewsCountOnly(t *testing.T) {
	results := []model.VideoResult{{ViewsCount: 500}}
	stats := analyzeResultFields(results)
	fields := stats["fields"].(map[string]int)
	if fields["has_views"] != 1 {
		t.Error("analyzeResultFields: ViewsCount alone should count as has_views")
	}
}

// ── renderSearchCSV ───────────────────────────────────────────────────────────

func makeTestSearchResponse(query string, results []model.VideoResult) *model.SearchResponse {
	return &model.SearchResponse{
		Ok: true,
		Data: model.SearchData{
			Query:   query,
			Results: results,
		},
	}
}

func TestRenderSearchCSV_Headers(t *testing.T) {
	rr := httptest.NewRecorder()
	resp := makeTestSearchResponse("test", nil)
	renderSearchCSV(rr, resp)

	if rr.Code != http.StatusOK {
		t.Errorf("renderSearchCSV: status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("renderSearchCSV: Content-Type = %q, want text/csv", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "title,url,source") {
		t.Errorf("renderSearchCSV: missing CSV header, got %q", body)
	}
}

func TestRenderSearchCSV_ResultRow(t *testing.T) {
	rr := httptest.NewRecorder()
	resp := makeTestSearchResponse("cats", []model.VideoResult{
		{Title: "Cat Video", URL: "https://example.com/cat", Source: "testsrc", Duration: "1:00"},
	})
	renderSearchCSV(rr, resp)

	body := rr.Body.String()
	if !strings.Contains(body, "Cat Video") {
		t.Errorf("renderSearchCSV: result title missing, got %q", body)
	}
	if !strings.Contains(body, "example.com/cat") {
		t.Errorf("renderSearchCSV: result URL missing, got %q", body)
	}
}

func TestRenderSearchCSV_QuoteEscape(t *testing.T) {
	rr := httptest.NewRecorder()
	resp := makeTestSearchResponse("q", []model.VideoResult{
		{Title: `He said "hello"`, URL: "https://example.com/"},
	})
	renderSearchCSV(rr, resp)

	body := rr.Body.String()
	if !strings.Contains(body, `""hello""`) {
		t.Errorf("renderSearchCSV: double-quote escaping failed, got %q", body)
	}
}

// ── renderSearchRSS ───────────────────────────────────────────────────────────

func TestRenderSearchRSS_ContentType(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.rss?q=test", nil)
	resp := makeTestSearchResponse("test", nil)
	renderSearchRSS(rr, req, resp, cfg)

	if rr.Code != http.StatusOK {
		t.Errorf("renderSearchRSS: status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "rss+xml") {
		t.Errorf("renderSearchRSS: Content-Type = %q, want application/rss+xml", ct)
	}
}

func TestRenderSearchRSS_ContainsRSSTag(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.rss?q=cats", nil)
	resp := makeTestSearchResponse("cats", nil)
	renderSearchRSS(rr, req, resp, cfg)

	body := rr.Body.String()
	if !strings.Contains(body, "version=\"2.0\"") {
		t.Errorf("renderSearchRSS: missing RSS version, got %q", body)
	}
}

func TestRenderSearchRSS_WithResults(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.rss?q=dogs", nil)
	resp := makeTestSearchResponse("dogs", []model.VideoResult{
		{Title: "Dog Video", URL: "https://example.com/dog", Source: "src1", Description: "A cute dog"},
	})
	renderSearchRSS(rr, req, resp, cfg)

	body := rr.Body.String()
	if !strings.Contains(body, "Dog Video") {
		t.Errorf("renderSearchRSS: result title missing, got %q", body)
	}
}

func TestRenderSearchRSS_ThumbnailDescription(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.rss?q=cats", nil)
	resp := makeTestSearchResponse("cats", []model.VideoResult{
		{Title: "Cat", URL: "https://example.com/cat", Thumbnail: "https://example.com/thumb.jpg"},
	})
	renderSearchRSS(rr, req, resp, cfg)

	body := rr.Body.String()
	if !strings.Contains(body, "thumb.jpg") {
		t.Errorf("renderSearchRSS: thumbnail not in description fallback, got %q", body)
	}
}

// ── renderSearchAtom ──────────────────────────────────────────────────────────

func TestRenderSearchAtom_ContentType(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.atom?q=test", nil)
	resp := makeTestSearchResponse("test", nil)
	renderSearchAtom(rr, req, resp, cfg)

	if rr.Code != http.StatusOK {
		t.Errorf("renderSearchAtom: status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "atom+xml") {
		t.Errorf("renderSearchAtom: Content-Type = %q, want application/atom+xml", ct)
	}
}

func TestRenderSearchAtom_ContainsAtomFeed(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.atom?q=birds", nil)
	resp := makeTestSearchResponse("birds", nil)
	renderSearchAtom(rr, req, resp, cfg)

	body := rr.Body.String()
	if !strings.Contains(body, "http://www.w3.org/2005/Atom") {
		t.Errorf("renderSearchAtom: Atom namespace missing, got %q", body)
	}
}

func TestRenderSearchAtom_WithResults(t *testing.T) {
	cfg := createTestConfig()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search.atom?q=birds", nil)
	resp := makeTestSearchResponse("birds", []model.VideoResult{
		{Title: "Bird Video", URL: "https://example.com/bird", Source: "src2", Description: "Lovely birds"},
	})
	renderSearchAtom(rr, req, resp, cfg)

	body := rr.Body.String()
	if !strings.Contains(body, "Bird Video") {
		t.Errorf("renderSearchAtom: result title missing, got %q", body)
	}
}

// ── HealthCheck ───────────────────────────────────────────────────────────────

func TestHealthCheck_JSONFormat(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck JSON: status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("HealthCheck JSON: Content-Type = %q, want application/json", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"status"`) {
		t.Errorf("HealthCheck JSON: missing status field, got %q", body)
	}
	if !strings.Contains(body, `"checks"`) {
		t.Errorf("HealthCheck JSON: missing checks field, got %q", body)
	}
}

func TestHealthCheck_PlainTextFormat(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HealthCheck plain: status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "status: ") {
		t.Errorf("HealthCheck plain: missing 'status:' line, got %q", body)
	}
	if !strings.Contains(body, "checks.database: ") {
		t.Errorf("HealthCheck plain: missing 'checks.database:' line, got %q", body)
	}
}

func TestHealthCheck_JSONContainsProject(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Test Vidveil") {
		t.Errorf("HealthCheck JSON: project name not in response, got %q", body)
	}
}

func TestHealthCheck_PendingRestart(t *testing.T) {
	cfg := createTestConfig()
	cfg.PendingRestart = true
	cfg.RestartReasons = []string{"config changed"}
	h := &SearchHandler{appConfig: cfg}

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "pending_restart") {
		t.Errorf("HealthCheck JSON: missing pending_restart field, got %q", body)
	}
}

func TestHealthCheck_PendingRestart_PlainText(t *testing.T) {
	cfg := createTestConfig()
	cfg.PendingRestart = true
	cfg.RestartReasons = []string{"cert renewed"}
	h := &SearchHandler{appConfig: cfg}

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.HealthCheck(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "pending_restart: true") {
		t.Errorf("HealthCheck plain: missing pending_restart line, got %q", body)
	}
}

// ── APIBangs ──────────────────────────────────────────────────────────────────

func TestAPIBangs_JSON(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/api/v1/bangs", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIBangs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIBangs JSON: status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("APIBangs JSON: Content-Type = %q, want application/json", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"ok"`) {
		t.Errorf("APIBangs JSON: missing ok field, got %q", body)
	}
	if !strings.Contains(body, `"count"`) {
		t.Errorf("APIBangs JSON: missing count field, got %q", body)
	}
}

func TestAPIBangs_PlainText(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/api/v1/bangs", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.APIBangs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIBangs plain: status = %d, want %d", rr.Code, http.StatusOK)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("APIBangs plain: Content-Type = %q, want text/plain", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "bangs:") {
		t.Errorf("APIBangs plain: missing 'bangs:' line, got %q", body)
	}
}

// ── BatchSearch ───────────────────────────────────────────────────────────────

func TestBatchSearch_MethodNotAllowed(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/api/v1/search/batch", nil)
	rr := httptest.NewRecorder()
	h.BatchSearch(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("BatchSearch GET: status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestBatchSearch_InvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("POST", "/api/v1/search/batch", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.BatchSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("BatchSearch invalid JSON: status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
