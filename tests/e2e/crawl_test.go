//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Universal coverage: full internal-link crawl and static-asset sweep
// (AI.md PART 28 -> "Universal Coverage (Every Project)").

// hrefPattern extracts href/src targets from server-rendered HTML.
var hrefPattern = regexp.MustCompile(`(?i)(?:href|src)\s*=\s*"([^"]+)"`)

// crawlSeeds are the entry points the crawler expands from. Everything else is
// discovered by following links in the server-rendered markup.
var crawlSeeds = []string{
	"/",
	"/search?q=crawl+probe",
	"/engines",
	"/bangs",
	"/favorites",
	"/preferences",
	"/server/about",
	"/server/help",
}

// skipCrawl reports whether a discovered link must not be followed: external
// origins, non-HTTP schemes, in-page anchors, and endpoints that stream, proxy
// remote content, or intentionally mutate state.
func skipCrawl(link string) bool {
	switch {
	case link == "", strings.HasPrefix(link, "#"):
		return true
	case strings.HasPrefix(link, "//"), strings.HasPrefix(link, "http://"), strings.HasPrefix(link, "https://"):
		return true
	case strings.HasPrefix(link, "mailto:"), strings.HasPrefix(link, "tel:"),
		strings.HasPrefix(link, "data:"), strings.HasPrefix(link, "javascript:"):
		return true
	case !strings.HasPrefix(link, "/"):
		return true
	}
	for _, prefix := range []string{
		"/proxy/",
		"/search/batch",
		"/search.rss",
		"/search.atom",
		"/favorites/export",
		"/favorites/import",
		"/server/preferences/export",
		"/server/preferences/import",
		"/announcements/dismiss",
		"/server/metrics",
		"/metrics",
	} {
		if strings.HasPrefix(link, prefix) {
			return true
		}
	}
	return false
}

// normalizeLink strips the fragment so the same page is not crawled twice.
func normalizeLink(link string) string {
	if idx := strings.Index(link, "#"); idx >= 0 {
		link = link[:idx]
	}
	return link
}

func TestCrawlNoDeadLinks(t *testing.T) {
	client := httpClient(t)

	const maxPages = 200
	seen := map[string]bool{}
	queue := append([]string(nil), crawlSeeds...)
	failures := map[string]string{}
	visited := 0

	for len(queue) > 0 && visited < maxPages {
		current := normalizeLink(queue[0])
		queue = queue[1:]
		if current == "" || seen[current] {
			continue
		}
		seen[current] = true
		visited++

		resp, body := get(t, client, current, nil)
		switch {
		case resp.StatusCode >= 500:
			failures[current] = fmt.Sprintf("server error %d", resp.StatusCode)
			saveArtifact(t, "crawl"+strings.NewReplacer("/", "-", "?", "-", "&", "-", "=", "-").Replace(current)+".html", body)
			continue
		case resp.StatusCode == http.StatusNotFound:
			failures[current] = "dead link (404)"
			continue
		case resp.StatusCode >= 400:
			failures[current] = fmt.Sprintf("unexpected status %d", resp.StatusCode)
			continue
		}

		if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			continue
		}
		for _, match := range hrefPattern.FindAllStringSubmatch(body, -1) {
			link := normalizeLink(strings.TrimSpace(match[1]))
			if skipCrawl(link) || seen[link] {
				continue
			}
			queue = append(queue, link)
		}
	}

	if visited >= maxPages {
		t.Logf("crawl stopped at the %d page ceiling with %d links still queued", maxPages, len(queue))
	}
	if len(failures) > 0 {
		paths := make([]string, 0, len(failures))
		for path := range failures {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			t.Errorf("crawl: %s -> %s", path, failures[path])
		}
	}
	if visited < len(crawlSeeds) {
		t.Errorf("crawl visited only %d pages, fewer than the %d seeds", visited, len(crawlSeeds))
	}
}

func TestCrawlStaticAssetsRespond200(t *testing.T) {
	client := httpClient(t)

	assets := map[string]bool{}
	for _, page := range crawlSeeds {
		_, body := get(t, client, page, nil)
		for _, match := range hrefPattern.FindAllStringSubmatch(body, -1) {
			link := normalizeLink(strings.TrimSpace(match[1]))
			if !strings.HasPrefix(link, "/static/") &&
				link != "/favicon.ico" &&
				link != "/apple-touch-icon.png" &&
				link != "/manifest.json" &&
				link != "/sw.js" {
				continue
			}
			assets[link] = true
		}
	}

	if len(assets) == 0 {
		t.Fatal("no static assets referenced by any crawled page")
	}

	paths := make([]string, 0, len(assets))
	for path := range assets {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		resp, _ := get(t, client, path, map[string]string{"Accept": "*/*"})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("static asset %s returned %d", path, resp.StatusCode)
		}
	}
}

func TestCrawlRedirectsCarryLocation(t *testing.T) {
	client := httpClient(t)

	// /server redirects to /server/about (PART 16 standard pages)
	resp, _ := get(t, client, "/server", nil)
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		t.Fatalf("GET /server: want a 3xx redirect, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); !strings.Contains(location, "/server/about") {
		t.Errorf("GET /server: want Location /server/about, got %q", location)
	}
}
