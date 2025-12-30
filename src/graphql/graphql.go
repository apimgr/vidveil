// SPDX-License-Identifier: MIT
// See AI.md PART 1 lines 482-486 for GraphQL structure
package graphql

import (
	"encoding/json"
	"net/http"

	"github.com/apimgr/vidveil/src/config"
)

// DetectTheme determines the UI theme (light/dark/auto) from request
// See AI.md PART 17 for theme detection rules
func DetectTheme(r *http.Request) string {
	// Check cookie first
	if cookie, err := r.Cookie("theme"); err == nil {
		if cookie.Value == "light" || cookie.Value == "dark" {
			return cookie.Value
		}
	}

	// Check query parameter
	if theme := r.URL.Query().Get("theme"); theme == "light" || theme == "dark" {
		return theme
	}

	// Default to auto (respects browser preference)
	return "auto"
}

// GraphiQL returns the GraphiQL UI handler
func GraphiQL(w http.ResponseWriter, r *http.Request) {
	theme := DetectTheme(r)
	
	bgColor := "#282a36" // dark theme default
	textColor := "#f8f8f2"
	if theme == "light" {
		bgColor = "#ffffff"
		textColor = "#000000"
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>VidVeil GraphQL API</title>
    <style>
        body {
            height: 100vh;
            margin: 0;
            width: 100%;
            overflow: hidden;
            background: ` + bgColor + `;
            color: ` + textColor + `;
        }
        #graphiql { height: 100vh; }
    </style>
    <script crossorigin src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
    <script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
    <link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
</head>
<body>
    <div id="graphiql">Loading...</div>
    <script src="https://unpkg.com/graphiql/graphiql.min.js" type="application/javascript"></script>
    <script>
        const fetcher = GraphiQL.createFetcher({
            url: '/graphql',
        });
        ReactDOM.render(
            React.createElement(GraphiQL, {
                fetcher: fetcher,
                defaultTheme: '` + theme + `',
            }),
            document.getElementById('graphiql'),
        );
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// GraphQLSchema returns the GraphQL schema
func GraphQLSchema(w http.ResponseWriter, r *http.Request) {
	schema := `
type Query {
  search(query: String!, page: Int): SearchResponse
  engines: [Engine!]!
  engine(name: String!): Engine
}

type SearchResponse {
  success: Boolean!
  data: SearchData!
  pagination: Pagination!
}

type SearchData {
  query: String!
  searchQuery: String
  results: [Video!]!
  enginesUsed: [String!]!
  enginesFailed: [String!]!
  searchTimeMs: Int!
  hasBang: Boolean
  bangEngines: [String!]
  cached: Boolean
}

type Video {
  id: String!
  title: String!
  url: String!
  thumbnail: String!
  previewUrl: String
  duration: String!
  durationSeconds: Int!
  views: String!
  viewsCount: Int!
  rating: Float
  quality: String
  source: String!
  sourceDisplay: String!
  published: String
  description: String
}

type Engine {
  name: String!
  displayName: String!
  enabled: Boolean!
  available: Boolean!
  features: [String!]!
  tier: Int!
}

type Pagination {
  currentPage: Int!
  totalPages: Int
  hasNext: Boolean!
  hasPrev: Boolean!
}
`

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(schema))
}

// Handler handles GraphQL queries
func Handler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Basic GraphQL handler stub
		// TODO: Implement full GraphQL resolver
		
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"message": "GraphQL endpoint - resolver implementation pending",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
