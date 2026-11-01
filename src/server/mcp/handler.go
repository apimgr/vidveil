// SPDX-License-Identifier: MIT
// Package mcp exposes the public OpenAPI operations through Streamable HTTP.
package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/apimgr/vidveil/src/swagger"
)

// Handler serves MCP requests through the existing API router. The router is
// called in-process so the API handlers retain their normal validation and
// authorization behavior.
type OperatorAuthorizer func(*http.Request) bool

type internalDispatchKey struct{}

// WithInternalDispatch marks the nested API request created for an MCP tool
// call. Middleware still runs, while the shared outer MCP request supplies the
// rate-limit charge and CSRF is skipped because API clients use Bearer auth or
// anonymous access rather than ambient browser cookies.
func WithInternalDispatch(ctx context.Context) context.Context {
	return context.WithValue(ctx, internalDispatchKey{}, true)
}

// IsInternalDispatch reports whether a request was dispatched from an MCP tool
// through the API router, after its outer MCP request already consumed the
// shared rate-limit allowance.
func IsInternalDispatch(ctx context.Context) bool {
	value, _ := ctx.Value(internalDispatchKey{}).(bool)
	return value
}

func Handler(cfg *config.AppConfig, api http.Handler, isOperator OperatorAuthorizer) http.Handler {
	streamable := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		server := sdk.NewServer(&sdk.Implementation{Name: "vidveil", Version: version.GetVersion()}, &sdk.ServerOptions{})
		if cfg == nil || api == nil {
			return server
		}
		operatorAuthorized := isOperator != nil && isOperator(r)
		for _, op := range operations(cfg, r, operatorAuthorized) {
			operation := op
			server.AddTool(&sdk.Tool{
				Name:        operation.name,
				Description: operation.description,
				InputSchema: operation.schema,
			}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				return callAPI(ctx, api, r, operation, req.Params.Arguments)
			})
		}
		return server
	}, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true, DisableLocalhostProtection: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg == nil || !cfg.Server.MCP.Enabled {
			http.NotFound(w, r)
			return
		}
		streamable.ServeHTTP(w, r)
	})
}

// OriginMiddleware rejects foreign browser origins while allowing the request's
// own origin and explicitly configured public origins.
func OriginMiddleware(cfg *config.AppConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !allowedOrigin(cfg, r, origin) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(cfg *config.AppConfig, r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	requestURL, err := url.Parse(urlvar.BuildURL(r, ""))
	if err == nil && strings.EqualFold(parsed.Scheme, requestURL.Scheme) && strings.EqualFold(parsed.Host, requestURL.Host) {
		return true
	}
	if cfg == nil {
		return false
	}
	for _, allowed := range cfg.Server.CORS.AllowedOrigins {
		if allowed != "" && allowed != "*" && strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

type operation struct {
	name        string
	method      string
	path        string
	description string
	schema      map[string]any
	parameters  []map[string]any
	requestBody bool
	multipart   bool
	formEncoded bool
}

func operations(cfg *config.AppConfig, r *http.Request, operatorAuthorized bool) []operation {
	var document map[string]any
	if err := json.Unmarshal([]byte(swagger.GenerateSpec(cfg, r)), &document); err != nil {
		return nil
	}
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		return nil
	}
	pathNames := make([]string, 0, len(paths))
	for path := range paths {
		pathNames = append(pathNames, path)
	}
	sort.Strings(pathNames)
	var result []operation
	for _, path := range pathNames {
		if !strings.HasPrefix(path, cfg.APIBasePath()+"/") || strings.Contains(path, "/metrics") || strings.Contains(path, "/server/preferences/tokens") {
			continue
		}
		methods, ok := paths[path].(map[string]any)
		if !ok {
			continue
		}
		methodNames := make([]string, 0, len(methods))
		for method := range methods {
			methodNames = append(methodNames, method)
		}
		sort.Strings(methodNames)
		for _, method := range methodNames {
			spec, ok := methods[method].(map[string]any)
			if !ok || (spec["x-admin-only"] == true && !operatorAuthorized) {
				continue
			}
			name, _ := spec["operationId"].(string)
			description, _ := spec["description"].(string)
			if description == "" {
				description, _ = spec["summary"].(string)
			}
			if name == "" || description == "" {
				continue
			}
			op := operation{name: name, method: strings.ToUpper(method), path: path, description: description, schema: map[string]any{"type": "object", "properties": map[string]any{}}}
			if parameters, ok := spec["parameters"].([]any); ok {
				for _, parameter := range parameters {
					if item, ok := parameter.(map[string]any); ok {
						op.parameters = append(op.parameters, item)
						if name, ok := item["name"].(string); ok {
							props := op.schema["properties"].(map[string]any)
							if schema, ok := item["schema"]; ok {
								props[name] = schema
							} else {
								props[name] = map[string]string{"type": "string"}
							}
							if item["required"] == true {
								required, _ := op.schema["required"].([]string)
								op.schema["required"] = append(required, name)
							}
						}
					}
				}
			}
			if body, ok := spec["requestBody"].(map[string]any); ok {
				if content, ok := body["content"].(map[string]any); ok {
					if media, ok := content["application/json"].(map[string]any); ok {
						if schema, ok := media["schema"].(map[string]any); ok {
							op.schema = schema
							op.requestBody = true
						}
					} else if media, ok := content["multipart/form-data"].(map[string]any); ok {
						if schema, ok := media["schema"].(map[string]any); ok {
							op.schema = multipartInputSchema(schema)
							op.requestBody = true
							op.multipart = true
						}
					} else if media, ok := content["application/x-www-form-urlencoded"].(map[string]any); ok {
						if schema, ok := media["schema"].(map[string]any); ok {
							op.schema = schema
							op.requestBody = true
							op.formEncoded = true
						}
					}
				}
			}
			result = append(result, op)
		}
	}
	return result
}

func callAPI(ctx context.Context, api http.Handler, incoming *http.Request, op operation, raw json.RawMessage) (*sdk.CallToolResult, error) {
	arguments := map[string]any{}
	if len(raw) != 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &arguments); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	apiPath := op.path
	query := url.Values{}
	for _, parameter := range op.parameters {
		name, _ := parameter["name"].(string)
		in, _ := parameter["in"].(string)
		value, exists := arguments[name]
		if !exists {
			continue
		}
		switch in {
		case "path":
			apiPath = strings.ReplaceAll(apiPath, "{"+name+"}", url.PathEscape(fmt.Sprint(value)))
		case "query":
			query.Set(name, fmt.Sprint(value))
		}
	}
	var body io.Reader
	contentType := "application/json"
	if op.requestBody {
		if op.multipart {
			var buffer bytes.Buffer
			writer := multipart.NewWriter(&buffer)
			for key, value := range arguments {
				if key == "backup" {
					encoded, ok := value.(string)
					if !ok {
						return nil, fmt.Errorf("backup must be base64 encoded")
					}
					data, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						return nil, fmt.Errorf("decode backup: %w", err)
					}
					part, err := writer.CreateFormFile(key, "backup.tar.gz")
					if err != nil {
						return nil, err
					}
					if _, err := part.Write(data); err != nil {
						return nil, err
					}
				} else if err := writer.WriteField(key, fmt.Sprint(value)); err != nil {
					return nil, err
				}
			}
			if err := writer.Close(); err != nil {
				return nil, err
			}
			body = &buffer
			contentType = writer.FormDataContentType()
		} else if op.formEncoded {
			values := url.Values{}
			for key, value := range arguments {
				values.Set(key, fmt.Sprint(value))
			}
			body = strings.NewReader(values.Encode())
			contentType = "application/x-www-form-urlencoded"
		} else {
			encoded, err := json.Marshal(arguments)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(encoded)
		}
	}
	ctx = WithInternalDispatch(ctx)
	apiRequest := httptest.NewRequestWithContext(ctx, op.method, apiPath+queryString(query), body)
	if op.requestBody {
		apiRequest.Header.Set("Content-Type", contentType)
	}
	if authorization := incoming.Header.Get("Authorization"); authorization != "" {
		apiRequest.Header.Set("Authorization", authorization)
	}
	apiRequest.Header.Set("Accept", "application/json")
	apiRequest.Header.Set("X-Request-Source", "mcp")
	apiRequest.RemoteAddr = incoming.RemoteAddr
	response := httptest.NewRecorder()
	api.ServeHTTP(response, apiRequest)
	result := &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: response.Body.String()}},
		IsError: response.Code < 200 || response.Code >= 300,
	}
	return result, nil
}

func multipartInputSchema(schema map[string]any) map[string]any {
	copySchema, err := json.Marshal(schema)
	if err != nil {
		return schema
	}
	var input map[string]any
	if json.Unmarshal(copySchema, &input) != nil {
		return schema
	}
	properties, _ := input["properties"].(map[string]any)
	if backup, ok := properties["backup"].(map[string]any); ok {
		backup["format"] = "base64"
		backup["description"] = "Backup archive encoded as base64"
	}
	return input
}

func queryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}
