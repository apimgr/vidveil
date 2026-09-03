//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// Tier 1 - Server-side rendering. Plain net/http, no browser at all.
// Every assertion here proves the initial HTML already carries real content,
// never an empty shell that JavaScript fills in later (AI.md PART 28).

// mustContain fails the test and stores the page for inspection when the
// expected marker is missing from the server-rendered HTML.
func mustContain(t *testing.T, name, body string, markers ...string) {
	t.Helper()
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			saveArtifact(t, name+".html", body)
			t.Errorf("%s: server-rendered HTML is missing %q", name, marker)
		}
	}
}

func TestSSRHomePage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-home", body,
		"<form",
		`name="q"`,
		"<title>",
		"</html>",
	)
	if !strings.Contains(body, `id="main-content"`) {
		t.Error("home page has no #main-content skip-link target")
	}
}

func TestSSRSearchPage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/search?q=e2e+probe", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /search: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-search", body,
		"<form",
		`name="q"`,
		"e2e probe",
	)
}

func TestSSREnginesPage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/engines", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /engines: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-engines", body, "<table", "</html>")
}

func TestSSRBangsPage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/bangs", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /bangs: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-bangs", body, "!", "</html>")
}

func TestSSRFavoritesPage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/favorites", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /favorites: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-favorites", body, "</html>")
}

func TestSSRPreferencesPage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/preferences", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /preferences: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-preferences", body,
		"<form",
		"results_per_page",
	)
}

func TestSSRStandardServerPages(t *testing.T) {
	client := httpClient(t)
	for _, path := range []string{
		"/server/about",
		"/server/help",
		"/server/privacy",
		"/server/terms",
		"/server/contact",
		"/server/healthz",
	} {
		resp, body := get(t, client, path, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", path, resp.StatusCode)
			continue
		}
		mustContain(t, "ssr"+strings.ReplaceAll(path, "/", "-"), body, "<title>", "</html>")
		for _, placeholder := range []string{"Lorem ipsum", "TODO", "PLACEHOLDER"} {
			if strings.Contains(body, placeholder) {
				t.Errorf("%s contains placeholder copy %q", path, placeholder)
			}
		}
	}
}

func TestSSRDocsPagesMatchRealRoutes(t *testing.T) {
	client := httpClient(t)
	for _, path := range []string{"/server/docs/swagger", "/server/docs/graphql"} {
		resp, body := get(t, client, path, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", path, resp.StatusCode)
			continue
		}
		mustContain(t, "ssr"+strings.ReplaceAll(path, "/", "-"), body, "</html>")
	}

	resp, _ := get(t, client, "/api/swagger", map[string]string{"Accept": "application/json"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/swagger: want 200, got %d", resp.StatusCode)
	}
}

func TestSSRHealthzContentNegotiation(t *testing.T) {
	client := httpClient(t)

	resp, body := get(t, client, "/api/healthz", map[string]string{"Accept": "application/json"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/healthz: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, `"status"`) {
		saveArtifact(t, "healthz.json", body)
		t.Error("healthz JSON has no status field")
	}
	if strings.Contains(body, `"ok"`) && strings.Contains(body, `"data"`) {
		t.Error("healthz must return a bare object, never the ok/data envelope")
	}

	resp, body = get(t, client, "/server/healthz.txt", map[string]string{"Accept": "text/plain"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /server/healthz.txt: want 200, got %d", resp.StatusCode)
	}
	if strings.Contains(body, "<html") {
		t.Error("healthz.txt returned HTML")
	}
}

func TestSSRSEOMetaTags(t *testing.T) {
	client := httpClient(t)
	_, body := get(t, client, "/", nil)
	for _, marker := range []string{
		`name="description"`,
		`name="viewport"`,
		`property="og:title"`,
		`rel="canonical"`,
	} {
		if !strings.Contains(body, marker) {
			saveArtifact(t, "seo-home.html", body)
			t.Errorf("home page missing SEO tag %s", marker)
		}
	}
}

func TestSSRWellKnownAndRootFiles(t *testing.T) {
	client := httpClient(t)
	for _, path := range []string{
		"/robots.txt",
		"/sitemap.xml",
		"/manifest.json",
		"/humans.txt",
		"/offline.html",
		"/.well-known/security.txt",
	} {
		resp, _ := get(t, client, path, map[string]string{"Accept": "*/*"})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", path, resp.StatusCode)
		}
	}
}

func TestSSRThemedErrorPage(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/this-route-does-not-exist", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown route: want 404, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-404", body, "<title>", "</html>")
	if strings.Contains(body, "goroutine ") || strings.Contains(body, "/src/server/") {
		t.Error("404 page leaks internal detail")
	}
}

func TestSSRI18NLanguageSwitching(t *testing.T) {
	client := httpClient(t)
	_, english := get(t, client, "/server/about?lang=en", nil)
	_, spanish := get(t, client, "/server/about?lang=es", nil)
	if english == spanish {
		saveArtifact(t, "i18n-en.html", english)
		saveArtifact(t, "i18n-es.html", spanish)
		t.Error("?lang=es rendered byte-identical output to ?lang=en")
	}

	// An unsupported language must silently fall back to English, never error
	resp, _ := get(t, client, "/server/about?lang=zz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unsupported ?lang: want 200 fallback, got %d", resp.StatusCode)
	}
}

func TestSSRSecurityHeaders(t *testing.T) {
	client := httpClient(t)
	resp, _ := get(t, client, "/", nil)
	for _, header := range []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Referrer-Policy",
		"Content-Security-Policy",
		"Permissions-Policy",
		"X-Request-ID",
	} {
		if resp.Header.Get(header) == "" {
			t.Errorf("response is missing required header %s", header)
		}
	}
}

func TestSSRCSRFTokenIssuedOnForms(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/preferences", nil)
	if !strings.Contains(body, "csrf_token") {
		saveArtifact(t, "csrf-preferences.html", body)
		t.Error("preferences form has no csrf_token field")
	}
	var found bool
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "csrf_token" {
			found = true
			if cookie.HttpOnly {
				t.Error("csrf_token cookie must not be HttpOnly - JS has to read it")
			}
			if cookie.SameSite != http.SameSiteStrictMode {
				t.Error("csrf_token cookie must be SameSite=Strict")
			}
		}
	}
	if !found {
		t.Error("no csrf_token cookie issued")
	}
}

func TestSSRCSRFRejectsMissingToken(t *testing.T) {
	client := httpClient(t)
	resp, err := client.PostForm(s.localBase+"/server/preferences/save", nil)
	if err != nil {
		t.Fatalf("POST without CSRF token: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST without CSRF token: want 403, got %d", resp.StatusCode)
	}
}
