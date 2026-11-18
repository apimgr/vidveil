// SPDX-License-Identifier: MIT
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			registered[strings.ToLower(method)+" "+route] = struct{}{}
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
			documented[strings.ToLower(method)+" "+path] = struct{}{}
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
}
