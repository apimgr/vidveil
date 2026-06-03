// SPDX-License-Identifier: MIT
// Tests for suggestions.go, performers.go, and manager helper functions
// that do not require network access.
package engine

import (
	"context"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// --- SetCustomTerms / getAllSuggestions ---

func TestSetCustomTerms_NoCustomTerms(t *testing.T) {
	origCustom := customTerms
	t.Cleanup(func() { customTerms = origCustom })

	customTerms = nil
	got := getAllSuggestions()
	if len(got) != len(SearchSuggestions) {
		t.Errorf("getAllSuggestions with no custom terms = %d, want %d", len(got), len(SearchSuggestions))
	}
}

func TestSetCustomTerms_AddsCustomTerms(t *testing.T) {
	origCustom := customTerms
	t.Cleanup(func() { customTerms = origCustom })

	SetCustomTerms([]string{"custom1", "custom2"})
	got := getAllSuggestions()
	wantLen := len(SearchSuggestions) + 2
	if len(got) != wantLen {
		t.Errorf("getAllSuggestions with 2 custom terms = %d, want %d", len(got), wantLen)
	}
}

func TestSetCustomTerms_ReplacesExistingCustomTerms(t *testing.T) {
	origCustom := customTerms
	t.Cleanup(func() { customTerms = origCustom })

	SetCustomTerms([]string{"first"})
	SetCustomTerms([]string{"second", "third"})
	got := getAllSuggestions()
	wantLen := len(SearchSuggestions) + 2
	if len(got) != wantLen {
		t.Errorf("after replacement, getAllSuggestions = %d, want %d", len(got), wantLen)
	}
}

func TestSetCustomTerms_EmptySliceClearsCustom(t *testing.T) {
	origCustom := customTerms
	t.Cleanup(func() { customTerms = origCustom })

	SetCustomTerms([]string{"term"})
	SetCustomTerms(nil)
	got := getAllSuggestions()
	if len(got) != len(SearchSuggestions) {
		t.Errorf("getAllSuggestions after SetCustomTerms(nil) = %d, want %d", len(got), len(SearchSuggestions))
	}
}

// --- AutocompleteSuggestions ---

func TestAutocompleteSuggestions_EmptyPrefix(t *testing.T) {
	got := AutocompleteSuggestions("", 10)
	if got != nil {
		t.Errorf("AutocompleteSuggestions('') = %v, want nil", got)
	}
}

func TestAutocompleteSuggestions_ZeroMax(t *testing.T) {
	got := AutocompleteSuggestions("teen", 0)
	if got != nil {
		t.Errorf("AutocompleteSuggestions with maxResults=0 = %v, want nil", got)
	}
}

func TestAutocompleteSuggestions_OnlyOneChar(t *testing.T) {
	got := AutocompleteSuggestions("t", 10)
	if got != nil {
		t.Errorf("AutocompleteSuggestions('t') = %v, want nil (too short)", got)
	}
}

func TestAutocompleteSuggestions_PrefixMatch(t *testing.T) {
	got := AutocompleteSuggestions("te", 10)
	if len(got) == 0 {
		t.Error("AutocompleteSuggestions('te') returned no results, want at least one ('teen')")
	}
}

func TestAutocompleteSuggestions_RespectsLimit(t *testing.T) {
	got := AutocompleteSuggestions("a", 3)
	if len(got) > 3 {
		t.Errorf("AutocompleteSuggestions with limit=3 returned %d results", len(got))
	}
}

func TestAutocompleteSuggestions_SortedByScore(t *testing.T) {
	got := AutocompleteSuggestions("an", 20)
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("results not sorted by score: got[%d].Score=%d > got[%d].Score=%d",
				i, got[i].Score, i-1, got[i-1].Score)
		}
	}
}

func TestAutocompleteSuggestions_ContainsMatch(t *testing.T) {
	// "milf" contains "il" — should appear as a contains-match.
	got := AutocompleteSuggestions("il", 20)
	if len(got) == 0 {
		t.Error("AutocompleteSuggestions('il') returned no results")
	}
}

// --- GetPopularSearches ---

func TestGetPopularSearches_ReturnsRequestedCount(t *testing.T) {
	got := GetPopularSearches(5)
	if len(got) != 5 {
		t.Errorf("GetPopularSearches(5) = %d, want 5", len(got))
	}
}

func TestGetPopularSearches_ExcessiveCountClamped(t *testing.T) {
	got := GetPopularSearches(9999)
	if len(got) == 0 {
		t.Error("GetPopularSearches(9999) returned empty slice")
	}
}

func TestGetPopularSearches_ZeroReturnsEmpty(t *testing.T) {
	got := GetPopularSearches(0)
	if len(got) != 0 {
		t.Errorf("GetPopularSearches(0) = %d, want 0", len(got))
	}
}

func TestGetPopularSearches_AllStringsNonEmpty(t *testing.T) {
	got := GetPopularSearches(10)
	for i, s := range got {
		if s == "" {
			t.Errorf("GetPopularSearches result[%d] is empty string", i)
		}
	}
}

// --- GetCategorizedSuggestions ---

func TestGetCategorizedSuggestions_ReturnsCategories(t *testing.T) {
	cats := GetCategorizedSuggestions()
	if len(cats) == 0 {
		t.Error("GetCategorizedSuggestions returned no categories")
	}
}

func TestGetCategorizedSuggestions_EachCategoryHasTerms(t *testing.T) {
	cats := GetCategorizedSuggestions()
	for _, cat := range cats {
		if cat.Name == "" {
			t.Error("category has empty Name")
		}
		if cat.DisplayName == "" {
			t.Errorf("category %q has empty DisplayName", cat.Name)
		}
		if len(cat.Terms) == 0 {
			t.Errorf("category %q has no Terms", cat.Name)
		}
	}
}

// --- AutocompleteCombined ---

func TestAutocompleteCombined_EmptyPrefix(t *testing.T) {
	got := AutocompleteCombined("", 10)
	if got != nil {
		t.Errorf("AutocompleteCombined('') = %v, want nil", got)
	}
}

func TestAutocompleteCombined_ZeroMax(t *testing.T) {
	got := AutocompleteCombined("teen", 0)
	if got != nil {
		t.Errorf("AutocompleteCombined with maxResults=0 = %v, want nil", got)
	}
}

func TestAutocompleteCombined_TooShortPrefix(t *testing.T) {
	got := AutocompleteCombined("a", 10)
	if got != nil {
		t.Errorf("AutocompleteCombined('a') = %v, want nil (too short)", got)
	}
}

func TestAutocompleteCombined_ReturnsResults(t *testing.T) {
	got := AutocompleteCombined("te", 10)
	if len(got) == 0 {
		t.Error("AutocompleteCombined('te', 10) returned no results")
	}
}

func TestAutocompleteCombined_RespectsLimit(t *testing.T) {
	got := AutocompleteCombined("an", 3)
	if len(got) > 3 {
		t.Errorf("AutocompleteCombined limit=3 returned %d results", len(got))
	}
}

func TestAutocompleteCombined_NoDuplicates(t *testing.T) {
	got := AutocompleteCombined("an", 50)
	seen := make(map[string]bool)
	for _, s := range got {
		key := s.Term
		if seen[key] {
			t.Errorf("duplicate term %q in AutocompleteCombined results", key)
		}
		seen[key] = true
	}
}

func TestAutocompleteCombined_PerformerPrefix(t *testing.T) {
	// Just verify no panic — some prefixes may match performers only.
	_ = AutocompleteCombined("ri", 20)
}

// --- GetRelatedSearches ---

func TestGetRelatedSearches_EmptyQueryReturnsNil(t *testing.T) {
	got := GetRelatedSearches("", 10)
	if got != nil {
		t.Errorf("GetRelatedSearches('') = %v, want nil", got)
	}
}

func TestGetRelatedSearches_ZeroMaxReturnsNil(t *testing.T) {
	got := GetRelatedSearches("teen", 0)
	if got != nil {
		t.Errorf("GetRelatedSearches with maxResults=0 = %v, want nil", got)
	}
}

func TestGetRelatedSearches_ReturnsResults(t *testing.T) {
	got := GetRelatedSearches("teen", 5)
	if len(got) == 0 {
		t.Error("GetRelatedSearches('teen', 5) returned no results")
	}
}

func TestGetRelatedSearches_RespectsLimit(t *testing.T) {
	got := GetRelatedSearches("asian", 3)
	if len(got) > 3 {
		t.Errorf("GetRelatedSearches limit=3 returned %d", len(got))
	}
}

func TestGetRelatedSearches_DoesNotReturnQueryItself(t *testing.T) {
	got := GetRelatedSearches("milf", 10)
	for _, term := range got {
		if term == "milf" {
			t.Error("GetRelatedSearches should not return the query itself")
		}
	}
}

// --- AutocompletePerformers ---

func TestAutocompletePerformers_EmptyPrefixReturnsNil(t *testing.T) {
	got := AutocompletePerformers("", 10)
	if got != nil {
		t.Errorf("AutocompletePerformers('') = %v, want nil", got)
	}
}

func TestAutocompletePerformers_ZeroMaxReturnsNil(t *testing.T) {
	got := AutocompletePerformers("riley", 0)
	if got != nil {
		t.Errorf("AutocompletePerformers with maxResults=0 = %v, want nil", got)
	}
}

func TestAutocompletePerformers_TooShortReturnsNil(t *testing.T) {
	got := AutocompletePerformers("r", 10)
	if got != nil {
		t.Errorf("AutocompletePerformers('r') = %v, want nil (too short)", got)
	}
}

func TestAutocompletePerformers_KnownPerformerMatch(t *testing.T) {
	got := AutocompletePerformers("riley", 10)
	if len(got) == 0 {
		t.Error("AutocompletePerformers('riley') returned 0 results, want at least 1")
	}
}

func TestAutocompletePerformers_RespectsLimit(t *testing.T) {
	got := AutocompletePerformers("a", 2)
	if len(got) > 2 {
		t.Errorf("AutocompletePerformers limit=2 returned %d results", len(got))
	}
}

func TestAutocompletePerformers_SortedByScore(t *testing.T) {
	got := AutocompletePerformers("an", 20)
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("results not sorted: got[%d].Score=%d > got[%d].Score=%d",
				i, got[i].Score, i-1, got[i-1].Score)
		}
	}
}

// --- GetPopularPerformers ---

func TestGetPopularPerformers_ReturnsRequestedCount(t *testing.T) {
	got := GetPopularPerformers(5)
	if len(got) != 5 {
		t.Errorf("GetPopularPerformers(5) = %d, want 5", len(got))
	}
}

func TestGetPopularPerformers_CappedAt20(t *testing.T) {
	got := GetPopularPerformers(999)
	if len(got) > 20 {
		t.Errorf("GetPopularPerformers(999) = %d, want <= 20", len(got))
	}
}

func TestGetPopularPerformers_ZeroReturnsEmpty(t *testing.T) {
	got := GetPopularPerformers(0)
	if len(got) != 0 {
		t.Errorf("GetPopularPerformers(0) = %d, want 0", len(got))
	}
}

// --- isAIGeneratedContent ---

// keywordInTitle tests whether a keyword match in the title is detected.
func TestIsAIGeneratedContent_MatchInTitle(t *testing.T) {
	// "deepfake" is a known filter keyword; test that title match works.
	if !isAIGeneratedContent("deepfake video", nil, []string{"deepfake"}) {
		t.Error("isAIGeneratedContent should return true when keyword in title")
	}
}

// keywordCaseInsensitive tests case-insensitive matching on title.
func TestIsAIGeneratedContent_MatchInTitleCaseInsensitive(t *testing.T) {
	if !isAIGeneratedContent("deepfake video", nil, []string{"DEEPFAKE"}) {
		t.Error("isAIGeneratedContent should be case-insensitive for title match")
	}
}

func TestIsAIGeneratedContent_MatchInTags(t *testing.T) {
	if !isAIGeneratedContent("normal title", []string{"deepfake", "real"}, []string{"deepfake"}) {
		t.Error("isAIGeneratedContent should return true when keyword in tags")
	}
}

func TestIsAIGeneratedContent_NoMatch(t *testing.T) {
	if isAIGeneratedContent("normal title", []string{"amateur"}, []string{"deepfake"}) {
		t.Error("isAIGeneratedContent should return false when no keyword matches")
	}
}

func TestIsAIGeneratedContent_EmptyKeywords(t *testing.T) {
	if isAIGeneratedContent("deepfake video", []string{"deepfake"}, nil) {
		t.Error("isAIGeneratedContent with empty keywords should return false")
	}
}

func TestIsAIGeneratedContent_EmptyTitle(t *testing.T) {
	if isAIGeneratedContent("", nil, []string{"deepfake"}) {
		t.Error("isAIGeneratedContent with empty title and no tags should return false")
	}
}

// --- resultMatchesAllTerms ---

func TestResultMatchesAllTerms_EmptyQuery(t *testing.T) {
	r := model.VideoResult{Title: "something"}
	// Empty query expands to nothing — return true (nothing to match against).
	if !resultMatchesAllTerms(r, "") {
		t.Error("resultMatchesAllTerms with empty query should return true")
	}
}

func TestResultMatchesAllTerms_MatchingTitle(t *testing.T) {
	r := model.VideoResult{Title: "Beautiful Asian Teen Amateur"}
	if !resultMatchesAllTerms(r, "amateur") {
		t.Error("resultMatchesAllTerms should return true when query matches title")
	}
}

func TestResultMatchesAllTerms_MatchingTags(t *testing.T) {
	r := model.VideoResult{
		Title: "Normal Title",
		Tags:  []string{"blonde", "amateur"},
	}
	if !resultMatchesAllTerms(r, "blonde") {
		t.Error("resultMatchesAllTerms should return true when query matches tags")
	}
}

func TestResultMatchesAllTerms_MatchingPerformer(t *testing.T) {
	r := model.VideoResult{
		Title:     "Normal Title",
		Performer: "Riley Reid",
	}
	if !resultMatchesAllTerms(r, "riley") {
		t.Error("resultMatchesAllTerms should return true when query matches performer")
	}
}

// --- SetTorProvider ---

func TestSetTorProvider_Nil_NoEngines(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetTorProvider(nil) panicked: %v", r)
		}
	}()
	m.SetTorProvider(nil)
}

func TestSetTorProvider_Nil_WithEngines(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetTorProvider(nil) with engines panicked: %v", r)
		}
	}()
	m.SetTorProvider(nil)
}

// --- applyConfig via InitializeEngines ---

func TestApplyConfig_DefaultEnginesEmpty_AllEnabled(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	enabled := m.EnabledCount()
	if enabled == 0 {
		t.Error("EnabledCount() = 0 after InitializeEngines with empty DefaultEngines")
	}
}

func TestApplyConfig_DefaultEnginesSpecified_LimitsEnabled(t *testing.T) {
	cfg := &config.AppConfig{}
	cfg.Search.DefaultEngines = []string{"pornhub", "xvideos"}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	enabled := m.EnabledCount()
	if enabled != 2 {
		t.Errorf("EnabledCount() = %d with 2 DefaultEngines, want 2", enabled)
	}
}

// --- getEnginesToUse ---

func TestGetEnginesToUse_EmptyNames_UsesAllEnabled(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	m.mu.RLock()
	engines := m.getEnginesToUse(nil)
	m.mu.RUnlock()

	if len(engines) == 0 {
		t.Error("getEnginesToUse(nil) returned no engines")
	}
}

func TestGetEnginesToUse_ByName_ReturnsSpecifiedEngine(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	m.mu.RLock()
	engines := m.getEnginesToUse([]string{"pornhub"})
	m.mu.RUnlock()

	if len(engines) != 1 {
		t.Errorf("getEnginesToUse(['pornhub']) returned %d engines, want 1", len(engines))
	}
}

func TestGetEnginesToUse_Tier1_ReturnsOnlyTier1(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	m.mu.RLock()
	engines := m.getEnginesToUse([]string{"tier1"})
	m.mu.RUnlock()

	for _, e := range engines {
		if e.Tier() != 1 {
			t.Errorf("tier1 filter returned engine %q with tier %d", e.Name(), e.Tier())
		}
	}
}

func TestGetEnginesToUse_Tier12_ReturnsTiers1And2(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	m.mu.RLock()
	engines := m.getEnginesToUse([]string{"tier12"})
	m.mu.RUnlock()

	for _, e := range engines {
		if e.Tier() > 2 {
			t.Errorf("tier12 filter returned engine %q with tier %d > 2", e.Name(), e.Tier())
		}
	}
}

func TestGetEnginesToUse_UnknownName_ReturnsEmpty(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	m.InitializeEngines()

	m.mu.RLock()
	engines := m.getEnginesToUse([]string{"no-such-engine"})
	m.mu.RUnlock()

	if len(engines) != 0 {
		t.Errorf("getEnginesToUse(['no-such-engine']) returned %d engines, want 0", len(engines))
	}
}

// --- SearchStream (empty engine list — channel closes immediately) ---

func TestSearchStream_NoEnginesClosesChannel(t *testing.T) {
	cfg := &config.AppConfig{}
	m := NewEngineManager(cfg)
	// No InitializeEngines — no engines registered.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := m.SearchStream(ctx, "test", 1, nil)

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Error("SearchStream channel did not close")
	}
}
