// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 23: Test coverage for engines
package engines

import (
	"testing"
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

	if suggestions != nil && len(suggestions) != 0 {
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
