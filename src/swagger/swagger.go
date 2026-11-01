// SPDX-License-Identifier: MIT
// See AI.md PART 14 for API/Swagger rules
package swagger

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
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

// GenerateSpec generates the OpenAPI 3.0 specification.
// servers[0].url is resolved per request via BuildURL (AI.md PART 8/12) —
// never a static/frozen value, so it matches the Host/proto the client
// actually used, including behind a reverse proxy or over Tor.
func GenerateSpec(appConfig *config.AppConfig, r *http.Request) string {
	apiBase := "/api/v1"
	if appConfig != nil {
		apiBase = appConfig.APIBasePath()
	}
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
				"url":         urlvar.BuildURL(r, ""),
				"description": "Current server",
			},
		},
		"paths": map[string]interface{}{
			apiBase + "/search": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "searchVideos",
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
							"description": "Search results with optional spell suggestion",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "object"},
								},
								"text/plain": map[string]interface{}{
									"schema": map[string]string{"type": "string"},
								},
							},
						},
					},
				},
			},
			apiBase + "/search/batch": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "batchSearch",
					"summary":     "Batch search",
					"description": "Search multiple queries in one request (max 5 queries)",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"queries": map[string]interface{}{
											"type":        "array",
											"maxItems":    5,
											"description": "List of search queries",
											"items":       map[string]string{"type": "string"},
										},
									},
									"required": []string{"queries"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Batch search results",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "object"},
								},
							},
						},
					},
				},
			},
			apiBase + "/engines": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "listEngines",
					"summary":     "List engines",
					"description": "Get all search engines with status and privacy scores",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Engine list with privacy metadata",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "array"},
								},
							},
						},
					},
				},
			},
			apiBase + "/engines/health": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getEngineHealth",
					"summary":     "Engine health",
					"description": "Get health status including circuit breaker state for each engine",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Engine health data",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "array"},
								},
							},
						},
					},
				},
			},
			apiBase + "/server/healthz": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getServerHealth",
					"summary":     "Health check",
					"description": "Get API health status (per AI.md PART 13)",
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
			apiBase + "/server/restore": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId":  "restoreServerBackup",
					"x-admin-only": true,
					"summary":      "Restore a backup",
					"description":  "Upload and verify a .tar.gz or .tar.gz.enc backup before restoring it. Requires the operator bearer token; encrypted archives also require the backup password.",
					"security":     []map[string]interface{}{{"BearerAuth": []string{}}},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"backup"},
									"properties": map[string]interface{}{
										"backup":   map[string]string{"type": "string", "format": "binary"},
										"password": map[string]string{"type": "string", "format": "password"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Backup verified and restored"},
						"400": map[string]interface{}{"description": "Invalid backup or encrypted backup password missing"},
						"401": map[string]interface{}{"description": "Operator token required"},
					},
				},
			},
			"/server/healthz": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check (frontend)",
					"description": "Frontend health endpoint with smart content negotiation (per AI.md PART 13)",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
						},
					},
				},
			},
			"/.well-known/vidveil.json": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Well-known metadata",
					"description": "Machine-readable server metadata and capability discovery",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Server metadata",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]string{"type": "object"},
								},
							},
						},
					},
				},
			},
			"/search.rss": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "RSS feed",
					"description": "Search results as RSS 2.0 feed",
					"parameters": []map[string]interface{}{
						{
							"name":        "q",
							"in":          "query",
							"required":    true,
							"description": "Search query",
							"schema":      map[string]string{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "RSS feed",
							"content": map[string]interface{}{
								"application/rss+xml": map[string]interface{}{
									"schema": map[string]string{"type": "string"},
								},
							},
						},
					},
				},
			},
			"/search.atom": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Atom feed",
					"description": "Search results as Atom 1.0 feed",
					"parameters": []map[string]interface{}{
						{
							"name":        "q",
							"in":          "query",
							"required":    true,
							"description": "Search query",
							"schema":      map[string]string{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Atom feed",
							"content": map[string]interface{}{
								"application/atom+xml": map[string]interface{}{
									"schema": map[string]string{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "operator token",
				},
			},
		},
	}

	paths := spec["paths"].(map[string]interface{})
	addOperation := func(path, method, id, summary, description string, parameters []map[string]interface{}, requestBody map[string]interface{}) {
		pathName := apiBase + path
		pathItem, ok := paths[pathName].(map[string]interface{})
		if !ok {
			pathItem = map[string]interface{}{}
			paths[pathName] = pathItem
		}
		operation := map[string]interface{}{
			"operationId": id,
			"summary":     summary,
			"description": description,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Successful response",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{"schema": map[string]string{"type": "object"}},
					},
				},
			},
		}
		if len(parameters) > 0 {
			operation["parameters"] = parameters
		}
		if requestBody != nil {
			operation["requestBody"] = requestBody
		}
		pathItem[method] = operation
	}
	queryParameter := func(name, description, typ string, required bool) map[string]interface{} {
		return map[string]interface{}{
			"name": name, "in": "query", "required": required, "description": description,
			"schema": map[string]string{"type": typ},
		}
	}
	pathParameter := func(name, description string) map[string]interface{} {
		return map[string]interface{}{
			"name": name, "in": "path", "required": true, "description": description,
			"schema": map[string]string{"type": "string"},
		}
	}
	objectBody := func(properties map[string]interface{}, required []string, mediaType string) map[string]interface{} {
		return map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{
				mediaType: map[string]interface{}{
					"schema": map[string]interface{}{"type": "object", "properties": properties, "required": required},
				},
			},
		}
	}
	stringProperty := func(description string) map[string]string {
		return map[string]string{"type": "string", "description": description}
	}
	addOperation("/bangs", "get", "listBangs", "List bang shortcuts", "Return all available bang shortcuts and their search engines.", nil, nil)
	addOperation("/bangs/autocomplete", "get", "autocompleteBangs", "Autocomplete bang shortcuts", "Return popular search terms or matching bang shortcuts.", []map[string]interface{}{queryParameter("q", "Search text or bang prefix", "string", false)}, nil)
	addOperation("/engines/{name}", "get", "getEngineDetails", "Get search engine details", "Return the status and capabilities of a named search engine.", []map[string]interface{}{pathParameter("name", "Engine identifier")}, nil)
	addOperation("/stats", "get", "getServerStats", "Get search statistics", "Return enabled and total search engine counts.", nil, nil)
	addOperation("/version", "get", "getServerVersion", "Get server version", "Return version, commit, build date, and official site.", nil, nil)
	addOperation("/server/about", "get", "getServerAbout", "Get server information", "Return the public server name, version, and description.", nil, nil)
	addOperation("/server/privacy", "get", "getServerPrivacy", "Get privacy information", "Return the server privacy policy and privacy signals.", nil, nil)
	addOperation("/server/help", "get", "getServerHelp", "Get server help", "Return usage and help information for this server.", nil, nil)
	addOperation("/server/terms", "get", "getServerTerms", "Get server terms", "Return the server terms of service.", nil, nil)
	addOperation("/server/contact", "post", "submitServerContact", "Submit a contact message", "Submit a message to the server contact form.", nil,
		objectBody(map[string]interface{}{
			"subject": stringProperty("Message subject"),
			"message": stringProperty("Message body"),
			"email":   stringProperty("Optional reply address"),
		}, []string{"subject", "message"}, "application/x-www-form-urlencoded"))
	for _, report := range []struct {
		path string
		id   string
		name string
	}{
		{"/server/reports/default", "submitDefaultReport", "Submit a browser security report"},
		{"/server/reports/nel", "submitNetworkErrorReport", "Submit a network error report"},
		{"/server/reports/csp", "submitCSPReport", "Submit a content security policy report"},
	} {
		addOperation(report.path, "post", report.id, report.name, report.name+" payload for security monitoring.", nil,
			objectBody(map[string]interface{}{}, nil, "application/json"))
	}
	addOperation("/server/preferences", "get", "getPreferences", "Get preferences", "Return effective guest preferences from cookies.", nil, nil)
	addOperation("/server/preferences", "post", "savePreferences", "Save preferences", "Apply a partial guest preference update.", nil,
		objectBody(map[string]interface{}{
			"theme":                         map[string]interface{}{"type": "string", "enum": []string{"dark", "light", "auto"}},
			"lang":                          stringProperty("Supported language tag"),
			"vidveil_pref_results_per_page": map[string]interface{}{"type": "string", "enum": []string{"0", "20", "50", "100"}},
			"vidveil_pref_open_new_tab":     map[string]interface{}{"type": "string", "enum": []string{"0", "1"}},
		}, nil, "application/json"))
	addOperation("/server/preferences/tokens", "get", "listOwnerTokens", "List owner tokens", "Return masked primary and linked-token metadata.", nil, nil)
	addOperation("/server/preferences/tokens", "post", "createLinkedToken", "Create linked token", "Create a linked owner token. The raw value is returned only in this response.", nil,
		objectBody(map[string]interface{}{"name": stringProperty("Token name"), "expires": stringProperty("never, 7d, 30d, 90d, 1y, or YYYY-MM-DD")}, []string{"name"}, "application/json"))
	addOperation("/server/preferences/tokens/primary/reveal", "post", "revealPrimaryToken", "Reveal primary token", "Decrypt and return the primary token for this explicit reveal action.", nil, nil)
	addOperation("/server/preferences/tokens/primary/rotate", "post", "rotatePrimaryToken", "Rotate primary token", "Replace the primary token in place; linked tokens remain active.", nil, nil)
	addOperation("/server/preferences/tokens/{id}/reveal", "post", "revealLinkedToken", "Reveal linked token", "Decrypt and return the linked token for this explicit reveal action.", []map[string]interface{}{pathParameter("id", "Linked token ID")}, nil)
	addOperation("/server/preferences/tokens/{id}/rotate", "post", "rotateLinkedToken", "Rotate linked token", "Replace a linked token in place.", []map[string]interface{}{pathParameter("id", "Linked token ID")}, nil)
	addOperation("/server/preferences/tokens/{id}", "patch", "updateLinkedToken", "Update linked token", "Change a linked token name or expiry.", []map[string]interface{}{pathParameter("id", "Linked token ID")},
		objectBody(map[string]interface{}{"name": stringProperty("Token name"), "expires": stringProperty("never, 7d, 30d, 90d, 1y, or YYYY-MM-DD")}, []string{"name"}, "application/json"))
	addOperation("/server/preferences/tokens/{id}", "delete", "revokeLinkedToken", "Revoke linked token", "Permanently revoke a linked token.", []map[string]interface{}{pathParameter("id", "Linked token ID")}, nil)
	for _, route := range []struct{ path, method string }{
		{"/server/preferences/tokens", "get"}, {"/server/preferences/tokens", "post"},
		{"/server/preferences/tokens/primary/reveal", "post"}, {"/server/preferences/tokens/primary/rotate", "post"},
		{"/server/preferences/tokens/{id}/reveal", "post"}, {"/server/preferences/tokens/{id}/rotate", "post"},
		{"/server/preferences/tokens/{id}", "patch"}, {"/server/preferences/tokens/{id}", "delete"},
	} {
		pathItem := paths[apiBase+route.path].(map[string]interface{})
		operation := pathItem[route.method].(map[string]interface{})
		operation["security"] = []map[string]interface{}{{"BearerAuth": []string{}}}
	}
	addOperation("/server/preferences/export", "get", "exportPreferences", "Export preferences", "Return effective preferences as a shareable import URL and code.", nil, nil)
	addOperation("/server/preferences/import", "get", "importPreferences", "Import preferences", "Apply preference values or a share code to the current browser.", []map[string]interface{}{
		queryParameter("theme", "Theme value", "string", false),
		queryParameter("lang", "Language code", "string", false),
		queryParameter("vidveil_pref_results_per_page", "Results per page preference", "string", false),
		queryParameter("vidveil_pref_open_new_tab", "Open links in new tab preference", "string", false),
		queryParameter("code", "Exported preferences code or URL", "string", false),
	}, nil)
	addOperation("/proxy/thumbnails", "get", "proxyThumbnail", "Fetch a remote thumbnail", "Fetch a validated public HTTP or HTTPS thumbnail URL through the server.", []map[string]interface{}{queryParameter("url", "Remote thumbnail URL", "string", true)}, nil)
	addOperation("/proxy/videos", "get", "proxyVideo", "Fetch a remote video", "Fetch a validated public HTTP or HTTPS video URL through the server.", []map[string]interface{}{queryParameter("url", "Remote video URL", "string", true)}, nil)

	data, _ := json.MarshalIndent(spec, "", "  ")
	return string(data)
}

// Handler returns the Swagger UI handler
func Handler(appConfig *config.AppConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Theme detection (light/dark/auto) - see theme.go
		theme := DetectTheme(r)
		lang := i18n.DetectLocale(r)
		dir := i18n.Direction(lang)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(generateSwaggerUI(appConfig, r, theme, lang, dir)))
	}
}

// SpecHandler returns the OpenAPI 3.0 specification in JSON format
func SpecHandler(appConfig *config.AppConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generated per request (not cached at handler-creation time) so
		// servers[0].url reflects the resolved Host/proto per AI.md PART 12.
		spec := GenerateSpec(appConfig, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}
}

// generateSwaggerUI generates server-side rendered API documentation
// Per AI.md PART 7: All assets embedded (no CDN). PART 16: Server-side rendered.
func generateSwaggerUI(appConfig *config.AppConfig, r *http.Request, theme, lang, dir string) string {
	spec := GenerateSpec(appConfig, r)

	// Parse spec to extract paths for rendering
	var specData map[string]interface{}
	if err := json.Unmarshal([]byte(spec), &specData); err != nil {
		return "<p>Unable to render the API specification.</p>"
	}

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

	// Theme is applied via the shared theme-{dark,light,auto} class on <html>
	// (AI.md PART 16) — colors come from common.css custom properties, never
	// inline styles or a separate hardcoded palette.
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s" dir="%s" class="theme-%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="robots" content="index,follow">
    <title>API Documentation - VidVeil</title>
    <link rel="stylesheet" href="/static/css/common.css">
    <link rel="stylesheet" href="/static/css/swagger.css">
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
                <a href="/api/swagger">View raw OpenAPI specification (JSON)</a>
            </div>
        </div>
    </div>
</body>
</html>`,
		lang, dir, theme,
		endpointRows,
	)
}
