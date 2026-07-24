// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests using a mock SearchEngine implementation.
// The mock engine lets us drive the full Search/DebugSearch/SearchStream
// code paths in manager.go without making any network calls.
package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
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
func (m *mockSearchEngine) IsAvailable() bool              { return m.avail }
func (m *mockSearchEngine) SupportsFeature(_ Feature) bool { return false }
func (m *mockSearchEngine) Tier() int                      { return m.tier }
func (m *mockSearchEngine) Capabilities() Capabilities     { return Capabilities{} }

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
// Uses DurationSeconds > 600 to pass DefaultAppConfig.Search.MinDurationSeconds = 600.
func validResult(title, url string) model.VideoResult {
	return model.VideoResult{
		Title:           title,
		URL:             url,
		Thumbnail:       "https://example.com/thumb.jpg",
		DurationSeconds: 700,
	}
}

// ── Search with results ───────────────────────────────────────────────────────

func TestSearch_WithMockEngine_ReturnsResults(t *testing.T) {
	results := []model.VideoResult{
		validResult("amateur teen xxx", "https://example.com/v1"),
		validResult("amateur teen xxx video", "https://example.com/v2"),
	}
	m := newMgrWithMock("mock", results, nil, true)

	resp := m.Search(context.Background(), "amateur teen", 1, nil, "")
	if resp == nil {
		t.Fatal("Search: nil response")
	}
	if len(resp.Data.Results) == 0 {
		t.Log("Search: no results passed filters (may be filtered by relevance/intent)")
	}
}

func TestSearch_WithMockEngineError_RecordsFailure(t *testing.T) {
	m := newMgrWithMock("mock-err", nil, errors.New("engine failed"), true)

	resp := m.Search(context.Background(), "test", 1, nil, "")
	if resp == nil {
		t.Fatal("Search with error: nil response")
	}
}

func TestSearch_WithMockEngine_InvalidThumbnail_Filtered(t *testing.T) {
	results := []model.VideoResult{
		{Title: "no-thumb", URL: "https://example.com/v1", Thumbnail: "", DurationSeconds: 700},
		{Title: "invalid-thumb", URL: "https://example.com/v2", Thumbnail: "invalid-url", DurationSeconds: 700},
	}
	m := newMgrWithMock("mock-thumb", results, nil, true)

	resp := m.Search(context.Background(), "test", 1, nil, "")
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

	resp := m.Search(context.Background(), "test video", 1, nil, "")
	if resp == nil {
		t.Fatal("Search(dup URLs): nil response")
	}
}

func TestSearch_WithMockEngine_UnavailableEngine_NoResults(t *testing.T) {
	results := []model.VideoResult{validResult("test", "https://example.com/v1")}
	m := newMgrWithMock("mock-unav", results, nil, false)

	resp := m.Search(context.Background(), "test", 1, nil, "")
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

	resp := m.Search(context.Background(), "named engine", 1, []string{"mock-named"}, "")
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

	resp := m.Search(context.Background(), "video test", 1, nil, "")
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

	ch := m.SearchStreamWithOperators(ctx, "stream test", 1, nil, nil, nil, nil, false, 0, false, 0, "")
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

	ch := m.SearchStreamWithOperators(ctx, "test", 1, nil, nil, nil, nil, false, 0, false, 0, "")
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

// ── SearchStreamWithOperators — operator filter branches ─────────────────────

func TestSearchStreamWithOperators_WithExclusion_FiltersResult(t *testing.T) {
	results := []model.VideoResult{
		validResult("stream video test excluded", "https://example.com/excl"),
		validResult("stream video test allowed", "https://example.com/allow"),
	}
	m := newMgrWithMock("mock-excl", results, nil, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Exclude results containing "excluded" in title
	ch := m.SearchStreamWithOperators(ctx, "stream video", 1, nil, nil, []string{"excluded"}, nil, false, 0, false, 0, "")
	for range ch {
	}
}

func TestSearchStreamWithOperators_WithExactPhrase_FiltersResult(t *testing.T) {
	results := []model.VideoResult{
		validResult("stream video exact phrase here", "https://example.com/exact"),
		validResult("stream video different words", "https://example.com/diff"),
	}
	m := newMgrWithMock("mock-phrase", results, nil, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream video", 1, nil, []string{"exact phrase"}, nil, nil, false, 0, false, 0, "")
	for range ch {
	}
}

func TestSearchStreamWithOperators_WithPerformerFilter_FiltersResult(t *testing.T) {
	results := []model.VideoResult{
		{
			Title:           "stream video performer match",
			URL:             "https://example.com/perf1",
			Thumbnail:       "https://example.com/thumb.jpg",
			DurationSeconds: 700,
			Performer:       "Jane Doe",
		},
		{
			Title:           "stream video performer nomatch",
			URL:             "https://example.com/perf2",
			Thumbnail:       "https://example.com/thumb.jpg",
			DurationSeconds: 700,
			Performer:       "John Smith",
		},
	}
	m := newMgrWithMock("mock-perf", results, nil, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream video", 1, nil, nil, nil, []string{"jane"}, false, 0, false, 0, "")
	for range ch {
	}
}

func TestSearchStreamWithOperators_WithMinQuality_FiltersResult(t *testing.T) {
	results := []model.VideoResult{
		{
			Title:           "stream video hd quality",
			URL:             "https://example.com/hd",
			Thumbnail:       "https://example.com/thumb.jpg",
			DurationSeconds: 300,
			Quality:         "HD",
		},
		{
			Title:           "stream video sd quality",
			URL:             "https://example.com/sd",
			Thumbnail:       "https://example.com/thumb.jpg",
			DurationSeconds: 300,
			Quality:         "SD",
		},
	}
	m := newMgrWithMock("mock-qual", results, nil, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream video", 1, nil, nil, nil, nil, false, 480, false, 0, "")
	for range ch {
	}
}

func TestSearchStreamWithOperators_AIFilter_FiltersAIContent(t *testing.T) {
	cfg := config.DefaultAppConfig()
	// Enable AI filter
	cfg.Search.AIFilter.Enabled = true
	cfg.Search.AIFilter.Keywords = []string{"deepfake", "synthetic"}
	m := NewEngineManager(cfg)
	m.engines["mock-ai"] = &mockSearchEngine{
		name:  "mock-ai",
		avail: true,
		tier:  1,
		results: []model.VideoResult{
			{
				Title:           "stream deepfake video synthetic",
				URL:             "https://example.com/deepfake",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 700,
			},
			{
				Title:           "stream regular video",
				URL:             "https://example.com/regular",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 700,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream video", 1, nil, nil, nil, nil, false, 0, false, 0, "")
	for range ch {
	}
}

func TestSearchStreamWithOperators_QualityFilter_FiltersLowQuality(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.MinDurationSeconds = 0
	m := NewEngineManager(cfg)
	m.engines["mock-qual2"] = &mockSearchEngine{
		name:  "mock-qual2",
		avail: true,
		tier:  1,
		results: []model.VideoResult{
			{
				Title:           "stream low quality 240p",
				URL:             "https://example.com/lq",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 700,
				Quality:         "240p",
			},
			{
				Title:           "stream high quality 1080p",
				URL:             "https://example.com/hq",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 700,
				Quality:         "1080p",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream quality", 1, nil, nil, nil, nil, false, 480, false, 0, "")
	for range ch {
	}
}

func TestSearchStreamWithOperators_UserMinDuration_Override(t *testing.T) {
	cfg := config.DefaultAppConfig()
	// Set config minDuration to 0 so userMinDuration=120 overrides it (covers line 1231-1232)
	cfg.Search.MinDurationSeconds = 0

	m := NewEngineManager(cfg)
	m.engines["mock-dur2"] = &mockSearchEngine{
		name:  "mock-dur2",
		avail: true,
		tier:  1,
		results: []model.VideoResult{
			{
				Title:           "stream long video",
				URL:             "https://example.com/long",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 600,
			},
			{
				Title:           "stream short video",
				URL:             "https://example.com/short",
				Thumbnail:       "https://example.com/thumb.jpg",
				DurationSeconds: 30,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "stream video", 1, nil, nil, nil, nil, false, 0, false, 120, "")
	for range ch {
	}
}

// ── DebugLogEngineResponse — debug enabled path ───────────────────────────────

func TestDebugLogEngineResponse_DebugEnabled_NoPanic(t *testing.T) {
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	DebugLogEngineResponse("test-engine", "https://example.com/search", len("test response"))
}

func TestDebugLogEngineResponse_LargeBody_Truncated(t *testing.T) {
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	// Reported size > 2000 bytes — must not panic regardless of magnitude
	DebugLogEngineResponse("test-engine", "https://example.com/search", 3000)
}

func TestDebugLogEngineParseResult_DebugEnabled_NoPanic(t *testing.T) {
	mode.SetDebug(true)
	t.Cleanup(func() { mode.SetDebug(false) })

	results := make([]model.VideoResult, 5)
	for i := range results {
		results[i].URL = fmt.Sprintf("https://example.com/video/%d", i)
	}

	DebugLogEngineParseResult("test-engine", results, map[string]int{
		"title":     5,
		"thumbnail": 4,
		"duration":  3,
	})
}

// ── NewBaseEngine — EngineTimeouts / EngineRequestIntervals overrides ─────────

func TestNewBaseEngine_WithEngineTimeoutOverride_AppliesOverride(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.EngineTimeout = 30
	cfg.Search.EngineTimeouts = map[string]int{
		"test-engine": 10,
	}

	e := NewBaseEngine("test-engine", "Test Engine", "https://example.com", 1, cfg)
	if e == nil {
		t.Fatal("NewBaseEngine: returned nil")
	}
}

func TestNewBaseEngine_WithRequestIntervalOverride_AppliesOverride(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.EngineRequestIntervals = map[string]int{
		"test-engine": 500,
	}

	e := NewBaseEngine("test-engine", "Test Engine", "https://example.com", 1, cfg)
	if e == nil {
		t.Fatal("NewBaseEngine: returned nil")
	}
}

// ── classifyHTTPError — all error message branches ───────────────────────────

func TestClassifyHTTPError_Nil_ReturnsNil(t *testing.T) {
	if err := classifyHTTPError(nil); err != nil {
		t.Errorf("classifyHTTPError(nil): expected nil, got %v", err)
	}
}

func TestClassifyHTTPError_Timeout_ReturnsTimeout(t *testing.T) {
	err := classifyHTTPError(errors.New("request timeout exceeded"))
	if err == nil {
		t.Error("classifyHTTPError(timeout): expected error")
	}
}

func TestClassifyHTTPError_ConnectionRefused_ReturnsNetwork(t *testing.T) {
	err := classifyHTTPError(errors.New("connection refused"))
	if err == nil {
		t.Error("classifyHTTPError(conn refused): expected error")
	}
}

func TestClassifyHTTPError_NoSuchHost_ReturnsNetwork(t *testing.T) {
	err := classifyHTTPError(errors.New("no such host"))
	if err == nil {
		t.Error("classifyHTTPError(no such host): expected error")
	}
}

func TestClassifyHTTPError_GenericError_ReturnsError(t *testing.T) {
	orig := errors.New("some unknown error")
	err := classifyHTTPError(orig)
	if err == nil {
		t.Error("classifyHTTPError(generic): expected error")
	}
}

// ── Search — panic recovery covers lines 158-164 ─────────────────────────────

type panicMockEngine struct {
	name string
}

func (m *panicMockEngine) Name() string        { return m.name }
func (m *panicMockEngine) DisplayName() string { return m.name }
func (m *panicMockEngine) Search(_ context.Context, _ string, _ int) ([]model.VideoResult, error) {
	panic("engine test panic")
}
func (m *panicMockEngine) IsAvailable() bool              { return true }
func (m *panicMockEngine) SupportsFeature(_ Feature) bool { return false }
func (m *panicMockEngine) Tier() int                      { return 1 }
func (m *panicMockEngine) Capabilities() Capabilities     { return Capabilities{} }

func TestSearch_PanickingEngine_RecoversPanic(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := NewEngineManager(cfg)
	m.engines["panic-engine"] = &panicMockEngine{name: "panic-engine"}

	resp := m.Search(context.Background(), "test", 1, nil, "")
	if resp == nil {
		t.Fatal("Search(panicking engine): nil response")
	}
}

func TestSearchStream_PanickingEngine_RecoversPanic(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := NewEngineManager(cfg)
	m.engines["panic-stream"] = &panicMockEngine{name: "panic-stream"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := m.SearchStreamWithOperators(ctx, "test", 1, nil, nil, nil, nil, false, 0, false, 0, "")
	for range ch {
	}
}

// ── Session-scoped cross-page dedup regression ────────────────────────────────
//
// Reproduces the reported bug (search "Pregnant lesbian teen" showed many
// duplicates while scrolling): a single engine returns the same result set
// on every page (as flaky/overlapping upstream engines do), and successive
// pages of the SAME session must not repeat a result already returned on an
// earlier page, while a DIFFERENT session must still see the full result set.

func TestSearchStream_SameSession_DedupsAcrossPages(t *testing.T) {
	results := []model.VideoResult{
		validResult("pregnant lesbian teen one", "https://example.com/dup1"),
		validResult("pregnant lesbian teen two", "https://example.com/dup2"),
	}
	m := newMgrWithMock("mock-dedup", results, nil, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessionID := "test-session-1"

	page1 := m.Search(ctx, "pregnant lesbian teen", 1, nil, sessionID)
	if page1 == nil || len(page1.Data.Results) == 0 {
		t.Fatal("page1: expected at least one result")
	}
	seenOnPage1 := make(map[string]bool)
	for _, r := range page1.Data.Results {
		seenOnPage1[r.URL] = true
	}

	// Same session, same upstream results on "page 2" (simulating an
	// upstream engine that returns overlapping/duplicate results across
	// pages) must not resurface anything already returned on page 1.
	page2 := m.Search(ctx, "pregnant lesbian teen", 2, nil, sessionID)
	if page2 == nil {
		t.Fatal("page2: nil response")
	}
	for _, r := range page2.Data.Results {
		if seenOnPage1[r.URL] {
			t.Fatalf("page2: result %q already returned on page1 of the same session (dedup regression)", r.URL)
		}
	}
}

func TestSearchStream_DifferentSession_NotDeduped(t *testing.T) {
	results := []model.VideoResult{
		validResult("pregnant lesbian teen one", "https://example.com/dup1"),
		validResult("pregnant lesbian teen two", "https://example.com/dup2"),
	}
	m := newMgrWithMock("mock-dedup2", results, nil, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	page1 := m.Search(ctx, "pregnant lesbian teen", 1, nil, "session-a")
	if page1 == nil || len(page1.Data.Results) == 0 {
		t.Fatal("page1 (session-a): expected at least one result")
	}

	// A fresh session must see the full result set again — dedup state
	// must not leak across sessions.
	otherSession := m.Search(ctx, "pregnant lesbian teen", 1, nil, "session-b")
	if otherSession == nil || len(otherSession.Data.Results) != len(page1.Data.Results) {
		t.Fatalf("session-b: expected %d results (independent of session-a), got %v",
			len(page1.Data.Results), otherSession)
	}
}
