package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

var preferenceKeyPattern = regexp.MustCompile(`^vidveil_pref_[a-z][a-z0-9_]{0,63}$`)
var preferenceValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

type preferenceResult struct {
	Theme   string            `json:"theme"`
	Lang    string            `json:"lang"`
	Prefs   map[string]string `json:"prefs"`
	Applied []string          `json:"applied"`
	Ignored []string          `json:"ignored"`
}

func (h *SearchHandler) effectivePreferences(r *http.Request) preferenceResult {
	prefs := map[string]string{}
	for _, c := range r.Cookies() {
		if preferenceKeyPattern.MatchString(c.Name) {
			if _, ok := validatePreference(c.Name, c.Value); ok {
				prefs[strings.TrimPrefix(c.Name, "vidveil_pref_")] = c.Value
			}
		}
	}
	lang := i18n.DefaultLocale
	if detected, ok := canonicalPreferenceLang(resolveLocale(r)); ok {
		lang = detected
	}
	if c, err := r.Cookie("lang"); err == nil {
		if valid, ok := canonicalPreferenceLang(c.Value); ok {
			lang = valid
		}
	}
	return preferenceResult{Theme: h.getRequestTheme(r), Lang: lang, Prefs: prefs, Applied: []string{}, Ignored: []string{}}
}

func validatePreference(name, value string) (string, bool) {
	if len(value) > 64 {
		return "", false
	}
	switch name {
	case "theme":
		return value, value == "dark" || value == "light" || value == "auto"
	case "lang":
		return canonicalPreferenceLang(value)
	}
	if !preferenceKeyPattern.MatchString(name) || !preferenceValuePattern.MatchString(value) {
		return "", false
	}
	enums := map[string]map[string]struct{}{
		"vidveil_pref_grid_density":         setOf("comfortable", "default", "compact"),
		"vidveil_pref_thumbnail_size":       setOf("small", "medium", "large"),
		"vidveil_pref_autoplay_preview":     setOf("0", "1"),
		"vidveil_pref_preview_delay":        setOf("0", "200", "500", "1000"),
		resultsPerPageCookieName:            setOf("0", "20", "50", "100"),
		openNewTabCookieName:                setOf("0", "1"),
		"vidveil_pref_default_preview_only": setOf("0", "1"),
		"vidveil_pref_show_ai_content":      setOf("0", "1"),
		"vidveil_pref_default_duration":     setOf("any", "short", "medium", "long"),
		"vidveil_pref_min_quality":          setOf("0", "240", "360", "480", "720", "1080", "2160"),
		"vidveil_pref_default_sort":         setOf("any", "duration-desc", "duration-asc", "views", "quality"),
		"vidveil_pref_min_duration":         setOf("0", "60", "180", "300", "600", "1200", "1800"),
		"vidveil_pref_max_history":          setOf("0", "10", "25", "50", "100"),
		"vidveil_pref_auto_clear_history":   setOf("0", "1", "7", "30"),
		"vidveil_pref_use_tor":              setOf("0", "1"),
	}
	if allowed, documented := enums[name]; documented {
		_, ok := allowed[value]
		return value, ok
	}
	return value, true
}

func setOf(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func canonicalPreferenceLang(value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, locale := range i18n.GlobalTranslator().AvailableLocales() {
		if strings.EqualFold(locale, value) {
			return locale, true
		}
	}
	if i := strings.IndexAny(value, "-_"); i > 0 {
		base := value[:i]
		for _, locale := range i18n.GlobalTranslator().AvailableLocales() {
			if strings.EqualFold(locale, base) {
				return locale, true
			}
		}
	}
	return "", false
}

func preferenceNames(values url.Values) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (h *SearchHandler) applyPreferences(w http.ResponseWriter, r *http.Request, values url.Values) preferenceResult {
	result := h.effectivePreferences(r)
	result.Applied, result.Ignored = []string{}, []string{}
	secure := strings.HasPrefix(urlSchemeForRequest(r), "https")
	seen := 0
	for _, name := range preferenceNames(values) {
		valueSet := values[name]
		if len(valueSet) > 1 {
			valueSet = valueSet[len(valueSet)-1:]
		}
		for _, raw := range valueSet {
			if seen >= 32 || len(raw) > 64 {
				result.Ignored = append(result.Ignored, name)
				continue
			}
			seen++
			value, ok := validatePreference(name, raw)
			if !ok {
				result.Ignored = append(result.Ignored, name)
				continue
			}
			if name == "code" || name == "redirect" {
				result.Ignored = append(result.Ignored, name)
				continue
			}
			http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: 31536000, Secure: secure, SameSite: http.SameSiteLaxMode})
			result.Applied = append(result.Applied, name)
			if name == "theme" {
				result.Theme = value
			} else if name == "lang" {
				result.Lang = value
			} else if strings.HasPrefix(name, "vidveil_pref_") {
				result.Prefs[strings.TrimPrefix(name, "vidveil_pref_")] = value
			}
		}
	}
	return result
}

func urlSchemeForRequest(r *http.Request) string {
	proto, _, _ := urlvar.GetURLVars(r)
	return proto
}

func (h *SearchHandler) PreferencesAPIGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	SendOK(w, h.effectivePreferences(r))
}

func (h *SearchHandler) PreferencesAPISet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	values := url.Values{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var input map[string]json.RawMessage
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&input); err != nil {
			SendError(w, "VALIDATION_FAILED", "Invalid preference payload")
			return
		}
		for key, raw := range input {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				values.Add(key, "")
				continue
			}
			values.Add(key, value)
		}
	} else {
		if err := r.ParseForm(); err == nil {
			values = r.PostForm
		}
	}
	result := h.applyPreferences(w, r, values)
	if len(result.Applied) == 0 {
		SendError(w, "VALIDATION_FAILED", "No valid preferences were supplied")
		return
	}
	SendOK(w, result)
}
