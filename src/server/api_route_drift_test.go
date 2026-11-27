// SPDX-License-Identifier: MIT
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/common/apiroute"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/graphql"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/swagger"
	"github.com/go-chi/chi/v5"
)

func TestOpenAPIRoutesMatchRegisteredAPIRoutes(t *testing.T) {
	srv := newTestServerWithOptions(t, func(cfg *config.AppConfig) {
		cfg.Server.Metrics.Enabled = true
	})
	registered := make(map[string]struct{})
	if err := chi.Walk(srv.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[strings.ToLower(method)+" "+normalizeContractRoute(route)] = struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("walk registered routes: %v", err)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	spec := swagger.GenerateSpec(srv.appConfig, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := json.Unmarshal([]byte(spec), &document); err != nil {
		t.Fatalf("decode generated OpenAPI: %v", err)
	}
	documented := make(map[string]struct{})
	for path, methods := range document.Paths {
		for method := range methods {
			route := strings.ToLower(method) + " " + normalizeContractRoute(path)
			if classifyContractRoute(normalizeContractRoute(path)) == contractRouteInternal {
				t.Errorf("OpenAPI must not document INTERNAL route %s", route)
			}
			documented[route] = struct{}{}
		}
	}
	for route := range documented {
		if _, ok := registered[route]; !ok {
			t.Errorf("OpenAPI documents unregistered route %s", route)
		}
	}
	for route := range registered {
		path := strings.SplitN(route, " ", 2)[1]
		if strings.HasPrefix(path, "/api/") && strings.Contains(path, "/debug/") {
			t.Errorf("AI.md PART 6 forbids debug endpoints in the versioned API: %s", route)
		}
		if strings.HasPrefix(path, srv.appConfig.APIBasePath()+"/") && classifyContractRoute(path) != contractRouteInternal {
			if _, ok := documented[route]; !ok {
				t.Errorf("registered API route is missing from OpenAPI: %s", route)
			}
		}
	}

	root := repositoryRoot(t)
	aiDoc, err := os.ReadFile(filepath.Join(root, "AI.md"))
	if err != nil {
		t.Fatalf("read AI.md route contract: %v", err)
	}
	ideaDoc, err := os.ReadFile(filepath.Join(root, "IDEA.md"))
	if err != nil {
		t.Fatalf("read IDEA.md route inventory: %v", err)
	}
	templateRoutes := parseAIMethodRoutes(string(aiDoc))
	pageInventory := parseAIBasePagePaths(string(aiDoc))
	for _, parseError := range templateRoutes.Errors {
		t.Errorf("AI.md route inventory: %s", parseError)
	}
	projectRoutes, err := parseIDEAMethodRoutes(string(ideaDoc))
	if err != nil {
		t.Fatal(err)
	}
	for route := range registered {
		parsed, inTemplate := templateRoutes.Routes[route]
		if inTemplate && parsed.Class != contractRoutePage {
			continue
		}
		if strings.HasPrefix(strings.SplitN(route, " ", 2)[1], srv.appConfig.APIBasePath()+"/") {
			if _, ok := projectRoutes[route]; !ok {
				t.Errorf("registered route %s is missing from both the AI.md template routes and IDEA.md ### API routes", route)
			}
		}
	}
	for route, template := range templateRoutes.Routes {
		if !contractRouteEnabled(template, srv.appConfig) {
			continue
		}
		if _, ok := registered[route]; !ok {
			t.Errorf("AI.md %s route is missing from the router: %s", template.Class, route)
		}
		_, inOpenAPI := documented[route]
		if template.Class == contractRouteAPI && !inOpenAPI {
			t.Errorf("AI.md API route is missing from OpenAPI: %s", route)
		}
		if template.Class != contractRouteAPI && inOpenAPI {
			t.Errorf("AI.md %s route must not appear in OpenAPI: %s", template.Class, route)
		}
		if template.Class == contractRoutePage {
			if _, registeredPage := registeredHubPageSet(srv.appConfig)[strings.SplitN(route, " ", 2)[1]]; registeredPage {
				if _, ok := registered[route]; !ok {
					t.Errorf("AI.md PAGE route is missing from the page registry/router: %s", route)
				}
			}
		}
	}
	for path := range registeredHubPageSet(srv.appConfig) {
		route := "get " + normalizeContractRoute(path)
		if _, ok := templateRoutes.Routes[route]; !ok {
			if _, listedInPageRegistry := pageInventory[path]; !listedInPageRegistry {
				t.Errorf("page registry path is missing from the AI.md PAGE inventory: GET %s", path)
			}
		}
		if _, ok := registered[route]; !ok {
			t.Errorf("page registry path is missing from router: GET %s", path)
		}
	}
	for _, prefix := range templateRoutes.Prefixes {
		if !contractRouteEnabled(prefix, srv.appConfig) {
			continue
		}
		prefixPath := strings.TrimSuffix(prefix.Path, "*")
		matched := 0
		for route := range registered {
			path := strings.SplitN(route, " ", 2)[1]
			if !strings.HasPrefix(path, prefixPath) {
				continue
			}
			matched++
			if _, explicit := templateRoutes.Routes[route]; !explicit {
				if _, projectRoute := projectRoutes[route]; !projectRoute {
					t.Errorf("registered route under AI.md prefix %s lacks an explicit method row: %s", prefix.Path, route)
				}
			}
		}
		if matched == 0 {
			t.Errorf("AI.md prefix %s has no registered routes", prefix.Path)
		}
	}

	assertInternalRoutesExcluded(t, srv, templateRoutes, documented, registered)
	for route := range projectRoutes {
		if _, ok := registered[route]; !ok {
			t.Errorf("IDEA.md ### API routes declares unregistered route %s", route)
		}
		if _, ok := documented[route]; !ok {
			t.Errorf("IDEA.md ### API routes declares route missing from OpenAPI: %s", route)
		}
	}
}

var ideaRoutePattern = regexp.MustCompile("^-\\s+`([A-Z]+)\\s+([^`]+)`:")

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate API route drift test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func parseIDEAMethodRoutes(document string) (map[string]struct{}, error) {
	routes := make(map[string]struct{})
	inRouteSection := false
	foundRouteSection := false
	for _, line := range strings.Split(document, "\n") {
		if strings.HasPrefix(line, "### ") {
			inRouteSection = line == "### API routes"
			foundRouteSection = foundRouteSection || inRouteSection
			continue
		}
		if !inRouteSection {
			continue
		}
		match := ideaRoutePattern.FindStringSubmatch(line)
		if match == nil {
			if strings.Contains(line, "No project-defined routes.") {
				continue
			}
			continue
		}
		route := normalizeContractRoute(strings.TrimSpace(match[2]))
		for _, method := range strings.Split(match[1], ",") {
			routes[strings.ToLower(strings.TrimSpace(method))+" "+route] = struct{}{}
		}
	}
	if !foundRouteSection {
		return nil, fmt.Errorf("IDEA.md is missing the required ### API routes subsection")
	}
	return routes, nil
}

func parseAIBasePagePaths(document string) map[string]struct{} {
	paths := make(map[string]struct{})
	inTable := false
	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "**Base pages by template:**" {
			inTable = true
			continue
		}
		if inTable && strings.HasPrefix(trimmed, "#") {
			break
		}
		if !inTable || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitMarkdownTableRow(trimmed)
		if len(cells) == 0 {
			continue
		}
		if match := contractRoutePathPattern.FindStringSubmatch(strings.TrimSpace(cells[0])); len(match) == 2 {
			paths[normalizeContractRoute(match[1])] = struct{}{}
		}
	}
	return paths
}

type contractRouteClass string

const (
	contractRouteAPI      contractRouteClass = "api"
	contractRoutePage     contractRouteClass = "page"
	contractRouteInternal contractRouteClass = "internal"
)

type contractRoute struct {
	Method    string
	Path      string
	Class     contractRouteClass
	Condition string
	Optional  bool
	Prefix    bool
}

type routeInventory struct {
	Routes   map[string]contractRoute
	Prefixes []contractRoute
	Errors   []string
}

var (
	contractRoutePathPattern = regexp.MustCompile("`(/[^`]+)`")
	contractConditionPattern = regexp.MustCompile(`server\.[a-zA-Z0-9_.]+`)
)

func parseAIMethodRoutes(document string) routeInventory {
	inventory := routeInventory{Routes: make(map[string]contractRoute)}
	lines := strings.Split(document, "\n")
	inFence := false
	headingCondition := ""
	illustrativeSection := false
	headingOptional := false

	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		line := strings.TrimSpace(lines[lineIndex])
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "#") {
			headingCondition = routeCondition(line)
			heading := strings.ToLower(line)
			headingOptional = strings.Contains(heading, "optional")
			illustrativeSection = strings.Contains(heading, "example") || strings.Contains(heading, "illustrative") || strings.Contains(heading, "docs/")
			continue
		}
		if strings.Contains(strings.ToLower(line), "illustrative example") || strings.Contains(strings.ToLower(line), "example - project-specific") {
			illustrativeSection = true
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}

		tableLines := []string{line}
		for nextIndex := lineIndex + 1; nextIndex < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[nextIndex]), "|"); nextIndex++ {
			tableLines = append(tableLines, strings.TrimSpace(lines[nextIndex]))
			lineIndex = nextIndex
		}
		if len(tableLines) < 3 {
			continue
		}

		headerCells := splitMarkdownTableRow(tableLines[0])
		pathColumn := tableColumn(headerCells, "endpoint", "path", "route")
		methodColumn := tableColumn(headerCells, "method", "methods")
		if pathColumn < 0 || methodColumn < 0 {
			continue
		}
		conditionColumn := tableColumn(headerCells, "present when", "condition", "mounted when")
		if illustrativeSection {
			continue
		}
		for _, row := range tableLines[2:] {
			cells := splitMarkdownTableRow(row)
			if pathColumn >= len(cells) || methodColumn >= len(cells) {
				continue
			}
			pathCell := strings.TrimSpace(cells[pathColumn])
			if pathCell == "`/api/{api_version}/{resource}`" {
				continue
			}
			paths := contractRoutePathPattern.FindAllStringSubmatch(pathCell, -1)
			if len(paths) == 0 {
				continue
			}
			if len(paths) != 1 || strings.TrimSpace(pathCell) != paths[0][0] {
				inventory.Errors = append(inventory.Errors, fmt.Sprintf("route cell %q must contain exactly one backticked path; aliases require separate rows", pathCell))
				continue
			}
			methodCell := strings.ToUpper(strings.TrimSpace(cells[methodColumn]))
			condition := headingCondition
			if conditionColumn >= 0 && conditionColumn < len(cells) {
				if rowCondition := contractConditionPattern.FindString(cells[conditionColumn]); rowCondition != "" {
					condition = rowCondition
				}
			}
			if rowCondition := routeCondition(strings.Join(cells, " ")); rowCondition != "" {
				condition = rowCondition
			}
			optional := headingOptional || condition != ""
			if methodCell == "" {
				inventory.Errors = append(inventory.Errors, fmt.Sprintf("route %q has no explicit method", pathCell))
				continue
			}

			for _, pathMatch := range paths {
				for _, candidate := range expandOptionalRoute(pathMatch[1]) {
					candidate = normalizeContractRoute(candidate)
					if methodCell == "ALL" {
						if !strings.HasSuffix(candidate, "/*") {
							inventory.Errors = append(inventory.Errors, fmt.Sprintf("ALL route %q must end in /*", candidate))
							continue
						}
						prefix := contractRoute{Method: "ALL", Path: candidate, Class: classifyContractRoute(candidate), Condition: condition, Optional: optional, Prefix: true}
						inventory.Prefixes = append(inventory.Prefixes, prefix)
						continue
					}
					for _, method := range strings.FieldsFunc(methodCell, func(character rune) bool {
						return character == ',' || character == '/'
					}) {
						method = strings.Trim(strings.TrimSpace(method), "`")
						if !supportedContractMethod(method) {
							inventory.Errors = append(inventory.Errors, fmt.Sprintf("route %q has unsupported or missing method %q", candidate, methodCell))
							continue
						}
						route := contractRoute{Method: method, Path: candidate, Class: classifyContractRoute(candidate), Condition: condition, Optional: optional}
						key := strings.ToLower(method) + " " + candidate
						inventory.Routes[key] = route
					}
				}
			}
		}
	}
	return inventory
}

func splitMarkdownTableRow(line string) []string {
	cells := strings.Split(strings.Trim(line, "|"), "|")
	for index := range cells {
		cells[index] = strings.TrimSpace(cells[index])
	}
	return cells
}

func tableColumn(cells []string, names ...string) int {
	for index, cell := range cells {
		name := strings.ToLower(strings.Trim(strings.TrimSpace(cell), "`* "))
		for _, candidate := range names {
			if name == candidate {
				return index
			}
		}
	}
	return -1
}

func routeCondition(value string) string {
	if match := contractConditionPattern.FindString(value); match != "" {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "when") || strings.Contains(lower, "gated") || strings.Contains(lower, "optional") || strings.Contains(lower, "mounted") || strings.Contains(lower, "condition") {
			return match
		}
	}
	if strings.Contains(strings.ToLower(value), "--debug") || strings.Contains(strings.ToUpper(value), "DEBUG=TRUE") {
		return "debug"
	}
	return ""
}

func supportedContractMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func classifyContractRoute(path string) contractRouteClass {
	internalPrefixes := []string{
		"/debug/", "/server/tor/", "/server/metrics", "/metrics",
		"/api/metrics", "/api/{api_version}/metrics", "/api/{api_version}/server/metrics",
	}
	for _, prefix := range internalPrefixes {
		if path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix) {
			return contractRouteInternal
		}
	}
	if strings.HasPrefix(path, "/api/") || path == "/mcp" {
		return contractRouteAPI
	}
	return contractRoutePage
}

func contractRouteEnabled(route contractRoute, appConfig *config.AppConfig) bool {
	if route.Condition == "" {
		return !route.Optional
	}
	if appConfig == nil {
		return false
	}
	switch route.Condition {
	case "debug":
		return mode.IsDebugEnabled()
	case "server.healthz.root.enabled":
		return appConfig.Server.Healthz.Root.Enabled
	case "server.metrics.enabled":
		return appConfig.Server.Metrics.Enabled
	case "server.metrics.root.enabled":
		return appConfig.Server.Metrics.Enabled && appConfig.Server.Metrics.Root.Enabled
	default:
		return false
	}
}

func registeredHubPageSet(appConfig *config.AppConfig) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, path := range handler.RegisteredHubPagePaths(appConfig) {
		paths[path] = struct{}{}
	}
	return paths
}

func assertInternalRoutesExcluded(t *testing.T, srv *Server, inventory routeInventory, documented, registered map[string]struct{}) {
	t.Helper()
	for key, route := range inventory.Routes {
		if route.Class != contractRouteInternal || !contractRouteEnabled(route, srv.appConfig) {
			continue
		}
		if _, ok := registered[key]; !ok {
			t.Errorf("AI.md INTERNAL route is missing from router: %s", key)
		}
		if _, ok := documented[key]; ok {
			t.Errorf("AI.md INTERNAL route appears in OpenAPI: %s", key)
		}
	}

	srv.appConfig.Web.LLMS.Enabled = true
	srv.appConfig.Web.LLMS.IncludeEndpoints = true
	llmsResponse := httptest.NewRecorder()
	srv.searchHandler.LlmsTxt(llmsResponse, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))
	for _, route := range inventory.Routes {
		if route.Class == contractRouteInternal && contractRouteEnabled(route, srv.appConfig) {
			if strings.Contains(llmsResponse.Body.String(), strings.ReplaceAll(route.Path, "{api_version}", srv.appConfig.Server.APIVersion)) {
				t.Errorf("AI.md INTERNAL route appears in llms.txt: %s %s", route.Method, route.Path)
			}
		}
	}

	gql := graphql.NewHandler(srv.appConfig, srv.engineMgr)
	introspection := httptest.NewRecorder()
	gqlRequest := httptest.NewRequest(http.MethodPost, srv.appConfig.APIBasePath()+"/server/graphql", strings.NewReader(`{"query":"{ __schema { queryType { fields { name } } } }"}`))
	gql.Handle(introspection, gqlRequest)
	var schema struct {
		Data struct {
			Schema struct {
				QueryType struct {
					Fields []struct {
						Name string `json:"name"`
					} `json:"fields"`
				} `json:"queryType"`
			} `json:"__schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal(introspection.Body.Bytes(), &schema); err != nil {
		t.Errorf("decode GraphQL introspection: %v", err)
	} else {
		for _, field := range schema.Data.Schema.QueryType.Fields {
			switch field.Name {
			case "metrics", "serviceMetrics", "debug", "debugEngines", "debugEngine":
				t.Errorf("INTERNAL operation is exposed as GraphQL field %q", field.Name)
			}
		}
	}

	mcpResponse := httptest.NewRecorder()
	mcpRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	mcpRequest.Header.Set("Content-Type", "application/json")
	mcpRequest.Header.Set("Accept", "application/json, text/event-stream")
	mcpRequest.Header.Set("Mcp-Method", "tools/list")
	if token := srv.appConfig.Server.Token; token != "" {
		mcpRequest.Header.Set("Authorization", "Bearer "+token)
	}
	srv.router.ServeHTTP(mcpResponse, mcpRequest)
	for _, definition := range apiroute.Definitions() {
		if definition.Internal && strings.Contains(mcpResponse.Body.String(), definition.OperationID) {
			t.Errorf("INTERNAL operation %s appears in MCP tools", definition.OperationID)
		}
	}
}

func TestParseAIMethodRoutesUsesNamedColumnsAndPreservesClasses(t *testing.T) {
	document := strings.Join([]string{
		"## API routes",
		"| Description | Methods | Route |",
		"|-------------|---------|-------|",
		"| Item API | GET/HEAD | `/api/{api_version}/items` |",
		"| Public API alias | POST | `/mcp` |",
		"| About page | GET | `/server/about` |",
		"| Metrics API | GET | `/api/{api_version}/server/metrics` |",
		"| Metrics API alias | GET | `/api/{api_version}/metrics` |",
		"",
		"### Debug API Endpoints (OPTIONAL: mounted only while `--debug` or `DEBUG=true` is set)",
		"| Purpose | Method | Endpoint |",
		"|---------|--------|----------|",
		"| Debug configuration | GET | `/debug/config` |",
		"",
		"### Optional health alias",
		"| Description | Path | Method |",
		"|-------------|------|--------|",
		"| Root health alias when `server.healthz.root.enabled` is true | `/healthz` | GET |",
		"",
		"### Admin prefix",
		"| Method | Route | Description |",
		"|--------|-------|-------------|",
		"| ALL | `/server/admin/*` | Admin route prefix |",
		"",
		"### Examples",
		"```markdown",
		"| Method | Path | Description |",
		"|--------|------|-------------|",
		"| GET | `/api/{api_version}/not-a-route` | Fenced example |",
		"```",
		"| Method | Path | Description |",
		"|--------|------|-------------|",
		"| GET | `/api/{api_version}/{resource}` | Illustrative generic route |",
	}, "\n")

	inventory := parseAIMethodRoutes(document)
	if len(inventory.Errors) != 0 {
		t.Fatalf("parse errors: %v", inventory.Errors)
	}
	assertRoute := func(key string, class contractRouteClass, condition string, optional bool) {
		t.Helper()
		route, ok := inventory.Routes[key]
		if !ok {
			t.Fatalf("missing route %q; parsed: %#v", key, inventory.Routes)
		}
		if route.Class != class || route.Condition != condition || route.Optional != optional {
			t.Errorf("route %q metadata = %#v; want class=%q condition=%q optional=%t", key, route, class, condition, optional)
		}
	}
	assertRoute("get /api/{api_version}/items", contractRouteAPI, "", false)
	assertRoute("head /api/{api_version}/items", contractRouteAPI, "", false)
	assertRoute("post /mcp", contractRouteAPI, "", false)
	assertRoute("get /server/about", contractRoutePage, "", false)
	assertRoute("get /api/{api_version}/server/metrics", contractRouteInternal, "", false)
	assertRoute("get /api/{api_version}/metrics", contractRouteInternal, "", false)
	assertRoute("get /debug/config", contractRouteInternal, "debug", true)
	assertRoute("get /healthz", contractRoutePage, "server.healthz.root.enabled", true)
	if _, ok := inventory.Routes["get /api/{api_version}/not-a-route"]; ok {
		t.Fatal("fenced example was parsed as a route")
	}
	if _, ok := inventory.Routes["get /api/{api_version}/{resource}"]; ok {
		t.Fatal("illustrative generic route was parsed as a template route")
	}
	if len(inventory.Prefixes) != 1 {
		t.Fatalf("parsed prefix count = %d, want 1 (%#v)", len(inventory.Prefixes), inventory.Prefixes)
	}
	prefix := inventory.Prefixes[0]
	if prefix.Path != "/server/admin/*" || !prefix.Prefix || prefix.Class != contractRoutePage {
		t.Errorf("prefix metadata = %#v", prefix)
	}
}

func TestParseAIMethodRoutesExpandsAliasesAndOptionalSegments(t *testing.T) {
	document := strings.Join([]string{
		"| Path | Method | Description |",
		"|------|--------|-------------|",
		"| `/api/{api_version}/metrics[/{service}]` | GET | Optional when `server.metrics.enabled` |",
		"| `/api/metrics` | GET, OPTIONS | API alias |",
		"| `/metrics` | GET, OPTIONS | Root alias |",
	}, "\n")

	inventory := parseAIMethodRoutes(document)
	if len(inventory.Errors) != 0 {
		t.Fatalf("parse errors: %v", inventory.Errors)
	}
	for _, route := range []string{
		"get /api/{api_version}/metrics",
		"get /api/{api_version}/metrics/{service}",
		"get /api/metrics",
		"options /api/metrics",
		"get /metrics",
		"options /metrics",
	} {
		if _, ok := inventory.Routes[route]; !ok {
			t.Errorf("missing expanded route %q; parsed: %#v", route, inventory.Routes)
		}
	}
	if got := inventory.Routes["get /api/{api_version}/metrics"].Condition; got != "server.metrics.enabled" {
		t.Errorf("optional metric condition = %q, want server.metrics.enabled", got)
	}
	if inventory.Routes["get /api/metrics"].Class != contractRouteInternal || inventory.Routes["get /metrics"].Class != contractRouteInternal {
		t.Errorf("metric aliases must remain INTERNAL: API=%#v root=%#v", inventory.Routes["get /api/metrics"], inventory.Routes["get /metrics"])
	}

	combined := parseAIMethodRoutes(strings.Join([]string{
		"| Path | Method | Description |",
		"|------|--------|-------------|",
		"| `/api/metrics` and `/metrics` | GET | Invalid combined aliases |",
	}, "\n"))
	if len(combined.Errors) != 1 || !strings.Contains(combined.Errors[0], "aliases require separate rows") {
		t.Errorf("combined alias row errors = %v; want one explicit separate-row error", combined.Errors)
	}
}

func TestNormalizeContractRouteHandlesVersionRootAndRepresentationSuffix(t *testing.T) {
	for actual, want := range map[string]string{
		"/api/v1":                     "/api/{api_version}",
		"/api/metrics":                "/api/metrics",
		"/api/v1/server/healthz.txt":  "/api/{api_version}/server/healthz",
		"/api/v1/server/healthz":      "/api/{api_version}/server/healthz",
		"/api/v1/server/metrics/loki": "/api/{api_version}/server/metrics/loki",
	} {
		if got := normalizeContractRoute(actual); got != want {
			t.Errorf("normalizeContractRoute(%q) = %q, want %q", actual, got, want)
		}
	}
}

func TestParseAIBasePagePathsReadsPageRegistryTable(t *testing.T) {
	document := strings.Join([]string{
		"**Base pages by template:**",
		"",
		"| Page | API | SERVER | HYBRID | Notes |",
		"|------|-----|--------|--------|-------|",
		"| `/server/help/mcp` | yes | yes | yes | Always registered |",
		"",
		"### Header and Navigation Conformance",
	}, "\n")
	if _, ok := parseAIBasePagePaths(document)["/server/help/mcp"]; !ok {
		t.Fatal("page registry parser omitted /server/help/mcp")
	}
}

func expandOptionalRoute(route string) []string {
	open := strings.IndexByte(route, '[')
	if open < 0 {
		return []string{route}
	}
	close := strings.IndexByte(route[open+1:], ']')
	if close < 0 {
		return []string{route}
	}
	close += open + 1
	base := route[:open] + route[close+1:]
	withOptional := route[:open] + route[open+1:close] + route[close+1:]
	return append(expandOptionalRoute(base), expandOptionalRoute(withOptional)...)
}

func normalizeContractRoute(route string) string {
	route = strings.TrimSpace(route)
	route = regexp.MustCompile(`^/api/v[0-9]+$`).ReplaceAllString(route, "/api/{api_version}")
	route = regexp.MustCompile(`^/api/v[0-9]+/`).ReplaceAllString(route, "/api/{api_version}/")
	route = regexp.MustCompile(`^(/api/\{api_version\}/server/healthz)\.txt$`).ReplaceAllString(route, "$1")
	return route
}
