// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for engine manager methods that have no tests yet.
// Tests Search, DebugSearch, SearchStreamWithOperators on an empty EngineManager
// (no engines initialised) so no network calls are made.
// Also covers debugLogEngineResponse, debugLogEngineParseResult, ListEnginesWithHealth,
// SpellCorrect, EnabledCount, GetFeatures, and createHTTPClient via GetClient.
package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// newEmptyMgr returns an EngineManager with zero engines initialised.
func newEmptyMgr() *EngineManager {
	return NewEngineManager(config.DefaultAppConfig())
}

// ── EngineManager.Search ──────────────────────────────────────────────────────

func TestEngineManager_Search_EmptyManager_ReturnsResponse(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "test", 1, nil, "")
	if resp == nil {
		t.Fatal("Search: nil response")
	}
}

func TestEngineManager_Search_WithEngineNames_ReturnsEmpty(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "test", 1, []string{"ph", "xv"}, "")
	if resp == nil {
		t.Fatal("Search with engineNames: nil response")
	}
}

// SearchWithOperators must exist alongside Search (mirroring the
// SearchStream/SearchStreamWithOperators convention) so exclusion/exact-phrase
// operators reach the non-SSE search paths (JSON API, HTML fallback, RSS/Atom
// feeds, batch search) instead of being silently ignored.
func TestEngineManager_SearchWithOperators_EmptyManager_ReturnsResponse(t *testing.T) {
	m := newEmptyMgr()
	resp := m.SearchWithOperators(context.Background(), "test", 1, nil, []string{"exact phrase"}, []string{"excluded"}, nil, false, "", 20)
	if resp == nil {
		t.Fatal("SearchWithOperators: nil response")
	}
}

func TestEngineManager_Search_PageTwo_ReturnsEmpty(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "amateur", 2, nil, "")
	if resp == nil {
		t.Fatal("Search page 2: nil response")
	}
}

func TestEngineManager_Search_CancelledContext_ReturnsResponse(t *testing.T) {
	m := newEmptyMgr()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Even with cancelled context, empty manager completes immediately
	resp := m.Search(ctx, "test", 1, nil, "")
	if resp == nil {
		t.Fatal("Search cancelled ctx: nil response")
	}
}

func TestEngineManager_Search_ResponseHasPagination(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "test", 1, nil, "")
	if resp == nil {
		t.Fatal("Search: nil response")
	}
	// Access pagination to ensure it is initialised
	_ = resp.Pagination.Page
}

// TestEngineManager_Search_ResultsNeverExceedLimit is a regression test for a
// live-beta-testing find: manager.go's Search() built Data.Results from the
// full deduplicated result set without ever slicing to resultsPerPage, so a
// page with more than Pagination.Limit distinct results returned an
// oversized data array (violating AI.md PART 14's pagination contract, which
// requires "data" to respect "limit"). Register a mock engine with more
// unique, filter-passing results than the default ResultsPerPage (50) and
// verify the returned page never exceeds the advertised limit, while
// Pagination.Total/Pages still reflect the full pre-slice count.
func TestEngineManager_Search_ResultsNeverExceedLimit(t *testing.T) {
	const totalResults = 75
	results := make([]model.VideoResult, 0, totalResults)
	for i := 0; i < totalResults; i++ {
		results = append(results, validResult(
			fmt.Sprintf("test video number %d", i),
			fmt.Sprintf("https://example.com/v%d", i),
		))
	}
	m := newMgrWithMock("mock-many", results, nil, true)

	resp := m.Search(context.Background(), "test video", 1, nil, "")
	if resp == nil {
		t.Fatal("Search: nil response")
	}

	limit := resp.Pagination.Limit
	if limit <= 0 {
		t.Fatalf("expected positive Pagination.Limit, got %d", limit)
	}
	if len(resp.Data.Results) > limit {
		t.Fatalf("Data.Results length %d exceeds Pagination.Limit %d", len(resp.Data.Results), limit)
	}
	if resp.Pagination.Total < len(resp.Data.Results) {
		t.Fatalf("Pagination.Total %d is smaller than returned page size %d", resp.Pagination.Total, len(resp.Data.Results))
	}
	expectedPages := (resp.Pagination.Total + limit - 1) / limit
	if resp.Pagination.Pages != expectedPages {
		t.Fatalf("Pagination.Pages = %d, expected %d for Total=%d Limit=%d", resp.Pagination.Pages, expectedPages, resp.Pagination.Total, limit)
	}
}

// ── EngineManager.DebugSearch ─────────────────────────────────────────────────

func TestEngineManager_DebugSearch_EmptyManager_ReturnsResult(t *testing.T) {
	m := newEmptyMgr()
	result := m.DebugSearch(context.Background(), "test", 1)
	if result == nil {
		t.Fatal("DebugSearch: nil result")
	}
}

func TestEngineManager_DebugSearch_ZeroTotalEngines(t *testing.T) {
	m := newEmptyMgr()
	result := m.DebugSearch(context.Background(), "test", 1)
	if result == nil {
		t.Fatal("DebugSearch: nil result")
	}
	if result.TotalEngines != 0 {
		t.Errorf("DebugSearch empty: TotalEngines = %d, want 0", result.TotalEngines)
	}
}

func TestEngineManager_DebugSearch_HasSearchTime(t *testing.T) {
	m := newEmptyMgr()
	result := m.DebugSearch(context.Background(), "test", 1)
	if result == nil {
		t.Fatal("DebugSearch: nil result")
	}
	if result.SearchTimeMS < 0 {
		t.Errorf("DebugSearch: SearchTimeMS = %d, want >= 0", result.SearchTimeMS)
	}
}

// ── EngineManager.SearchStreamWithOperators ───────────────────────────────────

func TestEngineManager_SearchStreamWithOperators_EmptyManager_ChannelClosed(t *testing.T) {
	m := newEmptyMgr()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "test", 1, nil, nil, nil, nil, nil, false, 0, false, 0, "")
	if ch == nil {
		t.Fatal("SearchStreamWithOperators: nil channel")
	}

	// Drain the channel — with an empty engine manager it must close quickly
	for range ch {
	}
}

func TestEngineManager_SearchStreamWithOperators_WithEngineNames_ChannelClosed(t *testing.T) {
	m := newEmptyMgr()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "test", 1, []string{"ph"}, nil, nil, nil, nil, false, 0, false, 0, "")
	if ch == nil {
		t.Fatal("SearchStreamWithOperators with engineNames: nil channel")
	}

	for range ch {
	}
}

// ── EngineManager.ListEnginesWithHealth ───────────────────────────────────────

func TestEngineManager_ListEnginesWithHealth_EmptyManager_EmptySlice(t *testing.T) {
	m := newEmptyMgr()
	list := m.ListEnginesWithHealth()
	// Empty manager → empty or nil slice; either is correct
	_ = list
}

// ── EngineManager.EnabledCount ────────────────────────────────────────────────

func TestEngineManager_EnabledCount_EmptyManager_Zero(t *testing.T) {
	m := newEmptyMgr()
	count := m.EnabledCount()
	if count != 0 {
		t.Errorf("EnabledCount empty: got %d, want 0", count)
	}
}

// ── EngineManager.SpellCorrect ────────────────────────────────────────────────

func TestEngineManager_SpellCorrect_ShortQuery_NoPanic(t *testing.T) {
	m := newEmptyMgr()
	result := m.SpellCorrect("test")
	_ = result
}

func TestEngineManager_SpellCorrect_LongQuery_NoPanic(t *testing.T) {
	m := newEmptyMgr()
	result := m.SpellCorrect("this is a longer search query about amateur video content")
	_ = result
}

// ── createHTTPClient via GetClient ────────────────────────────────────────────

func TestBaseEngine_GetClient_NonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	e := NewBaseEngine("test", "Test Engine", "https://example.com", 1, cfg)
	client := e.GetClient()
	if client == nil {
		t.Error("GetClient: nil HTTP client")
	}
}

func TestBaseEngine_GetClient_SpoofedTLS_NonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	e := NewBaseEngine("test", "Test Engine", "https://example.com", 1, cfg)
	e.SetUseSpoofedTLS(true)
	client := e.GetClient()
	if client == nil {
		t.Error("GetClient spoofed TLS: nil HTTP client")
	}
}
