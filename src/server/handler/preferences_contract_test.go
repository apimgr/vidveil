// SPDX-License-Identifier: MIT
package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPreferencesPageRendersInlineSyncAndNativeActions(t *testing.T) {
	setRealTemplatesFS(t)
	h := newRenderTestHandler()
	for _, userAgent := range []string{"Mozilla/5.0", "Lynx/2.9"} {
		req := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
		req.Header.Set("Accept", "text/html")
		req.Header.Set("User-Agent", userAgent)
		response := httptest.NewRecorder()
		h.PreferencesPage(response, req)
		body := response.Body.String()
		if response.Code != http.StatusOK {
			t.Fatalf("PreferencesPage(%q) status = %d", userAgent, response.Code)
		}
		if !strings.Contains(body, `id="sync-url"`) || !strings.Contains(body, `id="sync-code"`) || !strings.Contains(body, `name="code"`) {
			t.Errorf("PreferencesPage(%q) omitted inline export/import controls", userAgent)
		}
		if !strings.Contains(body, `name="consent_form" value="1"`) || !strings.Contains(body, `name="consent_analytics"`) || !strings.Contains(body, `name="ccpa_opt_out"`) {
			t.Errorf("PreferencesPage(%q) omitted consent controls", userAgent)
		}
		if !strings.Contains(body, `popovertarget="`) || !strings.Contains(body, `value="reset"`) || !strings.Contains(body, `href="/"`) {
			t.Errorf("PreferencesPage(%q) omitted native reset or Cancel controls", userAgent)
		}
		if strings.Contains(body, `action="/server/preferences/import"`) {
			t.Errorf("PreferencesPage(%q) rendered a separate import form", userAgent)
		}
		if !strings.Contains(body, `href="/server/preferences/tokens"`) || strings.Contains(body, `action="/server/preferences/token/reveal"`) || strings.Contains(body, `action="/server/preferences/token/restore"`) {
			t.Errorf("PreferencesPage(%q) did not keep token management on its own page", userAgent)
		}
	}
}

func TestPreferencesSaveCodeOverridesFormAndAppliesConsent(t *testing.T) {
	h := newRenderTestHandler()
	code := base64.RawURLEncoding.EncodeToString([]byte("lang=fr&theme=dark"))
	form := url.Values{
		"theme":               {"light"},
		"code":                {code},
		"consent_form":        {"1"},
		"consent_preferences": {"1"},
		"consent_analytics":   {"1"},
		"ccpa_opt_out":        {"1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	h.PreferencesSave(response, req)
	if response.Code != http.StatusSeeOther || response.Body.Len() != 0 {
		t.Fatalf("PreferencesSave status/body = %d/%q, want 303/empty", response.Code, response.Body.String())
	}
	cookies := map[string]*http.Cookie{}
	for _, cookie := range response.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	if cookies["theme"] == nil || cookies["theme"].Value != "dark" || cookies["lang"] == nil || cookies["lang"].Value != "fr" {
		t.Fatalf("imported code did not override form preferences: %#v", cookies)
	}
	if cookies["ccpa_opt_out"] == nil || cookies["ccpa_opt_out"].Value != "true" || cookies["cookie_consent"] == nil {
		t.Fatalf("web consent choices were not applied: %#v", cookies)
	}
	if !strings.Contains(cookies["cookie_consent"].Value, "%22analytics%22%3Atrue") {
		t.Errorf("analytics consent was not recorded: %q", cookies["cookie_consent"].Value)
	}
}

func TestPreferencesResetDeletesAppCookiesAndPreservesCredentials(t *testing.T) {
	h := newRenderTestHandler()
	form := url.Values{"action": {"reset"}, "redirect": {"/search?q=hello"}}
	req := httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, name := range []string{"theme", "lang", "vidveil_pref_results_per_page", "cookie_consent", "ccpa_opt_out", "vidveil_build", "content_ack", "age_verified", "dismissed_announcements", "forward_ip", "csrf_token", "vidveil_owner_token_ab5213f6"} {
		req.AddCookie(&http.Cookie{Name: name, Value: "present"})
	}
	response := httptest.NewRecorder()
	h.PreferencesSave(response, req)
	if response.Code != http.StatusSeeOther || response.Header().Get("Clear-Site-Data") != `"storage"` || response.Body.Len() != 0 {
		t.Fatalf("reset response = %d, Clear-Site-Data %q, body %q", response.Code, response.Header().Get("Clear-Site-Data"), response.Body.String())
	}
	deleted := map[string]bool{}
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			deleted[cookie.Name] = true
		}
	}
	for _, name := range []string{"theme", "lang", "vidveil_pref_results_per_page", "cookie_consent", "ccpa_opt_out", "vidveil_build", "content_ack", "age_verified", "dismissed_announcements", "forward_ip"} {
		if !deleted[name] {
			t.Errorf("reset did not delete %q", name)
		}
	}
	if deleted["csrf_token"] || deleted["vidveil_owner_token_ab5213f6"] {
		t.Error("reset deleted a CSRF or credential cookie")
	}
}

func TestApplyPreferencesDoesNotReportControlFieldsAsIgnored(t *testing.T) {
	h := newRenderTestHandler()
	values := url.Values{
		"csrf_token": {"token"}, "code": {"bad"}, "redirect": {"/"}, "action": {"save"},
		"consent_form": {"1"}, "consent_preferences": {"1"}, "consent_analytics": {"1"}, "ccpa_opt_out": {"1"},
		"theme": {"neon"},
	}
	response := httptest.NewRecorder()
	result := h.applyPreferences(response, httptest.NewRequest(http.MethodPost, "/", nil), values)
	if len(result.Ignored) != 1 || result.Ignored[0] != "theme" {
		t.Fatalf("ignored control fields = %#v, want only invalid theme", result.Ignored)
	}
}
