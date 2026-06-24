// SPDX-License-Identifier: MIT
// AI.md PART 28: Test coverage for engines
package engine

import (
	"context"
	"net/http"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestParseBangs_EmptyQuery(t *testing.T) {
	result := ParseBangs("")

	if result.Query != "" {
		t.Errorf("ParseBangs empty query = %q, want empty", result.Query)
	}

	if result.HasBang {
		t.Error("ParseBangs empty query should not have bang")
	}

	if len(result.Engines) != 0 {
		t.Errorf("ParseBangs empty query engines = %v, want empty", result.Engines)
	}
}

func TestParseBangs_NoBang(t *testing.T) {
	result := ParseBangs("test query")

	if result.Query != "test query" {
		t.Errorf("ParseBangs query = %q, want 'test query'", result.Query)
	}

	if result.HasBang {
		t.Error("ParseBangs should not have bang for plain query")
	}

	if len(result.Engines) != 0 {
		t.Errorf("ParseBangs engines = %v, want empty", result.Engines)
	}
}

func TestParseBangs_SingleBangPrefix(t *testing.T) {
	result := ParseBangs("!ph test query")

	if result.Query != "test query" {
		t.Errorf("ParseBangs query = %q, want 'test query'", result.Query)
	}

	if !result.HasBang {
		t.Error("ParseBangs should have bang")
	}

	if len(result.Engines) != 1 {
		t.Fatalf("ParseBangs engines = %v, want 1 engine", result.Engines)
	}

	if result.Engines[0] != "pornhub" {
		t.Errorf("ParseBangs engine = %q, want 'pornhub'", result.Engines[0])
	}
}

func TestParseBangs_SingleBangSuffix(t *testing.T) {
	result := ParseBangs("test query !rt")

	if result.Query != "test query" {
		t.Errorf("ParseBangs query = %q, want 'test query'", result.Query)
	}

	if !result.HasBang {
		t.Error("ParseBangs should have bang")
	}

	if len(result.Engines) != 1 {
		t.Fatalf("ParseBangs engines = %v, want 1 engine", result.Engines)
	}

	if result.Engines[0] != "redtube" {
		t.Errorf("ParseBangs engine = %q, want 'redtube'", result.Engines[0])
	}
}

func TestParseBangs_MultipleBangs(t *testing.T) {
	result := ParseBangs("!ph !rt test query")

	if result.Query != "test query" {
		t.Errorf("ParseBangs query = %q, want 'test query'", result.Query)
	}

	if !result.HasBang {
		t.Error("ParseBangs should have bang")
	}

	if len(result.Engines) != 2 {
		t.Fatalf("ParseBangs engines = %v, want 2 engines", result.Engines)
	}

	// Check both engines present
	hasPhub := false
	hasRt := false
	for _, e := range result.Engines {
		if e == "pornhub" {
			hasPhub = true
		}
		if e == "redtube" {
			hasRt = true
		}
	}

	if !hasPhub || !hasRt {
		t.Errorf("ParseBangs engines = %v, want pornhub and redtube", result.Engines)
	}
}

func TestParseBangs_DuplicateBangs(t *testing.T) {
	// !ph and !pornhub should both map to pornhub (no duplicate)
	result := ParseBangs("!ph !pornhub test")

	if len(result.Engines) != 1 {
		t.Errorf("ParseBangs should deduplicate engines, got %v", result.Engines)
	}

	if result.Engines[0] != "pornhub" {
		t.Errorf("ParseBangs engine = %q, want 'pornhub'", result.Engines[0])
	}
}

func TestParseBangs_UnknownBang(t *testing.T) {
	result := ParseBangs("!unknown test")

	if result.Query != "!unknown test" {
		t.Errorf("ParseBangs query = %q, want '!unknown test'", result.Query)
	}

	if result.InvalidBang != "!unknown" {
		t.Errorf("ParseBangs InvalidBang = %q, want '!unknown'", result.InvalidBang)
	}

	if len(result.Engines) != 0 {
		t.Errorf("ParseBangs engines = %v, want empty", result.Engines)
	}
}

func TestParseBangs_CaseInsensitive(t *testing.T) {
	result := ParseBangs("!PH test")

	if !result.HasBang {
		t.Error("ParseBangs should recognize uppercase bang")
	}

	if len(result.Engines) != 1 || result.Engines[0] != "pornhub" {
		t.Errorf("ParseBangs engines = %v, want [pornhub]", result.Engines)
	}
}

func TestParseBangs_FullEngineName(t *testing.T) {
	result := ParseBangs("!pornhub test")

	if !result.HasBang {
		t.Error("ParseBangs should recognize full engine name")
	}

	if len(result.Engines) != 1 || result.Engines[0] != "pornhub" {
		t.Errorf("ParseBangs engines = %v, want [pornhub]", result.Engines)
	}
}

func TestParseBangs_OnlyBang(t *testing.T) {
	result := ParseBangs("!ph")

	if result.Query != "" {
		t.Errorf("ParseBangs query = %q, want empty", result.Query)
	}

	if !result.HasBang {
		t.Error("ParseBangs should have bang")
	}

	if len(result.Engines) != 1 {
		t.Errorf("ParseBangs engines = %v, want 1 engine", result.Engines)
	}
}

func TestParseBangs_MidQueryBang(t *testing.T) {
	result := ParseBangs("test !ph query")

	if result.Query != "test query" {
		t.Errorf("ParseBangs query = %q, want 'test query'", result.Query)
	}

	if !result.HasBang {
		t.Error("ParseBangs should have bang in middle")
	}
}

func TestBangMapping(t *testing.T) {
	// Test that common bangs exist
	testCases := []struct {
		bang   string
		engine string
	}{
		{"ph", "pornhub"},
		{"pornhub", "pornhub"},
		{"xv", "xvideos"},
		{"xvideos", "xvideos"},
		{"rt", "redtube"},
		{"redtube", "redtube"},
		{"xh", "xhamster"},
		{"xhamster", "xhamster"},
		{"ep", "eporner"},
		{"eporner", "eporner"},
	}

	for _, tc := range testCases {
		t.Run(tc.bang, func(t *testing.T) {
			engine, ok := BangMapping[tc.bang]
			if !ok {
				t.Errorf("BangMapping missing '%s'", tc.bang)
				return
			}
			if engine != tc.engine {
				t.Errorf("BangMapping[%s] = %q, want %q", tc.bang, engine, tc.engine)
			}
		})
	}
}

func TestGetEngineBangs(t *testing.T) {
	bangs := GetEngineBangs("pornhub")

	if len(bangs) == 0 {
		t.Error("GetEngineBangs should return bangs for pornhub")
	}

	// Should include both !ph and !pornhub
	hasShort := false
	hasFull := false
	for _, b := range bangs {
		if b == "!ph" {
			hasShort = true
		}
		if b == "!pornhub" {
			hasFull = true
		}
	}

	if !hasShort || !hasFull {
		t.Errorf("GetEngineBangs(pornhub) = %v, should include !ph and !pornhub", bangs)
	}
}

func TestGetEngineBangs_Unknown(t *testing.T) {
	bangs := GetEngineBangs("unknownengine")

	if len(bangs) != 0 {
		t.Errorf("GetEngineBangs(unknown) = %v, want empty", bangs)
	}
}

func TestGetAllBangs(t *testing.T) {
	allBangs := GetAllBangs()

	if len(allBangs) == 0 {
		t.Error("GetAllBangs should return non-empty map")
	}

	// Check pornhub exists
	phBangs, ok := allBangs["pornhub"]
	if !ok {
		t.Error("GetAllBangs should include pornhub")
	}

	if len(phBangs) < 2 {
		t.Errorf("GetAllBangs[pornhub] = %v, should have multiple bangs", phBangs)
	}
}

func TestListBangs(t *testing.T) {
	bangs := ListBangs()

	if len(bangs) == 0 {
		t.Error("ListBangs should return non-empty list")
	}

	// Check structure
	for i, b := range bangs {
		if b.EngineName == "" {
			t.Errorf("ListBangs[%d].EngineName should not be empty", i)
		}
		if b.DisplayName == "" {
			t.Errorf("ListBangs[%d].DisplayName should not be empty", i)
		}
		if b.ShortCode == "" {
			t.Errorf("ListBangs[%d].ShortCode should not be empty", i)
		}
		if b.ShortCode[0] != '!' {
			t.Errorf("ListBangs[%d].ShortCode = %q, should start with !", i, b.ShortCode)
		}
	}

	// Should be unique engines
	seen := make(map[string]bool)
	for _, b := range bangs {
		if seen[b.EngineName] {
			t.Errorf("ListBangs has duplicate engine: %s", b.EngineName)
		}
		seen[b.EngineName] = true
	}
}

func TestAutocomplete_Empty(t *testing.T) {
	suggestions := Autocomplete("")

	if len(suggestions) != 0 {
		t.Errorf("Autocomplete('') = %v, want nil or empty", suggestions)
	}
}

func TestAutocomplete_MatchShortCode(t *testing.T) {
	suggestions := Autocomplete("ph")

	if len(suggestions) == 0 {
		t.Fatal("Autocomplete('ph') should return suggestions")
	}

	// Should include pornhub
	found := false
	for _, s := range suggestions {
		if s.EngineName == "pornhub" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Autocomplete('ph') = %v, should include pornhub", suggestions)
	}
}

func TestAutocomplete_MatchEngineName(t *testing.T) {
	suggestions := Autocomplete("porn")

	if len(suggestions) == 0 {
		t.Fatal("Autocomplete('porn') should return suggestions")
	}

	// Should include pornhub (and potentially others starting with porn)
	found := false
	for _, s := range suggestions {
		if s.EngineName == "pornhub" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Autocomplete('porn') = %v, should include pornhub", suggestions)
	}
}

func TestAutocomplete_Limit(t *testing.T) {
	// Search for something that matches many engines
	suggestions := Autocomplete("p")

	if len(suggestions) > 10 {
		t.Errorf("Autocomplete should limit to 10 results, got %d", len(suggestions))
	}
}

func TestAutocomplete_NoMatch(t *testing.T) {
	suggestions := Autocomplete("zzzzz")

	if len(suggestions) != 0 {
		t.Errorf("Autocomplete('zzzzz') = %v, want empty", suggestions)
	}
}

func TestAutocomplete_CaseInsensitive(t *testing.T) {
	suggestionsLower := Autocomplete("ph")
	suggestionsUpper := Autocomplete("PH")

	// Should get similar results regardless of case
	if len(suggestionsLower) == 0 || len(suggestionsUpper) == 0 {
		t.Skip("No suggestions returned")
	}

	foundLower := false
	foundUpper := false
	for _, s := range suggestionsLower {
		if s.EngineName == "pornhub" {
			foundLower = true
			break
		}
	}
	for _, s := range suggestionsUpper {
		if s.EngineName == "pornhub" {
			foundUpper = true
			break
		}
	}

	if foundLower != foundUpper {
		t.Error("Autocomplete should be case-insensitive")
	}
}

func TestAutocomplete_SuggestionFields(t *testing.T) {
	suggestions := Autocomplete("ph")

	if len(suggestions) == 0 {
		t.Skip("No suggestions returned")
	}

	s := suggestions[0]
	if s.Bang == "" {
		t.Error("Suggestion.Bang should not be empty")
	}
	if s.EngineName == "" {
		t.Error("Suggestion.EngineName should not be empty")
	}
	if s.DisplayName == "" {
		t.Error("Suggestion.DisplayName should not be empty")
	}
	if s.ShortCode == "" {
		t.Error("Suggestion.ShortCode should not be empty")
	}
}

func TestEngineDisplayNames(t *testing.T) {
	// Test that display names are proper cased
	testCases := []struct {
		engine  string
		display string
	}{
		{"pornhub", "PornHub"},
		{"xvideos", "XVideos"},
		{"redtube", "RedTube"},
		{"xhamster", "xHamster"},
		{"eporner", "Eporner"},
	}

	for _, tc := range testCases {
		t.Run(tc.engine, func(t *testing.T) {
			display := EngineDisplayNames[tc.engine]
			if display != tc.display {
				t.Errorf("EngineDisplayNames[%s] = %q, want %q", tc.engine, display, tc.display)
			}
		})
	}
}

func TestFeatureConstants(t *testing.T) {
	// Test that feature constants are defined
	features := []Feature{
		FeaturePagination,
		FeatureSorting,
		FeatureFiltering,
		FeatureThumbnailPreview,
	}

	// Each should have a unique value
	seen := make(map[Feature]bool)
	for _, f := range features {
		if seen[f] {
			t.Errorf("Feature constant %v is duplicated", f)
		}
		seen[f] = true
	}
}

// Context key tests: WithUserIP / GetUserIPFromContext

func TestWithUserIP_ForwardEnabled(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserIP(ctx, "1.2.3.4", true)
	ip, ok := GetUserIPFromContext(ctx)
	if !ok {
		t.Error("GetUserIPFromContext: expected ok=true when forwardEnabled=true")
	}
	if ip != "1.2.3.4" {
		t.Errorf("GetUserIPFromContext: got %q, want %q", ip, "1.2.3.4")
	}
}

func TestWithUserIP_ForwardDisabled(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserIP(ctx, "1.2.3.4", false)
	ip, ok := GetUserIPFromContext(ctx)
	if ok {
		t.Error("GetUserIPFromContext: expected ok=false when forwardEnabled=false")
	}
	if ip != "" {
		t.Errorf("GetUserIPFromContext: got %q, want empty when forwarding disabled", ip)
	}
}

func TestGetUserIPFromContext_EmptyContext(t *testing.T) {
	ip, ok := GetUserIPFromContext(context.Background())
	if ok {
		t.Error("GetUserIPFromContext: expected ok=false on empty context")
	}
	if ip != "" {
		t.Errorf("GetUserIPFromContext: got %q, want empty on empty context", ip)
	}
}

// Context key tests: WithTorPref / GetTorPrefFromContext

func TestWithTorPref_TrueValue(t *testing.T) {
	ctx := context.Background()
	val := true
	ctx = WithTorPref(ctx, &val)
	pref := GetTorPrefFromContext(ctx)
	if pref == nil {
		t.Fatal("GetTorPrefFromContext: expected non-nil *bool when pref was set")
	}
	if *pref != true {
		t.Errorf("GetTorPrefFromContext: got %v, want true", *pref)
	}
}

func TestWithTorPref_FalseValue(t *testing.T) {
	ctx := context.Background()
	val := false
	ctx = WithTorPref(ctx, &val)
	pref := GetTorPrefFromContext(ctx)
	if pref == nil {
		t.Fatal("GetTorPrefFromContext: expected non-nil *bool when pref was set to false")
	}
	if *pref != false {
		t.Errorf("GetTorPrefFromContext: got %v, want false", *pref)
	}
}

func TestWithTorPref_Nil(t *testing.T) {
	ctx := context.Background()
	ctx = WithTorPref(ctx, nil)
	pref := GetTorPrefFromContext(ctx)
	if pref != nil {
		t.Errorf("GetTorPrefFromContext: got %v, want nil when set with nil", pref)
	}
}

func TestGetTorPrefFromContext_EmptyContext(t *testing.T) {
	pref := GetTorPrefFromContext(context.Background())
	if pref != nil {
		t.Errorf("GetTorPrefFromContext: got %v, want nil on empty context", pref)
	}
}

// ParseDuration tests — engine package version (returns int seconds)

func TestParseDuration_MinutesSeconds(t *testing.T) {
	// "4:30" = 4*60 + 30 = 270
	got := ParseDuration("4:30")
	if got != 270 {
		t.Errorf("ParseDuration(\"4:30\") = %d, want 270", got)
	}
}

func TestParseDuration_HoursMinutesSeconds(t *testing.T) {
	// "1:23:45" = 1*3600 + 23*60 + 45 = 5025
	got := ParseDuration("1:23:45")
	if got != 5025 {
		t.Errorf("ParseDuration(\"1:23:45\") = %d, want 5025", got)
	}
}

func TestParseDuration_MinSuffix(t *testing.T) {
	// "12min" = 12*60 = 720
	got := ParseDuration("12min")
	if got != 720 {
		t.Errorf("ParseDuration(\"12min\") = %d, want 720", got)
	}
}

func TestParseDuration_MinSuffixWithSpace(t *testing.T) {
	// "12 min" = 12*60 = 720
	got := ParseDuration("12 min")
	if got != 720 {
		t.Errorf("ParseDuration(\"12 min\") = %d, want 720", got)
	}
}

func TestParseDuration_Empty(t *testing.T) {
	got := ParseDuration("")
	if got != 0 {
		t.Errorf("ParseDuration(\"\") = %d, want 0", got)
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	got := ParseDuration("invalid")
	if got != 0 {
		t.Errorf("ParseDuration(\"invalid\") = %d, want 0", got)
	}
}

// ParseViews tests — engine package version

func TestParseViews_Millions(t *testing.T) {
	got := ParseViews("1.2M")
	if got != 1200000 {
		t.Errorf("ParseViews(\"1.2M\") = %d, want 1200000", got)
	}
}

func TestParseViews_Thousands(t *testing.T) {
	got := ParseViews("500K")
	if got != 500000 {
		t.Errorf("ParseViews(\"500K\") = %d, want 500000", got)
	}
}

func TestParseViews_Billions(t *testing.T) {
	got := ParseViews("1.5B")
	if got != 1500000000 {
		t.Errorf("ParseViews(\"1.5B\") = %d, want 1500000000", got)
	}
}

func TestParseViews_CommaSeparated(t *testing.T) {
	got := ParseViews("1,234,567")
	if got != 1234567 {
		t.Errorf("ParseViews(\"1,234,567\") = %d, want 1234567", got)
	}
}

func TestParseViews_Empty(t *testing.T) {
	got := ParseViews("")
	if got != 0 {
		t.Errorf("ParseViews(\"\") = %d, want 0", got)
	}
}

// GenerateResultID tests

func TestGenerateResultID_NonEmpty(t *testing.T) {
	id := GenerateResultID("https://example.com/video/123", "pornhub")
	if id == "" {
		t.Error("GenerateResultID: expected non-empty string")
	}
}

func TestGenerateResultID_Deterministic(t *testing.T) {
	// Same inputs must produce the same output every time
	id1 := GenerateResultID("https://example.com/video/123", "xvideos")
	id2 := GenerateResultID("https://example.com/video/123", "xvideos")
	if id1 != id2 {
		t.Errorf("GenerateResultID is not deterministic: %q != %q", id1, id2)
	}
}

func TestGenerateResultID_DifferentURLs(t *testing.T) {
	// Different URLs must yield different IDs
	id1 := GenerateResultID("https://example.com/video/1", "pornhub")
	id2 := GenerateResultID("https://example.com/video/2", "pornhub")
	if id1 == id2 {
		t.Errorf("GenerateResultID: different URLs produced identical ID %q", id1)
	}
}

func TestGenerateResultID_DifferentSources(t *testing.T) {
	// Same URL, different source must yield different IDs
	id1 := GenerateResultID("https://example.com/video/1", "pornhub")
	id2 := GenerateResultID("https://example.com/video/1", "redtube")
	if id1 == id2 {
		t.Errorf("GenerateResultID: different sources produced identical ID %q", id1)
	}
}

// AddCookies tests

func TestAddCookies_ReturnsModifier(t *testing.T) {
	modifier := AddCookies(map[string]string{"session": "abc123"})
	if modifier == nil {
		t.Fatal("AddCookies: expected non-nil RequestModifier")
	}
}

func TestAddCookies_AppliesCookies(t *testing.T) {
	modifier := AddCookies(map[string]string{"token": "xyz", "pref": "en"})
	req, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	modifier(req)

	cookies := req.Cookies()
	if len(cookies) != 2 {
		t.Errorf("AddCookies: expected 2 cookies, got %d", len(cookies))
	}
	cookieMap := make(map[string]string, len(cookies))
	for _, c := range cookies {
		cookieMap[c.Name] = c.Value
	}
	if cookieMap["token"] != "xyz" {
		t.Errorf("AddCookies: cookie 'token' = %q, want 'xyz'", cookieMap["token"])
	}
	if cookieMap["pref"] != "en" {
		t.Errorf("AddCookies: cookie 'pref' = %q, want 'en'", cookieMap["pref"])
	}
}

func TestAddCookies_EmptyMap(t *testing.T) {
	// Empty cookie map must not panic
	modifier := AddCookies(map[string]string{})
	req, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	modifier(req)
	if len(req.Cookies()) != 0 {
		t.Errorf("AddCookies(empty): expected 0 cookies, got %d", len(req.Cookies()))
	}
}

// newTestBaseEngine creates a BaseEngine using DefaultAppConfig for use in unit tests
func newTestBaseEngine() *BaseEngine {
	cfg := config.DefaultAppConfig()
	return NewBaseEngine("testengine", "Test Engine", "https://example.com", 1, cfg)
}

// BaseEngine accessor method tests

func TestBaseEngine_Name(t *testing.T) {
	e := newTestBaseEngine()
	if e.Name() != "testengine" {
		t.Errorf("BaseEngine.Name() = %q, want 'testengine'", e.Name())
	}
}

func TestBaseEngine_DisplayName(t *testing.T) {
	e := newTestBaseEngine()
	if e.DisplayName() != "Test Engine" {
		t.Errorf("BaseEngine.DisplayName() = %q, want 'Test Engine'", e.DisplayName())
	}
}

func TestBaseEngine_Tier(t *testing.T) {
	e := newTestBaseEngine()
	if e.Tier() != 1 {
		t.Errorf("BaseEngine.Tier() = %d, want 1", e.Tier())
	}
}

func TestBaseEngine_IsAvailable_DefaultEnabled(t *testing.T) {
	e := newTestBaseEngine()
	if !e.IsAvailable() {
		t.Error("BaseEngine.IsAvailable() = false, want true for a freshly created engine")
	}
}

func TestBaseEngine_SetEnabled_Disable(t *testing.T) {
	e := newTestBaseEngine()
	e.SetEnabled(false)
	if e.IsAvailable() {
		t.Error("BaseEngine.IsAvailable() = true after SetEnabled(false), want false")
	}
}

func TestBaseEngine_SetEnabled_ReEnable(t *testing.T) {
	e := newTestBaseEngine()
	e.SetEnabled(false)
	e.SetEnabled(true)
	if !e.IsAvailable() {
		t.Error("BaseEngine.IsAvailable() = false after re-enabling, want true")
	}
}

func TestBaseEngine_Capabilities_DefaultZero(t *testing.T) {
	e := newTestBaseEngine()
	caps := e.Capabilities()
	// Freshly created engine should have zero-value capabilities
	if caps.HasPreview || caps.HasDownload || caps.HasDuration || caps.HasViews || caps.HasRating {
		t.Errorf("BaseEngine.Capabilities(): expected zero-value defaults, got %+v", caps)
	}
}

func TestBaseEngine_SetCapabilities(t *testing.T) {
	e := newTestBaseEngine()
	want := Capabilities{
		HasPreview:  true,
		HasViews:    true,
		HasDuration: true,
		APIType:     "html",
	}
	e.SetCapabilities(want)
	got := e.Capabilities()
	if got != want {
		t.Errorf("BaseEngine.Capabilities() after SetCapabilities = %+v, want %+v", got, want)
	}
}

func TestBaseEngine_BaseURL(t *testing.T) {
	e := newTestBaseEngine()
	if e.BaseURL() != "https://example.com" {
		t.Errorf("BaseEngine.BaseURL() = %q, want 'https://example.com'", e.BaseURL())
	}
}

// BuildSearchURL tests

func TestBaseEngine_BuildSearchURL_QueryAndPage(t *testing.T) {
	e := newTestBaseEngine()
	result := e.BuildSearchURL("/search?q={query}&page={page}", "test video", 2)
	// URL must contain the percent-encoded query and the page number
	if !containsStr(result, "test+video") && !containsStr(result, "test%20video") {
		t.Errorf("BuildSearchURL: query not encoded correctly in %q", result)
	}
	if !containsStr(result, "page=2") {
		t.Errorf("BuildSearchURL: page not substituted correctly in %q", result)
	}
	if !containsStr(result, "https://example.com") {
		t.Errorf("BuildSearchURL: base URL missing in %q", result)
	}
}

func TestBaseEngine_BuildSearchURL_FirstPage(t *testing.T) {
	e := newTestBaseEngine()
	result := e.BuildSearchURL("/videos/{query}/{page}", "cats", 1)
	if !containsStr(result, "cats") {
		t.Errorf("BuildSearchURL: query 'cats' missing in %q", result)
	}
	if !containsStr(result, "/1") {
		t.Errorf("BuildSearchURL: page 1 missing in %q", result)
	}
}

// containsStr is a simple substring helper used in URL assertion tests
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// DebugLogEngineResponse and DebugLogEngineParseResult smoke tests — just verify no panic

func TestDebugLogEngineResponse_NoPanic(t *testing.T) {
	DebugLogEngineResponse("testengine", "https://example.com/search", []byte("some body"))
}

func TestDebugLogEngineResponse_EmptyBody(t *testing.T) {
	DebugLogEngineResponse("testengine", "https://example.com/search", []byte{})
}

func TestDebugLogEngineResponse_LargeBody(t *testing.T) {
	// Body larger than the 2000-char truncation limit must not panic
	body := make([]byte, 4000)
	for i := range body {
		body[i] = 'x'
	}
	DebugLogEngineResponse("testengine", "https://example.com/search", body)
}

func TestDebugLogEngineParseResult_NoPanic(t *testing.T) {
	stats := map[string]int{"title": 10, "thumbnail": 8, "duration": 5}
	DebugLogEngineParseResult("testengine", 10, stats)
}

func TestDebugLogEngineParseResult_NilStats(t *testing.T) {
	DebugLogEngineParseResult("testengine", 0, nil)
}
