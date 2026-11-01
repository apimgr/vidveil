//go:build e2e

package e2e

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Tier 3 - full browser with JavaScript enabled. Enhanced flows, zero console
// errors, and no horizontal scrolling at mobile width (AI.md PART 28).

func assertNoConsoleErrors(t *testing.T, tb *tab, page string) {
	t.Helper()
	for _, msg := range tb.consoleErrors() {
		t.Errorf("%s produced a console error: %s", page, msg)
	}
}

func TestBrowserHomePageIsClean(t *testing.T) {
	tb := newTab(t)
	var title string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitVisible(`form input[name="q"]`, chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("navigate home: %v", err)
	}
	if strings.TrimSpace(title) == "" {
		t.Error("home page has an empty <title>")
	}
	assertNoConsoleErrors(t, tb, "/")
}

func TestBrowserSearchEnhancedFlow(t *testing.T) {
	tb := newTab(t)
	var location string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitVisible(`form.search-form--large input[name="q"]`, chromedp.ByQuery),
		chromedp.SendKeys(`form.search-form--large input[name="q"]`, "browser probe", chromedp.ByQuery),
		chromedp.Click(`form.search-form--large button[type="submit"]`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("browser search: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(tb.ctx, chromedp.Location(&location)); err != nil {
			t.Fatalf("read browser location: %v", err)
		}
		if strings.Contains(location, "/search") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(location, "/search") {
		t.Errorf("search did not land on /search, got %s", location)
	}
	assertNoConsoleErrors(t, tb, "/search")
}

func TestBrowserBangAutocomplete(t *testing.T) {
	tb := newTab(t)
	var status int64
	var fetchError string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitVisible(`form.search-form--large input[name="q"]`, chromedp.ByQuery),
		chromedp.Evaluate(`fetch('/api/v1/bangs/autocomplete?q=y').then(r => r.status).catch(e => { window.__autocompleteFetchError = e.name + ': ' + e.message; return 0; })`, &status,
			func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }),
	)
	if err != nil {
		t.Fatalf("bang autocomplete: %v", err)
	}
	if status != 200 {
		_ = chromedp.Run(tb.ctx, chromedp.Evaluate(`window.__autocompleteFetchError || ''`, &fetchError))
		t.Errorf("/api/v1/bangs/autocomplete returned %d (%s)", status, fetchError)
	}
	assertNoConsoleErrors(t, tb, "/")
}

func TestBrowserPreferencesEnhancedFlow(t *testing.T) {
	tb := newTab(t)
	var cookie, stored, density, debug string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/server/preferences"),
		chromedp.WaitVisible("form", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`JSON.stringify({form:!!document.getElementById('preferences-form'),managed:document.getElementById('preferences-form')?.dataset.managed,buttonForm:document.getElementById('preferences-save-btn')?.form?.id,grid:document.getElementById('grid-density')?.value,helper:typeof mirrorPreferencesToCookies})`, &debug),
		chromedp.SetValue("#grid-density", "compact", chromedp.ByQuery),
		chromedp.Click("#preferences-save-btn", chromedp.ByQuery),
		chromedp.Sleep(250*time.Millisecond),
		chromedp.Evaluate(`document.cookie`, &cookie),
		chromedp.Evaluate(`localStorage.getItem('vidveil_prefs') || ''`, &stored),
		chromedp.Navigate(s.browserBase+"/server/preferences"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('grid-density').value`, &density),
	)
	if err != nil {
		t.Fatalf("preferences page: %v", err)
	}
	if !strings.Contains(cookie, "vidveil_pref_grid_density=compact") {
		t.Errorf("preference save did not set the grid-density cookie: %q (page state %s)", cookie, debug)
	}
	if stored != "" {
		t.Errorf("preferences remained load-bearing in localStorage: %q", stored)
	}
	if density != "compact" {
		t.Errorf("server-rendered preference value = %q, want compact", density)
	}
	assertNoConsoleErrors(t, tb, "/server/preferences")
}

func TestBrowserFavoritesEnhancedFlow(t *testing.T) {
	tb := newTab(t)
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/favorites"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("favorites page: %v", err)
	}
	assertNoConsoleErrors(t, tb, "/favorites")
}

func TestBrowserThemeTogglesComputedStyle(t *testing.T) {
	tb := newTab(t)
	host, err := url.Parse(s.browserBase)
	if err != nil {
		t.Fatal(err)
	}
	var dark, light string
	err = chromedp.Run(tb.ctx,
		network.SetCookie("theme", "dark").WithDomain(host.Hostname()).WithPath("/"),
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(250*time.Millisecond),
		chromedp.Evaluate(`getComputedStyle(document.documentElement).getPropertyValue('--color-bg').trim()`, &dark),
		network.SetCookie("theme", "light").WithDomain(host.Hostname()).WithPath("/"),
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(250*time.Millisecond),
		chromedp.Evaluate(`getComputedStyle(document.documentElement).getPropertyValue('--color-bg').trim()`, &light),
	)
	if err != nil {
		t.Fatalf("theme toggle: %v", err)
	}
	if dark == light {
		t.Errorf("light and dark themes compute the same body background (%s)", dark)
	}
}

func TestBrowserResponsiveNoHorizontalScroll(t *testing.T) {
	tb := newTab(t)
	for _, path := range []string{"/", "/search?q=responsive", "/favorites", "/server/preferences", "/server/about"} {
		var overflow bool
		var overflowElements string
		err := chromedp.Run(tb.ctx,
			chromedp.EmulateViewport(375, 812),
			chromedp.Navigate(s.browserBase+path),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
			chromedp.Evaluate(`document.documentElement.scrollWidth > document.documentElement.clientWidth + 1`, &overflow),
			chromedp.Evaluate(`JSON.stringify(Array.from(document.querySelectorAll('body *')).map(e => { const r=e.getBoundingClientRect(); return {tag:e.tagName, className:typeof e.className==='string'?e.className:'', id:e.id, left:Math.round(r.left), right:Math.round(r.right), width:Math.round(r.width)}; }).filter(e => e.right > innerWidth + 1 || e.left < -1).slice(0,10))`, &overflowElements),
		)
		if err != nil {
			t.Errorf("responsive check %s: %v", path, err)
			continue
		}
		if overflow {
			t.Errorf("%s scrolls horizontally at 375x812: %s", path, overflowElements)
		}
	}
}

func TestBrowserSkipLinksAreFirstFocusable(t *testing.T) {
	tb := newTab(t)
	var href string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`(function () {
			const focusable = document.querySelectorAll('a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])');
			return focusable.length ? (focusable[0].getAttribute('href') || '') : '';
		})()`, &href),
	)
	if err != nil {
		t.Fatalf("skip link check: %v", err)
	}
	if !strings.Contains(href, "#main-content") && !strings.Contains(href, "#navigation") {
		t.Errorf("first focusable element is not a skip link, href=%q", href)
	}
}

func TestBrowserDocsPagesLoad(t *testing.T) {
	for _, path := range []string{"/server/docs/swagger", "/server/docs/graphql"} {
		tb := newTab(t)
		var title string
		err := chromedp.Run(tb.ctx,
			chromedp.Navigate(s.browserBase+path),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
			chromedp.Title(&title),
		)
		if err != nil {
			t.Errorf("docs page %s: %v", path, err)
			continue
		}
		if strings.TrimSpace(title) == "" {
			t.Errorf("%s has an empty title", path)
		}
		assertNoConsoleErrors(t, tb, path)
	}
}

func TestBrowserStaticAssetsAllLoad(t *testing.T) {
	tb := newTab(t)
	var failures []string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(time.Second),
		chromedp.Evaluate(`(function () {
			return performance.getEntriesByType('resource')
				.filter(e => e.responseStatus && e.responseStatus >= 400)
				.map(e => e.name + ' -> ' + e.responseStatus);
		})()`, &failures),
	)
	if err != nil {
		t.Fatalf("asset check: %v", err)
	}
	for _, failure := range failures {
		t.Errorf("asset failed to load: %s", failure)
	}
}
