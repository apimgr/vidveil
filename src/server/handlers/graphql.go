// SPDX-License-Identifier: MIT
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/services/engines"
)

// GraphQLHandler handles GraphQL requests
type GraphQLHandler struct {
	cfg       *config.Config
	engineMgr *engines.Manager
}

// NewGraphQLHandler creates a new GraphQL handler
func NewGraphQLHandler(cfg *config.Config, engineMgr *engines.Manager) *GraphQLHandler {
	return &GraphQLHandler{
		cfg:       cfg,
		engineMgr: engineMgr,
	}
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []GQLError  `json:"errors,omitempty"`
}

// GQLError represents a GraphQL error
type GQLError struct {
	Message string `json:"message"`
}

// Handle processes GraphQL requests
func (h *GraphQLHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req GraphQLRequest

	if r.Method == http.MethodGet {
		req.Query = r.URL.Query().Get("query")
		req.OperationName = r.URL.Query().Get("operationName")
		if v := r.URL.Query().Get("variables"); v != "" {
			json.Unmarshal([]byte(v), &req.Variables)
		}
	} else if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(GraphQLResponse{
				Errors: []GQLError{{Message: "Invalid request body"}},
			})
			return
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GraphQLResponse{
			Errors: []GQLError{{Message: "Method not allowed"}},
		})
		return
	}

	// Simple query parser (for common operations)
	result := h.executeQuery(req)
	json.NewEncoder(w).Encode(result)
}

// GraphiQL serves the GraphiQL interface
func (h *GraphQLHandler) GraphiQL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>GraphiQL - Vidveil</title>
    <style>
        body { height: 100%; margin: 0; width: 100%; overflow: hidden; }
        #graphiql { height: 100vh; }
    </style>
    <link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
</head>
<body>
    <div id="graphiql">Loading...</div>
    <script crossorigin src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/graphiql/graphiql.min.js"></script>
    <script>
        const fetcher = GraphiQL.createFetcher({
            url: '/graphql',
        });
        ReactDOM.render(
            React.createElement(GraphiQL, {
                fetcher,
                defaultEditorToolsVisibility: true,
            }),
            document.getElementById('graphiql'),
        );
    </script>
</body>
</html>`))
}

// executeQuery executes a GraphQL query
func (h *GraphQLHandler) executeQuery(req GraphQLRequest) GraphQLResponse {
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

	return GraphQLResponse{
		Errors: []GQLError{{Message: "Unknown query"}},
	}
}

func (h *GraphQLHandler) handleSearch(req GraphQLRequest) GraphQLResponse {
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
		return GraphQLResponse{
			Errors: []GQLError{{Message: "Missing search query"}},
		}
	}

	results := h.engineMgr.Search(nil, q, page, nil)

	// Convert results to GraphQL format
	gqlResults := make([]map[string]interface{}, len(results.Data.Results))
	for i, r := range results.Data.Results {
		gqlResults[i] = map[string]interface{}{
			"id":              r.ID,
			"title":           r.Title,
			"url":             r.URL,
			"thumbnail":       r.Thumbnail,
			"duration":        r.DurationSeconds,
			"durationStr":     r.Duration,
			"views":           r.ViewsCount,
			"viewsStr":        r.Views,
			"source":          r.Source,
			"sourceDisplay":   r.SourceDisplay,
			"description":     r.Description,
		}
	}

	return GraphQLResponse{
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

func (h *GraphQLHandler) handleEngines() GraphQLResponse {
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

	return GraphQLResponse{
		Data: map[string]interface{}{
			"engines": gqlEngines,
		},
	}
}

func (h *GraphQLHandler) handleHealth() GraphQLResponse {
	return GraphQLResponse{
		Data: map[string]interface{}{
			"health": map[string]interface{}{
				"status":         "ok",
				"enginesEnabled": h.engineMgr.EnabledCount(),
			},
		},
	}
}

func (h *GraphQLHandler) handleBangs() GraphQLResponse {
	bangs := engines.ListBangs()

	gqlBangs := make([]map[string]interface{}, len(bangs))
	for i, b := range bangs {
		gqlBangs[i] = map[string]interface{}{
			"bang":        b.Bang,
			"engineName":  b.EngineName,
			"displayName": b.DisplayName,
			"shortCode":   b.ShortCode,
		}
	}

	return GraphQLResponse{
		Data: map[string]interface{}{
			"bangs": gqlBangs,
		},
	}
}

func (h *GraphQLHandler) handleAutocomplete(req GraphQLRequest) GraphQLResponse {
	prefix := ""
	if v, ok := req.Variables["prefix"].(string); ok {
		prefix = v
	}

	if prefix == "" {
		return GraphQLResponse{
			Data: map[string]interface{}{
				"autocomplete": []interface{}{},
			},
		}
	}

	suggestions := engines.Autocomplete(prefix)

	gqlSuggestions := make([]map[string]interface{}, len(suggestions))
	for i, s := range suggestions {
		gqlSuggestions[i] = map[string]interface{}{
			"bang":        s.Bang,
			"engineName":  s.EngineName,
			"displayName": s.DisplayName,
			"shortCode":   s.ShortCode,
		}
	}

	return GraphQLResponse{
		Data: map[string]interface{}{
			"autocomplete": gqlSuggestions,
		},
	}
}

func (h *GraphQLHandler) handleIntrospection(query string) GraphQLResponse {
	schema := h.getSchema()

	if strings.Contains(query, "__schema") {
		return GraphQLResponse{
			Data: map[string]interface{}{
				"__schema": schema,
			},
		}
	}

	return GraphQLResponse{
		Data: map[string]interface{}{
			"__type": nil,
		},
	}
}

func (h *GraphQLHandler) getSchema() map[string]interface{} {
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

// GraphQLSchema returns the GraphQL schema definition
func (h *GraphQLHandler) GraphQLSchema(w http.ResponseWriter, r *http.Request) {
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

// Unused import guard
var _ = strconv.Itoa
