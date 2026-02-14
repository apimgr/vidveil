// SPDX-License-Identifier: MIT
// See AI.md PART 1 and PART 14 for API/Swagger rules
package swagger

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/apimgr/vidveil/src/config"
)

// DetectTheme determines the UI theme (light/dark/auto) from request
// See AI.md PART 16 for theme detection rules
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

// GenerateSpec generates the OpenAPI 3.0 specification
func GenerateSpec(appConfig *config.AppConfig) string {
	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "VidVeil API",
			"description": "Privacy-respecting meta search for adult video content",
			"version":     "1.0.0",
			"license": map[string]string{
				"name": "MIT",
				"url":  "https://opensource.org/licenses/MIT",
			},
		},
		"servers": []map[string]string{
			{
				"url":         "/",
				"description": "Current server",
			},
		},
		"paths": map[string]interface{}{
			"/api/v1/search": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Search videos",
					"description": "Search across multiple adult video engines",
					"parameters": []map[string]interface{}{
						{
							"name":        "q",
							"in":          "query",
							"required":    true,
							"description": "Search query (supports bang syntax: !ph, !xv, etc.)",
							"schema":      map[string]string{"type": "string"},
						},
						{
							"name":        "page",
							"in":          "query",
							"required":    false,
							"description": "Page number (default: 1)",
							"schema":      map[string]string{"type": "integer"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Search results",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "object"},
								},
							},
						},
					},
				},
			},
			"/api/v1/engines": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List engines",
					"description": "Get all search engines with status",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Engine list",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "array"},
								},
							},
						},
					},
				},
			},
			"/api/v1/healthz": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check",
					"description": "Get API health status (per PART 13)",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Healthy",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "object"},
								},
							},
						},
					},
				},
			},
			"/healthz": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Kubernetes health",
					"description": "Kubernetes-style health endpoint",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
						},
					},
				},
			},
		},
	}

	data, _ := json.MarshalIndent(spec, "", "  ")
	return string(data)
}

// Handler returns the Swagger UI handler
func Handler(appConfig *config.AppConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Theme detection (light/dark/auto) - see theme.go
		theme := DetectTheme(r)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(generateSwaggerUI(appConfig, theme)))
	}
}

// SpecHandler returns the OpenAPI 3.0 specification in JSON format
func SpecHandler(appConfig *config.AppConfig) http.HandlerFunc {
	spec := GenerateSpec(appConfig)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(spec))
	}
}

// generateSwaggerUI generates server-side rendered API documentation
// Per AI.md PART 7: All assets embedded (no CDN). PART 16: Server-side rendered.
func generateSwaggerUI(appConfig *config.AppConfig, theme string) string {
	spec := GenerateSpec(appConfig)

	// Parse spec to extract paths for rendering
	var specData map[string]interface{}
	json.Unmarshal([]byte(spec), &specData)

	// Build endpoint rows
	endpointRows := ""
	if paths, ok := specData["paths"].(map[string]interface{}); ok {
		for path, methods := range paths {
			if methodMap, ok := methods.(map[string]interface{}); ok {
				for method, details := range methodMap {
					summary := ""
					description := ""
					if detailMap, ok := details.(map[string]interface{}); ok {
						if s, ok := detailMap["summary"].(string); ok {
							summary = s
						}
						if d, ok := detailMap["description"].(string); ok {
							description = d
						}
					}
					endpointRows += fmt.Sprintf(
						`<tr><td><span class="method method-%s">%s</span></td><td><code>%s</code></td><td>%s</td><td>%s</td></tr>`,
						html.EscapeString(method),
						html.EscapeString(strings.ToUpper(method)),
						html.EscapeString(path),
						html.EscapeString(summary),
						html.EscapeString(description),
					)
				}
			}
		}
	}

	// Theme colors
	bg := "#1a1a2e"
	cardBg := "#16213e"
	text := "#e0e0e0"
	accent := "#a78bfa"
	border := "#333"
	if theme == "light" {
		bg = "#f5f5f5"
		cardBg = "#ffffff"
		text = "#333"
		accent = "#6d28d9"
		border = "#ddd"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>API Documentation - VidVeil</title>
    <style>
        *{box-sizing:border-box;margin:0;padding:0}
        body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:%s;color:%s;padding:20px}
        .container{max-width:960px;margin:0 auto}
        h1{margin-bottom:8px;color:%s;font-size:24px}
        .subtitle{color:%s;opacity:0.7;margin-bottom:24px;font-size:14px}
        .card{background:%s;border:1px solid %s;border-radius:8px;padding:20px;margin-bottom:20px}
        h2{font-size:18px;margin-bottom:12px;color:%s}
        table{width:100%%;border-collapse:collapse}
        th,td{text-align:left;padding:10px 12px;border-bottom:1px solid %s;font-size:14px}
        th{font-weight:600;color:%s;opacity:0.8;font-size:12px;text-transform:uppercase;letter-spacing:0.5px}
        code{font-family:"Fira Code",Consolas,monospace;font-size:13px}
        .method{display:inline-block;padding:2px 8px;border-radius:4px;font-size:11px;font-weight:700;text-transform:uppercase;font-family:monospace}
        .method-get{background:#22c55e22;color:#22c55e}
        .method-post{background:#3b82f622;color:#3b82f6}
        .method-put{background:#f59e0b22;color:#f59e0b}
        .method-delete{background:#ef444422;color:#ef4444}
        .spec-link{margin-top:16px;font-size:13px}
        .spec-link a{color:%s;text-decoration:none}
        .spec-link a:hover{text-decoration:underline}
        @media(max-width:600px){body{padding:10px}th,td{padding:8px 6px;font-size:13px}}
    </style>
</head>
<body>
    <div class="container">
        <h1>VidVeil API Documentation</h1>
        <p class="subtitle">OpenAPI 3.0 - Privacy-respecting meta search API</p>
        <div class="card">
            <h2>Endpoints</h2>
            <table>
                <thead><tr><th>Method</th><th>Path</th><th>Summary</th><th>Description</th></tr></thead>
                <tbody>%s</tbody>
            </table>
            <div class="spec-link">
                <a href="/openapi.json">View raw OpenAPI specification (JSON)</a>
            </div>
        </div>
    </div>
</body>
</html>`,
		bg, text, accent, text, cardBg, border, accent, border, text, accent,
		endpointRows,
	)
}
