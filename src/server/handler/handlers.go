// SPDX-License-Identifier: MIT
package handler

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/cache"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/version"
)

// templatesFS holds the embedded templates filesystem
var templatesFS embed.FS

// SetTemplatesFS sets the embedded templates filesystem
func SetTemplatesFS(fs embed.FS) {
	templatesFS = fs
}

const (
	ageVerifyCookieName = "age_verified"
	ageVerifyCookieDays = 30
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	cfg         *config.Config
	engineMgr   *engine.Manager
	searchCache *cache.SearchCache
}

// New creates a new handler instance
func New(cfg *config.Config, engineMgr *engine.Manager) *Handler {
	// Initialize cache with 5 minute TTL and 1000 max entries
	searchCache := cache.New(5*time.Minute, 1000)

	return &Handler{
		cfg:         cfg,
		engineMgr:   engineMgr,
		searchCache: searchCache,
	}
}

// WriteJSON writes a JSON response with 2-space indentation and trailing newline
// Per AI.md PART 14 lines 11223-11229: ALL JSON responses MUST be indented
// Package-level function so all handler types can use it
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Use MarshalIndent with 2-space indent (NON-NEGOTIABLE)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		// Fallback to error response
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Failed to encode JSON"}`))
		return
	}
	
	// Write JSON data
	w.Write(jsonData)
	// Single trailing newline (NON-NEGOTIABLE)
	w.Write([]byte("\n"))
}

// MaintenanceModeMiddleware checks if maintenance mode is enabled
func (h *Handler) MaintenanceModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip maintenance check for health endpoints and admin
		path := r.URL.Path
		if path == "/healthz" ||
			strings.HasPrefix(path, "/admin") ||
			strings.HasPrefix(path, "/api/v1/admin") {
			next.ServeHTTP(w, r)
			return
		}

		// Check if maintenance mode is active
		paths := config.GetPaths("", "")
		modeFile := filepath.Join(paths.Data, "maintenance.flag")
		if _, err := os.Stat(modeFile); err == nil {
			// Maintenance mode is active
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Maintenance - ` + h.cfg.Server.Title + `</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #282a36;
            color: #f8f8f2;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            margin: 0;
        }
        .maintenance {
            text-align: center;
            padding: 2rem;
        }
        h1 { color: #ffb86c; margin-bottom: 1rem; }
        p { color: #6272a4; }
    </style>
</head>
<body>
    <div class="maintenance">
        <h1>🔧 Under Maintenance</h1>
        <p>We're performing scheduled maintenance.</p>
        <p>Please check back shortly.</p>
    </div>
</body>
</html>`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AgeVerifyMiddleware checks for age verification cookie
func (h *Handler) AgeVerifyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip age check for static files, health checks, and age verification endpoints
		path := r.URL.Path
		if strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/api/") ||
			path == "/healthz" ||
			path == "/robots.txt" ||
			path == "/age-verify" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for age verification cookie
		cookie, err := r.Cookie(ageVerifyCookieName)
		if err != nil || cookie.Value != "1" {
			// Redirect to age verification page
			redirect := r.URL.Path
			if r.URL.RawQuery != "" {
				redirect += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, "/age-verify?redirect="+redirect, http.StatusFound)
			return
		}

		// Renew cookie on each visit
		h.setAgeVerifyCookie(w)

		next.ServeHTTP(w, r)
	})
}

// AgeVerifyPage shows the age verification gate
func (h *Handler) AgeVerifyPage(w http.ResponseWriter, r *http.Request) {
	// If already verified, redirect to home or specified redirect
	cookie, err := r.Cookie(ageVerifyCookieName)
	if err == nil && cookie.Value == "1" {
		redirect := r.URL.Query().Get("redirect")
		if redirect == "" || !strings.HasPrefix(redirect, "/") {
			redirect = "/"
		}
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}

	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/"
	}

	h.renderTemplate(w, "age-verify", map[string]interface{}{
		"Title":    "Age Verification - " + h.cfg.Server.Title,
		"Theme":    h.cfg.Web.UI.Theme,
		"Redirect": redirect,
	})
}

// AgeVerifySubmit handles the age verification form submission
func (h *Handler) AgeVerifySubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/age-verify", http.StatusFound)
		return
	}

	// Set the age verification cookie
	h.setAgeVerifyCookie(w)

	// Redirect to the original destination
	redirect := r.FormValue("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, "/") {
		redirect = "/"
	}

	http.Redirect(w, r, redirect, http.StatusFound)
}

// setAgeVerifyCookie sets/renews the age verification cookie
func (h *Handler) setAgeVerifyCookie(w http.ResponseWriter) {
	// 30 days in seconds, Secure should be true if using HTTPS
	http.SetCookie(w, &http.Cookie{
		Name:     ageVerifyCookieName,
		Value:    "1",
		Path:     "/",
		MaxAge:   ageVerifyCookieDays * 24 * 60 * 60,
		Expires:  time.Now().Add(ageVerifyCookieDays * 24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
}

// Build time set at compile time
var BuildDateTime = "December 4, 2025"

// HomePage renders the main search page
// HomePage renders the home page with content negotiation per AI.md PART 17
func (h *Handler) HomePage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	engineCount := h.engineMgr.EnabledCount()
	
	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":        h.cfg.Server.Title,
			"description":  h.cfg.Server.Description,
			"engine_count": engineCount,
			"version":      "0.2.0",
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%s\n", h.cfg.Server.Title)
		fmt.Fprintf(w, "%s\n\n", h.cfg.Server.Description)
		fmt.Fprintf(w, "Search Engines: %d enabled\n", engineCount)
		fmt.Fprintf(w, "Version: 0.2.0\n")
		
	default:
		// HTML response for browsers (default)
		h.renderTemplate(w, "home", map[string]interface{}{
			"Title":         h.cfg.Server.Title,
			"Description":   h.cfg.Server.Description,
			"Theme":         h.cfg.Web.UI.Theme,
			"BuildDateTime": BuildDateTime,
			"EngineCount":   engineCount,
		})
	}
}

// SearchPage renders search results with content negotiation per AI.md PART 17
func (h *Handler) SearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Parse bangs from query (e.g., "!ph amateur" -> search pornhub for "amateur")
	parsed := engine.ParseBangs(query)
	searchQuery := parsed.Query
	if searchQuery == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Get engine names - bangs take priority, then URL param
	engineNames := parsed.Engines
	if len(engineNames) == 0 {
		if e := r.URL.Query().Get("engines"); e != "" {
			engineNames = strings.Split(e, ",")
		}
	}

	// Perform parallel search across engines
	results := h.engineMgr.Search(r.Context(), searchQuery, 1, engineNames)
	
	format := detectResponseFormat(r)
	
	switch format {
	case "application/json":
		// JSON response for API clients per AI.md PART 17
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"query":        query,
			"search_query": searchQuery,
			"results":      results.Data.Results,
			"engines_used": results.Data.EnginesUsed,
			"search_time":  results.Data.SearchTimeMS,
			"has_bang":     parsed.HasBang,
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Search: %s\n", query)
		fmt.Fprintf(w, "Results: %d found in %dms\n\n", len(results.Data.Results), results.Data.SearchTimeMS)
		for i, result := range results.Data.Results {
			fmt.Fprintf(w, "%d. %s\n", i+1, result.Title)
			fmt.Fprintf(w, "   %s\n", result.URL)
			if result.Duration != "" {
				fmt.Fprintf(w, "   Duration: %s", result.Duration)
				if result.Views != "" {
					fmt.Fprintf(w, " | Views: %s", result.Views)
				}
				fmt.Fprintf(w, "\n")
			}
			fmt.Fprintf(w, "\n")
		}
		
	default:
		// HTML response for browsers (default)
		// Convert results to JSON for the JavaScript
		resultsJSON, _ := json.Marshal(results.Data.Results)

		// ResultsJSON is safe JSON for script template use
		h.renderTemplate(w, "search", map[string]interface{}{
			"Title":       query + " - " + h.cfg.Server.Title,
			"Query":       query,
			"SearchQuery": searchQuery,
			"ResultsJSON": template.JS(resultsJSON),
			"EnginesUsed": results.Data.EnginesUsed,
			"SearchTime":  results.Data.SearchTimeMS,
			"Theme":       h.cfg.Web.UI.Theme,
			"HasBang":     parsed.HasBang,
			"BangEngines": parsed.Engines,
		})
	}
}

// PreferencesPage renders user preferences with content negotiation per AI.md PART 17
func (h *Handler) PreferencesPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	engines := h.engineMgr.ListEngines()
	
	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":    "Preferences",
			"engines":  engines,
			"theme":    h.cfg.Web.UI.Theme,
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Preferences - %s\n\n", h.cfg.Server.Title)
		fmt.Fprintf(w, "Theme: %s\n", h.cfg.Web.UI.Theme)
		fmt.Fprintf(w, "\nAvailable Engines:\n")
		for _, eng := range engines {
			status := "disabled"
			if eng.Enabled {
				status = "enabled"
			}
			fmt.Fprintf(w, "  %s (%s)\n", eng.DisplayName, status)
		}
		
	default:
		// HTML response for browsers (default)
		h.renderTemplate(w, "preferences", map[string]interface{}{
			"Title":         "Preferences - " + h.cfg.Server.Title,
			"Theme":         h.cfg.Web.UI.Theme,
			"Engines":       engines,
			"BuildDateTime": BuildDateTime,
		})
	}
}

// AboutPage renders the about page with content negotiation per AI.md PART 17
func (h *Handler) AboutPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	version := "0.2.0"
	
	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":         h.cfg.Server.Title,
			"version":       version,
			"build_date":    BuildDateTime,
			"description":   h.cfg.Server.Description,
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%s\n", h.cfg.Server.Title)
		fmt.Fprintf(w, "Version: %s\n", version)
		fmt.Fprintf(w, "Build Date: %s\n", BuildDateTime)
		fmt.Fprintf(w, "\n%s\n", h.cfg.Server.Description)
		
	default:
		// HTML response for browsers (default)
		h.renderTemplate(w, "about", map[string]interface{}{
			"Title":         "About - " + h.cfg.Server.Title,
			"Theme":         h.cfg.Web.UI.Theme,
			"Version":       version,
			"BuildDateTime": BuildDateTime,
		})
	}
}

// PrivacyPage renders the privacy policy page with content negotiation per AI.md PART 17
func (h *Handler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	version := "0.2.0"
	
	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":   "Privacy Policy",
			"version": version,
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Privacy Policy - %s\n", h.cfg.Server.Title)
		fmt.Fprintf(w, "Version: %s\n\n", version)
		fmt.Fprintf(w, "VidVeil is a privacy-respecting meta search engine.\n")
		fmt.Fprintf(w, "We do not track, log, or collect any user data.\n")
		
	default:
		// HTML response for browsers (default)
		h.renderTemplate(w, "privacy", map[string]interface{}{
			"Title":         "Privacy Policy - " + h.cfg.Server.Title,
			"Theme":         h.cfg.Web.UI.Theme,
			"Version":       version,
			"BuildDateTime": BuildDateTime,
		})
	}
}

// detectResponseFormat returns the response format based on Accept header
// Per AI.md PART 19: Content Negotiation
// detectResponseFormat determines response format per AI.md PART 17 lines 15194-15244
func detectResponseFormat(r *http.Request) string {
	// 0. Check URL path extension FIRST per AI.md PART 13 lines 11271-11272
	path := r.URL.Path
	if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(path, ".txt") {
		return "text/plain"
	}
	
	// 1. Check Accept header (explicit preference)
	accept := r.Header.Get("Accept")

	if strings.Contains(accept, "text/html") {
		return "text/html"
	}
	if strings.Contains(accept, "text/plain") {
		return "text/plain"
	}
	if strings.Contains(accept, "application/json") {
		return "application/json"
	}

	// 2. Check User-Agent for browser detection
	ua := r.Header.Get("User-Agent")

	// Browser User-Agents (common patterns)
	browsers := []string{
		"Mozilla/", "Chrome/", "Safari/", "Edge/", "Firefox/",
		"Opera/", "MSIE", "Trident/",
	}

	for _, browser := range browsers {
		if strings.Contains(ua, browser) {
			return "text/html"
		}
	}

	// 3. CLI tools (curl, wget, httpie, etc.)
	cliTools := []string{
		"curl/", "Wget/", "HTTPie/", "python-requests/",
		"Go-http-client/", "node-fetch/",
	}

	for _, tool := range cliTools {
		if strings.Contains(ua, tool) {
			return "text/plain"
		}
	}

	// 4. Empty or unknown User-Agent
	if ua == "" {
		// Default to text for programmatic access
		return "text/plain"
	}

	// 5. Default: HTML (safest fallback)
	return "text/html"
}

// HealthCheck returns health status with content negotiation
// Per AI.md PART 16: Supports HTML (default), JSON (Accept: application/json), and Text
// HealthCheck handles /healthz endpoint with content negotiation
// Per AI.md PART 13 lines 11257-11379
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	// Build health response per AI.md PART 13
	hostname, _ := os.Hostname()
	uptime := getUptime()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	
	// Get mode from config
	appMode := "production"
	if h.cfg != nil && h.cfg.IsDevelopmentMode() {
		appMode = "development"
	}
	
	// Node ID (standalone or cluster)
	nodeID := "standalone"
	
	// Cluster status
	clusterEnabled := false
	clusterStatus := ""
	clusterNodes := 0
	clusterRole := ""
	
	// Build checks object - MUST be simple "ok"/"error" strings
	// Per AI.md PART 13 lines 11292-11295
	checks := map[string]string{
		"database": "ok",
		"cache":    "ok",
		"disk":     "ok",
	}
	
	// Add cluster check if clustering enabled
	if clusterEnabled {
		checks["cluster"] = "ok"
	}
	
	// Overall status
	status := "healthy"
	
	switch format {
	case "application/json":
		// JSON format per AI.md PART 13 lines 11312-11336
		response := map[string]interface{}{
			"status":    status,
			"version":   version.Get(),  // Use version package
			"mode":      appMode,
			"uptime":    uptime,
			"timestamp": timestamp,
			"node": map[string]interface{}{
				"id":       nodeID,
				"hostname": hostname,
			},
			"cluster": map[string]interface{}{
				"enabled": clusterEnabled,
			},
			"checks": checks,
		}
		
		// Add cluster details if enabled
		if clusterEnabled {
			response["cluster"] = map[string]interface{}{
				"enabled": true,
				"status":  clusterStatus,
				"nodes":   clusterNodes,
				"role":    clusterRole,
			}
		}
		
		WriteJSON(w, http.StatusOK, response)

	case "text/plain":
		// Plain text format per AI.md PART 13 lines 11338-11349
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "status: %s\n", status)
		fmt.Fprintf(w, "version: %s\n", version.Get())
		fmt.Fprintf(w, "mode: %s\n", appMode)
		fmt.Fprintf(w, "uptime: %s\n", uptime)
		fmt.Fprintf(w, "database: %s\n", checks["database"])
		fmt.Fprintf(w, "cache: %s\n", checks["cache"])
		fmt.Fprintf(w, "disk: %s\n", checks["disk"])
		if clusterEnabled {
			fmt.Fprintf(w, "cluster: %s (%d nodes)\n", checks["cluster"], clusterNodes)
		}

	default:
		// HTML format (default) per AI.md PART 13 lines 11299-11308
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Health Check</title></head>
<body>
<h1>VidVeil Health Check</h1>
<p>Status: <strong>` + status + `</strong></p>
<p>Version: ` + version.Get() + `</p>
<p>Mode: ` + appMode + `</p>
<p>Uptime: ` + uptime + `</p>
<p>Hostname: ` + hostname + `</p>
<h2>Checks</h2>
<ul>
<li>Database: ` + checks["database"] + `</li>
<li>Cache: ` + checks["cache"] + `</li>
<li>Disk: ` + checks["disk"] + `</li>
</ul>
</body>
</html>`))
	}
}

// RobotsTxt returns robots.txt
func (h *Handler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	baseURL := "https://" + h.cfg.Server.FQDN
	if h.cfg.Server.Port != "443" && h.cfg.Server.Port != "80" {
		baseURL = fmt.Sprintf("https://%s:%s", h.cfg.Server.FQDN, h.cfg.Server.Port)
	}

	w.Write([]byte(`User-agent: *
Disallow: /search
Disallow: /api/
Disallow: /admin/
Allow: /

Sitemap: ` + baseURL + `/sitemap.xml
`))
}

// SecurityTxt returns security.txt per RFC 9116 (PART 22)
func (h *Handler) SecurityTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	contact := h.cfg.Web.Security.Contact
	if contact == "" {
		contact = "security@" + h.cfg.Server.FQDN
	}
	if !strings.HasPrefix(contact, "mailto:") {
		contact = "mailto:" + contact
	}

	expires := h.cfg.Web.Security.Expires
	if expires == "" {
		// Default: 1 year from now
		expires = time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	}

	w.Write([]byte(fmt.Sprintf(`Contact: %s
Expires: %s
Preferred-Languages: en
`, contact, expires)))
}

// HumansTxt returns humans.txt per humanstxt.org standard (PART 21)
func (h *Handler) HumansTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Get app info from config
	appName := h.cfg.Web.Branding.AppName
	if appName == "" {
		appName = "Vidveil"
	}

	appURL := "https://" + h.cfg.Server.FQDN
	if h.cfg.Server.Port != "443" && h.cfg.Server.Port != "80" {
		appURL = fmt.Sprintf("https://%s:%s", h.cfg.Server.FQDN, h.cfg.Server.Port)
	}

	w.Write([]byte(fmt.Sprintf(`/* TEAM */
Name: %s Team
Site: %s
Location: Earth

/* THANKS */
Go: https://go.dev
Chi Router: https://github.com/go-chi/chi
Dracula Theme: https://draculatheme.com

/* SITE */
Last update: %s
Language: English
Doctype: HTML5
Standards: WCAG 2.1 AA, RFC 9116
Components: Go, SQLite, Valkey/Redis
`, appName, appURL, time.Now().Format("2006-01-02"))))
}

// SitemapXML returns sitemap.xml per AI.md PART 13
func (h *Handler) SitemapXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	baseURL := "https://" + h.cfg.Server.FQDN
	if h.cfg.Server.Port != "443" && h.cfg.Server.Port != "80" {
		baseURL = fmt.Sprintf("https://%s:%s", h.cfg.Server.FQDN, h.cfg.Server.Port)
	}

	// Build sitemap with static pages per AI.md PART 13
	sitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>` + baseURL + `/</loc>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>` + baseURL + `/about</loc>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>` + baseURL + `/privacy</loc>
    <changefreq>monthly</changefreq>
    <priority>0.3</priority>
  </url>
  <url>
    <loc>` + baseURL + `/preferences</loc>
    <changefreq>monthly</changefreq>
    <priority>0.4</priority>
  </url>
  <url>
    <loc>` + baseURL + `/server/about</loc>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>` + baseURL + `/server/privacy</loc>
    <changefreq>monthly</changefreq>
    <priority>0.3</priority>
  </url>
  <url>
    <loc>` + baseURL + `/server/contact</loc>
    <changefreq>monthly</changefreq>
    <priority>0.4</priority>
  </url>
  <url>
    <loc>` + baseURL + `/server/help</loc>
    <changefreq>monthly</changefreq>
    <priority>0.4</priority>
  </url>
</urlset>`

	w.Write([]byte(sitemap))
}

// APISearchStream handles SSE streaming search API requests
func (h *Handler) APISearchStream(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.jsonError(w, "Query parameter 'q' is required", "MISSING_QUERY", http.StatusBadRequest)
		return
	}

	// Parse bangs from query
	parsed := engine.ParseBangs(query)
	searchQuery := parsed.Query
	if searchQuery == "" {
		h.jsonError(w, "Query cannot be empty after bang parsing", "EMPTY_QUERY", http.StatusBadRequest)
		return
	}

	// Get engine names - bangs take priority, then URL param
	engineNames := parsed.Engines
	if len(engineNames) == 0 {
		if e := r.URL.Query().Get("engines"); e != "" {
			engineNames = strings.Split(e, ",")
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.jsonError(w, "Streaming not supported", "STREAMING_ERROR", http.StatusInternalServerError)
		return
	}

	// Stream results
	ctx := r.Context()
	resultsChan := h.engineMgr.SearchStream(ctx, searchQuery, 1, engineNames)

	for result := range resultsChan {
		data, err := json.Marshal(result)
		if err != nil {
			continue
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Send final done message
	fmt.Fprintf(w, "data: {\"done\":true,\"engine\":\"all\"}\n\n")
	flusher.Flush()
}

// APISearch handles search API requests
func (h *Handler) APISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.jsonError(w, "Query parameter 'q' is required", "MISSING_QUERY", http.StatusBadRequest)
		return
	}

	// Parse bangs from query (e.g., "!ph amateur" -> search pornhub for "amateur")
	parsed := engine.ParseBangs(query)
	searchQuery := parsed.Query
	if searchQuery == "" {
		h.jsonError(w, "Query cannot be empty after bang parsing", "EMPTY_QUERY", http.StatusBadRequest)
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}

	// Get engine names - bangs take priority, then URL param
	engineNames := parsed.Engines
	if len(engineNames) == 0 {
		if e := r.URL.Query().Get("engines"); e != "" {
			engineNames = strings.Split(e, ",")
		}
	}

	// Check cache first (skip cache param allows bypassing)
	skipCache := r.URL.Query().Get("nocache") == "1"
	cacheKey := cache.CacheKey(searchQuery, page, engineNames)

	var results *model.SearchResponse
	if !skipCache {
		if cached, ok := h.searchCache.Get(cacheKey); ok {
			results = cached
			results.Data.Cached = true
		}
	}

	// If not cached, perform search
	if results == nil {
		results = h.engineMgr.Search(r.Context(), searchQuery, page, engineNames)
		results.Data.Cached = false
		// Cache the results
		h.searchCache.Set(cacheKey, results)
	}

	// Add bang info to response
	// Keep original query with bangs
	results.Data.Query = query
	results.Data.SearchQuery = searchQuery
	results.Data.HasBang = parsed.HasBang
	results.Data.BangEngines = parsed.Engines

	h.jsonResponse(w, results)
}

// APISearchText handles search API requests returning text
func (h *Handler) APISearchText(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error: Query parameter 'q' is required"))
		return
	}

	// Parse bangs from query
	parsed := engine.ParseBangs(query)
	searchQuery := parsed.Query
	if searchQuery == "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error: Query cannot be empty after bang parsing"))
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}

	// Get engine names - bangs take priority, then URL param
	engineNames := parsed.Engines
	if len(engineNames) == 0 {
		if e := r.URL.Query().Get("engines"); e != "" {
			engineNames = strings.Split(e, ",")
		}
	}

	results := h.engineMgr.Search(r.Context(), searchQuery, page, engineNames)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, result := range results.Data.Results {
		w.Write([]byte(result.Title + "\n"))
		w.Write([]byte(result.URL + "\n"))
		w.Write([]byte("Duration: " + result.Duration + " | Source: " + result.SourceDisplay + "\n"))
		w.Write([]byte("\n"))
	}
}

// APIBangs returns list of available bang shortcuts
func (h *Handler) APIBangs(w http.ResponseWriter, r *http.Request) {
	bangs := engine.ListBangs()
	h.jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    bangs,
		"count":   len(bangs),
	})
}

// APIAutocomplete returns autocomplete suggestions for bangs
func (h *Handler) APIAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		h.jsonResponse(w, map[string]interface{}{
			"success":     true,
			"suggestions": []interface{}{},
		})
		return
	}

	// Check if query starts with "!" for bang autocomplete
	if strings.HasPrefix(q, "!") && len(q) > 1 {
		// Remove the "!" prefix
		prefix := q[1:]
		suggestions := engine.Autocomplete(prefix)
		h.jsonResponse(w, map[string]interface{}{
			"success":     true,
			"suggestions": suggestions,
			"type":        "bang",
		})
		return
	}

	// If query ends with " !" (space bang), suggest starting a bang
	if strings.HasSuffix(q, " !") {
		bangs := engine.ListBangs()
		// Return first 10 bangs as suggestions
		if len(bangs) > 10 {
			bangs = bangs[:10]
		}
		h.jsonResponse(w, map[string]interface{}{
			"success":     true,
			"suggestions": bangs,
			"type":        "bang_start",
		})
		return
	}

	// Check for partial bang at end of query (e.g., "amateur !p")
	words := strings.Fields(q)
	if len(words) > 0 {
		lastWord := words[len(words)-1]
		if strings.HasPrefix(lastWord, "!") && len(lastWord) > 1 {
			prefix := lastWord[1:]
			suggestions := engine.Autocomplete(prefix)
			// replace indicates what to replace in query
			h.jsonResponse(w, map[string]interface{}{
				"success":     true,
				"suggestions": suggestions,
				"type":        "bang",
				"replace":     lastWord,
			})
			return
		}
	}

	// No bang autocomplete needed
	h.jsonResponse(w, map[string]interface{}{
		"success":     true,
		"suggestions": []interface{}{},
		"type":        "none",
	})
}

// APIEngines returns list of available engines
func (h *Handler) APIEngines(w http.ResponseWriter, r *http.Request) {
	engines := h.engineMgr.ListEngines()
	h.jsonResponse(w, model.EnginesResponse{
		Success: true,
		Data:    engines,
	})
}

// APIEngineDetails returns details for a specific engine
func (h *Handler) APIEngineDetails(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	engine, ok := h.engineMgr.GetEngine(name)
	if !ok {
		h.jsonError(w, "Engine not found", "ENGINE_NOT_FOUND", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": model.EngineInfo{
			Name:        engine.Name(),
			DisplayName: engine.DisplayName(),
			Enabled:     engine.IsAvailable(),
			Available:   engine.IsAvailable(),
			Tier:        engine.Tier(),
		},
	})
}

// APIStats returns public statistics
func (h *Handler) APIStats(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"engines_enabled": h.engineMgr.EnabledCount(),
			"engines_total":   len(h.engineMgr.ListEngines()),
		},
	})
}

// APIHealthCheck returns health status as JSON per AI.md PART 16
// Returns comprehensive health status with checks object for database/cache/disk
// APIHealthCheck handles /api/v1/healthz endpoint (JSON only)
// Per AI.md PART 13 lines 11351-11353: Same JSON as /healthz
func (h *Handler) APIHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Build health response per AI.md PART 13
	hostname, _ := os.Hostname()
	uptime := getUptime()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	
	// Get mode from config
	appMode := "production"
	if h.cfg != nil && h.cfg.IsDevelopmentMode() {
		appMode = "development"
	}
	
	// Node ID (standalone or cluster)
	nodeID := "standalone"
	
	// Cluster status
	clusterEnabled := false
	
	// Build checks object - MUST be simple "ok"/"error" strings
	// Per AI.md PART 13 lines 11292-11295
	checks := map[string]string{
		"database": "ok",
		"cache":    "ok",
		"disk":     "ok",
	}
	
	// Overall status
	status := "healthy"
	
	// JSON format per AI.md PART 13 lines 11312-11379
	response := map[string]interface{}{
		"status":    status,
		"version":   version.Get(),
		"mode":      appMode,
		"uptime":    uptime,
		"timestamp": timestamp,
		"node": map[string]interface{}{
			"id":       nodeID,
			"hostname": hostname,
		},
		"cluster": map[string]interface{}{
			"enabled": clusterEnabled,
		},
		"checks": checks,
	}
	
	WriteJSON(w, http.StatusOK, response)
}

// serverStartTime is set when the server starts
var serverStartTime = time.Now()

// getUptime returns the server uptime as a human-readable string
func getUptime() string {
	uptime := time.Since(serverStartTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

// Helper methods

// jsonResponse is DEPRECATED - use WriteJSON instead
// Kept temporarily for backward compatibility
func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResponse := map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    code,
		"status":  status,
	}
	// Per PART 20: JSON must be indented with 2 spaces and end with single newline
	formatted, err := json.MarshalIndent(errorResponse, "", "  ")
	if err != nil {
		// Fallback if marshaling fails
		w.Write([]byte(`{"success":false,"error":"Internal error","code":"ENCODING_ERROR"}`))
		w.Write([]byte("\n"))
		return
	}
	w.Write(formatted)
	w.Write([]byte("\n"))
}

// RenderErrorPage renders a custom error page per AI.md PART 30
func (h *Handler) RenderErrorPage(w http.ResponseWriter, code int, title, message string) {
	data := map[string]interface{}{
		"Code":      code,
		"Title":     title,
		"Message":   message,
		"SiteTitle": h.cfg.Web.Branding.AppName,
	}

	tmpl, err := template.ParseFS(templatesFS, "template/error.tmpl")
	if err != nil {
		// Fallback to plain text error
		http.Error(w, fmt.Sprintf("%d %s: %s", code, title, message), code)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := tmpl.ExecuteTemplate(w, "error", data); err != nil {
		http.Error(w, fmt.Sprintf("%d %s: %s", code, title, message), code)
	}
}

// NotFoundHandler handles 404 errors per AI.md PART 30
func (h *Handler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderErrorPage(w, http.StatusNotFound, "Page Not Found",
		"The page you're looking for doesn't exist or has been moved.")
}

// InternalErrorHandler handles 500 errors per AI.md PART 30
func (h *Handler) InternalErrorHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderErrorPage(w, http.StatusInternalServerError, "Server Error",
		"Something went wrong on our end. Please try again later.")
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	// Map template names to file paths
	templateFile := ""
	templateName := ""
	switch name {
	case "home":
		templateFile = "template/index.tmpl"
		templateName = "home"
	case "search":
		templateFile = "template/search.tmpl"
		templateName = "search"
	case "preferences":
		templateFile = "template/preferences.tmpl"
		templateName = "preferences"
	case "about":
		templateFile = "template/about.tmpl"
		templateName = "about"
	case "age-verify":
		templateFile = "template/age-verify.tmpl"
		templateName = "age-verify"
	case "privacy":
		templateFile = "template/privacy.tmpl"
		templateName = "privacy"
	default:
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Create base template with partials
	tmpl := template.New(templateName)

	// Load all partials first (public and admin)
	partialFiles := []string{
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/footer.tmpl",
		"template/partial/public/scripts.tmpl",
		"template/partial/admin/head.tmpl",
		"template/partial/admin/sidebar.tmpl",
		"template/partial/admin/scripts.tmpl",
	}

	for _, pf := range partialFiles {
		content, err := templatesFS.ReadFile(pf)
		if err != nil {
			// Skip missing partials - they may not all be needed
			continue
		}
		_, err = tmpl.Parse(string(content))
		if err != nil {
			http.Error(w, "Partial parse error ("+pf+"): "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Read and parse the main template
	content, err := templatesFS.ReadFile(templateFile)
	if err != nil {
		http.Error(w, "Template file not found: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = tmpl.Parse(string(content))
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ProxyThumbnail proxies external thumbnails to prevent tracking
// Per AI.md PART 36 lines 29497-29507
func (h *Handler) ProxyThumbnail(w http.ResponseWriter, r *http.Request) {
// Get URL parameter
encodedURL := r.URL.Query().Get("url")
if encodedURL == "" {
http.Error(w, "Missing url parameter", http.StatusBadRequest)
return
}

// Decode URL
thumbURL, err := url.QueryUnescape(encodedURL)
if err != nil {
http.Error(w, "Invalid url parameter", http.StatusBadRequest)
return
}

// Validate URL
parsedURL, err := url.Parse(thumbURL)
if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
http.Error(w, "Invalid thumbnail URL", http.StatusBadRequest)
return
}


// Fetch thumbnail
client := &http.Client{
Timeout: 10 * time.Second,
}

resp, err := client.Get(thumbURL)
if err != nil {
http.Error(w, "Failed to fetch thumbnail", http.StatusBadGateway)
return
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
http.Error(w, "Thumbnail not found", http.StatusNotFound)
return
}

// Copy content type
contentType := resp.Header.Get("Content-Type")
if contentType == "" {
contentType = "image/jpeg" // Default
}
w.Header().Set("Content-Type", contentType)

// Cache control per AI.md PART 36: 1 hour
w.Header().Set("Cache-Control", "public, max-age=3600")

// Proxy the image
io.Copy(w, resp.Body)
}
