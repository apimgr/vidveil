// SPDX-License-Identifier: MIT
// Package apiroute defines the shared API route contract used by the router,
// OpenAPI generator, llms.txt, and response serializer.
package apiroute

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
)

// Definition is the static route metadata shared with build-time consumers.
type Definition struct {
	Method      string
	Path        string
	OperationID string
	Summary     string
	Auth        string
	Listed      bool
	Request     any
	Response    any
	Primary     string
	Message     string
	Binary      string
	SkipText    bool
	Condition   string
	Aliases     []Alias
}

// Alias describes an additional exact method/path mount for a canonical route.
type Alias struct {
	Method    string
	Path      string
	Condition string
}

// Route binds static metadata to the handler owned by one server instance.
type Route struct {
	Definition
	Handler http.HandlerFunc
}

// Mounter is implemented by chi routers and keeps this shared package
// independent from a particular router implementation.
type Mounter interface {
	Method(method, pattern string, handler http.Handler)
}

// Registry contains bound routes for one server instance.
type Registry struct {
	routes map[string]Route
	mounts map[string]string
}

// NewRegistry constructs an empty per-server route registry.
func NewRegistry() *Registry {
	return &Registry{routes: make(map[string]Route), mounts: make(map[string]string)}
}

// Add binds one route and rejects duplicate method/path contracts.
func (r *Registry) Add(route Route) error {
	if r == nil {
		return fmt.Errorf("nil API route registry")
	}
	if !supportedMethod(route.Method) || route.Path == "" || route.OperationID == "" {
		return fmt.Errorf("API route requires method, path, and operation ID")
	}
	if !strings.HasPrefix(route.Path, "/") {
		return fmt.Errorf("API route %q path must be absolute", route.OperationID)
	}
	if route.Listed && route.Auth == "operator" {
		return fmt.Errorf("operator-only API route %q cannot be listed", route.OperationID)
	}
	if r.routes == nil {
		r.routes = make(map[string]Route)
	}
	key := route.Method + " " + route.Path
	if existingID, exists := r.mounts[key]; exists {
		return fmt.Errorf("duplicate API mount %s (already owned by %s)", key, existingID)
	}
	for _, existing := range r.routes {
		if existing.OperationID == route.OperationID {
			return fmt.Errorf("duplicate API operation ID %q", route.OperationID)
		}
	}
	newMounts := []string{key}
	for _, alias := range route.Aliases {
		if !supportedMethod(alias.Method) || alias.Path == "" {
			return fmt.Errorf("API alias on %s requires method and path", route.OperationID)
		}
		aliasKey := alias.Method + " " + alias.Path
		if aliasKey == key {
			return fmt.Errorf("API alias duplicates canonical mount %s", key)
		}
		for _, pending := range newMounts {
			if pending == aliasKey {
				return fmt.Errorf("duplicate API alias %s on %s", aliasKey, route.OperationID)
			}
		}
		if existingID, exists := r.mounts[aliasKey]; exists {
			return fmt.Errorf("duplicate API mount %s (already owned by %s)", aliasKey, existingID)
		}
		newMounts = append(newMounts, aliasKey)
	}
	r.routes[key] = cloneRoute(route)
	if r.mounts == nil {
		r.mounts = make(map[string]string)
	}
	for _, mount := range newMounts {
		r.mounts[mount] = route.OperationID
	}
	return nil
}

func supportedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Routes returns defensive copies of all bound route metadata and handlers.
func (r *Registry) Routes() []Route {
	if r == nil {
		return nil
	}
	routes := make([]Route, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, cloneRoute(route))
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

// Definitions returns defensive copies of the static API contract for
// build-time generators that must not import the server package.
func Definitions() []Definition {
	definitions := make([]Definition, len(routeDefinitions))
	for index, definition := range routeDefinitions {
		definitions[index] = cloneDefinition(definition)
	}
	return definitions
}

// Mount registers every enabled route and alias, failing when an enabled
// route has no bound handler. apiBase replaces the canonical version token.
func (r *Registry) Mount(router Mounter, apiBase string, enabled func(string) bool, wrap func(Route, http.Handler) http.Handler) error {
	if router == nil {
		return fmt.Errorf("nil API route router")
	}
	for _, route := range r.Routes() {
		if !conditionEnabled(route.Condition, enabled) {
			continue
		}
		if route.Handler == nil {
			return fmt.Errorf("enabled API route %s %s has no handler", route.Method, route.Path)
		}
		mount := func(method, path string) {
			handler := http.Handler(route.Handler)
			if wrap != nil {
				handler = wrap(route, handler)
			}
			router.Method(method, expandPath(path, apiBase), handler)
		}
		mount(route.Method, route.Path)
		for _, alias := range route.Aliases {
			if conditionEnabled(alias.Condition, enabled) {
				mount(alias.Method, alias.Path)
			}
		}
	}
	return nil
}

func conditionEnabled(condition string, enabled func(string) bool) bool {
	return condition == "" || enabled == nil || enabled(condition)
}

func expandPath(path, apiBase string) string {
	return strings.ReplaceAll(path, "/api/{api_version}", apiBase)
}

func cloneRoute(route Route) Route {
	route.Definition = cloneDefinition(route.Definition)
	return route
}

func cloneDefinition(definition Definition) Definition {
	definition.Aliases = append([]Alias(nil), definition.Aliases...)
	definition.Request = cloneModel(definition.Request)
	definition.Response = cloneModel(definition.Response)
	return definition
}

func cloneModel(model any) any {
	if model == nil {
		return nil
	}
	return cloneValue(reflect.ValueOf(model)).Interface()
}

func cloneValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneValue(value.Elem())
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			result.SetMapIndex(cloneValue(iter.Key()), cloneValue(iter.Value()))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneValue(value.Index(index)))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneValue(value.Index(index)))
		}
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneValue(value.Elem()))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for index := 0; index < value.NumField(); index++ {
			if result.Field(index).CanSet() && value.Field(index).CanInterface() {
				result.Field(index).Set(cloneValue(value.Field(index)))
			}
		}
		return result
	default:
		return value
	}
}
