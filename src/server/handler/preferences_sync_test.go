// SPDX-License-Identifier: MIT
// AI.md PART 16: Coverage tests for the stateless cross-device preference
// sync path — PreferencesExport, PreferencesImport, exportablePreferenceQuery,
// decodePreferenceCode.
package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── exportablePreferenceQuery ───────────────────────────────────────────────

func TestExportablePreferenceQuery_DefaultsToThemeAndLang(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/export", nil)

	q := h.exportablePreferenceQuery(req)
	if q == "" {
		t.Fatal("exportablePreferenceQuery() returned empty string")
	}
	if got := req.URL.Query(); got.Get("theme") != "" {
		t.Errorf("exportablePreferenceQuery() must not mutate the request URL")
	}
}

func TestExportablePreferenceQuery_ReflectsCookies(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/export", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	req.AddCookie(&http.Cookie{Name: resultsPerPageCookieName, Value: "50"})

	q := h.exportablePreferenceQuery(req)
	if q != "lang=fr&theme=light&vidveil_pref_results_per_page=50" {
		t.Errorf("exportablePreferenceQuery() = %q, want canonical app preference included", q)
	}
}

// ── decodePreferenceCode ────────────────────────────────────────────────────

func TestDecodePreferenceCode_ValidCode_Decodes(t *testing.T) {
	code := base64.RawURLEncoding.EncodeToString([]byte("theme=dark&lang=fr"))
	got, ok := decodePreferenceCode(code)
	if !ok {
		t.Fatal("decodePreferenceCode() ok = false, want true")
	}
	if got != "theme=dark&lang=fr" {
		t.Errorf("decodePreferenceCode() = %q, want %q", got, "theme=dark&lang=fr")
	}
}

func TestDecodePreferenceCode_PastedFullURL_StripsPrefix(t *testing.T) {
	code := base64.RawURLEncoding.EncodeToString([]byte("theme=dark&lang=fr"))
	pasted := "https://example.com/server/preferences/import?" + code
	got, ok := decodePreferenceCode(pasted)
	if !ok {
		t.Fatal("decodePreferenceCode() ok = false, want true")
	}
	if got != "theme=dark&lang=fr" {
		t.Errorf("decodePreferenceCode() pasted URL = %q, want %q", got, "theme=dark&lang=fr")
	}
}

func TestDecodePreferenceCode_EmptyString_ReturnsFalse(t *testing.T) {
	if _, ok := decodePreferenceCode(""); ok {
		t.Error("decodePreferenceCode(\"\") ok = true, want false")
	}
}

func TestDecodePreferenceCode_PlainQueryAndPaddedCode(t *testing.T) {
	if got, ok := decodePreferenceCode("theme=dark&lang=fr"); !ok || got != "theme=dark&lang=fr" {
		t.Fatalf("plain query decode = %q, %v", got, ok)
	}
	code := base64.URLEncoding.EncodeToString([]byte("theme=dark&lang=fr"))
	if got, ok := decodePreferenceCode(code); !ok || got != "theme=dark&lang=fr" {
		t.Fatalf("padded code decode = %q, %v", got, ok)
	}
}

func TestDecodePreferenceCode_InvalidBase64_ReturnsFalse(t *testing.T) {
	if _, ok := decodePreferenceCode("not-valid-base64!!!"); ok {
		t.Error("decodePreferenceCode(invalid) ok = true, want false")
	}
}

func TestDecodePreferenceCode_OversizedCodeIsIgnored(t *testing.T) {
	if _, ok := decodePreferenceCode(strings.Repeat("a", 2049)); ok {
		t.Error("decodePreferenceCode(oversized) ok = true, want false")
	}
}

// ── PreferencesExport ────────────────────────────────────────────────────────

func TestPreferencesExport_HTML_RendersWithURLAndCode(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/export", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})

	h.PreferencesExport(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PreferencesExport HTML: status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.Len() == 0 {
		t.Error("PreferencesExport HTML: expected non-empty body")
	}
}

func TestPreferencesExport_JSON_ReturnsURLAndCode(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/export", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})

	h.PreferencesExport(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PreferencesExport JSON: status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"url"`) || !strings.Contains(body, `"code"`) {
		t.Errorf("PreferencesExport JSON: body = %s, want url and code fields", body)
	}
}

// ── PreferencesImport ────────────────────────────────────────────────────────

func TestPreferencesImport_ValidQueryParams_SetsCookiesAndRedirects(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=light&lang=es", nil)

	h.PreferencesImport(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("PreferencesImport: status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	got := map[string]string{}
	for _, c := range rr.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if got["theme"] != "light" {
		t.Errorf("PreferencesImport: theme cookie = %q, want light", got["theme"])
	}
	if got["lang"] != "es" {
		t.Errorf("PreferencesImport: lang cookie = %q, want es", got["lang"])
	}
}

func TestPreferencesImport_ValidCode_SetsCookiesAndRedirects(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	code := base64.RawURLEncoding.EncodeToString([]byte("theme=dark&lang=de"))
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?code="+code, nil)

	h.PreferencesImport(rr, req)

	got := map[string]string{}
	for _, c := range rr.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if got["theme"] != "dark" {
		t.Errorf("PreferencesImport code: theme cookie = %q, want dark", got["theme"])
	}
	if got["lang"] != "de" {
		t.Errorf("PreferencesImport code: lang cookie = %q, want de", got["lang"])
	}
}

func TestPreferencesImport_IsAdditiveAndIgnoresExcludedCookies(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=light&cookie_consent=all&vidveil_build=build&owner_token=secret&vidveil_pref_proxy_images=0", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	req.AddCookie(&http.Cookie{Name: resultsPerPageCookieName, Value: "50"})

	h.PreferencesImport(rr, req)

	got := map[string]string{}
	for _, cookie := range rr.Result().Cookies() {
		got[cookie.Name] = cookie.Value
	}
	if got["theme"] != "light" {
		t.Errorf("theme cookie = %q, want light", got["theme"])
	}
	if _, exists := got["lang"]; exists {
		t.Errorf("absent lang preference should not be rewritten: %#v", got)
	}
	if _, exists := got[resultsPerPageCookieName]; exists {
		t.Errorf("absent app preference should not be rewritten: %#v", got)
	}
	for _, name := range []string{"cookie_consent", "vidveil_build", "owner_token", "vidveil_pref_proxy_images"} {
		if _, exists := got[name]; exists {
			t.Errorf("excluded cookie %q was applied", name)
		}
	}
}

func TestPreferencesImport_OversizedCodeDoesNotSetCookies(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?code="+strings.Repeat("a", 2049), nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})

	h.PreferencesImport(rr, req)

	if rr.Code != http.StatusSeeOther || rr.Body.Len() != 0 || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("oversized import response = %d, body %q, cache %q", rr.Code, rr.Body.String(), rr.Header().Get("Cache-Control"))
	}
	if cookies := rr.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("oversized code must be ignored without rewriting cookies, got %#v", cookies)
	}
}

func TestPreferencesImport_InvalidTheme_DropsThemeSilently(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=not-a-theme&lang=en", nil)

	h.PreferencesImport(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("PreferencesImport invalid theme: status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "theme" {
			t.Errorf("PreferencesImport invalid theme: expected no theme cookie, got %q", c.Value)
		}
	}
}

func TestPreferencesImport_UnsupportedLang_DropsLangSilently(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=dark&lang=xx-not-real", nil)

	h.PreferencesImport(rr, req)

	got := map[string]string{}
	for _, c := range rr.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if got["theme"] != "dark" {
		t.Errorf("PreferencesImport unsupported lang: theme cookie = %q, want dark", got["theme"])
	}
	if _, ok := got["lang"]; ok {
		t.Errorf("PreferencesImport unsupported lang: expected no lang cookie, got %q", got["lang"])
	}
}

func TestPreferencesImport_NoReferer_RedirectsToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=dark", nil)

	h.PreferencesImport(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesImport no referer: Location = %q, want /", loc)
	}
	if rr.Body.Len() != 0 || rr.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("import response must be bodyless and no-store")
	}
}

func TestPreferencesImport_ValidReferer_RedirectsToReferer(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=dark", nil)
	req.Header.Set("Referer", "/search?q=test")

	h.PreferencesImport(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/search?q=test" {
		t.Errorf("PreferencesImport valid referer: Location = %q, want /search?q=test", loc)
	}
}

// safeReturnPath already rejects /server/preferences/export and /server/preferences/import
// as bounce-back targets (handlers.go's switch on path) — verify PreferencesImport
// falls back to /server/preferences instead of looping back to itself or /export.
func TestPreferencesImport_RefererIsImportPage_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=dark", nil)
	req.Header.Set("Referer", "/server/preferences/import")

	h.PreferencesImport(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesImport referer=self: Location = %q, want /", loc)
	}
}

func TestPreferencesImport_RefererIsExportPage_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/import?theme=dark", nil)
	req.Header.Set("Referer", "/server/preferences/export")

	h.PreferencesImport(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesImport referer=export: Location = %q, want /", loc)
	}
}

func TestSafeReturnPath_RejectsPreferencesExportAndImport(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)

	if got := safeReturnPath("/server/preferences/export", req); got != "" {
		t.Errorf("safeReturnPath(/server/preferences/export) = %q, want empty", got)
	}
	if got := safeReturnPath("/server/preferences/import", req); got != "" {
		t.Errorf("safeReturnPath(/server/preferences/import) = %q, want empty", got)
	}
}
