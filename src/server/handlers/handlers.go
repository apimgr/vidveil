// SPDX-License-Identifier: MIT
package handlers

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/engines"
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
	cfg       *config.Config
	engineMgr *engines.Manager
}

// New creates a new handler instance
func New(cfg *config.Config, engineMgr *engines.Manager) *Handler {
	return &Handler{
		cfg:       cfg,
		engineMgr: engineMgr,
	}
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
	http.SetCookie(w, &http.Cookie{
		Name:     ageVerifyCookieName,
		Value:    "1",
		Path:     "/",
		MaxAge:   ageVerifyCookieDays * 24 * 60 * 60, // 30 days in seconds
		Expires:  time.Now().Add(ageVerifyCookieDays * 24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // Set to true if using HTTPS
	})
}

// Build time set at compile time
var BuildDateTime = "December 4, 2025"

// HomePage renders the main search page
func (h *Handler) HomePage(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "home", map[string]interface{}{
		"Title":         h.cfg.Server.Title,
		"Description":   h.cfg.Server.Description,
		"Theme":         h.cfg.Web.UI.Theme,
		"BuildDateTime": BuildDateTime,
		"EngineCount":   h.engineMgr.EnabledCount(),
	})
}

// SearchPage renders search results with infinite scroll
func (h *Handler) SearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Get engine names if specified
	var engineNames []string
	if e := r.URL.Query().Get("engines"); e != "" {
		engineNames = strings.Split(e, ",")
	}

	// Perform parallel search across all engines
	results := h.engineMgr.Search(r.Context(), query, 1, engineNames)

	// Convert results to JSON for the JavaScript
	resultsJSON, _ := json.Marshal(results.Data.Results)

	h.renderTemplate(w, "search", map[string]interface{}{
		"Title":       query + " - " + h.cfg.Server.Title,
		"Query":       query,
		"ResultsJSON": template.JS(resultsJSON), // Safe JSON for script
		"EnginesUsed": results.Data.EnginesUsed,
		"SearchTime":  results.Data.SearchTimeMS,
		"Theme":       h.cfg.Web.UI.Theme,
	})
}

// PreferencesPage renders user preferences
func (h *Handler) PreferencesPage(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "preferences", map[string]interface{}{
		"Title":         "Preferences - " + h.cfg.Server.Title,
		"Theme":         h.cfg.Web.UI.Theme,
		"Engines":       h.engineMgr.ListEngines(),
		"BuildDateTime": BuildDateTime,
	})
}

// AboutPage renders the about page
func (h *Handler) AboutPage(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "about", map[string]interface{}{
		"Title":         "About - " + h.cfg.Server.Title,
		"Theme":         h.cfg.Web.UI.Theme,
		"Version":       "0.2.0",
		"BuildDateTime": BuildDateTime,
	})
}

// PrivacyPage renders the privacy policy page
func (h *Handler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "privacy", map[string]interface{}{
		"Title":         "Privacy Policy - " + h.cfg.Server.Title,
		"Theme":         h.cfg.Web.UI.Theme,
		"Version":       "0.2.0",
		"BuildDateTime": BuildDateTime,
	})
}

// HealthCheck returns health status as HTML
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Health Check</title></head>
<body>
<h1>Vidveil Health Check</h1>
<p>Status: <strong class="text-green">OK</strong></p>
<p>Engines: ` + strconv.Itoa(h.engineMgr.EnabledCount()) + ` enabled</p>
</body>
</html>`))
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

// SitemapXML returns sitemap.xml per TEMPLATE.md PART 13
func (h *Handler) SitemapXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	baseURL := "https://" + h.cfg.Server.FQDN
	if h.cfg.Server.Port != "443" && h.cfg.Server.Port != "80" {
		baseURL = fmt.Sprintf("https://%s:%s", h.cfg.Server.FQDN, h.cfg.Server.Port)
	}

	// Build sitemap with static pages per TEMPLATE.md PART 13
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

// APISearch handles search API requests
func (h *Handler) APISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.jsonError(w, "Query parameter 'q' is required", "MISSING_QUERY", http.StatusBadRequest)
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}

	var engineNames []string
	if e := r.URL.Query().Get("engines"); e != "" {
		engineNames = strings.Split(e, ",")
	}

	results := h.engineMgr.Search(r.Context(), query, page, engineNames)
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

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}

	var engineNames []string
	if e := r.URL.Query().Get("engines"); e != "" {
		engineNames = strings.Split(e, ",")
	}

	results := h.engineMgr.Search(r.Context(), query, page, engineNames)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, result := range results.Data.Results {
		w.Write([]byte(result.Title + "\n"))
		w.Write([]byte(result.URL + "\n"))
		w.Write([]byte("Duration: " + result.Duration + " | Source: " + result.SourceDisplay + "\n"))
		w.Write([]byte("\n"))
	}
}

// APIEngines returns list of available engines
func (h *Handler) APIEngines(w http.ResponseWriter, r *http.Request) {
	engines := h.engineMgr.ListEngines()
	h.jsonResponse(w, models.EnginesResponse{
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
		"data": models.EngineInfo{
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

// APIHealthCheck returns health status as JSON per TEMPLATE.md PART 23
// Returns comprehensive health status with checks object for database/cache/disk/engines
func (h *Handler) APIHealthCheck(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Build checks object with individual component health
	checks := make(map[string]interface{})
	overallHealthy := true

	// Database check
	dbStatus := map[string]interface{}{
		"status": "ok",
		"type":   h.cfg.Server.Database.Driver,
	}
	if h.cfg.Server.Database.Driver != "" && h.cfg.Server.Database.Driver != "none" {
		// Check database configuration (SQLite)
		if h.cfg.Server.Database.SQLite.Dir == "" && h.cfg.Server.Database.SQLite.ServerDB == "" {
			dbStatus["status"] = "unconfigured"
		} else {
			dbStatus["status"] = "ok"
			dbStatus["message"] = "configured"
		}
	} else {
		dbStatus["status"] = "disabled"
	}
	checks["database"] = dbStatus

	// Cache check
	cacheStatus := map[string]interface{}{
		"status": "ok",
		"type":   h.cfg.Server.Cache.Type,
	}
	if h.cfg.Server.Cache.Type == "" || h.cfg.Server.Cache.Type == "none" {
		cacheStatus["status"] = "disabled"
	} else if h.cfg.Server.Cache.Type == "memory" {
		cacheStatus["status"] = "ok"
		cacheStatus["message"] = "in-memory cache active"
	}
	checks["cache"] = cacheStatus

	// Disk check - check data directory
	diskStatus := map[string]interface{}{
		"status": "ok",
	}
	paths := config.GetPaths("", "")
	if info, err := os.Stat(paths.Data); err != nil {
		diskStatus["status"] = "error"
		diskStatus["error"] = err.Error()
		overallHealthy = false
	} else if !info.IsDir() {
		diskStatus["status"] = "error"
		diskStatus["error"] = "data path is not a directory"
		overallHealthy = false
	} else {
		diskStatus["status"] = "ok"
		diskStatus["path"] = paths.Data
		diskStatus["writable"] = true
	}
	checks["disk"] = diskStatus

	// Engines check
	enabledCount := h.engineMgr.EnabledCount()
	totalCount := len(h.engineMgr.ListEngines())
	enginesStatus := map[string]interface{}{
		"status":  "ok",
		"enabled": enabledCount,
		"total":   totalCount,
	}
	if enabledCount == 0 {
		enginesStatus["status"] = "warning"
		enginesStatus["message"] = "no engines enabled"
	}
	checks["engines"] = enginesStatus

	// Overall status
	overallStatus := "ok"
	if !overallHealthy {
		overallStatus = "degraded"
	}

	// Build response
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"status":    overallStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "0.2.0",
			"mode":      h.cfg.Server.Mode,
			"uptime":    getUptime(),
			"checks":    checks,
			"response_time_ms": time.Since(startTime).Milliseconds(),
		},
	}

	h.jsonResponse(w, response)
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

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    code,
		"status":  status,
	})
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	// Map template names to file paths
	templateFile := ""
	templateName := ""
	switch name {
	case "home":
		templateFile = "templates/index.tmpl"
		templateName = "home"
	case "search":
		templateFile = "templates/search.tmpl"
		templateName = "search"
	case "preferences":
		templateFile = "templates/preferences.tmpl"
		templateName = "preferences"
	case "about":
		templateFile = "templates/about.tmpl"
		templateName = "about"
	case "age-verify":
		templateFile = "templates/age-verify.tmpl"
		templateName = "age-verify"
	case "privacy":
		templateFile = "templates/privacy.tmpl"
		templateName = "privacy"
	default:
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Create base template with partials
	tmpl := template.New(templateName)

	// Load all partials first
	partialFiles := []string{
		"templates/partials/head.tmpl",
		"templates/partials/header.tmpl",
		"templates/partials/nav.tmpl",
		"templates/partials/footer.tmpl",
		"templates/partials/scripts.tmpl",
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
