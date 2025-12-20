// SPDX-License-Identifier: MIT
package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	translator := New()

	if translator == nil {
		t.Fatal("New() returned nil")
	}

	if translator.fallback != "en" {
		t.Errorf("Expected default fallback 'en', got '%s'", translator.fallback)
	}
}

func TestTranslate(t *testing.T) {
	translator := New()

	// Test existing key
	result := translator.T("en", "nav.home")
	if result == "" || result == "nav.home" {
		t.Errorf("Expected translation for 'nav.home', got '%s'", result)
	}

	// Test missing key returns key itself
	result = translator.T("en", "nonexistent.key")
	if result != "nonexistent.key" {
		t.Errorf("Expected 'nonexistent.key' for missing translation, got '%s'", result)
	}
}

func TestTranslateFormat(t *testing.T) {
	translator := New()

	// Test formatted translation with time.minutes (has %d)
	result := translator.TF("en", "time.minutes", 42)
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
	translator := New()

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
	translator := New()

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
	translator := New()

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
	translator := New()

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
			result := translator.T("en", key)
			if result == key {
				t.Errorf("Translation missing for key '%s'", key)
			}
		})
	}
}

func TestHasLocale(t *testing.T) {
	translator := New()

	if !translator.HasLocale("en") {
		t.Error("Should have 'en' locale")
	}

	if translator.HasLocale("nonexistent") {
		t.Error("Should not have 'nonexistent' locale")
	}
}

func TestAddTranslation(t *testing.T) {
	translator := New()

	translator.AddTranslation("en", "test.key", "Test Value")

	result := translator.T("en", "test.key")
	if result != "Test Value" {
		t.Errorf("Expected 'Test Value', got '%s'", result)
	}
}

func TestGetLocale(t *testing.T) {
	translator := New()

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
	result := T("en", "nav.home")
	if result == "" || result == "nav.home" {
		t.Errorf("T() should return translation, got '%s'", result)
	}

	result2 := TF("en", "time.minutes", 5)
	if result2 == "" || result2 == "time.minutes" {
		t.Errorf("TF() should return formatted translation, got '%s'", result2)
	}
}
