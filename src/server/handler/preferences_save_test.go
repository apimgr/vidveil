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

// ── PreferencesSave ──────────────────────────────────────────────────────────

func TestPreferencesSave_GetMethod_RedirectsWithoutSettingCookies(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/server/preferences/save", nil)

	h.PreferencesSave(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("PreferencesSave GET: status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != "/server/preferences" {
		t.Errorf("PreferencesSave GET: Location = %q, want /server/preferences", loc)
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
	form.Set("resultsPerPage", "50")
	form.Set("openNewTab", "on")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("PreferencesSave POST: status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != "/server/preferences" {
		t.Errorf("PreferencesSave POST: Location = %q, want /server/preferences", loc)
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

func TestPreferencesSave_OpenNewTabUnchecked_SetsCookieToZero(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "light")
	form.Set("resultsPerPage", "20")
	// openNewTab intentionally omitted — an absent checkbox means "unchecked".

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	got := map[string]string{}
	for _, c := range rr.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if got[openNewTabCookieName] != "0" {
		t.Errorf("PreferencesSave POST unchecked: %s cookie = %q, want 0", openNewTabCookieName, got[openNewTabCookieName])
	}
}

func TestPreferencesSave_InvalidThemeAndResultsPerPage_SkipsThoseCookies(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "not-a-real-theme")
	form.Set("resultsPerPage", "not-a-number")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
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

// ── safeReturnPath / return_to redirect ─────────────────────────────────────

func TestPreferencesSave_ValidReturnTo_RedirectsToOriginatingPage(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("return_to", "/search?q=test")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/search?q=test" {
		t.Errorf("PreferencesSave POST return_to: Location = %q, want /search?q=test", loc)
	}
}

func TestPreferencesSave_CrossHostReturnTo_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("return_to", "https://evil.example/phish")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/server/preferences" {
		t.Errorf("PreferencesSave POST cross-host return_to: Location = %q, want /server/preferences", loc)
	}
}

func TestPreferencesSave_ProtocolRelativeReturnTo_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("return_to", "//evil.example/phish")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/server/preferences" {
		t.Errorf("PreferencesSave POST protocol-relative return_to: Location = %q, want /server/preferences", loc)
	}
}

func TestPreferencesSave_SelfLoopReturnTo_FallsBackToPreferences(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()

	form := url.Values{}
	form.Set("theme", "dark")
	form.Set("return_to", "/server/preferences")

	req := httptest.NewRequest(http.MethodPost, "/server/preferences/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	h.PreferencesSave(rr, req)

	if loc := rr.Header().Get("Location"); loc != "/server/preferences" {
		t.Errorf("PreferencesSave POST self-loop return_to: Location = %q, want /server/preferences", loc)
	}
}
