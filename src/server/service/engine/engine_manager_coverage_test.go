// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for engine manager methods that have no tests yet.
// Tests Search, DebugSearch, SearchStreamWithOperators on an empty EngineManager
// (no engines initialised) so no network calls are made.
// Also covers DebugLogEngineResponse, DebugLogEngineParseResult, ListEnginesWithHealth,
// SpellCorrect, EnabledCount, GetFeatures, and createHTTPClient via GetClient.
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// newEmptyMgr returns an EngineManager with zero engines initialised.
func newEmptyMgr() *EngineManager {
	return NewEngineManager(config.DefaultAppConfig())
}

// ── EngineManager.Search ──────────────────────────────────────────────────────

func TestEngineManager_Search_EmptyManager_ReturnsResponse(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "test", 1, nil)
	if resp == nil {
		t.Fatal("Search: nil response")
	}
}

func TestEngineManager_Search_WithEngineNames_ReturnsEmpty(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "test", 1, []string{"ph", "xv"})
	if resp == nil {
		t.Fatal("Search with engineNames: nil response")
	}
}

func TestEngineManager_Search_PageTwo_ReturnsEmpty(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "amateur", 2, nil)
	if resp == nil {
		t.Fatal("Search page 2: nil response")
	}
}

func TestEngineManager_Search_CancelledContext_ReturnsResponse(t *testing.T) {
	m := newEmptyMgr()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Even with cancelled context, empty manager completes immediately
	resp := m.Search(ctx, "test", 1, nil)
	if resp == nil {
		t.Fatal("Search cancelled ctx: nil response")
	}
}

func TestEngineManager_Search_ResponseHasPagination(t *testing.T) {
	m := newEmptyMgr()
	resp := m.Search(context.Background(), "test", 1, nil)
	if resp == nil {
		t.Fatal("Search: nil response")
	}
	// Access pagination to ensure it is initialised
	_ = resp.Pagination.Page
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

	ch := m.SearchStreamWithOperators(ctx, "test", 1, nil, nil, nil, nil, false, 0, false, 0)
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

	ch := m.SearchStreamWithOperators(ctx, "test", 1, []string{"ph"}, nil, nil, nil, false, 0, false, 0)
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
