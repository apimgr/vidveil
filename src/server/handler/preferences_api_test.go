package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/engine"
)

func TestWithRequestTorPreferenceUsesAppCookie(t *testing.T) {
	for _, test := range []struct {
		value string
		want  *bool
	}{
		{value: "1", want: boolPointer(true)},
		{value: "0", want: boolPointer(false)},
		{value: "invalid", want: nil},
	} {
		r := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
		r.AddCookie(&http.Cookie{Name: "vidveil_pref_use_tor", Value: test.value})
		got := engine.GetTorPrefFromContext(withRequestTorPreference(context.Background(), r))
		if test.want == nil {
			if got != nil {
				t.Errorf("cookie %q produced Tor preference %v, want inherited nil", test.value, *got)
			}
			continue
		}
		if got == nil || *got != *test.want {
			t.Errorf("cookie %q produced Tor preference %v, want %v", test.value, got, *test.want)
		}
	}
}

func TestRequestSearchPreferencesHonorCookiesAndExplicitQueryValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	r.AddCookie(&http.Cookie{Name: "vidveil_pref_show_ai_content", Value: "1"})
	r.AddCookie(&http.Cookie{Name: "vidveil_pref_default_preview_only", Value: "0"})
	r.AddCookie(&http.Cookie{Name: "vidveil_pref_default_duration", Value: "short"})
	r.AddCookie(&http.Cookie{Name: "vidveil_pref_default_sort", Value: "views"})
	r.AddCookie(&http.Cookie{Name: "vidveil_pref_min_quality", Value: "720"})
	r.AddCookie(&http.Cookie{Name: "vidveil_pref_min_duration", Value: "180"})

	if !requestShowAIContent(r) || requestPreviewFirst(r) {
		t.Fatal("search preferences did not apply show-AI and preview-first cookies")
	}
	filters := parseResultFilterOptions(r)
	if filters.MaxDuration != 599 || filters.SortBy != "views" || filters.MinQuality != 720 || filters.UserMinDuration != 180 {
		t.Fatalf("cookie-backed filters = %#v", filters)
	}

	query := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&show_ai=0&preview_first=1&duration=&sort=&min_quality=0&min_duration=0", nil)
	query.AddCookie(&http.Cookie{Name: "vidveil_pref_show_ai_content", Value: "1"})
	query.AddCookie(&http.Cookie{Name: "vidveil_pref_default_preview_only", Value: "0"})
	query.AddCookie(&http.Cookie{Name: "vidveil_pref_default_duration", Value: "short"})
	query.AddCookie(&http.Cookie{Name: "vidveil_pref_default_sort", Value: "views"})
	query.AddCookie(&http.Cookie{Name: "vidveil_pref_min_quality", Value: "720"})
	query.AddCookie(&http.Cookie{Name: "vidveil_pref_min_duration", Value: "180"})
	if requestShowAIContent(query) || !requestPreviewFirst(query) {
		t.Fatal("explicit search query did not override show-AI and preview-first cookies")
	}
	filters = parseResultFilterOptions(query)
	if filters.MaxDuration != 0 || filters.SortBy != "" || filters.MinQuality != 0 || filters.UserMinDuration != 0 {
		t.Fatalf("explicit query filters were overridden by cookies: %#v", filters)
	}
}

func boolPointer(value bool) *bool { return &value }

func TestPreferencesImportDirectValuesOverrideCodeAndSetsCookieContract(t *testing.T) {
	h := newRenderTestHandler()
	raw := "theme=dark&lang=fr&vidveil_pref_results_per_page=50"
	code := base64.RawURLEncoding.EncodeToString([]byte(raw))
	req := httptest.NewRequest(http.MethodGet, "https://example.test/server/preferences/import?code="+url.QueryEscape(code)+"&theme=light", nil)
	rr := httptest.NewRecorder()
	h.PreferencesImport(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Body.Len() != 0 || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("import response status/body/cache = %d/%q/%q", rr.Code, rr.Body.String(), rr.Header().Get("Cache-Control"))
	}
	cookies := map[string]*http.Cookie{}
	for _, cookie := range rr.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	if cookies["theme"] == nil || cookies["theme"].Value != "light" {
		t.Fatalf("theme cookie = %#v", cookies["theme"])
	}
	if cookies[resultsPerPageCookieName] == nil || cookies[resultsPerPageCookieName].Value != "50" {
		t.Fatalf("app preference cookie = %#v", cookies[resultsPerPageCookieName])
	}
	for _, name := range []string{"theme", "lang", resultsPerPageCookieName} {
		cookie := cookies[name]
		if cookie == nil {
			t.Fatalf("missing %s cookie", name)
		}
		if cookie.HttpOnly || cookie.Path != "/" || cookie.MaxAge != 31536000 || cookie.SameSite != http.SameSiteLaxMode || !cookie.Secure {
			t.Errorf("%s cookie attributes = %#v", name, cookie)
		}
	}
	for _, cookie := range cookies {
		if cookie.Name == "cookie_consent" || strings.Contains(cookie.Name, "token") {
			t.Errorf("credential/consent cookie was applied: %s", cookie.Name)
		}
	}
}

func TestPreferencesAPIGetAndSet(t *testing.T) {
	h := newRenderTestHandler()
	get := httptest.NewRecorder()
	h.PreferencesAPIGet(get, httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"prefs"`) || !strings.Contains(get.Body.String(), `"ok": true`) {
		t.Fatalf("GET response = %d %s", get.Code, get.Body.String())
	}

	body := `{"theme":"dark","vidveil_pref_results_per_page":"100","cookie_consent":"all","vidveil_pref_proxy_images":"0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	set := httptest.NewRecorder()
	h.PreferencesAPISet(set, req)
	if set.Code != http.StatusOK || !strings.Contains(set.Body.String(), `"applied"`) || !strings.Contains(set.Body.String(), `"ignored"`) || !strings.Contains(set.Body.String(), `"vidveil_pref_proxy_images"`) {
		t.Fatalf("POST response = %d %s", set.Code, set.Body.String())
	}
	var response struct {
		Data preferenceResult `json:"data"`
	}
	if err := json.Unmarshal(set.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode preference response: %v", err)
	}
	if len(response.Data.Applied) != 3 || len(response.Data.Ignored) != 1 || response.Data.Applied[1] != "vidveil_pref_proxy_images" || response.Data.Ignored[0] != "cookie_consent" {
		t.Fatalf("preference result = applied %v, ignored %v", response.Data.Applied, response.Data.Ignored)
	}
	cookies := set.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected exactly three preference cookies, got %d", len(cookies))
	}
	cookieNames := map[string]bool{}
	for _, cookie := range cookies {
		cookieNames[cookie.Name] = true
	}
	if !cookieNames["vidveil_pref_proxy_images"] || !cookieNames["vidveil_pref_results_per_page"] {
		t.Fatalf("generic preference cookies were not applied: %v", cookieNames)
	}
}

func TestPreferencesAPISetRejectsWhenNoSuppliedValueIsValid(t *testing.T) {
	h := newRenderTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences", strings.NewReader(`{"theme":"neon"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.PreferencesAPISet(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid save status = %d, want %d: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestSafePreferenceRedirectRejectsExternalAndMalformedTargets(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.test/server/preferences/import", nil)
	for _, target := range []string{"//evil.example", "/\\evil.example", "https://evil.example/path", "/path\r\nLocation: https://evil.example"} {
		if got := safeRelativePreferenceRedirect(target, req); got != "" {
			t.Errorf("unsafe redirect %q accepted as %q", target, got)
		}
	}
	req.Header.Set("Referer", "http://example.test/search")
	if got := safeRefererPreferenceRedirect(req.Referer(), req); got != "" {
		t.Errorf("cross-scheme Referer accepted as %q", got)
	}
}

func TestPreferencesAPIJSONEnvelopeIsValid(t *testing.T) {
	h := newRenderTestHandler()
	rr := httptest.NewRecorder()
	h.PreferencesAPIGet(rr, httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences", nil))
	var response map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response["data"]) == 0 {
		t.Fatalf("missing data envelope: %s", rr.Body.String())
	}
}
