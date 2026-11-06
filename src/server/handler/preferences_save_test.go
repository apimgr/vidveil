// SPDX-License-Identifier: MIT
// AI.md PART 16: Coverage tests for the no-JS preferences save path —
// PreferencesSave, getRequestResultsPerPage, getRequestOpenNewTab.
package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── getRequestResultsPerPage ────────────────────────────────────────────────

func TestGetRequestResultsPerPage_NoCookie_ReturnsDefault(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)

	if got := h.getRequestResultsPerPage(req); got != "0" {
		t.Errorf("getRequestResultsPerPage() = %q, want %q", got, "0")
	}
}

func TestGetRequestResultsPerPage_ValidCookie_ReturnsValue(t *testing.T) {
	h := newRenderTestHandler()
	for _, want := range []string{"20", "50", "100"} {
		req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
		req.AddCookie(&http.Cookie{Name: resultsPerPageCookieName, Value: want})

		if got := h.getRequestResultsPerPage(req); got != want {
			t.Errorf("getRequestResultsPerPage() with cookie %q = %q, want %q", want, got, want)
		}
	}
}

func TestGetRequestResultsPerPage_InvalidCookie_ReturnsDefault(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	req.AddCookie(&http.Cookie{Name: resultsPerPageCookieName, Value: "9999"})

	if got := h.getRequestResultsPerPage(req); got != "0" {
		t.Errorf("getRequestResultsPerPage() with invalid cookie = %q, want %q", got, "0")
	}
}

// ── getRequestOpenNewTab ─────────────────────────────────────────────────────

func TestGetRequestOpenNewTab_NoCookie_ReturnsDefaultTrue(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)

	if got := h.getRequestOpenNewTab(req); got != true {
		t.Errorf("getRequestOpenNewTab() = %v, want true", got)
	}
}

func TestGetRequestOpenNewTab_CookieSetToOne_ReturnsTrue(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	req.AddCookie(&http.Cookie{Name: openNewTabCookieName, Value: "1"})

	if got := h.getRequestOpenNewTab(req); got != true {
		t.Errorf("getRequestOpenNewTab() with cookie=1 = %v, want true", got)
	}
}

func TestGetRequestOpenNewTab_CookieSetToZero_ReturnsFalse(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	req.AddCookie(&http.Cookie{Name: openNewTabCookieName, Value: "0"})

	if got := h.getRequestOpenNewTab(req); got != false {
		t.Errorf("getRequestOpenNewTab() with cookie=0 = %v, want false", got)
	}
}

func TestPreferencesPage_RendersSavedAppPreferenceCookies(t *testing.T) {
	setRealTemplatesFS(t)
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	req.Header.Set("Accept", "text/html")
	for name, value := range map[string]string{
		"vidveil_pref_max_history":        "25",
		"vidveil_pref_auto_clear_history": "7",
		"vidveil_pref_use_tor":            "1",
	} {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	h.PreferencesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("PreferencesPage status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	for _, expected := range []string{
		`<option value="25" selected>25</option>`,
		`<option value="7" selected>After 7 days</option>`,
		`id="use-tor" name="vidveil_pref_use_tor" value="1" checked`,
		`<input type="checkbox" checked disabled>`,
	} {
		if !strings.Contains(rr.Body.String(), expected) {
			t.Errorf("preferences page missing saved value %q", expected)
		}
	}
	if strings.Contains(rr.Body.String(), `name="vidveil_pref_proxy_images"`) {
		t.Error("thumbnail proxy must not be editable as a guest preference")
	}
}

func TestPreferencesPage_NoJSRestoresSavedEngineSelection(t *testing.T) {
	setRealTemplatesFS(t)
	h := newRenderTestHandler()
	h.engineMgr.InitializeEngines()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	req.Header.Set("User-Agent", "Lynx/2.9.0")
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: "vidveil_pref_engine_pornhub", Value: "1"})

	h.PreferencesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("PreferencesPage no-JS status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `name="engines" value="pornhub" checked`) {
		t.Error("no-JS preferences page did not restore the selected PornHub engine")
	}
	if strings.Contains(rr.Body.String(), `name="engines" value="xvideos" checked`) {
		t.Error("no-JS preferences page unexpectedly selected the unchecked XVideos engine")
	}
}

func TestPreferencesPage_EngineSelectionsDefaultToAllEnabled(t *testing.T) {
	setRealTemplatesFS(t)
	h := newRenderTestHandler()
	h.engineMgr.InitializeEngines()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	req.Header.Set("User-Agent", "Lynx/2.9.0")

	h.PreferencesPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("preferences page status = %d, want 200", rr.Code)
	}
	if len(h.engineMgr.ListEngines()) == 0 {
		t.Fatal("preferences page rendered no engine checkboxes")
	}
	for _, info := range h.engineMgr.ListEngines() {
		if !strings.Contains(rr.Body.String(), `name="engines" value="`+info.Name+`" checked>`) {
			t.Errorf("default engine checkbox %q was not checked", info.Name)
		}
	}
}

// ── PreferencesSave ──────────────────────────────────────────────────────────

func TestPreferencesSave_GetMethod_IsNotFound(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)

	h.PreferencesSave(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("PreferencesSave GET: status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	if len(rr.Result().Cookies()) != 0 {
		t.Errorf("PreferencesSave GET: expected no cookies set, got %d", len(rr.Result().Cookies()))
	}
}

func TestPreferencesSave_ValidPost_SetsAllCookiesAndRedirects(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set(resultsPerPageCookieName, "50")
	form.Set(openNewTabCookieName, "1")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("PreferencesSave POST: status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesSave POST: Location = %q, want /", loc)
	}

	got := map[string]string{}
	for _, c := range rr.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if got["theme"] != "dark" {
		t.Errorf("PreferencesSave POST: theme cookie = %q, want dark", got["theme"])
	}
	if got[resultsPerPageCookieName] != "50" {
		t.Errorf("PreferencesSave POST: %s cookie = %q, want 50", resultsPerPageCookieName, got[resultsPerPageCookieName])
	}
	if got[openNewTabCookieName] != "1" {
		t.Errorf("PreferencesSave POST: %s cookie = %q, want 1", openNewTabCookieName, got[openNewTabCookieName])
	}
}

func TestPreferencesSave_PersistsEngineSelectionIncludingUncheckedEngines(t *testing.T) {
	h := newRenderTestHandler()
	h.engineMgr.InitializeEngines()
	rr := httptest.NewRecorder()
	form := url.Values{}
	form.Set("engines_selection_set", "1")
	form.Add("engines", "pornhub")
	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	cookies := map[string]string{}
	for _, cookie := range rr.Result().Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
	if cookies[engineSelectionCookieName] != "1" {
		t.Errorf("engine selection marker = %q, want 1", cookies[engineSelectionCookieName])
	}
	if cookies[engineSelectionValueCookieName] != "pornhub" {
		t.Errorf("packed engine selection = %q, want pornhub", cookies[engineSelectionValueCookieName])
	}
	if len(rr.Result().Cookies()) > 30 {
		t.Errorf("preferences save emitted %d cookies; want a compact response", len(rr.Result().Cookies()))
	}
}

func TestPreferencesSave_OmittedPreferenceIsNotReset(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "light")
	form.Set(resultsPerPageCookieName, "20")
	// openNewTab intentionally omitted — an absent checkbox means "unchecked".

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	got := map[string]string{}
	for _, c := range rr.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if _, exists := got[openNewTabCookieName]; exists {
		t.Errorf("PreferencesSave POST: omitted preference was unexpectedly reset to %q", got[openNewTabCookieName])
	}
}

func TestPreferencesSave_InvalidThemeAndResultsPerPage_SkipsThoseCookies(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "not-a-real-theme")
	form.Set(resultsPerPageCookieName, "not-a-number")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	for _, c := range rr.Result().Cookies() {
		if c.Name == "theme" {
			t.Errorf("PreferencesSave POST: expected no theme cookie for invalid value, got %q", c.Value)
		}
		if c.Name == resultsPerPageCookieName {
			t.Errorf("PreferencesSave POST: expected no %s cookie for invalid value, got %q", resultsPerPageCookieName, c.Value)
		}
	}
}

// ── safeReturnPath / redirect parameter ─────────────────────────────────────

func TestPreferencesSave_ValidReturnTo_RedirectsToOriginatingPage(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("redirect", "/search?q=test")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/search?q=test" {
		t.Errorf("PreferencesSave POST redirect: Location = %q, want /search?q=test", loc)
	}
}

func TestPreferencesSave_CrossHostReturnTo_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("redirect", "https://evil.example/phish")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesSave POST cross-host redirect: Location = %q, want /", loc)
	}
}

func TestPreferencesSave_ProtocolRelativeReturnTo_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("redirect", "//evil.example/phish")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesSave POST protocol-relative redirect: Location = %q, want /", loc)
	}
}

func TestPreferencesSave_SelfLoopReturnTo_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("redirect", "/server/preferences")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("PreferencesSave POST self-loop redirect: Location = %q, want /", loc)
	}
}
