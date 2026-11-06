// SPDX-License-Identifier: MIT
package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/apimgr/vidveil/src/swagger"
)

type mcpHelpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type mcpHelpDocument struct {
	Enabled bool          `json:"enabled"`
	URL     string        `json:"url"`
	Aliases []string      `json:"aliases"`
	Auth    mcpHelpAuth   `json:"auth"`
	Tools   []mcpHelpTool `json:"tools"`
	Project string        `json:"project"`
}

type mcpHelpAuth struct {
	Scheme string `json:"scheme"`
}

// HelpMCPPage renders MCP connection guidance when the endpoint is enabled.
func (h *ServerHandler) HelpMCPPage(w http.ResponseWriter, r *http.Request) {
	if !h.appConfig.Server.MCP.Enabled {
		http.NotFound(w, r)
		return
	}

	tools := mcpAnonymousTools(h.appConfig, r)
	canonical := h.appConfig.APIBasePath() + "/server/mcp"
	h.renderServerTemplate(w, r, "server-help-mcp", map[string]interface{}{
		"MCPProjectName": "vidveil",
		"MCPURL":         urlvar.BuildURL(r, canonical),
		"MCPAPIURL":      urlvar.BuildURL(r, "/api/mcp"),
		"MCPAliasURL":    urlvar.BuildURL(r, "/mcp"),
		"MCPTools":       tools,
		"MCPProject":     h.appConfig.Server.Pages.Help.MCP,
	})
}

// APIHelpMCP returns connection metadata and the live anonymous tool list.
func (h *ServerHandler) APIHelpMCP(w http.ResponseWriter, r *http.Request) {
	if !h.appConfig.Server.MCP.Enabled {
		http.NotFound(w, r)
		return
	}

	canonical := h.appConfig.APIBasePath() + "/server/mcp"
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": mcpHelpDocument{
			Enabled: true,
			URL:     urlvar.BuildURL(r, canonical),
			Aliases: []string{urlvar.BuildURL(r, "/api/mcp"), urlvar.BuildURL(r, "/mcp")},
			Auth:    mcpHelpAuth{Scheme: "Bearer"},
			Tools:   mcpAnonymousTools(h.appConfig, r),
			Project: h.appConfig.Server.Pages.Help.MCP,
		},
	})
}

// mcpAnonymousTools derives the unauthenticated MCP list from the live
// OpenAPI document so help cannot drift from the registry used by tools/list.
func mcpAnonymousTools(appConfig *config.AppConfig, r *http.Request) []mcpHelpTool {
	if appConfig == nil {
		return []mcpHelpTool{}
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal([]byte(swagger.GenerateSpec(appConfig, r)), &document); err != nil {
		return []mcpHelpTool{}
	}

	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	var tools []mcpHelpTool
	for path, operations := range document.Paths {
		if !strings.HasPrefix(path, appConfig.APIBasePath()+"/") {
			continue
		}
		for method, raw := range operations {
			if !methods[method] {
				continue
			}
			var operation struct {
				ID          string `json:"operationId"`
				Description string `json:"description"`
				Security    []any  `json:"security"`
			}
			if json.Unmarshal(raw, &operation) != nil || operation.ID == "" || len(operation.Security) > 0 {
				continue
			}
			tools = append(tools, mcpHelpTool{Name: operation.ID, Description: operation.Description})
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}
