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

	"github.com/apimgr/vidveil/src/swagger"
	"github.com/go-chi/chi/v5"
)

func TestOpenAPIRoutesMatchRegisteredAPIRoutes(t *testing.T) {
	srv := newTestServer(t)
	registered := make(map[string]struct{})
	if err := chi.Walk(srv.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, srv.appConfig.APIBasePath()+"/") {
			registered[strings.ToLower(method)+" "+normalizeContractRoute(route)] = struct{}{}
		}
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
		if !strings.HasPrefix(path, srv.appConfig.APIBasePath()+"/") {
			continue
		}
		for method := range methods {
			documented[strings.ToLower(method)+" "+normalizeContractRoute(path)] = struct{}{}
		}
	}
	for route := range documented {
		if _, ok := registered[route]; !ok {
			t.Errorf("OpenAPI documents unregistered route %s", route)
		}
	}
	for route := range registered {
		if _, ok := documented[route]; !ok {
			t.Errorf("registered API route is missing from OpenAPI: %s", route)
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
	projectRoutes, err := parseIDEAMethodRoutes(string(ideaDoc))
	if err != nil {
		t.Fatal(err)
	}
	for route := range registered {
		if _, ok := templateRoutes[route]; ok {
			continue
		}
		if _, ok := projectRoutes[route]; !ok {
			t.Errorf("registered route %s is missing from both the AI.md template routes and IDEA.md ### API routes", route)
		}
	}
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
var inlineAPIRoutePattern = regexp.MustCompile(`\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(/api/(?:\{api_version\}|v1|[A-Za-z0-9_-]+)[^\s,;)]*)`)

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

func parseAIMethodRoutes(document string) map[string]struct{} {
	routes := make(map[string]struct{})
	for _, line := range strings.Split(document, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		route := strings.TrimSpace(cells[0])
		route = strings.Trim(route, " `")
		if !strings.HasPrefix(route, "/api/") || strings.Contains(route, "*") {
			continue
		}
		methodCell := strings.ToUpper(strings.TrimSpace(cells[1]))
		if methodCell == "ALL" || methodCell == "MIXED" || methodCell == "" {
			continue
		}
		for _, candidate := range expandOptionalRoute(route) {
			for _, method := range strings.Split(methodCell, ",") {
				method = strings.Trim(strings.TrimSpace(method), "`")
				if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete && method != http.MethodHead && method != http.MethodOptions {
					continue
				}
				routes[strings.ToLower(method)+" "+normalizeContractRoute(candidate)] = struct{}{}
			}
		}
	}
	for _, match := range inlineAPIRoutePattern.FindAllStringSubmatch(document, -1) {
		path := strings.TrimRight(match[2], ".:!?`")
		if strings.ContainsAny(path, "*[") {
			continue
		}
		routes[strings.ToLower(match[1])+" "+normalizeContractRoute(path)] = struct{}{}
	}
	// AI.md's API text-negotiation matrix names the read-only status API without
	// a method column; its scheduler section explicitly describes that API as
	// read-only, so its method is GET.
	if strings.Contains(document, "`/api/{api_version}/status`") && strings.Contains(document, "read-only through the server status API") {
		routes["get /api/{api_version}/status"] = struct{}{}
	}
	if strings.Contains(document, "text via the `.txt` suffix (`/api/{api_version}/server/healthz.txt`)") {
		routes["get /api/{api_version}/server/healthz.txt"] = struct{}{}
	}
	return routes
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
	route = regexp.MustCompile(`^/api/[^/]+/`).ReplaceAllString(route, "/api/{api_version}/")
	return route
}
