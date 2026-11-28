//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
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

func TestServerPageAccessibilityMarkup(t *testing.T) {
	client := httpClient(t)
	paths := []string{
		"/", "/search?q=accessibility", "/favorites", "/age-verify",
		"/content-restricted", "/server/about", "/server/privacy",
		"/server/contact", "/server/help", "/server/help/mcp",
		"/server/preferences", "/server/preferences/tokens", "/server/terms",
		"/server/dpo", "/server/security", "/server/security/policy",
		"/server/docs/swagger", "/server/docs/graphql", "/server/healthz",
	}
	for _, pagePath := range paths {
		t.Run(pagePath, func(t *testing.T) {
			pageClient := client
			if pagePath == "/age-verify" {
				pageClient = &http.Client{
					Timeout: 30 * time.Second,
					CheckRedirect: func(*http.Request, []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}
			}
			resp, body := get(t, pageClient, pagePath, map[string]string{"Accept": "text/html", "User-Agent": "Mozilla/5.0"})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s: status=%d body=%s", pagePath, resp.StatusCode, body)
			}
			doc, err := html.Parse(strings.NewReader(body))
			if err != nil {
				t.Fatalf("parse %s: %v", pagePath, err)
			}
			labelTargets := make(map[string]bool)
			var controls []map[string]string
			var controlNodes []*html.Node
			var headings []int
			pageTitleCount := 0
			walkHTML(doc, func(node *html.Node) {
				if node.Type != html.ElementNode {
					return
				}
				attrs := htmlAttrs(node)
				if node.Data == "img" && !hasHTMLAttr(node, "alt") {
					t.Errorf("%s has an image without alt text", pagePath)
				}
				if node.Data == "label" && attrs["for"] != "" {
					labelTargets[attrs["for"]] = true
				}
				if node.Data == "input" || node.Data == "select" || node.Data == "textarea" {
					typeValue := strings.ToLower(attrs["type"])
					if node.Data == "input" && (typeValue == "hidden" || typeValue == "submit" || typeValue == "button" || typeValue == "reset") {
						return
					}
					controls = append(controls, attrs)
					controlNodes = append(controlNodes, node)
				}
				if len(node.Data) == 2 && node.Data[0] == 'h' && node.Data[1] >= '1' && node.Data[1] <= '6' {
					headings = append(headings, int(node.Data[1]-'0'))
					if node.Data == "h1" && strings.Contains(" "+attrs["class"]+" ", " page-title ") {
						pageTitleCount++
					}
				}
			})
			if len(headings) == 0 || headings[0] != 1 {
				t.Errorf("%s must begin its heading hierarchy with h1, got %v", pagePath, headings)
			}
			h1Count := 0
			for _, level := range headings {
				if level == 1 {
					h1Count++
				}
			}
			if h1Count != 1 {
				t.Errorf("%s must contain exactly one h1, got %d", pagePath, h1Count)
			}
			if pageTitleCount != 1 {
				t.Errorf("%s must contain exactly one h1.page-title, got %d", pagePath, pageTitleCount)
			}
			for i := 1; i < len(headings); i++ {
				if headings[i] > headings[i-1]+1 {
					t.Errorf("%s skips from h%d to h%d", pagePath, headings[i-1], headings[i])
				}
			}
			for i, control := range controls {
				labeledByAncestor := false
				for ancestor := controlNodes[i].Parent; ancestor != nil; ancestor = ancestor.Parent {
					if ancestor.Type == html.ElementNode && ancestor.Data == "label" {
						labeledByAncestor = true
						break
					}
				}
				if control["aria-label"] == "" && control["aria-labelledby"] == "" && !labeledByAncestor && (control["id"] == "" || !labelTargets[control["id"]]) {
					t.Errorf("%s has an unlabelled %s control (id=%q)", pagePath, control["name"], control["id"])
				}
			}
		})
	}
}

func walkHTML(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func htmlAttrs(node *html.Node) map[string]string {
	attrs := make(map[string]string, len(node.Attr))
	for _, attr := range node.Attr {
		attrs[attr.Key] = attr.Val
	}
	return attrs
}

func hasHTMLAttr(node *html.Node, name string) bool {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
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

func TestAPIEnginesList(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/api/v1/engines", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/engines: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") || !json.Valid([]byte(body)) {
		t.Fatalf("GET /api/v1/engines: expected valid JSON, content type %q", resp.Header.Get("Content-Type"))
	}
}

func TestAPIBangsList(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/api/v1/bangs", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/bangs: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") || !json.Valid([]byte(body)) {
		t.Fatalf("GET /api/v1/bangs: expected valid JSON, content type %q", resp.Header.Get("Content-Type"))
	}
}

func TestOwnerTokenFavoritesAPI(t *testing.T) {
	client := httpClient(t)
	_, _ = get(t, client, "/", nil)
	baseURL, err := url.Parse(s.localBase)
	if err != nil {
		t.Fatal(err)
	}
	var csrf string
	for _, cookie := range client.Jar.Cookies(baseURL) {
		if cookie.Name == "csrf_token" {
			csrf = cookie.Value
		}
	}
	if csrf == "" {
		t.Fatal("initial page did not set a CSRF token")
	}
	request, err := http.NewRequest(http.MethodPost, s.localBase+"/api/v1/favorites", bytes.NewBufferString(`{"url":"https://example.test/e2e","title":"Owner-scoped favorite"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-CSRF-Token", csrf)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		_ = response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	var created struct {
		Data struct {
			OwnerToken string `json:"owner_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || created.Data.OwnerToken == "" {
		t.Fatalf("anonymous favorite creation = %d, owner token present=%t, body=%s", response.StatusCode, created.Data.OwnerToken != "", responseBody)
	}
	client.Jar.SetCookies(baseURL, append(client.Jar.Cookies(baseURL), &http.Cookie{Name: "vidveil_owner_token_ab5213f6", Value: created.Data.OwnerToken, Path: "/"}))

	ownedList, body := get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + created.Data.OwnerToken, "Accept": "application/json"})
	if ownedList.StatusCode != http.StatusOK || !strings.Contains(body, "Owner-scoped favorite") {
		t.Fatalf("owner bearer list = %d %s", ownedList.StatusCode, body)
	}
	mcpCall := func(rawToken string) (int, string) {
		t.Helper()
		payload := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"listFavorites","arguments":{},"_meta":{"io.modelcontextprotocol/clientCapabilities":{"sampling":{}},"io.modelcontextprotocol/protocolVersion":"2026-07-28"}},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`
		request, err := http.NewRequest(http.MethodPost, s.localBase+"/api/v1/server/mcp", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("MCP-Protocol-Version", "2026-07-28")
		request.Header.Set("Mcp-Method", "tools/call")
		request.Header.Set("Mcp-Name", "listFavorites")
		if rawToken != "" {
			request.Header.Set("Authorization", "Bearer "+rawToken)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		result, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, string(result)
	}
	if status, result := mcpCall(""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous MCP tools/call = %d %s, want 401", status, result)
	}
	if status, result := mcpCall(created.Data.OwnerToken); status != http.StatusOK || !strings.Contains(result, "Owner-scoped favorite") {
		t.Fatalf("owner-token MCP tools/call = %d %s", status, result)
	}
	unauthorized, body := get(t, client, "/api/v1/favorites", map[string]string{"Accept": "application/json"})
	if unauthorized.StatusCode != http.StatusUnauthorized || strings.Contains(body, "Owner-scoped favorite") {
		t.Fatalf("cookie-authenticated API list = %d %s", unauthorized.StatusCode, body)
	}
	callJSON := func(method, path, rawToken, payload string) (int, []byte) {
		t.Helper()
		request, err := http.NewRequest(method, s.localBase+path, strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Authorization", "Bearer "+rawToken)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body
	}
	status, apiBody := callJSON(http.MethodPost, "/api/v1/server/preferences/tokens", created.Data.OwnerToken, `{"name":"e2e automation","expires":"30d"}`)
	var linked struct {
		Data struct {
			ID    string `json:"id"`
			Token string `json:"linked_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(apiBody, &linked); err != nil || status != http.StatusCreated || linked.Data.Token == "" || linked.Data.ID == "" {
		t.Fatalf("linked-token creation = %d %s", status, apiBody)
	}
	status, apiBody = callJSON(http.MethodPatch, "/api/v1/server/preferences/tokens/"+linked.Data.ID, created.Data.OwnerToken, `{"name":"renamed e2e client","expires":"30d"}`)
	if status != http.StatusOK || !strings.Contains(string(apiBody), "renamed e2e client") {
		t.Fatalf("linked-token rename and expiry update = %d %s", status, apiBody)
	}
	linkedList, body := get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + linked.Data.Token, "Accept": "application/json"})
	if linkedList.StatusCode != http.StatusOK || !strings.Contains(body, "Owner-scoped favorite") {
		t.Fatalf("linked-token resource access = %d %s", linkedList.StatusCode, body)
	}
	if status, result := mcpCall(linked.Data.Token); status != http.StatusOK || !strings.Contains(result, "Owner-scoped favorite") {
		t.Fatalf("linked-token MCP tools/call = %d %s", status, result)
	}
	status, apiBody = callJSON(http.MethodGet, "/api/v1/server/preferences/tokens", linked.Data.Token, "")
	if status != http.StatusForbidden {
		t.Fatalf("linked-token management status = %d, want 403: %s", status, apiBody)
	}
	status, apiBody = callJSON(http.MethodPost, "/api/v1/server/preferences/tokens/"+linked.Data.ID+"/rotate", created.Data.OwnerToken, "")
	var linkedRotated struct {
		Data struct {
			Token string `json:"linked_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(apiBody, &linkedRotated); err != nil || status != http.StatusOK || linkedRotated.Data.Token == "" {
		t.Fatalf("linked-token rotation = %d %s", status, apiBody)
	}
	if response, body := get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + linked.Data.Token, "Accept": "application/json"}); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old linked token after rotation = %d %s", response.StatusCode, body)
	}
	if response, body := get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + linkedRotated.Data.Token, "Accept": "application/json"}); response.StatusCode != http.StatusOK || !strings.Contains(body, "Owner-scoped favorite") {
		t.Fatalf("rotated linked token access = %d %s", response.StatusCode, body)
	}
	status, apiBody = callJSON(http.MethodPost, "/api/v1/server/preferences/tokens/primary/rotate", created.Data.OwnerToken, "")
	var rotated struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(apiBody, &rotated); err != nil || status != http.StatusOK || rotated.Data.Token == "" {
		t.Fatalf("primary-token rotation = %d %s", status, apiBody)
	}
	status, apiBody = callJSON(http.MethodGet, "/api/v1/server/preferences/tokens", created.Data.OwnerToken, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("old primary token status = %d, want 401: %s", status, apiBody)
	}
	ownedList, body = get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + rotated.Data.Token, "Accept": "application/json"})
	if ownedList.StatusCode != http.StatusOK || !strings.Contains(body, "Owner-scoped favorite") {
		t.Fatalf("rotated primary resource access = %d %s", ownedList.StatusCode, body)
	}
	if linkedList, body = get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + linkedRotated.Data.Token, "Accept": "application/json"}); linkedList.StatusCode != http.StatusOK || !strings.Contains(body, "Owner-scoped favorite") {
		t.Fatalf("linked token after primary rotation = %d %s", linkedList.StatusCode, body)
	}
	expiredDate := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	status, apiBody = callJSON(http.MethodPatch, "/api/v1/server/preferences/tokens/"+linked.Data.ID, rotated.Data.Token, `{"expires_on":"`+expiredDate+`"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("past linked-token expiry = %d, want 400: %s", status, apiBody)
	}
	if response, body := get(t, client, "/api/v1/favorites", map[string]string{"Authorization": "Bearer " + linkedRotated.Data.Token, "Accept": "application/json"}); response.StatusCode != http.StatusOK {
		t.Fatalf("linked token after rejected expiry change = %d %s", response.StatusCode, body)
	}
	status, apiBody = callJSON(http.MethodDelete, "/api/v1/server/preferences/tokens/"+linked.Data.ID, rotated.Data.Token, "")
	if status != http.StatusOK {
		t.Fatalf("linked-token revoke = %d %s", status, apiBody)
	}
}

func TestAPIBangAutocomplete(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/api/v1/bangs/autocomplete?q=y", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/bangs/autocomplete: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") || !json.Valid([]byte(body)) {
		t.Fatalf("GET /api/v1/bangs/autocomplete: expected valid JSON, content type %q", resp.Header.Get("Content-Type"))
	}
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
	resp, body := get(t, client, "/server/preferences", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /server/preferences: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-preferences", body,
		"<form",
		"vidveil_pref_results_per_page",
	)
}

func TestSSRStandardServerPages(t *testing.T) {
	client := httpClient(t)
	for _, path := range []string{
		"/server/about",
		"/server/help",
		"/server/privacy",
		"/server/dpo",
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

func TestSSRMCPHelpAndOwnerTokenPages(t *testing.T) {
	client := httpClient(t)
	resp, body := get(t, client, "/server/help/mcp", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /server/help/mcp: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-mcp-help", body, `id="overview"`, `id="endpoint"`, `id="authentication"`, `id="connect"`, `id="tools"`, `id="limits"`, `id="troubleshooting"`, `id="project"`, "searchVideos")

	resp, body = get(t, client, "/api/v1/server/help/mcp", map[string]string{"Accept": "application/json"})
	if resp.StatusCode != http.StatusOK || !json.Valid([]byte(body)) || !strings.Contains(body, `"scheme": "Bearer"`) {
		t.Fatalf("GET /api/v1/server/help/mcp: status=%d body=%s", resp.StatusCode, body)
	}

	resp, body = get(t, client, "/server/preferences/tokens", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /server/preferences/tokens: want 200, got %d", resp.StatusCode)
	}
	mustContain(t, "ssr-owner-token-restore", body, `action="/server/preferences/token/restore"`, `name="token"`)
	if strings.Contains(body, `action="/server/preferences/token/reveal"`) {
		t.Error("owner-token page showed management actions without a primary cookie")
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
	resp, body := get(t, client, "/server/preferences", nil)
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
	resp, err := client.PostForm(s.localBase+"/server/preferences", nil)
	if err != nil {
		t.Fatalf("POST without CSRF token: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST without CSRF token: want 403, got %d", resp.StatusCode)
	}
}
