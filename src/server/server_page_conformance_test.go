// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"
)

func TestServerPageConformance(t *testing.T) {
	s := newTestServer(t)
	paths := []string{
		"/server/about", "/server/privacy", "/server/contact", "/server/dpo",
		"/server/help", "/server/terms", "/server/security",
		"/server/security/policy", "/server/security/thanks",
		"/server/preferences", "/server/docs/swagger", "/server/docs/graphql",
		"/server/healthz",
	}
	paths = append(paths, handler.RegisteredHubPagePaths(s.appConfig)...)
	paths = uniquePagePaths(paths)
	for _, path := range paths {
		t.Run(strings.TrimPrefix(path, "/"), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Accept", "text/html")
			request.Header.Set("User-Agent", "Mozilla/5.0")
			request.AddCookie(&http.Cookie{Name: "age_verified", Value: "1"})
			s.router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s returned %d: %s", path, recorder.Code, recorder.Body.String())
			}
			checkServerPageDocument(t, s, path, recorder.Body.String())
		})
	}

	t.Run("branded-404", func(t *testing.T) {
		const path = "/server/__page_conformance_missing__"
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Accept", "text/html")
		request.Header.Set("User-Agent", "Mozilla/5.0")
		request.AddCookie(&http.Cookie{Name: "age_verified", Value: "1"})
		s.router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want 404", path, recorder.Code)
		}
		checkServerPageDocument(t, s, path, recorder.Body.String())
	})

	t.Run("operator-header-fragment", func(t *testing.T) {
		defaultHeader := renderConformancePage(t, s, "/server/about")
		custom := newTestServerWithCfg(t, func(cfg *config.AppConfig) {
			cfg.Server.Branding.Content.Header.Format = "html"
			cfg.Server.Branding.Content.Header.Body.Text = "<p>Conformance header fragment</p>"
		})
		customHeader := renderConformancePage(t, custom, "/server/about")
		defaultChildren := elementChildren(defaultHeader)
		customChildren := elementChildren(customHeader)
		if len(defaultChildren) != 4 || len(customChildren) != 5 {
			t.Fatalf("header element counts: default=%d custom=%d, want 4 and 5", len(defaultChildren), len(customChildren))
		}
		for i := range defaultChildren {
			if defaultChildren[i].Data != customChildren[i].Data || attr(defaultChildren[i], "class") != attr(customChildren[i], "class") {
				t.Errorf("operator header fragment changed header element %d", i)
			}
		}
		last := customChildren[len(customChildren)-1]
		if last.Data != "p" || strings.TrimSpace(textContent(last)) != "Conformance header fragment" {
			t.Errorf("operator fragment is not the last header element: <%s>%s", last.Data, textContent(last))
		}
	})
}

func TestInteractiveAPIDocumentationAssetsAreSelfHosted(t *testing.T) {
	s := newTestServer(t)
	assets := map[string]string{
		"/static/vendor/swagger-ui/swagger-ui.css":                  "text/css",
		"/static/vendor/swagger-ui/swagger-ui-bundle.js":            "javascript",
		"/static/vendor/swagger-ui/swagger-ui-standalone-preset.js": "javascript",
		"/static/vendor/graphiql/graphiql.min.css":                  "text/css",
		"/static/vendor/graphiql/graphiql.min.js":                   "javascript",
		"/static/vendor/graphiql/react.production.min.js":           "javascript",
		"/static/vendor/graphiql/react-dom.production.min.js":       "javascript",
		"/static/js/docs-swagger.js":                                "javascript",
		"/static/js/docs-graphql.js":                                "javascript",
	}
	for path, contentType := range assets {
		t.Run(strings.TrimPrefix(path, "/static/"), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			s.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s returned %d", path, recorder.Code)
			}
			if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, contentType) {
				t.Errorf("GET %s Content-Type = %q, want to contain %q", path, got, contentType)
			}
			if recorder.Body.Len() == 0 {
				t.Errorf("GET %s returned an empty asset", path)
			}
		})
	}
}

func renderConformancePage(t *testing.T, s *Server, path string) *html.Node {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Accept", "text/html")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.AddCookie(&http.Cookie{Name: "age_verified", Value: "1"})
	s.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d: %s", path, recorder.Code, recorder.Body.String())
	}
	root, err := html.Parse(strings.NewReader(recorder.Body.String()))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return findNodes(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "header" && hasClass(node, "site-header")
	})[0]
}

func elementChildren(node *html.Node) []*html.Node {
	children := make([]*html.Node, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			children = append(children, child)
		}
	}
	return children
}

func uniquePagePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func checkServerPageDocument(t *testing.T, s *Server, pagePath, document string) {
	t.Helper()
	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	mainCount := len(findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "main" }))
	if mainCount != 1 {
		t.Errorf("%s has %d main elements, want 1", pagePath, mainCount)
	}
	titles := findNodes(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "h1" && hasClass(node, "page-title")
	})
	if len(titles) != 1 {
		t.Errorf("%s has %d h1.page-title elements, want 1", pagePath, len(titles))
	}
	containers := findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && hasClass(node, "container") })
	if len(containers) == 0 {
		t.Errorf("%s has no shared container", pagePath)
	}
	pageHeaders := findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && hasClass(node, "page-header") })
	if len(pageHeaders) != 1 {
		t.Errorf("%s has %d page headers, want 1", pagePath, len(pageHeaders))
	}
	if len(pageHeaders) == 1 && len(findDescendants(pageHeaders[0], func(node *html.Node) bool {
		return node.Type == html.ElementNode && hasClass(node, "page-lead")
	})) != 1 {
		t.Errorf("%s page header must contain one page lead", pagePath)
	}
	for _, container := range containers {
		widths := 0
		for _, width := range []string{"container-sm", "container-md", "container-lg"} {
			if hasClass(container, width) {
				widths++
			}
		}
		if widths != 1 {
			t.Errorf("%s container has %d width modifiers, want exactly 1", pagePath, widths)
		}
	}

	cards := findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && hasClass(node, "section-card") })
	if len(cards) == 0 {
		t.Errorf("%s has no section card", pagePath)
	}
	for _, card := range cards {
		labelledBy := attr(card, "aria-labelledby")
		if labelledBy == "" {
			t.Errorf("%s has an unlabelled section card", pagePath)
			continue
		}
		label := findNodes(card, func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "h2" && attr(node, "id") == labelledBy
		})
		if len(label) != 1 {
			t.Errorf("%s section card label %q does not identify exactly one h2", pagePath, labelledBy)
		}
	}
	for _, heading := range findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "h2" }) {
		if ancestorWithClass(heading, "section-card") == nil {
			t.Errorf("%s has an h2 outside a section card", pagePath)
		}
	}
	for _, card := range cards {
		if ancestorWithClass(card, "section-card") != nil {
			t.Errorf("%s nests section cards", pagePath)
		}
	}

	breadcrumbs := findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && hasClass(node, "breadcrumb") })
	if strings.HasPrefix(pagePath, "/server/preferences/") || strings.HasPrefix(pagePath, "/server/help/") || strings.HasPrefix(pagePath, "/server/security/") {
		if len(breadcrumbs) != 1 {
			t.Errorf("%s has %d breadcrumbs, want 1", pagePath, len(breadcrumbs))
		} else if len(findDescendants(breadcrumbs[0], func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "a" })) != 1 {
			t.Errorf("%s breadcrumb must contain exactly one link", pagePath)
		}
	} else if len(breadcrumbs) != 0 {
		t.Errorf("hub or top-level page %s has a breadcrumb", pagePath)
	}

	checkSkipLinks(t, pagePath, root)
	checkServerHeader(t, pagePath, root)
	checkProjectJavaScript(t, pagePath, root)
	checkNoInlineBehavior(t, pagePath, root)
	checkImagesAndForms(t, pagePath, root)
	checkTables(t, pagePath, root)
	checkInternalLinks(t, s, pagePath, root)
}

func checkProjectJavaScript(t *testing.T, pagePath string, root *html.Node) {
	t.Helper()
	appScripts := 0
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "script" {
			return
		}
		source := attr(node, "src")
		if strings.Contains(source, "/static/js/app.js") {
			appScripts++
		}
		if strings.Contains(source, "/static/js/") &&
			!strings.Contains(source, "/static/js/app.js") &&
			!strings.Contains(source, "/static/js/docs-swagger.js") &&
			!strings.Contains(source, "/static/js/docs-graphql.js") {
			t.Errorf("%s loads unapproved project JavaScript %q", pagePath, source)
		}
	})
	if appScripts != 1 {
		t.Errorf("%s loads app.js %d times, want exactly once", pagePath, appScripts)
	}
	if strings.Contains(pagePath, "/docs/swagger") && !strings.Contains(renderedScriptSources(root), "/static/js/docs-swagger.js") {
		t.Errorf("%s omits its documentation startup script", pagePath)
	}
	if strings.Contains(pagePath, "/docs/graphql") && !strings.Contains(renderedScriptSources(root), "/static/js/docs-graphql.js") {
		t.Errorf("%s omits its documentation startup script", pagePath)
	}
}

func renderedScriptSources(root *html.Node) string {
	var sources strings.Builder
	walkHTML(root, func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "script" {
			sources.WriteString(attr(node, "src"))
			sources.WriteByte('\n')
		}
	})
	return sources.String()
}

func checkSkipLinks(t *testing.T, pagePath string, root *html.Node) {
	t.Helper()
	focusable := make([]*html.Node, 0)
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || hasAttr(node, "hidden") || hasAttr(node, "disabled") {
			return
		}
		if (node.Data == "a" && attr(node, "href") != "") || node.Data == "button" || node.Data == "select" || node.Data == "textarea" || (node.Data == "input" && attr(node, "type") != "hidden") || (hasAttr(node, "tabindex") && attr(node, "tabindex") != "-1") {
			focusable = append(focusable, node)
		}
	})
	if len(focusable) < 2 || !hasClass(focusable[0], "skip-link") || !hasClass(focusable[1], "skip-link") || attr(focusable[0], "href") != "#main-content" || attr(focusable[1], "href") != "#navigation" {
		t.Errorf("%s does not put main and navigation skip links first", pagePath)
	}
}

func checkServerHeader(t *testing.T, pagePath string, root *html.Node) {
	t.Helper()
	headers := findNodes(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "header" && hasClass(node, "site-header")
	})
	if len(headers) != 1 {
		t.Errorf("%s has %d shared headers, want 1", pagePath, len(headers))
		return
	}
	children := elementChildren(headers[0])
	if len(children) != 4 || children[0].Data != "a" || !hasClass(children[0], "site-brand") || children[1].Data != "nav" || !hasClass(children[1], "nav-links") || children[2].Data != "label" || !hasClass(children[2], "nav-toggle") || children[3].Data != "div" || !hasClass(children[3], "header-actions") {
		t.Errorf("%s shared header elements are not in the required order", pagePath)
	}
	if len(children) > 0 && (attr(children[0], "href") != "/" || strings.TrimSpace(textContent(children[0])) == "") {
		t.Errorf("%s brand must be a named link to /", pagePath)
	}
	if len(children) > 3 {
		actions := make([]*html.Node, 0)
		for child := children[3].FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode {
				actions = append(actions, child)
			}
		}
		if len(actions) != 2 || actions[0].Data != "a" || attr(actions[0], "href") != "/server/preferences" || actions[1].Data != "form" || attr(actions[1], "method") != "post" {
			t.Errorf("%s header actions must contain preferences and the theme form", pagePath)
		}
	}
}

func checkNoInlineBehavior(t *testing.T, pagePath string, root *html.Node) {
	t.Helper()
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		if node.Data == "style" {
			t.Errorf("%s contains a style element", pagePath)
		}
		if node.Data == "script" && isExecutableScriptType(attr(node, "type")) && textContent(node) != "" {
			t.Errorf("%s contains executable inline script content", pagePath)
		}
		for _, attribute := range node.Attr {
			if strings.EqualFold(attribute.Key, "style") || strings.HasPrefix(strings.ToLower(attribute.Key), "on") {
				t.Errorf("%s contains forbidden %s attribute", pagePath, attribute.Key)
			}
		}
	})
}

func isExecutableScriptType(scriptType string) bool {
	scriptType = strings.ToLower(strings.TrimSpace(scriptType))
	return scriptType == "" || scriptType == "module" || strings.Contains(scriptType, "javascript") || strings.Contains(scriptType, "ecmascript")
}

func checkImagesAndForms(t *testing.T, pagePath string, root *html.Node) {
	t.Helper()
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		if node.Data == "img" && !hasAttr(node, "alt") {
			t.Errorf("%s has an image without alt text", pagePath)
		}
		if node.Data != "form" {
			return
		}
		method := strings.ToLower(attr(node, "method"))
		if method == "" {
			method = "get"
		}
		if method != "post" {
			for _, submit := range findDescendants(node, func(item *html.Node) bool {
				return (item.Data == "button" && (attr(item, "type") == "submit" || attr(item, "type") == "")) || (item.Data == "input" && (attr(item, "type") == "submit" || attr(item, "type") == "image"))
			}) {
				if !hasAttr(submit, "formaction") || strings.ToLower(attr(submit, "formmethod")) != "get" {
					t.Errorf("%s has a state-changing form without method=post", pagePath)
					break
				}
			}
		}
		if method == "post" && len(findDescendants(node, func(item *html.Node) bool { return item.Data == "input" && attr(item, "name") == "csrf_token" })) == 0 {
			t.Errorf("%s has a POST form without csrf_token", pagePath)
		}
		for _, control := range findDescendants(node, func(item *html.Node) bool {
			return item.Data == "input" && attr(item, "type") != "hidden" && attr(item, "type") != "submit" && attr(item, "type") != "button" && attr(item, "type") != "reset" || item.Data == "select" || item.Data == "textarea"
		}) {
			if !controlHasLabel(control, root) {
				t.Errorf("%s has an unlabeled %s control (%s)", pagePath, control.Data, attr(control, "name"))
			}
		}
	})
}

func checkTables(t *testing.T, pagePath string, root *html.Node) {
	t.Helper()
	for _, table := range findNodes(root, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "table" }) {
		if ancestorWithClass(table, "table-wrapper") == nil {
			t.Errorf("%s has a table outside .table-wrapper", pagePath)
		}
		if len(findDescendants(table, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "thead" })) == 0 {
			t.Errorf("%s has a table without thead", pagePath)
		}
		for _, heading := range findDescendants(table, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "th" }) {
			if attr(heading, "scope") != "col" {
				t.Errorf("%s has a table header without scope=col", pagePath)
			}
		}
	}
}

func checkInternalLinks(t *testing.T, s *Server, pagePath string, root *html.Node) {
	t.Helper()
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "a" {
			return
		}
		href := attr(node, "href")
		parsed, err := url.Parse(href)
		if err != nil || parsed.IsAbs() || strings.HasPrefix(href, "//") || parsed.Path == "" {
			return
		}
		matched := false
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			routeContext := chi.NewRouteContext()
			if s.router.Match(routeContext, method, parsed.Path) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("%s links to unresolved internal route %q", pagePath, href)
		}
	})
}

func controlHasLabel(control, root *html.Node) bool {
	if attr(control, "aria-label") != "" || attr(control, "aria-labelledby") != "" {
		return true
	}
	if id := attr(control, "id"); id != "" && len(findNodes(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "label" && attr(node, "for") == id
	})) > 0 {
		return true
	}
	for parent := control.Parent; parent != nil; parent = parent.Parent {
		if parent.Data == "label" {
			return true
		}
		if parent.Data == "form" {
			break
		}
	}
	return false
}

func ancestorWithClass(node *html.Node, class string) *html.Node {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if hasClass(parent, class) {
			return parent
		}
	}
	return nil
}

func hasClass(node *html.Node, class string) bool {
	for _, value := range strings.Fields(attr(node, "class")) {
		if value == class {
			return true
		}
	}
	return false
}

func attr(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func hasAttr(node *html.Node, name string) bool {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return true
		}
	}
	return false
}

func findNodes(root *html.Node, predicate func(*html.Node) bool) []*html.Node {
	result := make([]*html.Node, 0)
	walkHTML(root, func(node *html.Node) {
		if predicate(node) {
			result = append(result, node)
		}
	})
	return result
}

func findDescendants(root *html.Node, predicate func(*html.Node) bool) []*html.Node {
	result := make([]*html.Node, 0)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, func(node *html.Node) {
			if predicate(node) {
				result = append(result, node)
			}
		})
	}
	return result
}

func walkHTML(root *html.Node, visit func(*html.Node)) {
	visit(root)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func textContent(root *html.Node) string {
	var content strings.Builder
	walkHTML(root, func(node *html.Node) {
		if node.Type == html.TextNode {
			content.WriteString(node.Data)
		}
	})
	return strings.TrimSpace(content.String())
}
