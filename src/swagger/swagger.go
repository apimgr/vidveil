// SPDX-License-Identifier: MIT
// See AI.md PART 14 for API/Swagger rules
package swagger

//go:generate go run ../internal/specgen

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/apimgr/vidveil/src/common/apiroute"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

//go:embed openapi_generated.json
var generatedOpenAPIFS embed.FS

// GenerateSourceSpec returns the canonical versioned superset written by specgen.
func GenerateSourceSpec() string {
	request := &http.Request{Host: "generated.invalid", URL: &url.URL{Scheme: "https", Host: "generated.invalid"}}
	return generateSourceSpec(nil, request)
}

// GenerateSpec generates the OpenAPI 3.0 specification.
// servers[0].url is resolved per request via BuildURL (AI.md PART 8/12) —
// never a static/frozen value, so it matches the Host/proto the client
// actually used, including behind a reverse proxy or over Tor.
func generateSourceSpec(appConfig *config.AppConfig, r *http.Request) string {
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
				"url":         sourceServerURL(r),
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
					"bearerFormat": "VidVeil bearer token",
				},
				"OwnerTokenAuth": map[string]interface{}{
					"type": "http", "scheme": "bearer", "bearerFormat": "own_ token",
				},
				"OperatorTokenAuth": map[string]interface{}{
					"type": "http", "scheme": "bearer", "bearerFormat": "adm_ token",
				},
				"ServiceTokenAuth": map[string]interface{}{
					"type": "http", "scheme": "bearer", "bearerFormat": "mtr_ token",
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
	addOperation("/server/dpo", "get", "getServerDPO", "Get DPO contact", "Return the configured DPO contact and privacy policy URL.", nil, nil)
	addOperation("/server/contact", "get", "getServerContactForm", "Get contact form description", "Return the public contact form fields and captcha descriptor without exposing recipients.", nil, nil)
	addOperation("/server/auth/check", "get", "checkServerToken", "Check bearer token", "Return the kind of a valid bearer token without updating token use metadata or echoing the credential.", nil, nil)
	addOperation("/server/help", "get", "getServerHelp", "Get server help", "Return usage and help information for this server.", nil, nil)
	addOperation("/server/help/mcp", "get", "getServerHelpMCP", "Get MCP connection help", "Return MCP endpoint metadata, connection guidance, and tools available to a valid Bearer token.", nil, nil)
	addOperation("/server/terms", "get", "getServerTerms", "Get server terms", "Return the server terms of service.", nil, nil)
	addOperation("/server/security", "get", "getServerSecurity", "Get security reporting information", "Return public security contacts, disclosure policy links, and published key metadata.", nil, nil)
	addOperation("/server/security/policy", "get", "getServerSecurityPolicy", "Get the security disclosure policy", "Return the configured or built-in disclosure policy as Markdown.", nil, nil)
	addOperation("/server/security/thanks", "get", "getServerSecurityThanks", "List security acknowledgments", "Return opted-in security researchers in newest-first order.", nil, nil)
	setResponseSchema := func(path, method string, schema map[string]interface{}) {
		operation := paths[apiBase+path].(map[string]interface{})[method].(map[string]interface{})
		responses := operation["responses"].(map[string]interface{})
		response := responses["200"].(map[string]interface{})
		content := response["content"].(map[string]interface{})
		media := content["application/json"].(map[string]interface{})
		media["schema"] = schema
	}
	setResponseSchema("/server/security", "get", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"contacts":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"type": map[string]string{"type": "string"}, "value": map[string]string{"type": "string"}}, "required": []string{"type", "value"}}},
			"expires":    map[string]string{"type": "string", "format": "date-time"},
			"pgp":        map[string]interface{}{"type": "object", "properties": map[string]interface{}{"available": map[string]string{"type": "boolean"}, "url": map[string]interface{}{"type": "string", "format": "uri", "nullable": true}, "fingerprint": map[string]interface{}{"type": "string", "nullable": true}}, "required": []string{"available", "url", "fingerprint"}},
			"policy_url": map[string]string{"type": "string", "format": "uri"},
			"thanks_url": map[string]string{"type": "string", "format": "uri"},
			"report_url": map[string]string{"type": "string", "format": "uri"},
		},
		"required": []string{"contacts", "expires", "pgp", "policy_url", "thanks_url"},
	})
	setResponseSchema("/server/security/policy", "get", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"policy": map[string]string{"type": "string"}}, "required": []string{"policy"}})
	setResponseSchema("/server/security/thanks", "get", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"entries": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"credit": map[string]string{"type": "string"}, "date": map[string]string{"type": "string", "format": "date"}, "severity": map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low", "informational"}}}, "required": []string{"credit", "date", "severity"}}}}, "required": []string{"entries"}})
	addOperation("/server/swagger", "get", "getOpenAPISpec", "Get the OpenAPI specification", "Return the generated OpenAPI JSON document.", nil, nil)
	for _, annotation := range graphQLOperations {
		var parameters []map[string]interface{}
		for _, parameter := range annotation.parameters {
			parameters = append(parameters, queryParameter(parameter.name, parameter.description, parameter.schemaType, parameter.required))
		}
		var requestBody map[string]interface{}
		if annotation.requestBody {
			requestBody = objectBody(map[string]interface{}{
				"query":         stringProperty("GraphQL query document"),
				"operationName": stringProperty("Name of the operation to execute"),
				"variables":     map[string]interface{}{"type": "object"},
			}, []string{"query"}, "application/json")
		}
		addOperation(annotation.path, annotation.method, annotation.operationID, annotation.summary, annotation.description, parameters, requestBody)
	}
	addOperation("/server/mcp", "post", "executeMCP", "Call MCP tools", "Streamable HTTP endpoint for the Model Context Protocol.", nil,
		objectBody(map[string]interface{}{}, nil, "application/json"))
	addOperation("/favorites/", "get", "listFavorites", "List favorites", "List private favorites belonging to the current anonymous owner.", nil, nil)
	addOperation("/favorites/", "post", "addFavorite", "Add a favorite", "Save a result as a private favorite for the current anonymous owner.", nil,
		objectBody(map[string]interface{}{"url": stringProperty("Result URL"), "title": stringProperty("Result title"), "thumbnail": stringProperty("Thumbnail URL"), "source": stringProperty("Canonical source name")}, []string{"url", "title"}, "application/json"))
	addOperation("/favorites/import", "post", "importFavorites", "Import favorites", "Import private favorites from an export payload.", nil,
		objectBody(map[string]interface{}{"favorites": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"url": stringProperty("Result URL"), "title": stringProperty("Result title"), "thumbnail": stringProperty("Thumbnail URL"), "source": stringProperty("Canonical source name")}, "required": []string{"url", "title"}}}}, []string{"favorites"}, "application/json"))
	addOperation("/favorites/{id}", "delete", "removeFavorite", "Remove a favorite", "Remove one private favorite belonging to the current anonymous owner.", []map[string]interface{}{pathParameter("id", "Favorite identifier")}, nil)
	addOperation("/favorites/", "delete", "clearFavorites", "Clear favorites", "Remove all private favorites belonging to the current anonymous owner.", nil, nil)
	contactBody := objectBody(map[string]interface{}{
		"name":                 map[string]interface{}{"type": "string", "description": "Sender name", "maxLength": 100},
		"email":                map[string]interface{}{"type": "string", "format": "email", "description": "Reply address", "maxLength": 254},
		"subject":              map[string]interface{}{"type": "string", "description": "Message subject", "maxLength": 200},
		"message":              map[string]interface{}{"type": "string", "description": "Message body", "maxLength": 5000},
		"captcha_id":           stringProperty("Single-use captcha challenge id"),
		"captcha_answer":       stringProperty("Answer to the simple captcha challenge"),
		"g-recaptcha-response": stringProperty("Required when the configured provider is reCAPTCHA"),
		"h-captcha-response":   stringProperty("Required when the configured provider is hCaptcha"),
	}, []string{"name", "email", "subject", "message"}, "application/x-www-form-urlencoded")
	contactSchema := contactBody["content"].(map[string]interface{})["application/x-www-form-urlencoded"].(map[string]interface{})["schema"].(map[string]interface{})
	contactSchema["oneOf"] = []map[string]interface{}{
		{"required": []string{"captcha_id", "captcha_answer"}},
		{"required": []string{"g-recaptcha-response"}},
		{"required": []string{"h-captcha-response"}},
	}
	addOperation("/server/contact", "post", "submitServerContact", "Submit a contact message", "Submit a message to the server contact form; exactly one configured or built-in CAPTCHA response is required.", nil, contactBody)
	addOperation("/status", "get", "getOperatorStatus", "Get operator status", "Return maintenance diagnostics and recovery guidance. Requires the configured server operator bearer token.", nil, nil)
	operatorStatus := paths[apiBase+"/status"].(map[string]interface{})["get"].(map[string]interface{})
	operatorStatus["security"] = []map[string]interface{}{{"BearerAuth": []string{}}}
	operatorStatus["x-admin-only"] = true
	operatorStatus["responses"] = map[string]interface{}{
		"200": map[string]interface{}{"description": "Operator status and maintenance recovery guidance"},
		"401": map[string]interface{}{"description": "Operator bearer token required"},
	}
	authCheck := paths[apiBase+"/server/auth/check"].(map[string]interface{})["get"].(map[string]interface{})
	authCheck["security"] = []map[string]interface{}{{"BearerAuth": []string{}}}
	authCheck["responses"] = map[string]interface{}{
		"200": map[string]interface{}{"description": "Token is valid; response data contains the token kind"},
		"401": map[string]interface{}{"description": "Bearer token is missing or invalid"},
	}
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
	addOperation("/server/preferences/tokens/primary", "post", "issuePrimaryOwnerToken", "Issue primary owner token", "Create a new anonymous owner and return its primary token once. No bearer token is required; a valid existing token receives 409.", nil, nil)
	issuePrimary := paths[apiBase+"/server/preferences/tokens/primary"].(map[string]interface{})["post"].(map[string]interface{})
	issuePrimary["responses"] = map[string]interface{}{
		"201": map[string]interface{}{
			"description": "Primary owner token issued once",
			"content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{
				"type": "object", "properties": map[string]interface{}{"ok": map[string]string{"type": "boolean"}, "data": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"owner_token": stringProperty("Raw primary owner token; store securely")}}},
			}}},
		},
		"409": map[string]interface{}{"description": "A valid bearer token is already present"},
		"429": map[string]interface{}{"description": "The strict token-action rate limit was exceeded"},
	}
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
	addOperation("/server/preferences/sync", "get", "exportPreferences", "Export preferences", "Return effective preferences as a shareable import URL and code.", nil, nil)
	addOperation("/server/preferences/sync/import", "get", "importPreferences", "Import preferences", "Apply preference values or a share code to the current browser.", []map[string]interface{}{
		queryParameter("theme", "Theme value", "string", false),
		queryParameter("lang", "Language code", "string", false),
		queryParameter("vidveil_pref_results_per_page", "Results per page preference", "string", false),
		queryParameter("vidveil_pref_open_new_tab", "Open links in new tab preference", "string", false),
		queryParameter("code", "Exported preferences code or URL", "string", false),
	}, nil)
	addOperation("/proxy/thumbnails", "get", "proxyThumbnail", "Fetch a remote thumbnail", "Fetch a validated public HTTP or HTTPS thumbnail URL through the server.", []map[string]interface{}{queryParameter("url", "Remote thumbnail URL", "string", true)}, nil)
	addOperation("/proxy/videos", "get", "proxyVideo", "Fetch a remote video", "Fetch a validated public HTTP or HTTPS video URL through the server.", []map[string]interface{}{queryParameter("url", "Remote video URL", "string", true)}, nil)
	markCondition := func(path, method, condition string) {
		if item, ok := paths[path].(map[string]interface{}); ok {
			if operation, ok := item[method].(map[string]interface{}); ok {
				operation["x-mounted-when"] = condition
			}
		}
	}
	for _, route := range []struct{ path, method, id, summary string }{
		{apiBase + "/debug/engines", "get", "debugEngines", "List engine diagnostics"},
		{apiBase + "/debug/engines/{name}", "get", "debugEngine", "Get engine diagnostics"},
		{"/api/debug/engines", "get", "debugEnginesAlias", "List engine diagnostics"},
		{"/api/debug/engines/{name}", "get", "debugEngineAlias", "Get engine diagnostics"},
	} {
		path := strings.TrimPrefix(route.path, apiBase)
		parameters := []map[string]interface{}{}
		if strings.Contains(path, "{name}") {
			parameters = append(parameters, pathParameter("name", "Engine identifier"))
		}
		if strings.HasPrefix(route.path, "/api/debug/") {
			item := map[string]interface{}{"operationId": route.id, "summary": route.summary, "parameters": parameters, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Engine diagnostics"}}, "security": []map[string]interface{}{{"BearerAuth": []string{}}}, "x-mounted-when": "debug"}
			if paths[route.path] == nil {
				paths[route.path] = map[string]interface{}{}
			}
			paths[route.path].(map[string]interface{})[route.method] = item
		} else {
			addOperation(path, route.method, route.id, route.summary, "Operator-only engine diagnostics.", parameters, nil)
			item := paths[apiBase+path].(map[string]interface{})[route.method].(map[string]interface{})
			item["security"] = []map[string]interface{}{{"BearerAuth": []string{}}}
			markCondition(apiBase+path, route.method, "debug")
		}
	}
	for _, servicePath := range []string{"/server/metrics", "/server/metrics/{service}"} {
		parameters := []map[string]interface{}{}
		if strings.Contains(servicePath, "{service}") {
			parameters = append(parameters, pathParameter("service", "Metrics service"))
		}
		addOperation(servicePath, "get", "getMetrics", "Get service metrics", "Prometheus metrics endpoint.", parameters, nil)
		item := paths[apiBase+servicePath].(map[string]interface{})["get"].(map[string]interface{})
		item["security"] = []map[string]interface{}{{"BearerAuth": []string{}}}
		markCondition(apiBase+servicePath, "get", "server.metrics.enabled")
	}
	for _, servicePath := range []string{"/metrics", "/metrics/{service}", "/api/metrics", "/api/metrics/{service}"} {
		parameters := []map[string]interface{}{}
		if strings.Contains(servicePath, "{service}") {
			parameters = append(parameters, pathParameter("service", "Metrics service"))
		}
		item := map[string]interface{}{"operationId": "getMetricsAlias", "summary": "Get service metrics", "parameters": parameters, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Prometheus metrics"}}, "security": []map[string]interface{}{{"BearerAuth": []string{}}}, "x-mounted-when": "server.metrics.root.enabled"}
		if strings.HasPrefix(servicePath, "/api/") {
			item["x-mounted-when"] = "server.metrics.enabled"
		}
		if paths[servicePath] == nil {
			paths[servicePath] = map[string]interface{}{}
		}
		paths[servicePath].(map[string]interface{})["get"] = item
	}
	for _, alias := range []struct{ path, method, operationID, summary string }{
		{"/api/swagger", "get", "getOpenAPISpecAlias", "Get the OpenAPI specification"},
		{"/api/graphql", "post", "executeGraphQLAlias", "Execute a GraphQL query"},
		{"/api/healthz", "get", "getHealthAlias", "Get server health"},
		{"/api/mcp", "post", "executeMCPAlias", "Call MCP tools"},
		{"/mcp", "post", "executeMCPRootAlias", "Call MCP tools"},
	} {
		paths[alias.path] = map[string]interface{}{alias.method: map[string]interface{}{"operationId": alias.operationID, "summary": alias.summary, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Direct alias of the canonical endpoint"}}}}
	}

	synchronizeRouteDefinitions(paths, apiBase)
	data, _ := json.MarshalIndent(spec, "", "  ")
	return string(data)
}

func sourceServerURL(r *http.Request) string {
	if r != nil && r.Host == "generated.invalid" {
		return "https://generated.invalid"
	}
	return urlvar.BuildURL(r, "")
}

// synchronizeRouteDefinitions makes the route registry authoritative for the
// API path/method inventory while retaining detailed legacy schemas where the
// registered model is intentionally generic.
func synchronizeRouteDefinitions(paths map[string]interface{}, apiBase string) {
	type operationKey struct{ path, method string }
	wanted := make(map[operationKey]map[string]interface{})
	for _, definition := range apiroute.Definitions() {
		if definition.Internal {
			continue
		}
		path := strings.ReplaceAll(definition.Path, "/api/{api_version}", apiBase)
		method := strings.ToLower(definition.Method)
		operation := routeOperation(paths, path, method, definition)
		wanted[operationKey{path, method}] = operation
		for _, alias := range definition.Aliases {
			aliasPath := strings.ReplaceAll(alias.Path, "/api/{api_version}", apiBase)
			aliasMethod := strings.ToLower(alias.Method)
			aliasOperation := cloneOperation(operation)
			aliasOperation["operationId"] = aliasOperationID(definition.OperationID, aliasPath)
			if alias.Condition != "" {
				aliasOperation["x-mounted-when"] = alias.Condition
			}
			wanted[operationKey{aliasPath, aliasMethod}] = aliasOperation
		}
	}

	for path, raw := range paths {
		if !isRegistryAPIPath(path) {
			continue
		}
		operations, ok := raw.(map[string]interface{})
		if !ok {
			delete(paths, path)
			continue
		}
		for method := range operations {
			if _, exists := wanted[operationKey{path, method}]; !exists {
				delete(operations, method)
			}
		}
		if len(operations) == 0 {
			delete(paths, path)
		}
	}
	for key, operation := range wanted {
		pathItem, ok := paths[key.path].(map[string]interface{})
		if !ok {
			pathItem = make(map[string]interface{})
			paths[key.path] = pathItem
		}
		pathItem[key.method] = operation
	}
}

func routeOperation(paths map[string]interface{}, path, method string, definition apiroute.Definition) map[string]interface{} {
	var operation map[string]interface{}
	if item, ok := paths[path].(map[string]interface{}); ok {
		operation, _ = item[method].(map[string]interface{})
	}
	if operation == nil {
		operation = map[string]interface{}{
			"responses": map[string]interface{}{
				"200": map[string]interface{}{"description": "Successful response"},
			},
		}
	}
	operation = cloneOperation(operation)
	operation["operationId"] = definition.OperationID
	operation["summary"] = definition.Summary
	operation["x-listed"] = definition.Listed
	if definition.Auth == "operator" {
		operation["x-admin-only"] = true
	} else {
		delete(operation, "x-admin-only")
	}
	scheme := ""
	switch definition.Auth {
	case "owner":
		scheme = "OwnerTokenAuth"
	case "operator":
		scheme = "OperatorTokenAuth"
	case "service":
		scheme = "ServiceTokenAuth"
	case "bearer":
		scheme = "BearerAuth"
	}
	if scheme != "" {
		operation["security"] = []map[string]interface{}{{scheme: []string{}}}
	} else {
		delete(operation, "security")
	}
	if definition.Condition != "" {
		operation["x-mounted-when"] = definition.Condition
	} else {
		delete(operation, "x-mounted-when")
	}
	if definition.Binary != "" {
		operation["responses"] = map[string]interface{}{
			"200": map[string]interface{}{"description": definition.Summary, "content": map[string]interface{}{
				definition.Binary: map[string]interface{}{"schema": map[string]string{"type": "string", "format": "binary"}},
			}},
		}
	} else {
		applyRouteModels(operation, definition)
	}
	return operation
}

func applyRouteModels(operation map[string]interface{}, definition apiroute.Definition) {
	if definition.Request != nil && definition.Method != http.MethodGet && definition.Method != http.MethodHead {
		if _, exists := operation["requestBody"]; !exists {
			if schema, ok := requestBodySchema(definition); ok {
				operation["requestBody"] = map[string]interface{}{
					"required": true,
					"content":  map[string]interface{}{"application/json": map[string]interface{}{"schema": schema}},
				}
			}
		}
	}
	if definition.Response == nil {
		return
	}
	responses, ok := operation["responses"].(map[string]interface{})
	if !ok {
		responses = map[string]interface{}{}
		operation["responses"] = responses
	}
	response, ok := responses["200"].(map[string]interface{})
	if !ok {
		response = map[string]interface{}{"description": "Successful response"}
		responses["200"] = response
	}
	content, ok := response["content"].(map[string]interface{})
	if !ok {
		content = map[string]interface{}{}
		response["content"] = content
	}
	media, ok := content["application/json"].(map[string]interface{})
	if !ok {
		media = map[string]interface{}{}
		content["application/json"] = media
	}
	if _, exists := media["schema"]; !exists || isGenericSchema(media["schema"]) {
		media["schema"] = openAPISchema(definition.Response)
	}
}

func requestBodySchema(definition apiroute.Definition) (map[string]interface{}, bool) {
	schema := openAPISchema(definition.Request)
	if reflect.TypeOf(definition.Request).Name() == "RawMessage" {
		return schema, true
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return schema, false
	}
	pathFields := make(map[string]struct{})
	for index := 0; index < len(definition.Path); index++ {
		if definition.Path[index] != '{' {
			continue
		}
		end := strings.IndexByte(definition.Path[index+1:], '}')
		if end < 0 {
			break
		}
		pathFields[definition.Path[index+1:index+1+end]] = struct{}{}
		index += end + 1
	}
	for field := range pathFields {
		delete(properties, field)
	}
	return schema, len(properties) > 0
}

func openAPISchema(model any) map[string]interface{} {
	return schemaForValue(reflect.ValueOf(model))
}

func schemaForValue(value reflect.Value) map[string]interface{} {
	if !value.IsValid() {
		return map[string]interface{}{"type": "object"}
	}
	if value.Type().Name() == "RawMessage" || (value.Type().PkgPath() == "time" && value.Type().Name() == "Time") {
		return schemaForType(value.Type())
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return map[string]interface{}{"type": "object", "nullable": true}
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.String:
		if value.String() == "string" || value.String() == "integer" || value.String() == "number" || value.String() == "boolean" || value.String() == "array" || value.String() == "object" {
			return map[string]interface{}{"type": value.String()}
		}
		return map[string]interface{}{"type": "string"}
	case reflect.Slice, reflect.Array:
		items := map[string]interface{}{"type": "object"}
		if value.Len() > 0 {
			items = schemaForValue(value.Index(0))
		} else if value.Type().Elem().Kind() != reflect.Interface {
			items = schemaForType(value.Type().Elem())
		}
		return map[string]interface{}{"type": "array", "items": items}
	case reflect.Map:
		if value.Len() == 0 {
			return map[string]interface{}{"type": "object", "additionalProperties": schemaForType(value.Type().Elem())}
		}
		properties := make(map[string]interface{}, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			properties[iter.Key().String()] = schemaForValue(iter.Value())
		}
		return map[string]interface{}{"type": "object", "properties": properties}
	case reflect.Struct:
		properties := make(map[string]interface{}, value.NumField())
		var required []string
		for index := 0; index < value.NumField(); index++ {
			field := value.Type().Field(index)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			if field.Anonymous && name == field.Name {
				embedded := schemaForType(field.Type)
				if embeddedProperties, ok := embedded["properties"].(map[string]interface{}); ok {
					for property, schema := range embeddedProperties {
						properties[property] = schema
					}
				}
				if embeddedRequired, ok := embedded["required"].([]string); ok {
					required = append(required, embeddedRequired...)
				}
				continue
			}
			fieldValue := value.Field(index)
			if field.Type.Kind() == reflect.Interface && !fieldValue.IsNil() {
				properties[name] = schemaForValue(fieldValue)
			} else {
				properties[name] = schemaForType(field.Type)
			}
			if !strings.Contains(field.Tag.Get("json"), "omitempty") {
				required = append(required, name)
			}
		}
		schema := map[string]interface{}{"type": "object", "properties": properties}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	default:
		return map[string]interface{}{"type": "object"}
	}
}

func schemaForType(typ reflect.Type) map[string]interface{} {
	nullable := false
	for typ.Kind() == reflect.Pointer {
		nullable = true
		typ = typ.Elem()
	}
	if typ.Name() == "RawMessage" {
		return withNullable(map[string]interface{}{"type": "object"}, nullable)
	}
	if typ.PkgPath() == "time" && typ.Name() == "Time" {
		return withNullable(map[string]interface{}{"type": "string", "format": "date-time"}, nullable)
	}
	switch typ.Kind() {
	case reflect.Bool:
		return withNullable(map[string]interface{}{"type": "boolean"}, nullable)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return withNullable(map[string]interface{}{"type": "integer"}, nullable)
	case reflect.Float32, reflect.Float64:
		return withNullable(map[string]interface{}{"type": "number"}, nullable)
	case reflect.String:
		return withNullable(map[string]interface{}{"type": "string"}, nullable)
	case reflect.Slice, reflect.Array:
		return withNullable(map[string]interface{}{"type": "array", "items": schemaForType(typ.Elem())}, nullable)
	case reflect.Map:
		return withNullable(map[string]interface{}{"type": "object", "additionalProperties": schemaForType(typ.Elem())}, nullable)
	case reflect.Struct:
		return withNullable(schemaForValue(reflect.New(typ).Elem()), nullable)
	default:
		return withNullable(map[string]interface{}{"type": "object"}, nullable)
	}
}

func withNullable(schema map[string]interface{}, nullable bool) map[string]interface{} {
	if nullable {
		schema["nullable"] = true
	}
	return schema
}

func isGenericSchema(value any) bool {
	schema, ok := value.(map[string]interface{})
	return !ok || (schema["type"] == "object" && schema["properties"] == nil)
}

func cloneOperation(operation map[string]interface{}) map[string]interface{} {
	data, err := json.Marshal(operation)
	if err != nil {
		return map[string]interface{}{}
	}
	var clone map[string]interface{}
	if err := json.Unmarshal(data, &clone); err != nil {
		return map[string]interface{}{}
	}
	return clone
}

func aliasOperationID(operationID, path string) string {
	var suffix strings.Builder
	upper := true
	for _, character := range path {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			if upper && character >= 'a' && character <= 'z' {
				character -= 'a' - 'A'
			}
			suffix.WriteRune(character)
			upper = false
		} else {
			upper = true
		}
	}
	return operationID + "Alias" + suffix.String()
}

func isRegistryAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/mcp" || path == "/metrics" ||
		strings.HasPrefix(path, "/metrics/") || path == "/server/metrics" || strings.HasPrefix(path, "/server/metrics/")
}

// GenerateSpec adapts the embedded generated superset to the current request.
func GenerateSpec(appConfig *config.AppConfig, r *http.Request) string {
	return generateEmbeddedSpec(appConfig, r, false)
}

// GenerateMCPToolSpec returns operation metadata for MCP's internal tool
// catalog, including operations whose visibility depends on caller identity.
func GenerateMCPToolSpec(appConfig *config.AppConfig, r *http.Request) string {
	return generateEmbeddedSpec(appConfig, r, true)
}

func generateEmbeddedSpec(appConfig *config.AppConfig, r *http.Request, includeUnlisted bool) string {
	base, err := generatedOpenAPIFS.ReadFile("openapi_generated.json")
	if err != nil {
		return "{}"
	}
	var document map[string]interface{}
	if json.Unmarshal(base, &document) != nil {
		return "{}"
	}
	paths, ok := document["paths"].(map[string]interface{})
	if !ok {
		return "{}"
	}
	apiBase := "/api/v1"
	if appConfig != nil {
		apiBase = appConfig.APIBasePath()
	}
	for path, rawOperations := range paths {
		operations, ok := rawOperations.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.HasPrefix(path, "/api/v1/") {
			delete(paths, path)
			path = apiBase + strings.TrimPrefix(path, "/api/v1")
			paths[path] = operations
		}
		for method, rawOperation := range operations {
			operation, ok := rawOperation.(map[string]interface{})
			if !ok {
				continue
			}
			condition, _ := operation["x-mounted-when"].(string)
			if condition != "" && !routeConditionEnabled(condition, appConfig) {
				delete(operations, method)
				continue
			}
			delete(operation, "x-mounted-when")
			delete(operation, "x-listed")
			if !includeUnlisted {
				delete(operation, "x-admin-only")
			}
		}
		if len(operations) == 0 {
			delete(paths, path)
		}
	}
	servers, ok := document["servers"].([]interface{})
	if ok && len(servers) > 0 {
		if server, ok := servers[0].(map[string]interface{}); ok {
			server["url"] = urlvar.BuildURL(r, "")
		}
	}
	output, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(output)
}

func routeConditionEnabled(condition string, appConfig *config.AppConfig) bool {
	switch condition {
	case "debug":
		return mode.IsDebugEnabled() || (appConfig != nil && appConfig.IsDebugMode())
	case "server.metrics.enabled":
		return appConfig != nil && appConfig.Server.Metrics.Enabled
	case "server.metrics.root.enabled":
		return appConfig != nil && appConfig.Server.Metrics.Enabled && appConfig.Server.Metrics.Root.Enabled
	default:
		return false
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
