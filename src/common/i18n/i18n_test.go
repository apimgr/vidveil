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

// TestDirection covers Direction() for all RTL locales, LTR locales, subtags, and empty input.
func TestDirection(t *testing.T) {
	tests := []struct {
		locale string
		want   string
	}{
		// RTL base locales
		{"ar", "rtl"},
		{"fa", "rtl"},
		{"he", "rtl"},
		{"ur", "rtl"},
		{"ps", "rtl"},
		{"sd", "rtl"},
		{"yi", "rtl"},
		// RTL with subtag (hyphen separator)
		{"ar-SA", "rtl"},
		{"he-IL", "rtl"},
		// RTL with underscore separator
		{"fa_IR", "rtl"},
		// LTR locales
		{"en", "ltr"},
		{"fr", "ltr"},
		{"de", "ltr"},
		{"zh", "ltr"},
		{"ja", "ltr"},
		{"es", "ltr"},
		// LTR with subtag
		{"en-US", "ltr"},
		// Mixed case normalised
		{"AR", "rtl"},
		{"HE-IL", "rtl"},
		// Empty input
		{"", "ltr"},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			got := Direction(tt.locale)
			if got != tt.want {
				t.Errorf("Direction(%q) = %q, want %q", tt.locale, got, tt.want)
			}
		})
	}
}

// TestDetectLocale covers DetectLocale() priority: query param > cookie > Accept-Language > default.
func TestDetectLocale(t *testing.T) {
	t.Run("query_param_wins", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=FR", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: "de"})
		req.Header.Set("Accept-Language", "ja")
		got := DetectLocale(req)
		if got != "fr" {
			t.Errorf("DetectLocale() with ?lang=FR = %q, want %q", got, "fr")
		}
	})

	t.Run("cookie_wins_over_header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: "DE"})
		req.Header.Set("Accept-Language", "ja")
		got := DetectLocale(req)
		if got != "de" {
			t.Errorf("DetectLocale() with cookie=DE = %q, want %q", got, "de")
		}
	})

	t.Run("accept_language_first_tag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Language", "FR-FR,fr;q=0.9,en;q=0.8")
		got := DetectLocale(req)
		if got != "fr-fr" {
			t.Errorf("DetectLocale() Accept-Language = %q, want %q", got, "fr-fr")
		}
	})

	t.Run("accept_language_with_quality_only", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Language", "en;q=0.5")
		got := DetectLocale(req)
		if got != "en" {
			t.Errorf("DetectLocale() Accept-Language q-only = %q, want %q", got, "en")
		}
	})

	t.Run("no_signals_returns_default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		got := DetectLocale(req)
		if got != DefaultLocale {
			t.Errorf("DetectLocale() no signals = %q, want %q", got, DefaultLocale)
		}
	})

	t.Run("empty_lang_query_skipped", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=", nil)
		got := DetectLocale(req)
		if got != DefaultLocale {
			t.Errorf("DetectLocale() empty ?lang= = %q, want %q", got, DefaultLocale)
		}
	})

	t.Run("whitespace_lang_query_skipped", func(t *testing.T) {
		// URL-encode spaces so httptest.NewRequest accepts the URL.
		req := httptest.NewRequest("GET", "/?lang=%20%20", nil)
		got := DetectLocale(req)
		if got != DefaultLocale {
			t.Errorf("DetectLocale() whitespace ?lang= = %q, want %q", got, DefaultLocale)
		}
	})

	t.Run("empty_cookie_value_skipped", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: ""})
		got := DetectLocale(req)
		if got != DefaultLocale {
			t.Errorf("DetectLocale() empty cookie = %q, want %q", got, DefaultLocale)
		}
	})

	t.Run("accept_language_semicolon_split", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Language", "zh;q=0.8")
		got := DetectLocale(req)
		if got != "zh" {
			t.Errorf("DetectLocale() semicolon split = %q, want %q", got, "zh")
		}
	})
}

// TestLoadDefaultTranslations verifies that loadDefaultTranslations() populates
// the expected English keys when called directly on a bare Translator.
func TestLoadDefaultTranslations(t *testing.T) {
	tr := &Translator{
		translations: make(map[string]map[string]string),
		fallback:     DefaultLocale,
	}
	tr.loadDefaultTranslations()

	if _, ok := tr.translations["en"]; !ok {
		t.Fatal("loadDefaultTranslations() did not create 'en' locale")
	}

	required := []string{
		"app.name", "nav.home", "search.placeholder", "footer.powered",
		"action.save", "status.enabled", "error.generic", "time.now",
	}
	for _, k := range required {
		if v, ok := tr.translations["en"][k]; !ok || v == "" {
			t.Errorf("loadDefaultTranslations() missing or empty key %q", k)
		}
	}
}

// TestLoadEmbeddedTranslations exercises loadEmbeddedTranslations() by
// creating a fresh Translator and verifying that the embedded locale files
// (en, ar, de, …) are all loaded.
func TestLoadEmbeddedTranslations(t *testing.T) {
	tr := &Translator{
		translations: make(map[string]map[string]string),
		fallback:     DefaultLocale,
	}
	tr.loadEmbeddedTranslations()

	if len(tr.translations) == 0 {
		t.Fatal("loadEmbeddedTranslations() loaded no translations")
	}

	if _, ok := tr.translations["en"]; !ok {
		t.Error("loadEmbeddedTranslations() did not load 'en' locale")
	}

	// ar.json is present in the embedded FS — must be loaded too.
	if _, ok := tr.translations["ar"]; !ok {
		t.Error("loadEmbeddedTranslations() did not load 'ar' locale")
	}
}

// TestTranslateFallback covers the locale-fallback chain in Translate():
// unknown locale → language-only subtag → fallback locale → key passthrough.
func TestTranslateFallback(t *testing.T) {
	tr := NewTranslator()

	t.Run("subtag_falls_back_to_lang", func(t *testing.T) {
		// "en-US" is not a loaded locale but "en" is; should return English value.
		got := tr.Translate("en-US", "nav.home")
		want := tr.Translate("en", "nav.home")
		if got != want {
			t.Errorf("Translate(en-US) = %q, want %q", got, want)
		}
	})

	t.Run("unknown_locale_falls_back_to_default", func(t *testing.T) {
		got := tr.Translate("xx", "nav.home")
		want := tr.Translate("en", "nav.home")
		if got != want {
			t.Errorf("Translate(xx) = %q, want %q", got, want)
		}
	})

	t.Run("missing_key_returns_key", func(t *testing.T) {
		got := tr.Translate("en", "totally.nonexistent.key.xyz")
		if got != "totally.nonexistent.key.xyz" {
			t.Errorf("Translate() missing key = %q, want key passthrough", got)
		}
	})

	t.Run("known_rtl_locale_ar", func(t *testing.T) {
		// ar.json is embedded — translation should NOT fall back to English.
		got := tr.Translate("ar", "nav.home")
		enVal := tr.Translate("en", "nav.home")
		if got == "nav.home" {
			t.Error("Translate(ar, nav.home) returned key passthrough; ar.json not loaded?")
		}
		if got == enVal {
			t.Error("Translate(ar, nav.home) returned English value; ar.json translation not used")
		}
	})
}

// TestGetLocaleAllPaths exercises all four branches of GetLocale().
func TestGetLocaleAllPaths(t *testing.T) {
	tr := NewTranslator()

	t.Run("query_param_known_locale", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=en", nil)
		got := tr.GetLocale(req)
		if got != "en" {
			t.Errorf("GetLocale() ?lang=en = %q, want en", got)
		}
	})

	t.Run("query_param_unknown_locale_skipped", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=xx", nil)
		req.Header.Set("Accept-Language", "en")
		got := tr.GetLocale(req)
		if got != "en" {
			t.Errorf("GetLocale() unknown ?lang=xx should fall to Accept-Language, got %q", got)
		}
	})

	t.Run("cookie_known_locale", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: "ar"})
		got := tr.GetLocale(req)
		if got != "ar" {
			t.Errorf("GetLocale() cookie=ar = %q, want ar", got)
		}
	})

	t.Run("cookie_unknown_locale_falls_to_header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: "xx"})
		req.Header.Set("Accept-Language", "en")
		got := tr.GetLocale(req)
		if got != "en" {
			t.Errorf("GetLocale() unknown cookie falls to header, got %q", got)
		}
	})

	t.Run("accept_language_subtag_resolution", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// "en-US" is not a loaded locale but "en" is — should resolve via subtag stripping.
		req.Header.Set("Accept-Language", "en-US,de;q=0.5")
		got := tr.GetLocale(req)
		if got != "en" {
			t.Errorf("GetLocale() en-US subtag = %q, want en", got)
		}
	})

	t.Run("no_signals_returns_fallback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		got := tr.GetLocale(req)
		if got != tr.fallback {
			t.Errorf("GetLocale() no signals = %q, want %q", got, tr.fallback)
		}
	})
}

// TestAddTranslationNewLocale checks AddTranslation() for a brand-new locale
// (the branch where the locale map doesn't exist yet).
func TestAddTranslationNewLocale(t *testing.T) {
	tr := NewTranslator()

	tr.AddTranslation("zz", "hello", "Zello")

	got := tr.Translate("zz", "hello")
	if got != "Zello" {
		t.Errorf("AddTranslation new locale: got %q, want %q", got, "Zello")
	}
}

// TestAddTranslationOverwrite verifies that a second AddTranslation call
// overwrites the previous value for the same key.
func TestAddTranslationOverwrite(t *testing.T) {
	tr := NewTranslator()

	tr.AddTranslation("en", "test.overwrite", "first")
	tr.AddTranslation("en", "test.overwrite", "second")

	got := tr.Translate("en", "test.overwrite")
	if got != "second" {
		t.Errorf("AddTranslation overwrite: got %q, want %q", got, "second")
	}
}

// TestLoadTranslations covers LoadTranslations() for both a new locale and
// merging into an existing one.
func TestLoadTranslations(t *testing.T) {
	t.Run("new_locale", func(t *testing.T) {
		tr := NewTranslator()
		tr.LoadTranslations("zz", map[string]string{
			"greeting": "Hola",
			"farewell": "Adios",
		})
		if tr.Translate("zz", "greeting") != "Hola" {
			t.Error("LoadTranslations new locale: greeting not loaded")
		}
		if tr.Translate("zz", "farewell") != "Adios" {
			t.Error("LoadTranslations new locale: farewell not loaded")
		}
	})

	t.Run("merge_into_existing", func(t *testing.T) {
		tr := NewTranslator()
		original := tr.Translate("en", "nav.home")

		tr.LoadTranslations("en", map[string]string{
			"custom.extra": "Extra",
		})

		// Existing key must survive the merge.
		if tr.Translate("en", "nav.home") != original {
			t.Error("LoadTranslations merge: existing key was clobbered")
		}
		if tr.Translate("en", "custom.extra") != "Extra" {
			t.Error("LoadTranslations merge: new key not present")
		}
	})

	t.Run("overwrite_existing_key", func(t *testing.T) {
		tr := NewTranslator()
		tr.LoadTranslations("en", map[string]string{
			"nav.home": "Overwritten",
		})
		if tr.Translate("en", "nav.home") != "Overwritten" {
			t.Error("LoadTranslations: overwrite of existing key did not take effect")
		}
	})
}

// TestGetAllTranslations covers GetAllTranslations() for a known locale,
// verifies the result is a copy (not the internal map), and tests the nil
// return for an unknown locale.
func TestGetAllTranslations(t *testing.T) {
	tr := NewTranslator()

	t.Run("known_locale_returns_copy", func(t *testing.T) {
		got := tr.GetAllTranslations("en")
		if got == nil {
			t.Fatal("GetAllTranslations(en) returned nil")
		}
		if _, ok := got["nav.home"]; !ok {
			t.Error("GetAllTranslations(en): nav.home key missing")
		}
		// Mutate the returned map; the internal map must be unaffected.
		got["nav.home"] = "MUTATED"
		original := tr.Translate("en", "nav.home")
		if original == "MUTATED" {
			t.Error("GetAllTranslations returned a reference to the internal map, not a copy")
		}
	})

	t.Run("unknown_locale_returns_nil", func(t *testing.T) {
		got := tr.GetAllTranslations("xx-unknown")
		if got != nil {
			t.Errorf("GetAllTranslations(unknown) = %v, want nil", got)
		}
	})
}

// TestMiddlewareAllPaths exercises every locale-resolution branch inside Middleware.
func TestMiddlewareAllPaths(t *testing.T) {
	tr := NewTranslator()

	noop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Got-Locale", r.Header.Get("X-Locale"))
		w.WriteHeader(http.StatusOK)
	})

	wrapped := tr.Middleware(noop)

	t.Run("query_param_sets_cookie_and_locale", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=ar", nil)
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		if got := rr.Header().Get("X-Got-Locale"); got != "ar" {
			t.Errorf("Middleware ?lang=ar: X-Locale = %q, want ar", got)
		}

		// Cookie must have been written.
		found := false
		for _, c := range rr.Result().Cookies() {
			if c.Name == "lang" && c.Value == "ar" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Middleware ?lang=ar: lang cookie not set in response")
		}
	})

	t.Run("unknown_query_param_ignored", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=xx", nil)
		req.Header.Set("Accept-Language", "en")
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		locale := rr.Header().Get("X-Got-Locale")
		if locale == "xx" {
			t.Error("Middleware: unknown ?lang=xx should not be used as locale")
		}
	})

	t.Run("cookie_sets_locale", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: "ar"})
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		if got := rr.Header().Get("X-Got-Locale"); got != "ar" {
			t.Errorf("Middleware cookie=ar: X-Locale = %q, want ar", got)
		}
	})

	t.Run("unknown_cookie_falls_to_accept_language", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lang", Value: "xx"})
		req.Header.Set("Accept-Language", "en")
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		if got := rr.Header().Get("X-Got-Locale"); got != "en" {
			t.Errorf("Middleware unknown cookie: X-Locale = %q, want en", got)
		}
	})

	t.Run("no_signals_uses_fallback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		if got := rr.Header().Get("X-Got-Locale"); got != DefaultLocale {
			t.Errorf("Middleware no signals: X-Locale = %q, want %q", got, DefaultLocale)
		}
	})

	t.Run("non_tls_cookie_not_secure", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?lang=en", nil)
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)

		found := false
		for _, c := range rr.Result().Cookies() {
			if c.Name == "lang" {
				found = true
				if c.Secure {
					t.Error("Middleware non-TLS request: cookie should not be Secure")
				}
				break
			}
		}
		if !found {
			t.Error("Middleware: lang cookie not set for ?lang=en")
		}
	})
}

// TestTemplateFuncWithArgs exercises the args branch of TemplateFunc.
func TestTemplateFuncWithArgs(t *testing.T) {
	tr := NewTranslator()

	t.Run("with_format_args", func(t *testing.T) {
		fn := tr.TemplateFunc("en")
		got := fn("time.minutes", 5)
		if got == "time.minutes" {
			t.Error("TemplateFunc with args: key passthrough, translation not applied")
		}
		if got == "" {
			t.Error("TemplateFunc with args: returned empty string")
		}
	})

	t.Run("without_args_uses_translate", func(t *testing.T) {
		fn := tr.TemplateFunc("en")
		got := fn("nav.home")
		want := tr.Translate("en", "nav.home")
		if got != want {
			t.Errorf("TemplateFunc no args = %q, want %q", got, want)
		}
	})

	t.Run("unknown_locale_falls_back", func(t *testing.T) {
		fn := tr.TemplateFunc("xx")
		got := fn("nav.home")
		want := tr.Translate("en", "nav.home")
		if got != want {
			t.Errorf("TemplateFunc unknown locale = %q, want %q (fallback)", got, want)
		}
	})
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
