// SPDX-License-Identifier: MIT
package apiroute

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type testMounter struct {
	paths []string
}

func (m *testMounter) Method(method, pattern string, _ http.Handler) {
	m.paths = append(m.paths, method+" "+pattern)
}

func TestRegistryRejectsDuplicateCanonicalMethodAndPath(t *testing.T) {
	registry := NewRegistry()
	first := Route{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/items", OperationID: "listItems"}, Handler: http.NotFound}
	if err := registry.Add(first); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add(Route{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/items", OperationID: "getItems"}, Handler: http.NotFound}); err == nil {
		t.Fatal("duplicate canonical method/path was accepted")
	}
}

func TestRegistryRejectsDuplicateOperationID(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Add(Route{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/one", OperationID: "read"}, Handler: http.NotFound}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add(Route{Definition: Definition{Method: http.MethodPost, Path: "/api/{api_version}/two", OperationID: "read"}, Handler: http.NotFound}); err == nil {
		t.Fatal("duplicate operation ID was accepted")
	}
}

func TestRegistryRejectsAliasCollisions(t *testing.T) {
	registry := NewRegistry()
	first := Route{Definition: Definition{
		Method: http.MethodGet, Path: "/api/{api_version}/items", OperationID: "listItems",
		Aliases: []Alias{{Method: http.MethodGet, Path: "/api/items"}},
	}, Handler: http.NotFound}
	if err := registry.Add(first); err != nil {
		t.Fatal(err)
	}
	cases := []Route{
		{Definition: Definition{Method: http.MethodGet, Path: "/api/items", OperationID: "getAlias"}, Handler: http.NotFound},
		{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/other", OperationID: "duplicateAlias", Aliases: []Alias{{Method: http.MethodGet, Path: "/api/items"}}}, Handler: http.NotFound},
	}
	for _, route := range cases {
		if err := registry.Add(route); err == nil {
			t.Fatalf("mount collision for %s was accepted", route.OperationID)
		}
	}
}

func TestRegistryMountExpandsPathsAndFiltersConditions(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Add(Route{Definition: Definition{
		Method: http.MethodGet, Path: "/api/{api_version}/items", OperationID: "listItems", Condition: "enabled",
		Aliases: []Alias{{Method: http.MethodPost, Path: "/api/items", Condition: "alias-enabled"}},
	}, Handler: http.NotFound}); err != nil {
		t.Fatal(err)
	}
	mounter := &testMounter{}
	if err := registry.Mount(mounter, "/api/v2", func(string) bool { return false }, nil); err != nil {
		t.Fatal(err)
	}
	if len(mounter.paths) != 0 {
		t.Fatalf("disabled routes mounted: %v", mounter.paths)
	}
	if err := registry.Mount(mounter, "/api/v2", func(condition string) bool { return condition == "enabled" }, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/v2/items"}
	if len(mounter.paths) != len(want) {
		t.Fatalf("mounted paths = %v, want %v", mounter.paths, want)
	}
	for index := range want {
		if mounter.paths[index] != want[index] {
			t.Fatalf("mounted paths = %v, want %v", mounter.paths, want)
		}
	}
	if err := registry.Mount(mounter, "/api/v2", func(string) bool { return true }, nil); err != nil {
		t.Fatal(err)
	}
	if mounter.paths[len(mounter.paths)-1] != "POST /api/items" {
		t.Fatalf("conditional alias was not mounted when enabled: %v", mounter.paths)
	}
}

func TestRegistryMountRejectsMissingEnabledHandler(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Add(Route{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/items", OperationID: "listItems"}}); err != nil {
		t.Fatal(err)
	}
	err := registry.Mount(&testMounter{}, "/api/v1", nil, nil)
	if err == nil {
		t.Fatal("enabled route without handler was mounted")
	}
}

func TestRegistryDefinitionsAreDefensiveCopies(t *testing.T) {
	definitions := Definitions()
	aliasIndex := -1
	modelIndex := -1
	for index, definition := range definitions {
		if aliasIndex < 0 && len(definition.Aliases) > 0 {
			aliasIndex = index
		}
		if definition.OperationID == "listBangs" {
			modelIndex = index
		}
	}
	if aliasIndex < 0 || modelIndex < 0 {
		t.Fatal("expected route definitions with aliases and mutable request models")
	}
	definitions[aliasIndex].Aliases[0].Path = "/mutated"
	request := definitions[modelIndex].Request.(map[string]any)
	request["mutated"] = true
	fresh := Definitions()
	for _, definition := range fresh {
		if definition.Aliases != nil && definition.Aliases[0].Path == "/mutated" {
			t.Fatal("Definitions leaked a mutable alias slice")
		}
		if definition.OperationID == "listBangs" {
			if _, exists := definition.Request.(map[string]any)["mutated"]; exists {
				t.Fatal("Definitions leaked a mutable request model")
			}
		}
	}
}

func TestDefinitionsCanBeBoundAsOneUniqueRegistry(t *testing.T) {
	registry := NewRegistry()
	for _, definition := range Definitions() {
		if err := registry.Add(Route{Definition: definition, Handler: http.NotFound}); err != nil {
			t.Fatalf("add %s (%s %s): %v", definition.OperationID, definition.Method, definition.Path, err)
		}
	}
	if got, want := len(registry.Routes()), len(Definitions()); got != want {
		t.Fatalf("bound route count = %d, want %d", got, want)
	}
}

func TestRegistryMountUsesBoundHandler(t *testing.T) {
	registry := NewRegistry()
	called := false
	if err := registry.Add(Route{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/items", OperationID: "listItems"}, Handler: func(http.ResponseWriter, *http.Request) { called = true }}); err != nil {
		t.Fatal(err)
	}
	mounter := &capturingMounter{}
	if err := registry.Mount(mounter, "/api/v1", nil, nil); err != nil {
		t.Fatal(err)
	}
	mounter.handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/items", nil))
	if !called {
		t.Fatal("bound route handler did not run")
	}
}

func TestRegistrySeparatesDebugMountingFromInternalVisibility(t *testing.T) {
	registry := NewRegistry()
	for _, route := range []Route{
		{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/metrics", OperationID: "metrics", Internal: true, Condition: "metrics"}, Handler: http.NotFound},
		{Definition: Definition{Method: http.MethodGet, Path: "/api/{api_version}/debug", OperationID: "debug", Internal: true, DebugOnly: true}, Handler: http.NotFound},
	} {
		if err := registry.Add(route); err != nil {
			t.Fatal(err)
		}
	}
	mounter := &testMounter{}
	if err := registry.Mount(mounter, "/api/v1", func(condition string) bool { return condition == "metrics" }, nil); err != nil {
		t.Fatal(err)
	}
	if len(mounter.paths) != 1 || mounter.paths[0] != "GET /api/v1/metrics" {
		t.Fatalf("internal/debug route mounts = %v, want only always-mounted metrics route", mounter.paths)
	}
	if err := registry.Mount(mounter, "/api/v1", func(condition string) bool { return condition == "debug" || condition == "metrics" }, nil); err != nil {
		t.Fatal(err)
	}
	debugMounted := false
	for _, path := range mounter.paths {
		debugMounted = debugMounted || path == "GET /api/v1/debug"
	}
	if !debugMounted {
		t.Fatalf("debug-only route did not mount when enabled: %v", mounter.paths)
	}
}

func TestHealthDefinitionHasTextSuffixAlias(t *testing.T) {
	for _, definition := range Definitions() {
		if definition.OperationID != "getServerHealth" {
			continue
		}
		for _, alias := range definition.Aliases {
			if alias.Method == http.MethodGet && alias.Path == "/api/{api_version}/server/healthz.txt" {
				return
			}
		}
		t.Fatal("health route is missing its .txt API alias")
	}
	t.Fatal("health route definition is missing")
}

type capturingMounter struct {
	handler http.Handler
}

func (m *capturingMounter) Method(_, _ string, handler http.Handler) {
	m.handler = handler
}
