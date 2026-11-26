// SPDX-License-Identifier: MIT
// GraphQL schema contract for VidVeil read operations (AI.md PART 14).
package graphql

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/graph-gophers/graphql-go"
)

// schemaAnnotations are the source declarations for the generated GraphQL
// schema. Descriptions use i18n keys and are resolved when building the schema.
var schemaAnnotations = []schemaTypeAnnotation{
	{name: "Query", descriptionKey: "graphql.type_descriptions.query", fields: []schemaFieldAnnotation{
		{name: "search", args: "query: String!, page: Int, engines: String", typeName: "SearchResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "searchBatch", args: "queries: [BatchQueryInput!]!", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "bangs", typeName: "[Bang!]!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "autocomplete", args: "prefix: String!", typeName: "[Bang!]!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "engines", typeName: "[Engine!]!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "health", typeName: "Health!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "engineHealth", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "engineDetails", args: "name: String!", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "stats", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "version", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "healthz", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverAbout", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverPrivacy", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverDpo", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverContact", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverHelp", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverHelpMcp", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverTerms", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverSecurity", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverSecurityPolicy", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "serverSecurityThanks", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "operatorStatus", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "preferences", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "preferenceSync", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "preferenceTokens", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "favorites", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
		{name: "openAPI", typeName: "JSONResult!", descriptionKey: "graphql.field_descriptions.query"},
	}},
	{name: "SearchResult", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "query", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "results", typeName: "[VideoResult!]!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "enginesUsed", typeName: "[String!]!", descriptionKey: "graphql.field_descriptions.value"}, {name: "enginesFailed", typeName: "[String!]!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "searchTimeMs", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"}, {name: "pagination", typeName: "Pagination!", descriptionKey: "graphql.field_descriptions.value"},
	}},
	{name: "VideoResult", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "id", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "title", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "url", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "thumbnail", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "duration", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"}, {name: "durationStr", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "views", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"}, {name: "viewsStr", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "source", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "sourceDisplay", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "description", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
	}},
	{name: "Bang", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "bang", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "engineName", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "displayName", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "shortCode", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
	}},
	{name: "Engine", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "name", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "displayName", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "enabled", typeName: "Boolean!", descriptionKey: "graphql.field_descriptions.value"}, {name: "available", typeName: "Boolean!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "tier", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"}, {name: "features", typeName: "[String!]!", descriptionKey: "graphql.field_descriptions.value"},
	}},
	{name: "Pagination", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "page", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"}, {name: "limit", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "total", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"}, {name: "pages", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"},
	}},
	{name: "Health", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "status", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"}, {name: "enginesEnabled", typeName: "Int!", descriptionKey: "graphql.field_descriptions.value"},
	}},
	{name: "JSONResult", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "json", typeName: "String!", descriptionKey: "graphql.field_descriptions.json"},
	}},
	{name: "BatchQueryInput", kind: "input", descriptionKey: "graphql.type_descriptions.object", fields: []schemaFieldAnnotation{
		{name: "q", typeName: "String!", descriptionKey: "graphql.field_descriptions.value"},
		{name: "page", typeName: "Int", descriptionKey: "graphql.field_descriptions.value"},
		{name: "engines", typeName: "String", descriptionKey: "graphql.field_descriptions.value"},
	}},
}

type schemaTypeAnnotation struct {
	name, descriptionKey string
	kind                 string
	fields               []schemaFieldAnnotation
}

type schemaFieldAnnotation struct {
	name, args, typeName, descriptionKey string
}

func renderSchemaAnnotations(translate func(string) string) string {
	var b strings.Builder
	for _, typ := range schemaAnnotations {
		if description := strings.TrimSpace(translate(typ.descriptionKey)); description != "" {
			fmt.Fprintf(&b, "\"\"\"%s\"\"\"\n", strings.ReplaceAll(description, `"""`, `'''`))
		}
		kind := typ.kind
		if kind == "" {
			kind = "type"
		}
		fmt.Fprintf(&b, "%s %s {\n", kind, typ.name)
		for _, field := range typ.fields {
			if description := strings.TrimSpace(translate(field.descriptionKey)); description != "" {
				fmt.Fprintf(&b, "  \"\"\"%s\"\"\"\n", strings.ReplaceAll(description, `"""`, `'''`))
			}
			args := ""
			if field.args != "" {
				args = "(" + field.args + ")"
			}
			fmt.Fprintf(&b, "  %s%s: %s\n", field.name, args, field.typeName)
		}
		b.WriteString("}\n\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

//go:embed schema_generated.graphql
var schemaSDL string

// SchemaSource is consumed by the deterministic generator to write the embedded artifact.
func SchemaSource() string {
	return renderSchemaAnnotations(func(key string) string { return i18n.Translate(i18n.DefaultLocale, key) })
}

// Handler owns the executable GraphQL schema and its read-only resolvers.
type Handler struct {
	appConfig      *config.AppConfig
	engineMgr      *engine.EngineManager
	searchRecorder func(string)
	readers        map[string]ReadOperation
	schema         *graphql.Schema
	schemaCache    map[string]*graphql.Schema
	schemaMu       sync.Mutex
}

// NewHandler constructs the query-only schema with bounded parsing and execution.
func NewHandler(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *Handler {
	h := &Handler{appConfig: appConfig, engineMgr: engineMgr, readers: make(map[string]ReadOperation)}
	h.schemaCache = make(map[string]*graphql.Schema)
	h.schema = h.buildSchema(i18n.DefaultLocale)
	h.schemaCache[i18n.DefaultLocale] = h.schema
	return h
}

func (h *Handler) buildSchema(lang string) *graphql.Schema {
	sdl := renderSchemaAnnotations(func(key string) string {
		return i18n.Translate(lang, key)
	})
	if lang == i18n.DefaultLocale && schemaSDL != "" {
		sdl = schemaSDL
	}
	return graphql.MustParseSchema(sdl, &queryResolver{handler: h},
		graphql.MaxDepth(12),
		graphql.MaxQueryLength(16<<10),
		graphql.MaxParallelism(8),
	)
}

func (h *Handler) schemaForLocale(lang string) *graphql.Schema {
	if lang == "" {
		lang = i18n.DefaultLocale
	}
	h.schemaMu.Lock()
	defer h.schemaMu.Unlock()
	if schema := h.schemaCache[lang]; schema != nil {
		return schema
	}
	schema := h.buildSchema(lang)
	h.schemaCache[lang] = schema
	return schema
}

// ReadOperation adapts an existing JSON REST read handler without bypassing
// its request authentication, cookie, ownership, or content-negotiation rules.
type ReadOperation func(*http.Request, map[string]any) (string, error)

// SetReadOperation wires one existing REST read operation into GraphQL.
func (h *Handler) SetReadOperation(name string, operation ReadOperation) {
	if h.readers == nil {
		h.readers = make(map[string]ReadOperation)
	}
	h.readers[name] = operation
}

// SetSearchRecorder wires the application callback for privacy-filtered search feed entries.
func (h *Handler) SetSearchRecorder(record func(string)) {
	h.searchRecorder = record
}
