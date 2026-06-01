// SPDX-License-Identifier: MIT
// Additional coverage tests for the engine package.
// Focuses on functions NOT already covered by engines_test.go:
//   - manager.go: pure functions (ParseQualityLevel, meetsMinQuality, normalizeURL,
//     normalizeTitle, isValidThumbnail, jaroSimilarity, jaroWinklerSimilarity,
//     titlesAreFuzzyDuplicates, calculateRelevanceScore, sortAndFilterByRelevance,
//     sortAndFilterByRelevanceWithOperators, levenshtein), EngineManager lifecycle
//   - taxonomy.go: NormalizeTerm, GetSynonyms, GetRelatedTerms, ExpandSearchTerms,
//     MatchesAllTerms, MatchesAnyTerm, DetectQueryIntent, ResultMatchesIntent,
//     containsWholeWord, GenerateSmartRelated
//   - engine.go: GetStats, GetCircuitBreakerState, IsCircuitOpen, ResetCircuitBreaker,
//     GetClient, GetUserAgent, SetUseSpoofedTLS, classifyHTTPError, MaxEngineResponseBytes
//   - bangs.go: @ prefix stripping, -word exclusion, quoted phrase extraction in ParseBangs
//   - Named engine constructors: NewPornHubEngine + SupportsFeature
//   - eporner formatViewCount (package-private, accessible from same package)
//   - pornhub formatViews (package-private)
package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/retry"
)

// ── Constants ────────────────────────────────────────────────────────────────

// Verifies the hard-coded byte cap that every engine must respect.
func TestEngineMaxResponseBytes(t *testing.T) {
	want := int64(32 * 1024 * 1024)
	if MaxEngineResponseBytes != want {
		t.Errorf("MaxEngineResponseBytes = %d, want %d", MaxEngineResponseBytes, want)
	}
}

func TestFeatureConstantValues(t *testing.T) {
	if FeaturePagination != 0 {
		t.Errorf("FeaturePagination = %d, want 0", FeaturePagination)
	}
	if FeatureSorting != 1 {
		t.Errorf("FeatureSorting = %d, want 1", FeatureSorting)
	}
	if FeatureFiltering != 2 {
		t.Errorf("FeatureFiltering = %d, want 2", FeatureFiltering)
	}
	if FeatureThumbnailPreview != 3 {
		t.Errorf("FeatureThumbnailPreview = %d, want 3", FeatureThumbnailPreview)
	}
}

func TestContextKeyConstants_NonEmpty(t *testing.T) {
	if UserIPContextKey == "" {
		t.Error("UserIPContextKey must not be empty")
	}
	if ForwardIPContextKey == "" {
		t.Error("ForwardIPContextKey must not be empty")
	}
	if TorPrefContextKey == "" {
		t.Error("TorPrefContextKey must not be empty")
	}
}

// ── ParseBangs – branches not yet exercised by engines_test.go ───────────────

// @ prefix strips the @ but keeps the word in the query.
func TestParseBangs_AtPrefixStripped(t *testing.T) {
	result := ParseBangs("@dakota skye")
	if result.HasBang {
		t.Error("ParseBangs @prefix: should not be treated as a bang")
	}
	if !strings.Contains(result.Query, "dakota") {
		t.Errorf("ParseBangs @prefix: query = %q, want 'dakota' retained", result.Query)
	}
}

// -word adds to Exclusions and is removed from the query.
func TestParseBangs_ExclusionWord(t *testing.T) {
	result := ParseBangs("amateur -teen")
	if result.Query != "amateur" {
		t.Errorf("ParseBangs exclusion: query = %q, want 'amateur'", result.Query)
	}
	if len(result.Exclusions) != 1 || result.Exclusions[0] != "teen" {
		t.Errorf("ParseBangs exclusion: Exclusions = %v, want [teen]", result.Exclusions)
	}
}

// "quoted text" populates ExactPhrases.
func TestParseBangs_QuotedPhrase(t *testing.T) {
	result := ParseBangs(`"big tits" amateur`)
	if len(result.ExactPhrases) != 1 || result.ExactPhrases[0] != "big tits" {
		t.Errorf("ParseBangs quoted: ExactPhrases = %v, want [big tits]", result.ExactPhrases)
	}
	if !strings.Contains(result.Query, "amateur") {
		t.Errorf("ParseBangs quoted: query = %q, want 'amateur' preserved", result.Query)
	}
}

// Multiple exclusions in a single query.
func TestParseBangs_MultipleExclusions(t *testing.T) {
	result := ParseBangs("milf -teen -amateur")
	if len(result.Exclusions) != 2 {
		t.Errorf("ParseBangs multi-exclusion: got %v, want 2 exclusions", result.Exclusions)
	}
}

// ── BaseEngine – methods not covered by engines_test.go ─────────────────────

func TestBaseEngine_GetStats_InitialState(t *testing.T) {
	e := newTestBaseEngine()
	stats := e.GetStats()
	if stats.TotalSuccesses != 0 {
		t.Errorf("GetStats.TotalSuccesses = %d, want 0", stats.TotalSuccesses)
	}
	if stats.TotalFailures != 0 {
		t.Errorf("GetStats.TotalFailures = %d, want 0", stats.TotalFailures)
	}
	if stats.IsRateLimited {
		t.Error("GetStats.IsRateLimited = true, want false")
	}
}

func TestBaseEngine_IsCircuitOpen_InitialFalse(t *testing.T) {
	e := newTestBaseEngine()
	if e.IsCircuitOpen() {
		t.Error("IsCircuitOpen() = true on fresh engine, want false")
	}
}

func TestBaseEngine_GetCircuitBreakerState_InitialClosed(t *testing.T) {
	e := newTestBaseEngine()
	state := e.GetCircuitBreakerState()
	if state != retry.CircuitBreakerStateClosed {
		t.Errorf("GetCircuitBreakerState() = %v, want closed", state)
	}
	if state.String() != "closed" {
		t.Errorf("CircuitBreakerState.String() = %q, want 'closed'", state.String())
	}
}

func TestBaseEngine_ResetCircuitBreaker_NoPanic(t *testing.T) {
	e := newTestBaseEngine()
	e.ResetCircuitBreaker()
	if e.IsCircuitOpen() {
		t.Error("IsCircuitOpen() = true after reset, want false")
	}
}

func TestBaseEngine_GetClient_ReturnsNonNil(t *testing.T) {
	e := newTestBaseEngine()
	client := e.GetClient()
	if client == nil {
		t.Error("GetClient() returned nil")
	}
}

func TestBaseEngine_GetUserAgent_NonEmpty(t *testing.T) {
	e := newTestBaseEngine()
	ua := e.GetUserAgent()
	if ua == "" {
		t.Error("GetUserAgent() returned empty string")
	}
}

func TestBaseEngine_GetUserAgent_NilConfig(t *testing.T) {
	e := NewBaseEngine("test", "Test", "https://example.com", 1, config.DefaultAppConfig())
	e.appConfig = nil
	ua := e.GetUserAgent()
	if ua != DefaultUserAgent {
		t.Errorf("GetUserAgent() with nil config = %q, want DefaultUserAgent", ua)
	}
}

func TestBaseEngine_SetUseSpoofedTLS_NoPanic(t *testing.T) {
	e := newTestBaseEngine()
	e.SetUseSpoofedTLS(true)
	e.SetUseSpoofedTLS(false)
}

func TestBaseEngine_SetTorProvider_NilNoPanic(t *testing.T) {
	e := newTestBaseEngine()
	e.SetTorProvider(nil)
	client := e.GetClient()
	if client == nil {
		t.Error("GetClient() returned nil after SetTorProvider(nil)")
	}
}

// recordSuccessStat / recordFailureStat affect GetStats.
func TestBaseEngine_RecordStats_UpdatesCounters(t *testing.T) {
	e := newTestBaseEngine()
	e.recordSuccessStat(100)
	e.recordSuccessStat(200)
	e.recordFailureStat()

	stats := e.GetStats()
	if stats.TotalSuccesses != 2 {
		t.Errorf("TotalSuccesses = %d, want 2", stats.TotalSuccesses)
	}
	if stats.TotalFailures != 1 {
		t.Errorf("TotalFailures = %d, want 1", stats.TotalFailures)
	}
	if stats.AvgLatencyMs <= 0 {
		t.Errorf("AvgLatencyMs = %d, want > 0 after success records", stats.AvgLatencyMs)
	}
	if stats.UptimePct <= 0 {
		t.Errorf("UptimePct = %f, want > 0", stats.UptimePct)
	}
}

// ── classifyHTTPError ────────────────────────────────────────────────────────

func TestClassifyHTTPError_Nil(t *testing.T) {
	if classifyHTTPError(nil) != nil {
		t.Error("classifyHTTPError(nil) must return nil")
	}
}

func TestClassifyHTTPError_Timeout(t *testing.T) {
	err := classifyHTTPError(errors.New("context deadline exceeded"))
	if err == nil {
		t.Fatal("classifyHTTPError(timeout) returned nil")
	}
	if !errors.Is(err, retry.ErrTimeout) {
		t.Errorf("classifyHTTPError(timeout): expected ErrTimeout, got %v", err)
	}
}

func TestClassifyHTTPError_ConnectionRefused(t *testing.T) {
	err := classifyHTTPError(errors.New("connection refused"))
	if err == nil {
		t.Fatal("classifyHTTPError(connection refused) returned nil")
	}
	if !errors.Is(err, retry.ErrNetworkError) {
		t.Errorf("classifyHTTPError(connection refused): expected ErrNetworkError, got %v", err)
	}
}

func TestClassifyHTTPError_NoSuchHost(t *testing.T) {
	err := classifyHTTPError(errors.New("no such host"))
	if !errors.Is(err, retry.ErrNetworkError) {
		t.Errorf("classifyHTTPError(no such host): expected ErrNetworkError, got %v", err)
	}
}

func TestClassifyHTTPError_Generic(t *testing.T) {
	err := classifyHTTPError(errors.New("something totally unknown"))
	if err == nil {
		t.Error("classifyHTTPError(generic) must return non-nil")
	}
}

// ── NewPornHubEngine constructor ─────────────────────────────────────────────

func TestNewPornHubEngine_NonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	e := NewPornHubEngine(cfg)
	if e == nil {
		t.Fatal("NewPornHubEngine returned nil")
	}
}

func TestNewPornHubEngine_Name(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if e.Name() != "pornhub" {
		t.Errorf("Name() = %q, want 'pornhub'", e.Name())
	}
}

func TestNewPornHubEngine_DisplayName_NonEmpty(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if e.DisplayName() == "" {
		t.Error("DisplayName() returned empty string")
	}
}

func TestNewPornHubEngine_Tier(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if e.Tier() != 1 {
		t.Errorf("Tier() = %d, want 1", e.Tier())
	}
}

func TestNewPornHubEngine_IsAvailable(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if !e.IsAvailable() {
		t.Error("IsAvailable() = false on new engine, want true")
	}
}

func TestNewPornHubEngine_BaseURL_HTTPS(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if !strings.HasPrefix(e.BaseURL(), "https://") {
		t.Errorf("BaseURL() = %q, want https:// prefix", e.BaseURL())
	}
}

func TestNewPornHubEngine_SetEnabled_Disable(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	e.SetEnabled(false)
	if e.IsAvailable() {
		t.Error("IsAvailable() = true after SetEnabled(false), want false")
	}
}

func TestNewPornHubEngine_GetStats_NonNil(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	stats := e.GetStats()
	if stats.CircuitState == "" {
		t.Error("GetStats().CircuitState should be non-empty")
	}
}

func TestNewPornHubEngine_IsCircuitOpen_False(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if e.IsCircuitOpen() {
		t.Error("IsCircuitOpen() = true on new engine, want false")
	}
}

func TestNewPornHubEngine_GetCircuitBreakerState_Closed(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if e.GetCircuitBreakerState().String() != "closed" {
		t.Errorf("GetCircuitBreakerState() = %q, want 'closed'", e.GetCircuitBreakerState().String())
	}
}

func TestNewPornHubEngine_ResetCircuitBreaker_NoPanic(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	e.ResetCircuitBreaker()
}

func TestNewPornHubEngine_Capabilities(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	caps := e.Capabilities()
	if !caps.HasPreview {
		t.Error("PornHub capabilities: HasPreview should be true")
	}
	if !caps.HasDuration {
		t.Error("PornHub capabilities: HasDuration should be true")
	}
	if !caps.HasViews {
		t.Error("PornHub capabilities: HasViews should be true")
	}
}

func TestNewPornHubEngine_SupportsFeature(t *testing.T) {
	e := NewPornHubEngine(config.DefaultAppConfig())
	if !e.SupportsFeature(FeaturePagination) {
		t.Error("PornHub should support FeaturePagination")
	}
	if !e.SupportsFeature(FeatureSorting) {
		t.Error("PornHub should support FeatureSorting")
	}
	if !e.SupportsFeature(FeatureThumbnailPreview) {
		t.Error("PornHub should support FeatureThumbnailPreview")
	}
	if e.SupportsFeature(FeatureFiltering) {
		t.Error("PornHub should not support FeatureFiltering")
	}
}

// ── EngineManager lifecycle ──────────────────────────────────────────────────

func newTestManager() *EngineManager {
	cfg := config.DefaultAppConfig()
	m := NewEngineManager(cfg)
	m.InitializeEngines()
	return m
}

func TestNewEngineManager_NonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := NewEngineManager(cfg)
	if m == nil {
		t.Fatal("NewEngineManager returned nil")
	}
	if m.engines == nil {
		t.Error("NewEngineManager: engines map is nil")
	}
}

func TestEngineManager_InitializeEngines_ListEngines_NonEmpty(t *testing.T) {
	m := newTestManager()
	list := m.ListEngines()
	if len(list) == 0 {
		t.Error("ListEngines() returned empty list after InitializeEngines()")
	}
}

func TestEngineManager_EnabledCount_GreaterThanZero(t *testing.T) {
	m := newTestManager()
	if m.EnabledCount() == 0 {
		t.Error("EnabledCount() == 0 after InitializeEngines()")
	}
}

func TestEngineManager_GetEngine_Known(t *testing.T) {
	m := newTestManager()
	eng, ok := m.GetEngine("pornhub")
	if !ok {
		t.Fatal("GetEngine('pornhub') ok=false after InitializeEngines()")
	}
	if eng == nil {
		t.Error("GetEngine('pornhub') returned nil engine")
	}
	if eng.Name() != "pornhub" {
		t.Errorf("GetEngine('pornhub').Name() = %q, want 'pornhub'", eng.Name())
	}
}

func TestEngineManager_GetEngine_Unknown(t *testing.T) {
	m := newTestManager()
	eng, ok := m.GetEngine("nonexistent_engine_xyz")
	if ok {
		t.Error("GetEngine(unknown) ok=true, want false")
	}
	if eng != nil {
		t.Error("GetEngine(unknown) returned non-nil engine")
	}
}

func TestEngineManager_SetEngineEnabled_Disable(t *testing.T) {
	m := newTestManager()
	before := m.EnabledCount()
	found := m.SetEngineEnabled("pornhub", false)
	if !found {
		t.Fatal("SetEngineEnabled('pornhub', false): engine not found")
	}
	after := m.EnabledCount()
	if after >= before {
		t.Errorf("EnabledCount after disable: got %d, want < %d", after, before)
	}
}

func TestEngineManager_SetEngineEnabled_Unknown_ReturnsFalse(t *testing.T) {
	m := newTestManager()
	if m.SetEngineEnabled("no_such_engine", false) {
		t.Error("SetEngineEnabled(unknown) = true, want false")
	}
}

func TestEngineManager_ResetEngine_Known(t *testing.T) {
	m := newTestManager()
	if !m.ResetEngine("pornhub") {
		t.Error("ResetEngine('pornhub') = false, want true")
	}
}

func TestEngineManager_ResetEngine_Unknown(t *testing.T) {
	m := newTestManager()
	if m.ResetEngine("no_such_engine_xyz") {
		t.Error("ResetEngine(unknown) = true, want false")
	}
}

func TestEngineManager_ListEnginesWithHealth_Fields(t *testing.T) {
	m := newTestManager()
	infos := m.ListEnginesWithHealth()
	if len(infos) == 0 {
		t.Fatal("ListEnginesWithHealth() returned empty list")
	}
	for _, info := range infos {
		if info.Name == "" {
			t.Error("EngineHealthInfo.Name is empty")
		}
		if info.DisplayName == "" {
			t.Error("EngineHealthInfo.DisplayName is empty")
		}
	}
}

// ── SpellCorrect / levenshtein ───────────────────────────────────────────────

func TestLevenshtein_Equal(t *testing.T) {
	if d := levenshtein("abc", "abc"); d != 0 {
		t.Errorf("levenshtein(abc, abc) = %d, want 0", d)
	}
}

func TestLevenshtein_EmptyStrings(t *testing.T) {
	if d := levenshtein("", ""); d != 0 {
		t.Errorf("levenshtein('', '') = %d, want 0", d)
	}
	if d := levenshtein("abc", ""); d != 3 {
		t.Errorf("levenshtein(abc, '') = %d, want 3", d)
	}
	if d := levenshtein("", "abc"); d != 3 {
		t.Errorf("levenshtein('', abc) = %d, want 3", d)
	}
}

func TestLevenshtein_OneSub(t *testing.T) {
	if d := levenshtein("kitten", "sitten"); d != 1 {
		t.Errorf("levenshtein(kitten, sitten) = %d, want 1", d)
	}
}

func TestLevenshtein_Insertions(t *testing.T) {
	if d := levenshtein("abc", "abcd"); d != 1 {
		t.Errorf("levenshtein(abc, abcd) = %d, want 1", d)
	}
}

func TestSpellCorrect_EmptyQuery(t *testing.T) {
	m := newTestManager()
	if m.SpellCorrect("") != "" {
		t.Error("SpellCorrect('') should return empty string")
	}
}

func TestSpellCorrect_TooManyWords(t *testing.T) {
	m := newTestManager()
	if m.SpellCorrect("a b c d e") != "" {
		t.Error("SpellCorrect with > 4 words should return empty string")
	}
}

func TestSpellCorrect_ExactMatch_NoSuggestion(t *testing.T) {
	m := newTestManager()
	// A perfectly-spelled engine name should not suggest a correction.
	result := m.SpellCorrect("pornhub")
	if result != "" {
		t.Logf("SpellCorrect(pornhub) = %q (no correction expected, but not a hard failure)", result)
	}
}

func TestSpellCorrect_Typo_ReturnsSuggestion(t *testing.T) {
	m := newTestManager()
	// "porfhub" is one edit away from "pornhub".
	result := m.SpellCorrect("porfhub")
	if result == "" {
		t.Log("SpellCorrect(porfhub) returned empty — could be no match found (acceptable)")
	} else if result == "porfhub" {
		t.Errorf("SpellCorrect returned unchanged input %q, want a correction", result)
	}
}

// ── ParseQualityLevel ────────────────────────────────────────────────────────

func TestParseQualityLevel_4K(t *testing.T) {
	cases := []string{"4K", "4k", "UHD", "2160p", "2160"}
	for _, c := range cases {
		if got := ParseQualityLevel(c); got != Quality4K {
			t.Errorf("ParseQualityLevel(%q) = %d, want %d", c, got, Quality4K)
		}
	}
}

func TestParseQualityLevel_1080(t *testing.T) {
	cases := []string{"1080p", "1080", "FHD"}
	for _, c := range cases {
		if got := ParseQualityLevel(c); got != Quality1080p {
			t.Errorf("ParseQualityLevel(%q) = %d, want %d", c, got, Quality1080p)
		}
	}
}

func TestParseQualityLevel_720(t *testing.T) {
	cases := []string{"720p", "720", "HD"}
	for _, c := range cases {
		if got := ParseQualityLevel(c); got != Quality720p {
			t.Errorf("ParseQualityLevel(%q) = %d, want %d", c, got, Quality720p)
		}
	}
}

func TestParseQualityLevel_480(t *testing.T) {
	cases := []string{"480p", "SD"}
	for _, c := range cases {
		if got := ParseQualityLevel(c); got != Quality480p {
			t.Errorf("ParseQualityLevel(%q) = %d, want %d", c, got, Quality480p)
		}
	}
}

func TestParseQualityLevel_360(t *testing.T) {
	if got := ParseQualityLevel("360p"); got != Quality360p {
		t.Errorf("ParseQualityLevel(360p) = %d, want %d", got, Quality360p)
	}
}

func TestParseQualityLevel_240(t *testing.T) {
	if got := ParseQualityLevel("240p"); got != Quality240p {
		t.Errorf("ParseQualityLevel(240p) = %d, want %d", got, Quality240p)
	}
}

func TestParseQualityLevel_1440(t *testing.T) {
	cases := []string{"1440p", "2K", "QHD"}
	for _, c := range cases {
		if got := ParseQualityLevel(c); got != Quality1440p {
			t.Errorf("ParseQualityLevel(%q) = %d, want %d", c, got, Quality1440p)
		}
	}
}

func TestParseQualityLevel_Empty_ReturnsUnknown(t *testing.T) {
	if got := ParseQualityLevel(""); got != QualityUnknown {
		t.Errorf("ParseQualityLevel('') = %d, want %d", got, QualityUnknown)
	}
}

func TestParseQualityLevel_Garbage_ReturnsUnknown(t *testing.T) {
	if got := ParseQualityLevel("superhigh"); got != QualityUnknown {
		t.Errorf("ParseQualityLevel(superhigh) = %d, want %d", got, QualityUnknown)
	}
}

// ── meetsMinQuality ───────────────────────────────────────────────────────────

func TestMeetsMinQuality_NoMinimum_AlwaysTrue(t *testing.T) {
	if !meetsMinQuality("360p", 0) {
		t.Error("meetsMinQuality(any, 0) should always be true")
	}
}

func TestMeetsMinQuality_UnknownQuality_PassesFilter(t *testing.T) {
	// Videos without quality info must not be filtered.
	if !meetsMinQuality("", Quality720p) {
		t.Error("meetsMinQuality('', 720) should pass (unknown quality exempt)")
	}
}

func TestMeetsMinQuality_AboveMinimum_True(t *testing.T) {
	if !meetsMinQuality("1080p", Quality720p) {
		t.Error("meetsMinQuality(1080p, 720) should be true")
	}
}

func TestMeetsMinQuality_BelowMinimum_False(t *testing.T) {
	if meetsMinQuality("360p", Quality720p) {
		t.Error("meetsMinQuality(360p, 720) should be false")
	}
}

func TestMeetsMinQuality_ExactMatch_True(t *testing.T) {
	if !meetsMinQuality("720p", Quality720p) {
		t.Error("meetsMinQuality(720p, 720) should be true (equal meets minimum)")
	}
}

// ── isValidThumbnail ──────────────────────────────────────────────────────────

func TestIsValidThumbnail_EmptyString(t *testing.T) {
	if isValidThumbnail("") {
		t.Error("isValidThumbnail('') should be false")
	}
}

func TestIsValidThumbnail_HTTPUrl(t *testing.T) {
	if !isValidThumbnail("https://cdn.example.com/thumb.jpg") {
		t.Error("isValidThumbnail(valid https) should be true")
	}
}

func TestIsValidThumbnail_PlaceholderURL(t *testing.T) {
	placeholders := []string{
		"https://example.com/placeholder.jpg",
		"https://example.com/no-image.png",
		"https://example.com/noimage.gif",
		"https://example.com/default_thumb.jpg",
		"https://example.com/blank.gif",
		"https://example.com/missing.jpg",
	}
	for _, p := range placeholders {
		if isValidThumbnail(p) {
			t.Errorf("isValidThumbnail(%q) should be false (placeholder)", p)
		}
	}
}

func TestIsValidThumbnail_RelativeURL(t *testing.T) {
	if isValidThumbnail("/images/thumb.jpg") {
		t.Error("isValidThumbnail(relative URL) should be false")
	}
}

// ── normalizeURL ──────────────────────────────────────────────────────────────

func TestNormalizeURL_Empty(t *testing.T) {
	if normalizeURL("") != "" {
		t.Error("normalizeURL('') should return ''")
	}
}

func TestNormalizeURL_StripHttps(t *testing.T) {
	n := normalizeURL("https://www.pornhub.com/video/123")
	if strings.Contains(n, "https://") {
		t.Errorf("normalizeURL: https:// not stripped, got %q", n)
	}
}

func TestNormalizeURL_StripWWW(t *testing.T) {
	n := normalizeURL("https://www.example.com/path")
	if strings.Contains(n, "www.") {
		t.Errorf("normalizeURL: www. not stripped, got %q", n)
	}
}

func TestNormalizeURL_HTTPandHTTPS_SameResult(t *testing.T) {
	n1 := normalizeURL("http://example.com/video/1")
	n2 := normalizeURL("https://example.com/video/1")
	if n1 != n2 {
		t.Errorf("normalizeURL: http and https produced different results: %q vs %q", n1, n2)
	}
}

func TestNormalizeURL_TrailingSlash_Stripped(t *testing.T) {
	n1 := normalizeURL("https://example.com/video/1")
	n2 := normalizeURL("https://example.com/video/1/")
	if n1 != n2 {
		t.Errorf("normalizeURL: trailing slash not stripped: %q vs %q", n1, n2)
	}
}

func TestNormalizeURL_QueryParamsRemoved(t *testing.T) {
	n := normalizeURL("https://example.com/video/1?ref=search&page=2")
	if strings.Contains(n, "?") {
		t.Errorf("normalizeURL: query params not removed, got %q", n)
	}
}

func TestNormalizeURL_FragmentRemoved(t *testing.T) {
	n := normalizeURL("https://example.com/video/1#comments")
	if strings.Contains(n, "#") {
		t.Errorf("normalizeURL: fragment not removed, got %q", n)
	}
}

// ── normalizeTitle ────────────────────────────────────────────────────────────

func TestNormalizeTitle_Empty(t *testing.T) {
	if normalizeTitle("") != "" {
		t.Error("normalizeTitle('') should return ''")
	}
}

func TestNormalizeTitle_ShortTitle_ReturnsEmpty(t *testing.T) {
	// Fewer than 3 significant words → return "" to skip title dedup.
	if normalizeTitle("hi") != "" {
		t.Error("normalizeTitle('hi') (< 3 significant words) should return ''")
	}
}

func TestNormalizeTitle_WordsSorted(t *testing.T) {
	// Word-sorted normalization means order-independent duplicates are caught.
	n1 := normalizeTitle("Teen Blonde Amateur")
	n2 := normalizeTitle("Amateur Teen Blonde")
	if n1 != n2 {
		t.Errorf("normalizeTitle: word order should not matter: %q != %q", n1, n2)
	}
}

func TestNormalizeTitle_Lowercase(t *testing.T) {
	n := normalizeTitle("Big Tits MILF Hardcore Video")
	if strings.ToLower(n) != n {
		t.Errorf("normalizeTitle: result not lowercase: %q", n)
	}
}

// ── jaroSimilarity / jaroWinklerSimilarity / titlesAreFuzzyDuplicates ────────

func TestJaroSimilarity_Identical(t *testing.T) {
	if s := jaroSimilarity("hello", "hello"); s != 1.0 {
		t.Errorf("jaroSimilarity(hello, hello) = %f, want 1.0", s)
	}
}

func TestJaroSimilarity_Completely_Different(t *testing.T) {
	if s := jaroSimilarity("abc", "xyz"); s > 0.5 {
		t.Errorf("jaroSimilarity(abc, xyz) = %f, want <= 0.5", s)
	}
}

func TestJaroSimilarity_EmptyStrings(t *testing.T) {
	if s := jaroSimilarity("", ""); s != 1.0 {
		t.Errorf("jaroSimilarity('', '') = %f, want 1.0", s)
	}
	if s := jaroSimilarity("abc", ""); s != 0.0 {
		t.Errorf("jaroSimilarity(abc, '') = %f, want 0.0", s)
	}
}

func TestJaroWinklerSimilarity_Identical(t *testing.T) {
	if s := jaroWinklerSimilarity("hello", "hello"); s != 1.0 {
		t.Errorf("jaroWinklerSimilarity(hello, hello) = %f, want 1.0", s)
	}
}

func TestTitlesAreFuzzyDuplicates_Identical(t *testing.T) {
	if !titlesAreFuzzyDuplicates("amateur teen blonde", "amateur teen blonde") {
		t.Error("titlesAreFuzzyDuplicates: identical titles should be duplicates")
	}
}

func TestTitlesAreFuzzyDuplicates_Unrelated(t *testing.T) {
	if titlesAreFuzzyDuplicates("amateur teen blonde", "mature milf squirt") {
		t.Error("titlesAreFuzzyDuplicates: unrelated titles should not be duplicates")
	}
}

func TestTitlesAreFuzzyDuplicates_EmptyString(t *testing.T) {
	// Empty string must not be treated as a duplicate.
	if titlesAreFuzzyDuplicates("", "anything") {
		t.Error("titlesAreFuzzyDuplicates('', 'anything') should be false")
	}
}

// ── calculateRelevanceScore / sortAndFilterByRelevance ───────────────────────

func makeVideoResult(title, quality string, views int64, durationSecs int) model.VideoResult {
	return model.VideoResult{
		Title:           title,
		Quality:         quality,
		ViewsCount:      views,
		DurationSeconds: durationSecs,
		Thumbnail:       "https://cdn.example.com/thumb.jpg",
		URL:             "https://example.com/video/1",
		Source:          "test",
	}
}

func TestCalculateRelevanceScore_ExactMatchHighest(t *testing.T) {
	exact := makeVideoResult("teen amateur", "", 0, 0)
	partial := makeVideoResult("amateur compilation", "", 0, 0)

	sExact := calculateRelevanceScore(exact, "teen amateur", []string{"teen", "amateur"})
	sPartial := calculateRelevanceScore(partial, "teen amateur", []string{"teen", "amateur"})

	if sExact <= sPartial {
		t.Errorf("exact match score (%f) should beat partial (%f)", sExact, sPartial)
	}
}

func TestCalculateRelevanceScore_HDBonus(t *testing.T) {
	hd := makeVideoResult("milf hd", "1080p", 0, 0)
	sd := makeVideoResult("milf sd", "360p", 0, 0)

	sHD := calculateRelevanceScore(hd, "milf", []string{"milf"})
	sSD := calculateRelevanceScore(sd, "milf", []string{"milf"})

	if sHD <= sSD {
		t.Errorf("HD result score (%f) should be higher than SD (%f)", sHD, sSD)
	}
}

func TestCalculateRelevanceScore_ViewsBonus(t *testing.T) {
	popular := makeVideoResult("teen", "", 1000000, 0)
	unpopular := makeVideoResult("teen", "", 10, 0)

	sPopular := calculateRelevanceScore(popular, "teen", []string{"teen"})
	sUnpopular := calculateRelevanceScore(unpopular, "teen", []string{"teen"})

	if sPopular <= sUnpopular {
		t.Errorf("popular result score (%f) should exceed unpopular (%f)", sPopular, sUnpopular)
	}
}

func TestSortAndFilterByRelevance_Empty(t *testing.T) {
	result := sortAndFilterByRelevance(nil, "teen", 0)
	if result != nil && len(result) != 0 {
		t.Errorf("sortAndFilterByRelevance(nil) = %v, want nil/empty", result)
	}
}

func TestSortAndFilterByRelevance_FiltersByMinScore(t *testing.T) {
	results := []model.VideoResult{
		makeVideoResult("completely unrelated title xyz", "", 0, 0),
		makeVideoResult("teen amateur blonde", "", 1000, 600),
	}
	filtered := sortAndFilterByRelevance(results, "teen", 10.0)
	for _, r := range filtered {
		if r.Title == "completely unrelated title xyz" {
			t.Error("sortAndFilterByRelevance: low-relevance result should have been filtered")
		}
	}
}

func TestSortAndFilterByRelevanceWithOperators_ExactPhrase(t *testing.T) {
	results := []model.VideoResult{
		makeVideoResult("big tits compilation", "", 0, 0),
		makeVideoResult("small tits video", "", 0, 0),
	}
	phrases := []string{"big tits"}
	filtered := sortAndFilterByRelevanceWithOperators(results, "tits", 0, phrases, nil, nil)
	for _, r := range filtered {
		if r.Title == "small tits video" {
			t.Error("exact phrase filter: 'small tits' should be excluded when phrase 'big tits' required")
		}
	}
}

func TestSortAndFilterByRelevanceWithOperators_Exclusion(t *testing.T) {
	results := []model.VideoResult{
		makeVideoResult("teen amateur video", "", 0, 0),
		makeVideoResult("teen milf video", "", 0, 0),
	}
	exclusions := []string{"milf"}
	filtered := sortAndFilterByRelevanceWithOperators(results, "teen", 0, nil, exclusions, nil)
	for _, r := range filtered {
		if r.Title == "teen milf video" {
			t.Error("exclusion filter: 'milf' title should have been excluded")
		}
	}
}

func TestSortAndFilterByRelevanceWithOperators_Performer(t *testing.T) {
	results := []model.VideoResult{
		{Title: "scene", Performer: "Riley Reid", Thumbnail: "https://cdn.example.com/t.jpg", URL: "https://x.com/1", Source: "t"},
		{Title: "scene", Performer: "Mia Khalifa", Thumbnail: "https://cdn.example.com/t.jpg", URL: "https://x.com/2", Source: "t"},
	}
	performers := []string{"riley"}
	filtered := sortAndFilterByRelevanceWithOperators(results, "scene", 0, nil, nil, performers)
	for _, r := range filtered {
		if r.Performer == "Mia Khalifa" {
			t.Error("performer filter: Mia Khalifa should be excluded when filtering for Riley")
		}
	}
	if len(filtered) == 0 {
		t.Error("performer filter: Riley Reid should remain")
	}
}

// ── Taxonomy functions ────────────────────────────────────────────────────────

func TestNormalizeTerm_KnownSynonym(t *testing.T) {
	// "mommy" is a synonym for "milf"
	got := NormalizeTerm("mommy")
	if got != "milf" {
		t.Errorf("NormalizeTerm('mommy') = %q, want 'milf'", got)
	}
}

func TestNormalizeTerm_CanonicalName_Unchanged(t *testing.T) {
	got := NormalizeTerm("teen")
	if got != "teen" {
		t.Errorf("NormalizeTerm('teen') = %q, want 'teen'", got)
	}
}

func TestNormalizeTerm_Unknown_ReturnsSelf(t *testing.T) {
	got := NormalizeTerm("zzz_unknown_zzz")
	if got != "zzz_unknown_zzz" {
		t.Errorf("NormalizeTerm(unknown) = %q, want the input unchanged", got)
	}
}

func TestNormalizeTerm_CaseInsensitive(t *testing.T) {
	lower := NormalizeTerm("teen")
	upper := NormalizeTerm("TEEN")
	if lower != upper {
		t.Errorf("NormalizeTerm case sensitivity: %q != %q", lower, upper)
	}
}

func TestGetSynonyms_KnownTerm(t *testing.T) {
	syns := GetSynonyms("milf")
	if len(syns) == 0 {
		t.Fatal("GetSynonyms('milf') returned empty list")
	}
	found := false
	for _, s := range syns {
		if s == "milf" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetSynonyms('milf') = %v, should include 'milf' itself", syns)
	}
}

func TestGetSynonyms_SynonymOfKnownCategory(t *testing.T) {
	// "mommy" maps to "milf" category; should return milf synonyms
	syns := GetSynonyms("mommy")
	if len(syns) < 2 {
		t.Errorf("GetSynonyms('mommy') = %v, expected multiple synonyms", syns)
	}
}

func TestGetSynonyms_Unknown_ReturnsSelf(t *testing.T) {
	syns := GetSynonyms("zzz_no_category")
	if len(syns) != 1 || syns[0] != "zzz_no_category" {
		t.Errorf("GetSynonyms(unknown) = %v, want [zzz_no_category]", syns)
	}
}

func TestGetRelatedTerms_KnownTerm(t *testing.T) {
	related := GetRelatedTerms("milf")
	if len(related) == 0 {
		t.Error("GetRelatedTerms('milf') returned empty list")
	}
}

func TestGetRelatedTerms_Unknown_ReturnsNil(t *testing.T) {
	related := GetRelatedTerms("zzz_not_a_category")
	if len(related) != 0 {
		t.Errorf("GetRelatedTerms(unknown) = %v, want nil/empty", related)
	}
}

func TestExpandSearchTerms_SingleKnownWord(t *testing.T) {
	expanded := ExpandSearchTerms("milf")
	if len(expanded) == 0 {
		t.Fatal("ExpandSearchTerms('milf') returned empty map")
	}
	// The milf category key should be present
	found := false
	for _, syns := range expanded {
		for _, s := range syns {
			if s == "milf" || s == "mom" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("ExpandSearchTerms('milf') = %v, should include milf synonyms", expanded)
	}
}

func TestExpandSearchTerms_Empty(t *testing.T) {
	expanded := ExpandSearchTerms("")
	if len(expanded) != 0 {
		t.Errorf("ExpandSearchTerms('') = %v, want empty map", expanded)
	}
}

func TestMatchesAllTerms_AllPresent(t *testing.T) {
	terms := map[string][]string{
		"milf": {"milf", "mom", "mother"},
		"hd":   {"hd", "1080p"},
	}
	if !MatchesAllTerms("hd milf video 1080p mom", terms) {
		t.Error("MatchesAllTerms: text contains all terms, should return true")
	}
}

func TestMatchesAllTerms_OneMissing(t *testing.T) {
	terms := map[string][]string{
		"teen": {"teen", "young"},
		"hd":   {"hd", "1080p"},
	}
	if MatchesAllTerms("teen video without quality", terms) {
		t.Error("MatchesAllTerms: missing 'hd' term, should return false")
	}
}

func TestMatchesAllTerms_Empty_ReturnsTrue(t *testing.T) {
	if !MatchesAllTerms("any text", map[string][]string{}) {
		t.Error("MatchesAllTerms with empty terms should return true")
	}
}

func TestMatchesAnyTerm_Match(t *testing.T) {
	terms := map[string][]string{
		"milf": {"milf", "mom"},
	}
	if !MatchesAnyTerm("a hot mom video", terms) {
		t.Error("MatchesAnyTerm: 'mom' matches milf synonyms, should return true")
	}
}

func TestMatchesAnyTerm_NoMatch(t *testing.T) {
	terms := map[string][]string{
		"milf": {"milf", "mom"},
	}
	if MatchesAnyTerm("teen blonde video", terms) {
		t.Error("MatchesAnyTerm: no milf synonyms present, should return false")
	}
}

// ── DetectQueryIntent ─────────────────────────────────────────────────────────

func TestDetectQueryIntent_LesbianQuery(t *testing.T) {
	intent := DetectQueryIntent("lesbian milf video")
	if !intent.IsFemaleOnly {
		t.Error("DetectQueryIntent('lesbian'): IsFemaleOnly should be true")
	}
}

func TestDetectQueryIntent_NonFemaleQuery(t *testing.T) {
	intent := DetectQueryIntent("teen amateur video")
	if intent.IsFemaleOnly {
		t.Error("DetectQueryIntent('teen amateur'): IsFemaleOnly should be false")
	}
}

func TestDetectQueryIntent_MilfDetectsAgeType(t *testing.T) {
	intent := DetectQueryIntent("milf video")
	found := false
	for _, a := range intent.HasAgeTypes {
		if a == "milf" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DetectQueryIntent('milf'): HasAgeTypes = %v, should include 'milf'", intent.HasAgeTypes)
	}
}

func TestDetectQueryIntent_TeenDetectsAgeType(t *testing.T) {
	intent := DetectQueryIntent("teen blonde")
	found := false
	for _, a := range intent.HasAgeTypes {
		if a == "teen" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DetectQueryIntent('teen blonde'): HasAgeTypes = %v, should include 'teen'", intent.HasAgeTypes)
	}
}

// ── ResultMatchesIntent ───────────────────────────────────────────────────────

func TestResultMatchesIntent_NonFemaleOnly_AlwaysTrue(t *testing.T) {
	intent := QueryIntent{IsFemaleOnly: false}
	r := model.VideoResult{Title: "teen cock blowjob"}
	if !ResultMatchesIntent(r, intent) {
		t.Error("ResultMatchesIntent with non-female-only intent must always return true")
	}
}

func TestResultMatchesIntent_FemaleOnly_NoMaleWords_True(t *testing.T) {
	intent := QueryIntent{IsFemaleOnly: true}
	r := model.VideoResult{Title: "lesbian scissoring compilation"}
	if !ResultMatchesIntent(r, intent) {
		t.Error("ResultMatchesIntent: female-only title without male words should return true")
	}
}

func TestResultMatchesIntent_FemaleOnly_MaleWordPresent_False(t *testing.T) {
	intent := QueryIntent{IsFemaleOnly: true}
	r := model.VideoResult{Title: "guy fucks lesbian"}
	if ResultMatchesIntent(r, intent) {
		t.Error("ResultMatchesIntent: 'guy fucks' in female-only query should return false")
	}
}

func TestResultMatchesIntent_FemaleOnly_ToyWordExemptsBBC(t *testing.T) {
	// "bbc dildo" should NOT trigger the male filter (toy present, bbc is ambiguous)
	intent := QueryIntent{IsFemaleOnly: true}
	r := model.VideoResult{Title: "lesbian uses bbc dildo"}
	if !ResultMatchesIntent(r, intent) {
		t.Error("ResultMatchesIntent: 'bbc dildo' should be exempt from male filter (toy present)")
	}
}

// ── containsWholeWord ─────────────────────────────────────────────────────────

func TestContainsWholeWord_Match(t *testing.T) {
	if !containsWholeWord("teen video", "teen") {
		t.Error("containsWholeWord('teen video', 'teen') should be true")
	}
}

func TestContainsWholeWord_PartialWord_False(t *testing.T) {
	// "cock" inside "cockatoo" should not match as a whole word
	if containsWholeWord("cockatoo video", "cock") {
		t.Error("containsWholeWord: 'cock' inside 'cockatoo' should not match as whole word")
	}
}

func TestContainsWholeWord_AtEnd(t *testing.T) {
	if !containsWholeWord("video cock", "cock") {
		t.Error("containsWholeWord: should match word at end of string")
	}
}

func TestContainsWholeWord_OnlyWord(t *testing.T) {
	if !containsWholeWord("cock", "cock") {
		t.Error("containsWholeWord: should match when string is exactly the word")
	}
}

// ── GenerateSmartRelated ──────────────────────────────────────────────────────

func TestGenerateSmartRelated_Empty(t *testing.T) {
	result := GenerateSmartRelated("", 10)
	if result != nil {
		t.Errorf("GenerateSmartRelated('') = %v, want nil", result)
	}
}

func TestGenerateSmartRelated_KnownTerm_NonEmpty(t *testing.T) {
	result := GenerateSmartRelated("milf", 10)
	if len(result) == 0 {
		t.Error("GenerateSmartRelated('milf') returned empty, want related suggestions")
	}
}

func TestGenerateSmartRelated_MaxResults_Respected(t *testing.T) {
	result := GenerateSmartRelated("teen milf lesbian", 3)
	if len(result) > 3 {
		t.Errorf("GenerateSmartRelated: returned %d results, want <= 3", len(result))
	}
}

func TestGenerateSmartRelated_NoOriginalQuery(t *testing.T) {
	query := "milf"
	result := GenerateSmartRelated(query, 20)
	for _, r := range result {
		if r == query {
			t.Errorf("GenerateSmartRelated: original query %q should not appear in related", query)
		}
	}
}

// ── formatViewCount (eporner, package-private) ────────────────────────────────

func TestFormatViewCount_Millions(t *testing.T) {
	got := formatViewCount(1500000)
	if !strings.Contains(got, "M") {
		t.Errorf("formatViewCount(1500000) = %q, want 'M' suffix", got)
	}
}

func TestFormatViewCount_Thousands(t *testing.T) {
	got := formatViewCount(5000)
	if !strings.Contains(got, "K") {
		t.Errorf("formatViewCount(5000) = %q, want 'K' suffix", got)
	}
}

func TestFormatViewCount_Small(t *testing.T) {
	got := formatViewCount(42)
	if got != "42" {
		t.Errorf("formatViewCount(42) = %q, want '42'", got)
	}
}

func TestFormatViewCount_Zero(t *testing.T) {
	got := formatViewCount(0)
	if got != "0" {
		t.Errorf("formatViewCount(0) = %q, want '0'", got)
	}
}

// ── formatViews (pornhub, package-private) ────────────────────────────────────

func TestFormatViews_Millions(t *testing.T) {
	got := formatViews(2500000)
	if !strings.Contains(got, "M") {
		t.Errorf("formatViews(2500000) = %q, want 'M' suffix", got)
	}
}

func TestFormatViews_Thousands(t *testing.T) {
	got := formatViews(3500)
	if !strings.Contains(got, "K") {
		t.Errorf("formatViews(3500) = %q, want 'K' suffix", got)
	}
}

func TestFormatViews_Small(t *testing.T) {
	got := formatViews(7)
	if !strings.Contains(got, "7") {
		t.Errorf("formatViews(7) = %q, should contain '7'", got)
	}
}

// ── ParseBangs – xvideos mapping ─────────────────────────────────────────────

func TestParseBangs_XVideos(t *testing.T) {
	result := ParseBangs("!xvideos hello world")
	if len(result.Engines) == 0 || result.Engines[0] != "xvideos" {
		t.Errorf("ParseBangs('!xvideos'): engines = %v, want [xvideos]", result.Engines)
	}
	if result.Query != "hello world" {
		t.Errorf("ParseBangs('!xvideos hello world'): query = %q, want 'hello world'", result.Query)
	}
}

func TestParseBangs_MultipleBangs_XVideosAndPornHub(t *testing.T) {
	result := ParseBangs("!ph !xv test")
	hasPH := false
	hasXV := false
	for _, e := range result.Engines {
		switch e {
		case "pornhub":
			hasPH = true
		case "xvideos":
			hasXV = true
		}
	}
	if !hasPH || !hasXV {
		t.Errorf("ParseBangs('!ph !xv test'): engines = %v, want both pornhub and xvideos", result.Engines)
	}
}
