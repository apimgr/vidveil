// SPDX-License-Identifier: MIT
// Per AI.md PART 14: GraphQL handler MUST be in src/graphql/graphql.go
package graphql

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// Handler handles GraphQL requests
type Handler struct {
	appConfig *config.AppConfig
	engineMgr *engine.EngineManager
}

// NewHandler creates a new GraphQL handler
func NewHandler(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *Handler {
	return &Handler{
		appConfig: appConfig,
		engineMgr: engineMgr,
	}
}

// Request represents a GraphQL request
type Request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// Response represents a GraphQL response
type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []Error     `json:"errors,omitempty"`
}

// Error represents a GraphQL error
type Error struct {
	Message string `json:"message"`
}

// Handle processes GraphQL requests
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req Request

	if r.Method == http.MethodGet {
		req.Query = r.URL.Query().Get("query")
		req.OperationName = r.URL.Query().Get("operationName")
		if v := r.URL.Query().Get("variables"); v != "" {
			json.Unmarshal([]byte(v), &req.Variables)
		}
	} else if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, Response{
				Errors: []Error{{Message: "Invalid request body"}},
			})
			return
		}
	} else {
		writeJSON(w, http.StatusMethodNotAllowed, Response{
			Errors: []Error{{Message: "Method not allowed"}},
		})
		return
	}

	// Simple query parser (for common operations)
	result := h.executeQuery(req)
	writeJSON(w, http.StatusOK, result)
}

// GraphiQL serves the GraphQL explorer interface
// Per AI.md PART 16: Server-side rendered, no client-side frameworks
func (h *Handler) GraphiQL(w http.ResponseWriter, r *http.Request) {
	queryStr := ""
	resultHTML := ""

	if r.Method == http.MethodPost {
		r.ParseForm()
		queryStr = r.FormValue("query")
		if queryStr != "" {
			req := Request{Query: queryStr}
			result := h.executeQuery(req)
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			resultHTML = `<div class="result"><h2>Result</h2><pre>` + html.EscapeString(string(jsonBytes)) + `</pre></div>`
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <title>GraphQL Explorer - VidVeil</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        *{box-sizing:border-box;margin:0;padding:0}
        body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#1a1a2e;color:#e0e0e0;padding:20px}
        .container{max-width:900px;margin:0 auto}
        h1{margin-bottom:20px;color:#a78bfa;font-size:24px}
        h2{font-size:16px;margin-bottom:8px;color:#c0c0c0}
        .editor{display:flex;flex-direction:column;gap:12px}
        label{font-weight:600;color:#c0c0c0}
        textarea{width:100%%;min-height:180px;padding:12px;font-family:"Fira Code",Consolas,monospace;font-size:14px;background:#16213e;color:#e0e0e0;border:1px solid #333;border-radius:6px;resize:vertical;tab-size:2}
        textarea:focus{outline:none;border-color:#a78bfa}
        button{padding:10px 24px;background:#a78bfa;color:#fff;border:none;border-radius:6px;font-size:14px;font-weight:600;cursor:pointer;align-self:flex-start}
        button:hover{background:#8b5cf6}
        .result{margin-top:20px}
        .result pre{background:#16213e;padding:16px;border-radius:6px;overflow-x:auto;white-space:pre-wrap;word-break:break-word;font-family:"Fira Code",Consolas,monospace;font-size:13px;border:1px solid #333;max-height:500px;overflow-y:auto}
        .examples{margin-top:24px;padding-top:20px;border-top:1px solid #333}
        .examples ul{list-style:none;margin-top:8px}
        .examples li{margin-bottom:8px}
        .examples code{background:#16213e;padding:4px 8px;border-radius:4px;font-size:13px;font-family:"Fira Code",Consolas,monospace;display:inline-block;word-break:break-word}
        @media(max-width:600px){body{padding:10px}textarea{min-height:140px;font-size:13px}}
    </style>
</head>
<body>
    <div class="container">
        <h1>GraphQL Explorer</h1>
        <form method="POST" class="editor">
            <label for="query">Query</label>
            <textarea id="query" name="query" placeholder="{ health { status enginesEnabled } }">%s</textarea>
            <button type="submit">Execute</button>
        </form>
        %s
        <div class="examples">
            <h2>Example Queries</h2>
            <ul>
                <li><code>{ health { status enginesEnabled } }</code></li>
                <li><code>{ engines { name displayName enabled available } }</code></li>
                <li><code>{ bangs { bang engineName displayName shortCode } }</code></li>
                <li><code>{ search(query: "test") { query results { title url source } searchTimeMs } }</code></li>
                <li><code>{ autocomplete(prefix: "x") { bang displayName } }</code></li>
            </ul>
        </div>
    </div>
</body>
</html>`, html.EscapeString(queryStr), resultHTML)
}

// executeQuery executes a GraphQL query
func (h *Handler) executeQuery(req Request) Response {
	query := strings.TrimSpace(req.Query)

	// Handle introspection
	if strings.Contains(query, "__schema") || strings.Contains(query, "__type") {
		return h.handleIntrospection(query)
	}

	// Parse simple queries - check more specific patterns first
	// Use query field names to avoid false matches (e.g., "enginesEnabled" matching "engines")
	if strings.Contains(query, "search(") || strings.Contains(query, "search {") {
		return h.handleSearch(req)
	}
	// Check for bangs query (must check before health)
	if strings.Contains(query, "bangs {") || strings.Contains(query, "bangs{") || strings.Contains(query, "bangs") {
		return h.handleBangs()
	}
	// Check for autocomplete query
	if strings.Contains(query, "autocomplete(") || strings.Contains(query, "autocomplete {") {
		return h.handleAutocomplete(req)
	}
	// Check for health query (must check before engines because "enginesEnabled" would match "engines")
	if strings.Contains(query, "health {") || strings.Contains(query, "health{") || (strings.Contains(query, "health") && !strings.Contains(query, "engines {")) {
		return h.handleHealth()
	}
	if strings.Contains(query, "engines {") || strings.Contains(query, "engines{") {
		return h.handleEngines()
	}

	return Response{
		Errors: []Error{{Message: "Unknown query"}},
	}
}

func (h *Handler) handleSearch(req Request) Response {
	// Extract query parameter
	q := ""
	page := 1

	if v, ok := req.Variables["query"].(string); ok {
		q = v
	}
	if v, ok := req.Variables["page"].(float64); ok {
		page = int(v)
	}

	if q == "" {
		return Response{
			Errors: []Error{{Message: "Missing search query"}},
		}
	}

	results := h.engineMgr.Search(nil, q, page, nil)

	// Convert results to GraphQL format
	gqlResults := make([]map[string]interface{}, len(results.Data.Results))
	for i, r := range results.Data.Results {
		gqlResults[i] = map[string]interface{}{
			"id":            r.ID,
			"title":         r.Title,
			"url":           r.URL,
			"thumbnail":     r.Thumbnail,
			"duration":      r.DurationSeconds,
			"durationStr":   r.Duration,
			"views":         r.ViewsCount,
			"viewsStr":      r.Views,
			"source":        r.Source,
			"sourceDisplay": r.SourceDisplay,
			"description":   r.Description,
		}
	}

	return Response{
		Data: map[string]interface{}{
			"search": map[string]interface{}{
				"query":         results.Data.Query,
				"results":       gqlResults,
				"enginesUsed":   results.Data.EnginesUsed,
				"enginesFailed": results.Data.EnginesFailed,
				"searchTimeMs":  results.Data.SearchTimeMS,
				"pagination": map[string]interface{}{
					"page":  results.Pagination.Page,
					"limit": results.Pagination.Limit,
					"total": results.Pagination.Total,
					"pages": results.Pagination.Pages,
				},
			},
		},
	}
}

func (h *Handler) handleEngines() Response {
	engines := h.engineMgr.ListEngines()

	gqlEngines := make([]map[string]interface{}, len(engines))
	for i, e := range engines {
		gqlEngines[i] = map[string]interface{}{
			"name":        e.Name,
			"displayName": e.DisplayName,
			"enabled":     e.Enabled,
			"available":   e.Available,
			"tier":        e.Tier,
			"features":    e.Features,
		}
	}

	return Response{
		Data: map[string]interface{}{
			"engines": gqlEngines,
		},
	}
}

func (h *Handler) handleHealth() Response {
	return Response{
		Data: map[string]interface{}{
			"health": map[string]interface{}{
				"status":         "ok",
				"enginesEnabled": h.engineMgr.EnabledCount(),
			},
		},
	}
}

func (h *Handler) handleBangs() Response {
	bangs := engine.ListBangs()

	gqlBangs := make([]map[string]interface{}, len(bangs))
	for i, b := range bangs {
		gqlBangs[i] = map[string]interface{}{
			"bang":        b.Bang,
			"engineName":  b.EngineName,
			"displayName": b.DisplayName,
			"shortCode":   b.ShortCode,
		}
	}

	return Response{
		Data: map[string]interface{}{
			"bangs": gqlBangs,
		},
	}
}

func (h *Handler) handleAutocomplete(req Request) Response {
	prefix := ""
	if v, ok := req.Variables["prefix"].(string); ok {
		prefix = v
	}

	if prefix == "" {
		return Response{
			Data: map[string]interface{}{
				"autocomplete": []interface{}{},
			},
		}
	}

	suggestions := engine.Autocomplete(prefix)

	gqlSuggestions := make([]map[string]interface{}, len(suggestions))
	for i, s := range suggestions {
		gqlSuggestions[i] = map[string]interface{}{
			"bang":        s.Bang,
			"engineName":  s.EngineName,
			"displayName": s.DisplayName,
			"shortCode":   s.ShortCode,
		}
	}

	return Response{
		Data: map[string]interface{}{
			"autocomplete": gqlSuggestions,
		},
	}
}

func (h *Handler) handleIntrospection(query string) Response {
	schema := h.getSchema()

	if strings.Contains(query, "__schema") {
		return Response{
			Data: map[string]interface{}{
				"__schema": schema,
			},
		}
	}

	return Response{
		Data: map[string]interface{}{
			"__type": nil,
		},
	}
}

func (h *Handler) getSchema() map[string]interface{} {
	return map[string]interface{}{
		"queryType": map[string]interface{}{
			"name": "Query",
		},
		"types": []map[string]interface{}{
			{
				"name": "Query",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{
						"name": "search",
						"args": []map[string]interface{}{
							{"name": "query", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
							{"name": "page", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
							{"name": "engines", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
						},
						"type": map[string]interface{}{"name": "SearchResult", "kind": "OBJECT"},
					},
					{
						"name": "bangs",
						"args": []map[string]interface{}{},
						"type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "Bang", "kind": "OBJECT"}},
					},
					{
						"name": "autocomplete",
						"args": []map[string]interface{}{
							{"name": "prefix", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
						},
						"type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "Bang", "kind": "OBJECT"}},
					},
					{
						"name": "engines",
						"args": []map[string]interface{}{},
						"type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "Engine", "kind": "OBJECT"}},
					},
					{
						"name": "health",
						"args": []map[string]interface{}{},
						"type": map[string]interface{}{"name": "Health", "kind": "OBJECT"},
					},
				},
			},
			{
				"name": "SearchResult",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{"name": "query", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "results", "type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "Result", "kind": "OBJECT"}}},
					{"name": "enginesUsed", "type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "String", "kind": "SCALAR"}}},
					{"name": "enginesFailed", "type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "String", "kind": "SCALAR"}}},
					{"name": "searchTimeMs", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "pagination", "type": map[string]interface{}{"name": "Pagination", "kind": "OBJECT"}},
				},
			},
			{
				"name": "Result",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{"name": "id", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "title", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "url", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "thumbnail", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "duration", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "durationStr", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "views", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "viewsStr", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "source", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "sourceDisplay", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "description", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
				},
			},
			{
				"name": "Engine",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{"name": "name", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "displayName", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "enabled", "type": map[string]interface{}{"name": "Boolean", "kind": "SCALAR"}},
					{"name": "available", "type": map[string]interface{}{"name": "Boolean", "kind": "SCALAR"}},
					{"name": "tier", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "features", "type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"name": "String", "kind": "SCALAR"}}},
				},
			},
			{
				"name": "Pagination",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{"name": "page", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "limit", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "total", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
					{"name": "pages", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
				},
			},
			{
				"name": "Health",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{"name": "status", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "enginesEnabled", "type": map[string]interface{}{"name": "Int", "kind": "SCALAR"}},
				},
			},
			{
				"name": "Bang",
				"kind": "OBJECT",
				"fields": []map[string]interface{}{
					{"name": "bang", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "engineName", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "displayName", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
					{"name": "shortCode", "type": map[string]interface{}{"name": "String", "kind": "SCALAR"}},
				},
			},
			{"name": "String", "kind": "SCALAR"},
			{"name": "Int", "kind": "SCALAR"},
			{"name": "Float", "kind": "SCALAR"},
			{"name": "Boolean", "kind": "SCALAR"},
		},
	}
}

// Schema returns the GraphQL schema definition
func (h *Handler) Schema(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(`type Query {
  search(query: String!, page: Int, engines: String): SearchResult
  bangs: [Bang]
  autocomplete(prefix: String!): [Bang]
  engines: [Engine]
  health: Health
}

type SearchResult {
  query: String!
  results: [Result]
  enginesUsed: [String]
  enginesFailed: [String]
  searchTimeMs: Int
  pagination: Pagination
}

type Result {
  id: String!
  title: String!
  url: String!
  thumbnail: String
  duration: Int
  durationStr: String
  views: Int
  viewsStr: String
  source: String!
  sourceDisplay: String
  description: String
}

type Bang {
  bang: String!
  engineName: String!
  displayName: String!
  shortCode: String!
}

type Engine {
  name: String!
  displayName: String!
  enabled: Boolean!
  available: Boolean!
  tier: Int!
  features: [String]
}

type Pagination {
  page: Int!
  limit: Int!
  total: Int!
  pages: Int!
}

type Health {
  status: String!
  enginesEnabled: Int!
}
`))
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
