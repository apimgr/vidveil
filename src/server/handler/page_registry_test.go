// SPDX-License-Identifier: MIT
package handler

import (
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
	for _, page := range pagesForHub("preferences", config.DefaultAppConfig()) {
		if got := strings.Count(preferences.Body.String(), `href="`+page.Path+`"`); got != 1 {
			t.Errorf("preferences hub link count for %q = %d, want 1", page.Path, got)
		}
	}

	server := newServerHandler(t)
	help := httptest.NewRecorder()
	helpRequest := httptest.NewRequest(http.MethodGet, "/server/help", nil)
	helpRequest.Header.Set("User-Agent", "Mozilla/5.0")
	server.HelpPage(help, helpRequest)
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
		if payload[i]["path"] != page.Path || payload[i]["title"] == "" || payload[i]["description"] == "" {
			t.Errorf("API help page payload[%d] = %#v", i, payload[i])
		}
	}
	apiHelp := httptest.NewRecorder()
	apiRequest := httptest.NewRequest(http.MethodGet, "/api/v1/server/help", nil)
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
}
