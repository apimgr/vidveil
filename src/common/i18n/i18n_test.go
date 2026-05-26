// SPDX-License-Identifier: MIT
// AI.md PART 30: I18N & A11Y - Internationalization Tests
package i18n

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestNew(t *testing.T) {
	translator := NewTranslator()

	if translator == nil {
		t.Fatal("New() returned nil")
	}

	if translator.fallback != "en" {
		t.Errorf("Expected default fallback 'en', got '%s'", translator.fallback)
	}
}

func TestTranslate(t *testing.T) {
	translator := NewTranslator()

	// Test existing key
	result := translator.Translate("en", "nav.home")
	if result == "" || result == "nav.home" {
		t.Errorf("Expected translation for 'nav.home', got '%s'", result)
	}

	// Test missing key returns key itself
	result = translator.Translate("en", "nonexistent.key")
	if result != "nonexistent.key" {
		t.Errorf("Expected 'nonexistent.key' for missing translation, got '%s'", result)
	}
}

func TestTranslateFormat(t *testing.T) {
	translator := NewTranslator()

	// Test formatted translation with time.minutes (has %d)
	result := translator.TranslateFormat("en", "time.minutes", 42)
	if result == "" {
		t.Error("Expected formatted translation, got empty string")
	}

	// Should contain the number
	if result == "time.minutes" {
		t.Error("Translation not found or formatting failed")
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		header   string
		expected []string
	}{
		{"en-US,en;q=0.9", []string{"en-us", "en"}},
		{"en", []string{"en"}},
		{"fr-FR,fr;q=0.9,en;q=0.8", []string{"fr-fr", "fr", "en"}},
		{"de-DE", []string{"de-de"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := parseAcceptLanguage(tt.header)
			if len(tt.expected) == 0 && len(result) != 0 {
				t.Errorf("parseAcceptLanguage(%q) = %v, want empty", tt.header, result)
			}
			if len(tt.expected) > 0 && len(result) == 0 {
				t.Errorf("parseAcceptLanguage(%q) returned empty, want %v", tt.header, tt.expected)
			}
		})
	}
}

func TestAvailableLocales(t *testing.T) {
	translator := NewTranslator()

	locales := translator.AvailableLocales()

	if len(locales) == 0 {
		t.Error("Expected at least one available locale")
	}

	// English should always be available
	found := false
	for _, locale := range locales {
		if locale == "en" {
			found = true
			break
		}
	}

	if !found {
		t.Error("English should be available")
	}
}

func TestMiddleware(t *testing.T) {
	translator := NewTranslator()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get locale from X-Locale header (set by middleware)
		locale := r.Header.Get("X-Locale")
		if locale == "" {
			t.Error("X-Locale header not set by middleware")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := translator.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Language", "en-US")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestTemplateFunc(t *testing.T) {
	translator := NewTranslator()

	fn := translator.TemplateFunc("en")

	if fn == nil {
		t.Fatal("TemplateFunc() returned nil")
	}

	// Test the function
	result := fn("nav.home")
	if result == "" || result == "nav.home" {
		t.Errorf("Expected translation, got '%s'", result)
	}
}

func TestTranslationKeys(t *testing.T) {
	translator := NewTranslator()

	// Test common keys that should exist
	keys := []string{
		"nav.home",
		"nav.search",
		"nav.preferences",
		"nav.about",
		"search.placeholder",
		"search.button",
		"footer.powered",
		"action.submit",
		"action.cancel",
		"status.loading",
		"error.generic",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			result := translator.Translate("en", key)
			if result == key {
				t.Errorf("Translation missing for key '%s'", key)
			}
		})
	}
}

func TestHasLocale(t *testing.T) {
	translator := NewTranslator()

	if !translator.HasLocale("en") {
		t.Error("Should have 'en' locale")
	}

	if translator.HasLocale("nonexistent") {
		t.Error("Should not have 'nonexistent' locale")
	}
}

func TestAddTranslation(t *testing.T) {
	translator := NewTranslator()

	translator.AddTranslation("en", "test.key", "Test Value")

	result := translator.Translate("en", "test.key")
	if result != "Test Value" {
		t.Errorf("Expected 'Test Value', got '%s'", result)
	}
}

func TestGetLocale(t *testing.T) {
	translator := NewTranslator()

	// Test with Accept-Language header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Language", "en-US")

	locale := translator.GetLocale(req)
	if locale != "en" {
		t.Errorf("Expected 'en', got '%s'", locale)
	}

	// Test with lang query param
	req2 := httptest.NewRequest("GET", "/test?lang=en", nil)
	locale2 := translator.GetLocale(req2)
	if locale2 != "en" {
		t.Errorf("Expected 'en', got '%s'", locale2)
	}
}

func TestGlobalTranslator(t *testing.T) {
	// Test the global convenience functions
	result := Translate("en", "nav.home")
	if result == "" || result == "nav.home" {
		t.Errorf("Translate() should return translation, got '%s'", result)
	}

	result2 := TranslateFormat("en", "time.minutes", 5)
	if result2 == "" || result2 == "time.minutes" {
		t.Errorf("TranslateFormat() should return formatted translation, got '%s'", result2)
	}
}

// TestLocaleKeyCompleteness is the build-time key validation required by AI.md PART 30.
// Every key present in en.json MUST exist in all other locale files.
// Missing keys cause the CI build to fail (test failure).
func TestLocaleKeyCompleteness(t *testing.T) {
	// Load all locale files from the embedded FS
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		t.Fatalf("cannot read locales directory: %v", err)
	}

	locales := make(map[string]map[string]json.RawMessage)
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 6 {
			continue
		}
		name := e.Name()
		if name[len(name)-5:] != ".json" {
			continue
		}
		lang := name[:len(name)-5]

		data, err := localesFS.ReadFile("locales/" + name)
		if err != nil {
			t.Errorf("cannot read locales/%s: %v", name, err)
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.Errorf("cannot parse locales/%s: %v", name, err)
			continue
		}
		locales[lang] = m
	}

	enKeys, ok := locales["en"]
	if !ok {
		t.Fatal("en.json not found in embedded locales")
	}

	// Build sorted key list for deterministic output
	keys := make([]string, 0, len(enKeys))
	for k := range enKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Verify all non-English locales contain every key from en.json
	for lang, trans := range locales {
		if lang == "en" {
			continue
		}
		for _, key := range keys {
			if _, exists := trans[key]; !exists {
				t.Errorf("locale %s missing key %q (present in en.json)", lang, key)
			}
		}
	}
}
