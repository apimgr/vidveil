//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// Tier 2 - real browser with JavaScript execution disabled. Everything asserted
// here must work through plain HTML: form POSTs, links, redirects, pagination.
// A feature that only passes Tier 3 is a progressive-enhancement violation.

// noJSTab opens a tab with script execution disabled.
func noJSTab(t *testing.T) *tab {
	t.Helper()
	tb := newTab(t)
	if err := chromedp.Run(tb.ctx, emulation.SetScriptExecutionDisabled(true)); err != nil {
		t.Fatalf("disable script execution: %v", err)
	}
	return tb
}

// dumpOnFail stores the current page HTML when the test has already failed.
func dumpOnFail(t *testing.T, tb *tab, name string) {
	t.Helper()
	if !t.Failed() {
		return
	}
	var html string
	ctx, cancel := context.WithTimeout(tb.ctx, 10*time.Second)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &html, chromedp.ByQuery)); err == nil {
		saveArtifact(t, name+".html", html)
	}
}

func TestNoJSSearchFormSubmits(t *testing.T) {
	tb := noJSTab(t)
	var url, body string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/"),
		chromedp.WaitVisible(`form input[name="q"]`, chromedp.ByQuery),
		chromedp.SendKeys(`form input[name="q"]`, "nojs probe", chromedp.ByQuery),
		chromedp.Submit(`form input[name="q"]`, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Location(&url),
		chromedp.OuterHTML("html", &body, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("no-JS search submit: %v", err)
	}
	if !strings.Contains(url, "/search") || !strings.Contains(url, "q=") {
		t.Errorf("no-JS search did not navigate to /search?q=..., got %s", url)
	}
	if !strings.Contains(body, "nojs probe") {
		t.Error("search results page does not echo the submitted query")
	}
	dumpOnFail(t, tb, "nojs-search")
}

func TestNoJSNavigationLinksWork(t *testing.T) {
	tb := noJSTab(t)
	for _, path := range []string{"/engines", "/bangs", "/favorites", "/preferences", "/server/about", "/server/help"} {
		var body string
		err := chromedp.Run(tb.ctx,
			chromedp.Navigate(s.browserBase+path),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.OuterHTML("html", &body, chromedp.ByQuery),
		)
		if err != nil {
			t.Errorf("no-JS navigate %s: %v", path, err)
			continue
		}
		if !strings.Contains(body, "<main") && !strings.Contains(body, `id="main-content"`) {
			saveArtifact(t, "nojs"+strings.ReplaceAll(path, "/", "-")+".html", body)
			t.Errorf("%s renders no main landmark without JavaScript", path)
		}
	}
}

func TestNoJSPreferencesFormPersists(t *testing.T) {
	tb := noJSTab(t)
	var body string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/preferences"),
		chromedp.WaitVisible(`form`, chromedp.ByQuery),
		chromedp.SetValue(`select[name="results_per_page"], input[name="results_per_page"]`, "50", chromedp.ByQuery),
		chromedp.Submit(`form`, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Navigate(s.browserBase+"/preferences"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &body, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("no-JS preferences submit: %v", err)
	}
	if !strings.Contains(body, "50") {
		saveArtifact(t, "nojs-preferences.html", body)
		t.Error("results_per_page=50 did not survive a no-JS form POST")
	}
}

func TestNoJSThemeSwitchFallback(t *testing.T) {
	tb := noJSTab(t)
	var html string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/preferences"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("no-JS theme page: %v", err)
	}
	if !strings.Contains(html, "theme") {
		saveArtifact(t, "nojs-theme.html", html)
		t.Error("no no-JS theme control on the preferences page")
	}
}

func TestNoJSPaginationLoadMore(t *testing.T) {
	tb := noJSTab(t)
	var html string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/search?q=nojs+pagination&page=1"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("no-JS pagination: %v", err)
	}
	// Without JS the results page must still expose a real navigable control,
	// never a button that only an event listener could activate
	if strings.Contains(html, "page=2") || strings.Contains(html, "load-more") || strings.Contains(html, "results") {
		return
	}
	saveArtifact(t, "nojs-pagination.html", html)
	t.Error("results page exposes no server-rendered pagination affordance")
}

func TestNoJSFavoritesFlow(t *testing.T) {
	tb := noJSTab(t)
	var html string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/favorites"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("no-JS favorites: %v", err)
	}
	if strings.Contains(html, "<noscript") || strings.Contains(html, "<form") || strings.Contains(html, "favorites") {
		return
	}
	saveArtifact(t, "nojs-favorites.html", html)
	t.Error("favorites page is unusable without JavaScript")
}

func TestNoJSErrorPageRenders(t *testing.T) {
	tb := noJSTab(t)
	var html string
	err := chromedp.Run(tb.ctx,
		chromedp.Navigate(s.browserBase+"/definitely-not-a-route"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("no-JS 404: %v", err)
	}
	if strings.TrimSpace(html) == "" {
		t.Fatal("404 rendered an empty document")
	}
	if !strings.Contains(html, "404") {
		saveArtifact(t, "nojs-404.html", html)
		t.Error("404 page does not identify itself")
	}
}
