//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

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
		chromedp.WaitVisible(`form input[name="q"]`, chromedp.ByQuery),
		chromedp.SendKeys(`form input[name="q"]`, "browser probe", chromedp.ByQuery),
		chromedp.Submit(`form input[name="q"]`, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(time.Second),
		chromedp.Location(&location),
	)
	if err != nil {
		t.Fatalf("browser search: %v", err)
	}
	if !strings.Contains(location, "/search") {
		t.Errorf("search did not land on /search, got %s", location)
	}
	assertNoConsoleErrors(t, tb, "/search")
}

func TestBrowserBangAutocomplete(t *testing.T) {
	tb := newTab(t)
	var status int64
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/bangs"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`fetch('/bangs/autocomplete?q=y').then(r => r.status)`, &status,
			func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }),
	)
	if err != nil {
		t.Fatalf("bang autocomplete: %v", err)
	}
	if status != 200 {
		t.Errorf("/bangs/autocomplete returned %d", status)
	}
	assertNoConsoleErrors(t, tb, "/bangs")
}

func TestBrowserPreferencesEnhancedFlow(t *testing.T) {
	tb := newTab(t)
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/preferences"),
		chromedp.WaitVisible("form", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("preferences page: %v", err)
	}
	assertNoConsoleErrors(t, tb, "/preferences")
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
	var dark, light string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/?theme=dark"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`getComputedStyle(document.body).backgroundColor`, &dark),
		chromedp.Navigate(s.browserBase+"/?theme=light"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`getComputedStyle(document.body).backgroundColor`, &light),
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
	for _, path := range []string{"/", "/search?q=responsive", "/engines", "/preferences", "/server/about"} {
		var overflow bool
		err := chromedp.Run(tb.ctx,
			chromedp.EmulateViewport(375, 812),
			chromedp.Navigate(s.browserBase+path),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
			chromedp.Evaluate(`document.documentElement.scrollWidth > document.documentElement.clientWidth + 1`, &overflow),
		)
		if err != nil {
			t.Errorf("responsive check %s: %v", path, err)
			continue
		}
		if overflow {
			t.Errorf("%s scrolls horizontally at 375x812", path)
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
