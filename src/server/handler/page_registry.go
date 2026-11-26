// SPDX-License-Identifier: MIT
// AI.md PART 16: registered pages drive the help and preferences hubs.
package handler

import (
	"sort"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/config"
)

type hubPage struct {
	Path           string
	TitleKey       string
	DescriptionKey string
	Hub            string
	Order          int
	Enabled        func(*config.AppConfig) bool
}

func alwaysEnabled(*config.AppConfig) bool { return true }

var registeredHubPages = []hubPage{
	{Path: "/server/preferences/tokens", TitleKey: "owner_tokens.title", DescriptionKey: "owner_tokens.summary", Hub: "preferences", Order: 1, Enabled: alwaysEnabled},
	{Path: "/server/preferences/sync", TitleKey: "prefs.sync", DescriptionKey: "prefs.sync_desc", Hub: "preferences", Order: 2, Enabled: alwaysEnabled},
	{Path: "/server/help/mcp", TitleKey: "help.mcp_link", DescriptionKey: "help.mcp_desc", Hub: "help", Order: 1, Enabled: alwaysEnabled},
}

func pagesForHub(hub string, appConfig *config.AppConfig) []hubPage {
	pages := make([]hubPage, 0, len(registeredHubPages))
	for _, page := range registeredHubPages {
		if page.Hub == hub && page.Enabled != nil && page.Enabled(appConfig) {
			pages = append(pages, page)
		}
	}
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].Order < pages[j].Order })
	return pages
}

func isRegisteredHubPage(path string, appConfig *config.AppConfig) bool {
	for _, page := range registeredHubPages {
		if page.Path == path && page.Enabled != nil && page.Enabled(appConfig) {
			return true
		}
	}
	return false
}

// RegisteredHubPagePaths returns the enabled preference and help pages in
// their hub order for route and rendered-page conformance checks.
func RegisteredHubPagePaths(appConfig *config.AppConfig) []string {
	pages := make([]hubPage, 0, len(registeredHubPages))
	for _, hub := range []string{"preferences", "help"} {
		pages = append(pages, pagesForHub(hub, appConfig)...)
	}
	paths := make([]string, 0, len(pages))
	for _, page := range pages {
		paths = append(paths, page.Path)
	}
	return paths
}

func pagePayload(pages []hubPage, locale string) []map[string]string {
	result := make([]map[string]string, 0, len(pages))
	for _, page := range pages {
		result = append(result, map[string]string{
			"path":        page.Path,
			"title":       i18n.Translate(locale, page.TitleKey),
			"description": i18n.Translate(locale, page.DescriptionKey),
		})
	}
	return result
}
