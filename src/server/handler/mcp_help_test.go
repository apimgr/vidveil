// SPDX-License-Identifier: MIT
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelpMCPPageRendersResolvedConnectionGuide(t *testing.T) {
	t.Setenv("DOMAIN", "vidveil.example")
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "https://vidveil.example/server/help/mcp", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rr := httptest.NewRecorder()
	h.HelpMCPPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("HelpMCPPage status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	for _, want := range []string{
		`id="overview"`, `id="endpoint"`, `id="authentication"`, `id="connect"`,
		`id="tools"`, `id="limits"`, `id="troubleshooting"`, `id="project"`,
		`data-copy="https://vidveil.example/api/v1/server/mcp"`,
		`https://vidveil.example/api/v1/server/mcp`,
		`https://vidveil.example/api/mcp`, `https://vidveil.example/mcp`,
		`searchVideos`, `claude mcp add --transport http vidveil`, "$MCP_TOKEN",
		`MCP-Protocol-Version: 2026-07-28`, `Mcp-Method: initialize`,
		`io.modelcontextprotocol/protocolVersion`, `2026-07-28`,
	} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Errorf("MCP help page does not contain %q", want)
		}
	}
}

func TestHelpMCPPageAndAPIAlwaysAvailable(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)

	page := httptest.NewRecorder()
	pageRequest := httptest.NewRequest(http.MethodGet, "/server/help/mcp", nil)
	pageRequest.Header.Set("User-Agent", "Mozilla/5.0")
	h.HelpMCPPage(page, pageRequest)
	if page.Code != http.StatusOK {
		t.Errorf("HelpMCPPage status = %d, want %d", page.Code, http.StatusOK)
	}

	api := httptest.NewRecorder()
	h.APIHelpMCP(api, httptest.NewRequest(http.MethodGet, "/api/v1/server/help/mcp", nil))
	if api.Code != http.StatusOK {
		t.Errorf("APIHelpMCP status = %d, want %d", api.Code, http.StatusOK)
	}

	help := httptest.NewRecorder()
	helpRequest := httptest.NewRequest(http.MethodGet, "/server/help", nil)
	helpRequest.Header.Set("User-Agent", "Mozilla/5.0")
	h.HelpPage(help, helpRequest)
	if help.Code != http.StatusOK {
		t.Fatalf("HelpPage status = %d, want %d", help.Code, http.StatusOK)
	}
	if !strings.Contains(help.Body.String(), "/server/help/mcp") {
		t.Error("HelpPage should expose the MCP guide link because MCP is always available")
	}
}

func TestAPIHelpMCPReturnsFixedEnvelopeAndPublicOperations(t *testing.T) {
	t.Setenv("DOMAIN", "vidveil.example")
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "https://vidveil.example/api/v1/server/help/mcp", nil)
	rr := httptest.NewRecorder()
	h.APIHelpMCP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("APIHelpMCP status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Enabled bool     `json:"enabled"`
			URL     string   `json:"url"`
			Aliases []string `json:"aliases"`
			Auth    struct {
				Scheme string `json:"scheme"`
			} `json:"auth"`
			Tools   []mcpHelpTool `json:"tools"`
			Project string        `json:"project"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || !response.Data.Enabled {
		t.Errorf("response status fields = ok:%t enabled:%t, want both true", response.OK, response.Data.Enabled)
	}
	if response.Data.URL != "https://vidveil.example/api/v1/server/mcp" {
		t.Errorf("url = %q", response.Data.URL)
	}
	if len(response.Data.Aliases) != 2 || response.Data.Aliases[0] != "https://vidveil.example/api/mcp" || response.Data.Aliases[1] != "https://vidveil.example/mcp" {
		t.Errorf("aliases = %#v", response.Data.Aliases)
	}
	if response.Data.Auth.Scheme != "Bearer" {
		t.Errorf("auth scheme = %q, want Bearer", response.Data.Auth.Scheme)
	}
	if !strings.Contains(response.Data.Project, "searchVideos") || !strings.Contains(response.Data.Project, "!ph") {
		t.Errorf("project help = %q, want VidVeil-specific MCP guidance", response.Data.Project)
	}
	foundSearch := false
	foundMCPHelp := false
	for _, tool := range response.Data.Tools {
		if tool.Name == "searchVideos" && tool.Description != "" {
			foundSearch = true
		}
		if tool.Name == "getServerHelpMCP" {
			foundMCPHelp = true
		}
		if strings.Contains(tool.Name, "Token") || strings.Contains(tool.Name, "token") || strings.Contains(tool.Name, "Preferences") {
			t.Errorf("excluded operation %q appeared in MCP help metadata", tool.Name)
		}
	}
	if !foundSearch {
		t.Errorf("public API operations = %#v, missing searchVideos", response.Data.Tools)
	}
	if !foundMCPHelp {
		t.Errorf("public API operations = %#v, missing getServerHelpMCP", response.Data.Tools)
	}
}
