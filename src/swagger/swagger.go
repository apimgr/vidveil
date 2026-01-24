// SPDX-License-Identifier: MIT
// See AI.md PART 1 and PART 14 for API/Swagger rules
package swagger

import (
	"encoding/json"
	"net/http"

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
			"/api/search": map[string]interface{}{
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
			"/api/engines": map[string]interface{}{
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
			"/api/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check",
					"description": "Get API health status",
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

func generateSwaggerUI(appConfig *config.AppConfig, theme string) string {
	// Dark theme default
	bgColor := "#282a36"
	if theme == "light" {
		bgColor = "#ffffff"
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Vidveil API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; background: ` + bgColor + `; }
        .swagger-ui { max-width: 1200px; margin: 0 auto; }
        .swagger-ui .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.json",
                dom_id: '#swagger-ui',
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout",
                deepLinking: true
            });
        };
    </script>
</body>
</html>`
}
