// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 25: Internationalization (i18n) Support
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

//go:embed translations/*.json
var translationsFS embed.FS

// DefaultLocale is the default locale per TEMPLATE.md
const DefaultLocale = "en"

// Translator handles translations per TEMPLATE.md PART 25
type Translator struct {
	// translations: locale -> key -> translation
	translations map[string]map[string]string
	fallback     string
	mu           sync.RWMutex
}

// New creates a new translator
func New() *Translator {
	t := &Translator{
		translations: make(map[string]map[string]string),
		fallback:     DefaultLocale,
	}

	// Load embedded translations
	t.loadEmbeddedTranslations()

	return t
}

// loadEmbeddedTranslations loads translations from embedded files
func (t *Translator) loadEmbeddedTranslations() {
	files, err := translationsFS.ReadDir("translations")
	if err != nil {
		// No embedded translations, use defaults
		t.loadDefaultTranslations()
		return
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		locale := strings.TrimSuffix(file.Name(), ".json")
		data, err := translationsFS.ReadFile("translations/" + file.Name())
		if err != nil {
			continue
		}

		var trans map[string]string
		if err := json.Unmarshal(data, &trans); err != nil {
			continue
		}

		t.translations[locale] = trans
	}

	// Ensure English exists
	if _, ok := t.translations["en"]; !ok {
		t.loadDefaultTranslations()
	}
}

// loadDefaultTranslations loads default English translations
func (t *Translator) loadDefaultTranslations() {
	t.translations["en"] = map[string]string{
		// Common
		"app.name":        "Vidveil",
		"app.tagline":     "Privacy-respecting adult video meta search",
		"app.description": "Search across multiple adult video sites without tracking",

		// Navigation
		"nav.home":        "Home",
		"nav.search":      "Search",
		"nav.preferences": "Preferences",
		"nav.about":       "About",
		"nav.privacy":     "Privacy",
		"nav.admin":       "Admin",

		// Search
		"search.placeholder":  "Search for videos...",
		"search.button":       "Search",
		"search.no_results":   "No results found",
		"search.loading":      "Searching...",
		"search.results":      "Results",
		"search.results_for":  "Results for",
		"search.load_more":    "Load More",
		"search.engines":      "Search Engines",
		"search.all_engines":  "All Engines",
		"search.select_all":   "Select All",
		"search.deselect_all": "Deselect All",

		// Age verification
		"age.title":    "Age Verification Required",
		"age.question": "Are you 18 years of age or older?",
		"age.yes":      "Yes, I am 18 or older",
		"age.no":       "No, I am under 18",
		"age.warning":  "This website contains adult content. You must be 18 years or older to enter.",
		"age.remember": "Remember my choice for 30 days",

		// Preferences
		"prefs.title":          "Preferences",
		"prefs.theme":          "Theme",
		"prefs.theme.dark":     "Dark",
		"prefs.theme.light":    "Light",
		"prefs.theme.auto":     "Auto (System)",
		"prefs.engines":        "Search Engines",
		"prefs.safe_search":    "Safe Search",
		"prefs.results_page":   "Results per Page",
		"prefs.save":           "Save Preferences",
		"prefs.saved":          "Preferences saved!",
		"prefs.reset":          "Reset to Defaults",

		// About
		"about.title":       "About Vidveil",
		"about.description": "Vidveil is a privacy-respecting adult video meta search engine.",
		"about.features":    "Features",
		"about.source":      "Source Code",
		"about.license":     "License",

		// Privacy
		"privacy.title":   "Privacy Policy",
		"privacy.summary": "We do not track you. We do not store your searches. We do not use cookies for tracking.",

		// Admin
		"admin.login":           "Login",
		"admin.logout":          "Logout",
		"admin.username":        "Username",
		"admin.password":        "Password",
		"admin.remember":        "Remember me",
		"admin.dashboard":       "Dashboard",
		"admin.settings":        "Settings",
		"admin.engines":         "Engines",
		"admin.logs":            "Logs",
		"admin.system":          "System",
		"admin.backup":          "Backup",
		"admin.invalid_creds":   "Invalid username or password",
		"admin.session_expired": "Session expired, please login again",

		// Common actions
		"action.save":    "Save",
		"action.cancel":  "Cancel",
		"action.delete":  "Delete",
		"action.edit":    "Edit",
		"action.confirm": "Confirm",
		"action.close":   "Close",
		"action.back":    "Back",
		"action.next":    "Next",
		"action.submit":  "Submit",
		"action.reset":   "Reset",
		"action.refresh": "Refresh",
		"action.copy":    "Copy",
		"action.copied":  "Copied!",

		// Status
		"status.enabled":  "Enabled",
		"status.disabled": "Disabled",
		"status.online":   "Online",
		"status.offline":  "Offline",
		"status.healthy":  "Healthy",
		"status.unhealthy": "Unhealthy",
		"status.loading":  "Loading...",
		"status.error":    "Error",
		"status.success":  "Success",
		"status.warning":  "Warning",

		// Errors
		"error.generic":       "An error occurred",
		"error.not_found":     "Page not found",
		"error.server":        "Server error",
		"error.unauthorized":  "Unauthorized",
		"error.forbidden":     "Access denied",
		"error.rate_limited":  "Too many requests, please try again later",
		"error.invalid_input": "Invalid input",

		// Time
		"time.now":       "Just now",
		"time.minutes":   "%d minutes ago",
		"time.hours":     "%d hours ago",
		"time.days":      "%d days ago",
		"time.duration":  "Duration",
		"time.views":     "views",

		// Footer
		"footer.copyright": "All rights reserved",
		"footer.powered":   "Powered by Vidveil",
	}
}

// T translates a key for the given locale
func (t *Translator) T(locale, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try exact locale
	if trans, ok := t.translations[locale]; ok {
		if val, ok := trans[key]; ok {
			return val
		}
	}

	// Try language part only (e.g., "en" from "en-US")
	if idx := strings.Index(locale, "-"); idx > 0 {
		lang := locale[:idx]
		if trans, ok := t.translations[lang]; ok {
			if val, ok := trans[key]; ok {
				return val
			}
		}
	}

	// Fall back to default locale
	if trans, ok := t.translations[t.fallback]; ok {
		if val, ok := trans[key]; ok {
			return val
		}
	}

	// Return key if no translation found
	return key
}

// TF translates with format arguments
func (t *Translator) TF(locale, key string, args ...interface{}) string {
	return fmt.Sprintf(t.T(locale, key), args...)
}

// GetLocale extracts the preferred locale from an HTTP request per TEMPLATE.md
func (t *Translator) GetLocale(r *http.Request) string {
	// Check query parameter first
	if locale := r.URL.Query().Get("lang"); locale != "" {
		if t.HasLocale(locale) {
			return locale
		}
	}

	// Check cookie
	if cookie, err := r.Cookie("locale"); err == nil && cookie.Value != "" {
		if t.HasLocale(cookie.Value) {
			return cookie.Value
		}
	}

	// Parse Accept-Language header per TEMPLATE.md PART 25
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		locales := parseAcceptLanguage(acceptLang)
		for _, locale := range locales {
			if t.HasLocale(locale) {
				return locale
			}
			// Try language part
			if idx := strings.Index(locale, "-"); idx > 0 {
				if t.HasLocale(locale[:idx]) {
					return locale[:idx]
				}
			}
		}
	}

	return t.fallback
}

// parseAcceptLanguage parses the Accept-Language header
func parseAcceptLanguage(header string) []string {
	var locales []string
	parts := strings.Split(header, ",")
	for _, part := range parts {
		// Remove quality value
		if idx := strings.Index(part, ";"); idx > 0 {
			part = part[:idx]
		}
		locale := strings.TrimSpace(part)
		if locale != "" {
			locales = append(locales, strings.ToLower(locale))
		}
	}
	return locales
}

// HasLocale checks if a locale is available
func (t *Translator) HasLocale(locale string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.translations[locale]
	return ok
}

// AvailableLocales returns all available locales
func (t *Translator) AvailableLocales() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	locales := make([]string, 0, len(t.translations))
	for locale := range t.translations {
		locales = append(locales, locale)
	}
	return locales
}

// AddTranslation adds or updates a translation
func (t *Translator) AddTranslation(locale, key, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.translations[locale]; !ok {
		t.translations[locale] = make(map[string]string)
	}
	t.translations[locale][key] = value
}

// LoadTranslations loads translations from a map
func (t *Translator) LoadTranslations(locale string, trans map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.translations[locale]; !ok {
		t.translations[locale] = make(map[string]string)
	}

	for k, v := range trans {
		t.translations[locale][k] = v
	}
}

// GetAllTranslations returns all translations for a locale
func (t *Translator) GetAllTranslations(locale string) map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if trans, ok := t.translations[locale]; ok {
		// Return a copy
		result := make(map[string]string)
		for k, v := range trans {
			result[k] = v
		}
		return result
	}

	return nil
}

// Middleware returns an HTTP middleware that adds the translator to context
func (t *Translator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := t.GetLocale(r)
		// Add locale to request context - can be retrieved by handlers
		r.Header.Set("X-Locale", locale)
		next.ServeHTTP(w, r)
	})
}

// TemplateFunc returns a template function for use in Go templates
func (t *Translator) TemplateFunc(locale string) func(key string, args ...interface{}) string {
	return func(key string, args ...interface{}) string {
		if len(args) > 0 {
			return t.TF(locale, key, args...)
		}
		return t.T(locale, key)
	}
}

// Global translator instance
var global *Translator
var once sync.Once

// Global returns the global translator instance
func Global() *Translator {
	once.Do(func() {
		global = New()
	})
	return global
}

// T is a convenience function for the global translator
func T(locale, key string) string {
	return Global().T(locale, key)
}

// TF is a convenience function for the global translator with formatting
func TF(locale, key string, args ...interface{}) string {
	return Global().TF(locale, key, args...)
}
