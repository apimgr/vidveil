// SPDX-License-Identifier: MIT
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestPageRegistryDrivesHubLinksAndHelpPayload(t *testing.T) {
	setRealTemplatesFS(t)
	search := newRenderTestHandler()
	preferences := httptest.NewRecorder()
	preferencesRequest := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	preferencesRequest.Header.Set("User-Agent", "Mozilla/5.0")
	search.PreferencesPage(preferences, preferencesRequest)
	if strings.Contains(preferences.Body.String(), `class="theme-toggle-form"`) {
		t.Error("preferences page must omit the separate header theme form")
	}
	for _, page := range pagesForHub("preferences", config.DefaultAppConfig()) {
		if got := strings.Count(preferences.Body.String(), `href="`+page.Path+`"`); got != 1 {
			t.Errorf("preferences hub link count for %q = %d, want 1", page.Path, got)
		}
	}

	server := newServerHandler(t)
	server.appConfig.Server.Pages.Help.Content = config.LocalizedContentBody{
		Values: map[string]string{"fr": "Guide de VidVeil", "default": "Default guide"},
		Order:  []string{"fr", "default"},
	}
	help := httptest.NewRecorder()
	helpRequest := httptest.NewRequest(http.MethodGet, "/server/help?lang=fr", nil)
	helpRequest.Header.Set("User-Agent", "Mozilla/5.0")
	server.HelpPage(help, helpRequest)
	if !strings.Contains(help.Body.String(), "Guide de VidVeil") {
		t.Error("help HTML did not render localized Markdown content")
	}
	if !strings.Contains(help.Body.String(), `class="theme-toggle-form"`) {
		t.Error("help page must retain the header theme form")
	}
	pages := pagesForHub("help", config.DefaultAppConfig())
	for _, page := range pages {
		if got := strings.Count(help.Body.String(), `href="`+page.Path+`"`); got != 1 {
			t.Errorf("help hub link count for %q = %d, want 1", page.Path, got)
		}
	}
	payload := pagePayload(pages, "en")
	if len(payload) != len(pages) {
		t.Fatalf("API help page payload has %d pages, want %d", len(payload), len(pages))
	}
	for i, page := range pages {
		if payload[i]["path"] != page.Path || payload[i]["title"] == "" || len(payload[i]) != 2 {
			t.Errorf("API help page payload[%d] = %#v", i, payload[i])
		}
	}
	apiHelp := httptest.NewRecorder()
	apiRequest := httptest.NewRequest(http.MethodGet, "/api/v1/server/help?lang=fr", nil)
	apiRequest.Header.Set("Accept", "application/json")
	server.APIHelp(apiHelp, apiRequest)
	if apiHelp.Code != http.StatusOK || !strings.Contains(apiHelp.Body.String(), `"pages"`) {
		t.Fatalf("API help omitted registered pages: %d %s", apiHelp.Code, apiHelp.Body.String())
	}
	for _, page := range pages {
		if !strings.Contains(apiHelp.Body.String(), `"`+page.Path+`"`) {
			t.Errorf("API help omitted registered page %q", page.Path)
		}
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(apiHelp.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["format"] != "markdown" || envelope.Data["title"] == "" || envelope.Data["content"] != "Guide de VidVeil" {
		t.Errorf("API help data = %#v", envelope.Data)
	}
	apiPages, ok := envelope.Data["pages"].([]any)
	if !ok || len(apiPages) != len(pages) {
		t.Fatalf("API help pages = %#v, want %d entries", envelope.Data["pages"], len(pages))
	}
	for _, value := range apiPages {
		page, ok := value.(map[string]any)
		if !ok || len(page) != 2 || page["title"] == nil || page["path"] == nil {
			t.Errorf("API help page fields = %#v, want exactly title and path", value)
		}
	}
	fallback := httptest.NewRecorder()
	server.appConfig.Server.Pages.Help.Content = config.LocalizedContentBody{
		Values: map[string]string{"fr": "Guide de VidVeil"},
		Order:  []string{"fr"},
	}
	fallbackRequest := httptest.NewRequest(http.MethodGet, "/api/v1/server/help?lang=es", nil)
	fallbackRequest.Header.Set("Accept", "application/json")
	server.APIHelp(fallback, fallbackRequest)
	if !strings.Contains(fallback.Body.String(), config.DefaultHelpContent) {
		t.Errorf("API help missing-locale fallback did not use built-in content: %s", fallback.Body.String())
	}
}
