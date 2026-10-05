// SPDX-License-Identifier: MIT
// Regression tests for the open-redirect guard on user-supplied ?redirect /
// redirect= form fields (age-verify and content-restricted acknowledgment
// flows, and the favorites toggle's return path). The previous check was a bare
// strings.HasPrefix(target, "/"), which accepts protocol-relative targets
// ("//evil.com", "/\evil.com") that browsers resolve to an off-site host.
package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// offSiteRedirects are targets that must never survive safeLocalRedirect.
var offSiteRedirects = []struct {
	name   string
	target string
}{
	{"protocol-relative", "//evil.example/phish"},
	{"triple-slash", "///evil.example/phish"},
	{"backslash", "/\\evil.example/phish"},
	{"absolute-http", "http://evil.example/phish"},
	{"absolute-https", "https://evil.example/phish"},
	{"scheme-relative-https", "\\\\evil.example/phish"},
	{"relative-no-slash", "evil.example/phish"},
	{"tab-prefixed", "/\tevil.example"},
	{"cr-injected", "/x\r\nLocation: https://evil.example"},
	{"empty", ""},
}

func TestSafeLocalRedirect_RejectsOffSiteTargets(t *testing.T) {
	for _, tc := range offSiteRedirects {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeLocalRedirect(tc.target); got != "/" {
				t.Errorf("safeLocalRedirect(%q) = %q, want /", tc.target, got)
			}
		})
	}
}

func TestSafeLocalRedirect_AllowsSameSitePaths(t *testing.T) {
	for _, target := range []string{"/", "/search?q=cat", "/favorites?page=2", "/a/b#frag"} {
		if got := safeLocalRedirect(target); got != target {
			t.Errorf("safeLocalRedirect(%q) = %q, want unchanged", target, got)
		}
	}
}

// Each redirect-emitting handler must send a same-origin Location header for
// every off-site target. The rendered page paths (AgeVerifyPage,
// ContentRestrictedPage) are covered via the submit handlers below, which
// share the same guard.
func TestAgeVerifySubmit_ProtocolRelativeRedirect_FallsBackHome(t *testing.T) {
	h := newAPITestHandler()
	form := url.Values{"redirect": {"//evil.example/phish"}}
	r := httptest.NewRequest(http.MethodPost, "/age-verify/submit", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AgeVerifySubmit(w, r)

	assertSameSiteLocation(t, "AgeVerifySubmit", w)
}

func TestAgeVerifySubmit_BackslashRedirect_FallsBackHome(t *testing.T) {
	h := newAPITestHandler()
	form := url.Values{"redirect": {"/\\evil.example/phish"}}
	r := httptest.NewRequest(http.MethodPost, "/age-verify/submit", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AgeVerifySubmit(w, r)

	assertSameSiteLocation(t, "AgeVerifySubmit", w)
}

func TestAgeVerifySubmit_SameSiteRedirect_Preserved(t *testing.T) {
	h := newAPITestHandler()
	form := url.Values{"redirect": {"/search?q=cat"}}
	r := httptest.NewRequest(http.MethodPost, "/age-verify/submit", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.AgeVerifySubmit(w, r)

	if loc := w.Header().Get("Location"); loc != "/search?q=cat" {
		t.Errorf("AgeVerifySubmit same-site redirect: Location = %q, want /search?q=cat", loc)
	}
}

func TestAgeVerifyPage_AlreadyVerified_ProtocolRelativeRedirect_FallsBackHome(t *testing.T) {
	h := newMiscTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/age-verify?redirect=%2F%2Fevil.example%2Fphish", nil)
	r.AddCookie(&http.Cookie{Name: ageVerifyCookieName, Value: "1"})
	w := httptest.NewRecorder()

	h.AgeVerifyPage(w, r)

	assertSameSiteLocation(t, "AgeVerifyPage", w)
}

func TestContentRestrictedSubmit_ProtocolRelativeRedirect_FallsBackHome(t *testing.T) {
	h := newAPITestHandler()
	form := url.Values{"redirect": {"//evil.example/phish"}}
	r := httptest.NewRequest(http.MethodPost, "/content-restricted/submit", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.ContentRestrictedSubmit(w, r)

	assertSameSiteLocation(t, "ContentRestrictedSubmit", w)
}

func TestContentRestrictedPage_AlreadyAcked_ProtocolRelativeRedirect_FallsBackHome(t *testing.T) {
	h := newAPITestHandler()
	r := httptest.NewRequest(http.MethodGet, "/content-restricted?redirect=%2F%2Fevil.example%2Fphish", nil)
	r.AddCookie(&http.Cookie{Name: contentRestrictionAckCookieName, Value: "1"})
	w := httptest.NewRecorder()

	h.ContentRestrictedPage(w, r)

	assertSameSiteLocation(t, "ContentRestrictedPage", w)
}

func TestFavoritesToggle_BackslashRedirect_FallsBackFavorites(t *testing.T) {
	h := newAPITestHandler()
	form := url.Values{
		"redirect": {"/\\evil.example/phish"},
		"url":      {"https://example.com/video"},
		"title":    {"Example"},
	}
	r := httptest.NewRequest(http.MethodPost, "/favorites/save", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.FavoritesSave(w, r)

	// The guard makes the handler fall back to its own /favorites default.
	if loc := w.Header().Get("Location"); loc != "/favorites" {
		t.Errorf("FavoritesSave backslash redirect: Location = %q, want /favorites", loc)
	}
}

// assertSameSiteLocation fails when a 302's Location would send the browser to
// another host: either an absolute URL with a foreign host, or a
// protocol-relative / backslash-prefixed target.
func assertSameSiteLocation(t *testing.T, handlerName string, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusFound {
		t.Fatalf("%s: status = %d, want 302", handlerName, w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatalf("%s: empty Location header", handlerName)
	}
	if strings.HasPrefix(loc, "//") || strings.HasPrefix(loc, "/\\") || strings.HasPrefix(loc, `\\`) {
		t.Errorf("%s: Location = %q is protocol-relative (open redirect)", handlerName, loc)
		return
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Errorf("%s: unparseable Location %q: %v", handlerName, loc, err)
		return
	}
	if u.Host != "" {
		t.Errorf("%s: Location = %q points at foreign host %q", handlerName, loc, u.Host)
	}
	if !strings.HasPrefix(u.Path, "/") {
		t.Errorf("%s: Location = %q is not rooted at /", handlerName, loc)
	}
}
