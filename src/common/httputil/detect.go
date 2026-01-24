// SPDX-License-Identifier: MIT
// Package httputil provides HTTP client detection and response utilities
// See AI.md PART 14
package httputil

import (
	"net/http"
	"strings"
)

// IsOurCliClient detects our own CLI client
// Our CLI is INTERACTIVE (TUI/GUI) - receives JSON, renders itself
func IsOurCliClient(r *http.Request, projectName string) bool {
	ua := r.Header.Get("User-Agent")
	return strings.HasPrefix(ua, projectName+"-cli/")
}

// IsTextBrowser detects text-mode browsers (lynx, w3m, links, etc.)
// Text browsers are INTERACTIVE but do NOT support JavaScript
// They receive no-JS HTML alternative (server-rendered, standard form POST)
func IsTextBrowser(r *http.Request) bool {
	ua := strings.ToLower(r.Header.Get("User-Agent"))

	// Text browsers - INTERACTIVE, NO JavaScript support
	// lynx: Lynx - classic text browser
	// w3m: w3m - text browser with table support
	// links: Links - text browser (note: space after for matching)
	// elinks: ELinks - enhanced links
	// browsh: Browsh - modern text browser
	// carbonyl: Carbonyl - Chromium in terminal
	// netsurf: NetSurf - lightweight browser (limited JS)
	textBrowsers := []string{
		"lynx/",
		"w3m/",
		"links ",
		"links/",
		"elinks/",
		"browsh/",
		"carbonyl/",
		"netsurf",
	}
	for _, browser := range textBrowsers {
		if strings.Contains(ua, browser) {
			return true
		}
	}
	return false
}

// IsHttpTool detects HTTP tools (curl, wget, httpie, etc.)
// HTTP tools are NON-INTERACTIVE - they just dump output
func IsHttpTool(r *http.Request) bool {
	ua := strings.ToLower(r.Header.Get("User-Agent"))

	httpTools := []string{
		"curl/", "wget/", "httpie/",
		"libcurl/", "python-requests/",
		"go-http-client/", "axios/", "node-fetch/",
	}
	for _, tool := range httpTools {
		if strings.Contains(ua, tool) {
			return true
		}
	}

	// No User-Agent = likely HTTP tool (non-interactive)
	if ua == "" {
		return true
	}

	return false
}

// IsNonInteractiveClient detects clients that need pre-formatted text
// ONLY HTTP tools are non-interactive
// Our CLI client and text browsers are INTERACTIVE (handle their own rendering)
func IsNonInteractiveClient(r *http.Request, projectName string) bool {
	// Our CLI client is INTERACTIVE - receives JSON
	if IsOurCliClient(r, projectName) {
		return false
	}

	// Text browsers are INTERACTIVE - receive no-JS HTML, render it themselves
	if IsTextBrowser(r) {
		return false
	}

	// HTTP tools are NON-INTERACTIVE - need pre-formatted text
	if IsHttpTool(r) {
		return true
	}

	return false
}
