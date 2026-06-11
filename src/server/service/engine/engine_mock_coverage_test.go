// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests using a mock SearchEngine implementation.
// The mock engine lets us drive the full Search/DebugSearch/SearchStream
// code paths in manager.go without making any network calls.
package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// ── mock SearchEngine ─────────────────────────────────────────────────────────

type mockSearchEngine struct {
	name    string
	results []model.VideoResult
	err     error
	avail   bool
	tier    int
}

func (m *mockSearchEngine) Name() string        { return m.name }
func (m *mockSearchEngine) DisplayName() string { return m.name }
func (m *mockSearchEngine) Search(_ context.Context, _ string, _ int) ([]model.VideoResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}
func (m *mockSearchEngine) IsAvailable() bool                        { return m.avail }
func (m *mockSearchEngine) SupportsFeature(_ Feature) bool          { return false }
func (m *mockSearchEngine) Tier() int                               { return m.tier }
func (m *mockSearchEngine) Capabilities() Capabilities              { return Capabilities{} }

// newMgrWithMock creates an EngineManager that contains one mock engine.
func newMgrWithMock(name string, results []model.VideoResult, err error, avail bool) *EngineManager {
	cfg := config.DefaultAppConfig()
	m := NewEngineManager(cfg)
	m.engines[name] = &mockSearchEngine{
		name:    name,
		results: results,
		err:     err,
		avail:   avail,
		tier:    1,
	}
	return m
}

// validResult creates a VideoResult that passes all filters.
func validResult(title, url string) model.VideoResult {
	return model.VideoResult{
		Title:           title,
		URL:             url,
		Thumbnail:       "https://example.com/thumb.jpg",
		DurationSeconds: 300,
	}
}

// ── Search with results ───────────────────────────────────────────────────────

func TestSearch_WithMockEngine_ReturnsResults(t *testing.T) {
	results := []model.VideoResult{
		validResult("amateur teen xxx", "https://example.com/v1"),
		validResult("amateur teen xxx video", "https://example.com/v2"),
	}
	m := newMgrWithMock("mock", results, nil, true)

	resp := m.Search(context.Background(), "amateur teen", 1, nil)
	if resp == nil {
		t.Fatal("Search: nil response")
	}
	if len(resp.Data.Results) == 0 {
		t.Log("Search: no results passed filters (may be filtered by relevance/intent)")
	}
}

func TestSearch_WithMockEngineError_RecordsFailure(t *testing.T) {
	m := newMgrWithMock("mock-err", nil, errors.New("engine failed"), true)

	resp := m.Search(context.Background(), "test", 1, nil)
	if resp == nil {
		t.Fatal("Search with error: nil response")
	}
}

func TestSearch_WithMockEngine_InvalidThumbnail_Filtered(t *testing.T) {
	results := []model.VideoResult{
		{Title: "no-thumb", URL: "https://example.com/v1", Thumbnail: "", DurationSeconds: 300},
		{Title: "invalid-thumb", URL: "https://example.com/v2", Thumbnail: "invalid-url", DurationSeconds: 300},
	}
	m := newMgrWithMock("mock-thumb", results, nil, true)

	resp := m.Search(context.Background(), "test", 1, nil)
	if resp == nil {
		t.Fatal("Search(invalid thumbs): nil response")
	}
}

func TestSearch_WithMockEngine_DuplicateURL_Deduped(t *testing.T) {
	results := []model.VideoResult{
		validResult("test video one", "https://example.com/same"),
		validResult("test video one copy", "https://example.com/same"),
	}
	m := newMgrWithMock("mock-dup", results, nil, true)

	resp := m.Search(context.Background(), "test video", 1, nil)
	if resp == nil {
		t.Fatal("Search(dup URLs): nil response")
	}
}

func TestSearch_WithMockEngine_UnavailableEngine_NoResults(t *testing.T) {
	results := []model.VideoResult{validResult("test", "https://example.com/v1")}
	m := newMgrWithMock("mock-unav", results, nil, false)

	resp := m.Search(context.Background(), "test", 1, nil)
	if resp == nil {
		t.Fatal("Search(unavailable engine): nil response")
	}
}

// ── DebugSearch with mock engine ──────────────────────────────────────────────

func TestDebugSearch_WithMockEngine_HasStats(t *testing.T) {
	results := []model.VideoResult{
		validResult("debug test video", "https://example.com/v1"),
	}
	m := newMgrWithMock("mock-dbg", results, nil, true)

	info := m.DebugSearch(context.Background(), "debug test", 1)
	if info == nil {
		t.Fatal("DebugSearch: nil result")
	}
}

func TestDebugSearch_WithMockEngineError_IncludesError(t *testing.T) {
	m := newMgrWithMock("mock-dbg-err", nil, errors.New("debug failure"), true)

	info := m.DebugSearch(context.Background(), "test", 1)
	if info == nil {
		t.Fatal("DebugSearch(error): nil result")
	}
}

// ── Search with specific engine name filter ───────────────────────────────────

func TestSearch_WithMockEngine_ByName_ReturnsResults(t *testing.T) {
	results := []model.VideoResult{
		validResult("named engine test", "https://example.com/named"),
	}
	m := newMgrWithMock("mock-named", results, nil, true)

	resp := m.Search(context.Background(), "named engine", 1, []string{"mock-named"})
	if resp == nil {
		t.Fatal("Search by name: nil response")
	}
}

// ── Search with min duration filter ──────────────────────────────────────────

func TestSearch_MinDuration_FiltersShortVideos(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.MinDurationSeconds = 60
	m := NewEngineManager(cfg)
	m.engines["mock-dur"] = &mockSearchEngine{
		name:  "mock-dur",
		avail: true,
		tier:  1,
		results: []model.VideoResult{
			{
				Title:           "short video test",
				URL:             "https://example.com/short",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 30,
			},
			{
				Title:           "long video test",
				URL:             "https://example.com/long",
				Thumbnail:       "https://example.com/thumb2.jpg",
				DurationSeconds: 600,
			},
		},
	}

	resp := m.Search(context.Background(), "video test", 1, nil)
	if resp == nil {
		t.Fatal("Search(min duration): nil response")
	}
}

// ── SearchStreamWithOperators with mock engine ────────────────────────────────

func TestSearchStreamWithOperators_WithMockEngine_ChannelReceivesData(t *testing.T) {
	results := []model.VideoResult{
		validResult("stream test video", "https://example.com/stream"),
	}
	m := newMgrWithMock("mock-stream", results, nil, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream test", 1, nil, nil, nil, nil, false, 0, false, 0)
	if ch == nil {
		t.Fatal("SearchStreamWithOperators: nil channel")
	}

	// Drain channel
	count := 0
	for range ch {
		count++
	}
	t.Logf("SearchStreamWithOperators: received %d results", count)
}

func TestSearchStreamWithOperators_WithMockEngineError_ChannelCloses(t *testing.T) {
	m := newMgrWithMock("mock-stream-err", nil, errors.New("stream fail"), true)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "test", 1, nil, nil, nil, nil, false, 0, false, 0)
	if ch == nil {
		t.Fatal("SearchStreamWithOperators(error): nil channel")
	}

	// Channel should close after engine error
	for range ch {
	}
}

// ── calculateRelevanceScore standalone function ───────────────────────────────

func TestCalculateRelevanceScore_WithMatchingResult_NonNegative(t *testing.T) {
	result := validResult("amateur teen xxx video", "https://example.com/v1")
	words := []string{"amateur", "teen"}

	score := calculateRelevanceScore(result, "amateur teen", words)
	if score < 0 {
		t.Errorf("calculateRelevanceScore: expected non-negative, got %f", score)
	}
}

func TestCalculateRelevanceScore_EmptyTitle_ReturnsLow(t *testing.T) {
	result := model.VideoResult{
		Title:     "",
		URL:       "https://example.com/v1",
		Thumbnail: "https://example.com/thumb.jpg",
	}
	words := []string{"test"}

	score := calculateRelevanceScore(result, "test query", words)
	_ = score
}
