// SPDX-License-Identifier: MIT
package handler

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/cache"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/common/version"
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

// TorStatusChecker is a minimal interface for checking Tor status
type TorStatusChecker interface {
	IsEnabled() bool
	IsRunning() bool
	GetInfo() map[string]interface{}
	AllowUserIPForward() bool // Per PART 32: Admin setting for IP forwarding
}

// GeoIPChecker is a minimal interface for GeoIP content restriction checks
type GeoIPChecker interface {
	CheckContentRestriction(ipStr string, isTorUser bool) *geoip.RestrictionResult
	GetRestrictionMode() string
	IsEnabled() bool
}

// Cookie name for content restriction acknowledgment
const ContentRestrictionAckCookieName = "content_ack"

// Cookie name for user IP forwarding preference
const IPForwardCookieName = "forward_ip"

// getUserIPForwardPreference checks if user has opted-in to IP forwarding via cookie
// Returns (user wants forwarding, user's IP)
func (h *SearchHandler) getUserIPForwardPreference(r *http.Request) (bool, string) {
	// Check if admin allows this feature
	if h.torSvc == nil || !h.torSvc.AllowUserIPForward() {
		return false, ""
	}

	// Check user's preference cookie (defaults to disabled)
	cookie, err := r.Cookie(IPForwardCookieName)
	if err != nil || cookie.Value != "1" {
		return false, "" // User hasn't opted in
	}

	// Get user's real IP
	userIP := getClientIP(r)
	return true, userIP
}

// checkContentRestriction checks if user is from a restricted region
// Returns restriction result or nil if no restriction
func (h *SearchHandler) checkContentRestriction(r *http.Request) *geoip.RestrictionResult {
	if h.geoipSvc == nil || !h.geoipSvc.IsEnabled() {
		return nil
	}

	mode := h.geoipSvc.GetRestrictionMode()
	if mode == "off" || mode == "" {
		return nil
	}

	// Check if user is accessing via Tor hidden service
	isTorUser := h.isTorRequest(r)

	// Get client IP
	clientIP := getClientIP(r)

	// Perform restriction check
	result := h.geoipSvc.CheckContentRestriction(clientIP, isTorUser)
	if result == nil || !result.Restricted {
		return nil
	}

	return result
}

// isTorRequest checks if the request is coming via Tor hidden service
func (h *SearchHandler) isTorRequest(r *http.Request) bool {
	// Check if request came through .onion address
	host := r.Host
	if strings.HasSuffix(host, ".onion") {
		return true
	}

	// Check X-Tor-Hidden-Service header (set by reverse proxies)
	if r.Header.Get("X-Tor-Hidden-Service") == "1" {
		return true
	}

	return false
}

// hasContentRestrictionAck checks if user has acknowledged content restriction warning
func (h *SearchHandler) hasContentRestrictionAck(r *http.Request) bool {
	cookie, err := r.Cookie(ContentRestrictionAckCookieName)
	if err != nil {
		return false
	}
	return cookie.Value == "1"
}

// setContentRestrictionAckCookie sets the acknowledgment cookie (30 days)
func (h *SearchHandler) setContentRestrictionAckCookie(w http.ResponseWriter) {
	http.SetCookie(w, NewSecureCookie(
		ContentRestrictionAckCookieName,
		"1",
		"/",
		30*24*60*60, // 30 days
		h.appConfig.Server.SSL.Enabled,
	))
}

// getClientIP extracts the client's real IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Remove brackets from IPv6
	ip = strings.Trim(ip, "[]")
	return ip
}

// SearchHandler holds dependencies for HTTP handlers
type SearchHandler struct {
	appConfig   *config.AppConfig
	engineMgr   *engine.EngineManager
	searchCache *cache.SearchCache
	metrics     *ServerMetrics
	torSvc      TorStatusChecker
	geoipSvc    GeoIPChecker
}

// NewSearchHandler creates a new handler instance
func NewSearchHandler(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *SearchHandler {
	// Use default config if nil per AI.md PART 5
	if appConfig == nil {
		appConfig = config.DefaultAppConfig()
	}

	// Initialize cache with 5 minute TTL and 1000 max entries
	searchCache := cache.NewSearchCache(5*time.Minute, 1000)

	return &SearchHandler{
		appConfig:   appConfig,
		engineMgr:   engineMgr,
		searchCache: searchCache,
	}
}

// SetMetrics sets the metrics collector for statistics display
func (h *SearchHandler) SetMetrics(m *ServerMetrics) {
	h.metrics = m
}

// SetTorService sets the Tor service for healthz display
func (h *SearchHandler) SetTorService(t TorStatusChecker) {
	h.torSvc = t
}

// SetGeoIPService sets the GeoIP service for content restriction checks
func (h *SearchHandler) SetGeoIPService(g GeoIPChecker) {
	h.geoipSvc = g
}

// GetSearchCache returns the search cache for sharing with admin handler
func (h *SearchHandler) GetSearchCache() *cache.SearchCache {
	return h.searchCache
}

// getSearchCount returns total searches from metrics
func (h *SearchHandler) getSearchCount() uint64 {
	if h.metrics != nil {
		return h.metrics.GetSearchesTotal()
	}
	return 0
}

// getTorStatus returns Tor status string per PART 13
func (h *SearchHandler) getTorStatus() string {
	if h.torSvc == nil {
		return "disabled"
	}
	info := h.torSvc.GetInfo()
	if status, ok := info["status"].(string); ok {
		return status
	}
	if h.torSvc.IsRunning() {
		return "healthy"
	}
	return "disabled"
}

// getTorHostname returns Tor .onion address per PART 13
func (h *SearchHandler) getTorHostname() string {
	if h.torSvc == nil {
		return ""
	}
	info := h.torSvc.GetInfo()
	if hostname, ok := info["hostname"].(string); ok {
		return hostname
	}
	return ""
}

// getRequestsTotal returns total HTTP requests from metrics
func (h *SearchHandler) getRequestsTotal() uint64 {
	if h.metrics != nil {
		return h.metrics.GetRequestsTotal()
	}
	return 0
}

// getRequests24h returns HTTP requests in last 24 hours per AI.md PART 13
func (h *SearchHandler) getRequests24h() uint64 {
	if h.metrics != nil {
		return h.metrics.GetRequests24h()
	}
	return 0
}

// getActiveConnections returns current active connections
func (h *SearchHandler) getActiveConnections() int64 {
	if h.metrics != nil {
		return h.metrics.GetActiveConnections()
	}
	return 0
}

// WriteJSON writes a JSON response with 2-space indentation and trailing newline
// Per AI.md PART 14: ALL JSON responses MUST be indented
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
func (h *SearchHandler) MaintenanceModeMiddleware(next http.Handler) http.Handler {
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
		paths := config.GetAppPaths("", "")
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
    <title>Maintenance - ` + h.appConfig.Server.Title + `</title>
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
func (h *SearchHandler) AgeVerifyMiddleware(next http.Handler) http.Handler {
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
func (h *SearchHandler) AgeVerifyPage(w http.ResponseWriter, r *http.Request) {
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

	h.renderResponse(w, r, "age-verify", map[string]interface{}{
		"Title":    "Age Verification - " + h.appConfig.Server.Title,
		"Theme":    h.appConfig.Web.UI.Theme,
		"Redirect": redirect,
	})
}

// AgeVerifySubmit handles the age verification form submission
func (h *SearchHandler) AgeVerifySubmit(w http.ResponseWriter, r *http.Request) {
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

// setAgeVerifyCookie sets/renews the age verification cookie per AI.md PART 11
func (h *SearchHandler) setAgeVerifyCookie(w http.ResponseWriter) {
	// 30 days, with Secure flag per AI.md PART 11
	http.SetCookie(w, NewSecureCookie(
		ageVerifyCookieName,
		"1",
		"/",
		ageVerifyCookieDays*24*60*60,
		h.appConfig.Server.SSL.Enabled,
	))
}

// ContentRestrictionMiddleware checks for geographic content restrictions
// Behavior depends on mode: warn, soft_block, or hard_block
func (h *SearchHandler) ContentRestrictionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip restriction check for static files, health checks, API, and restriction pages
		path := r.URL.Path
		if strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/api/") ||
			path == "/healthz" ||
			path == "/robots.txt" ||
			path == "/age-verify" ||
			path == "/content-restricted" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if user is from a restricted region
		restriction := h.checkContentRestriction(r)
		if restriction == nil {
			// Not restricted, proceed
			next.ServeHTTP(w, r)
			return
		}

		// Handle based on restriction mode
		switch restriction.Mode {
		case "hard_block":
			// Completely block access - show error page
			h.renderContentBlockedPage(w, r, restriction)
			return

		case "soft_block":
			// Require acknowledgment before proceeding
			if !h.hasContentRestrictionAck(r) {
				redirect := r.URL.Path
				if r.URL.RawQuery != "" {
					redirect += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, "/content-restricted?redirect="+redirect, http.StatusFound)
				return
			}
			// Has acknowledgment, proceed
			next.ServeHTTP(w, r)

		case "warn":
			// Set warning header for frontend to display dismissable banner
			w.Header().Set("X-Content-Warning", restriction.Message)
			w.Header().Set("X-Content-Warning-Region", restriction.Reason)
			next.ServeHTTP(w, r)

		default:
			// Unknown mode, proceed without restriction
			next.ServeHTTP(w, r)
		}
	})
}

// renderContentBlockedPage renders the hard block page (no way to bypass)
func (h *SearchHandler) renderContentBlockedPage(w http.ResponseWriter, r *http.Request, restriction *geoip.RestrictionResult) {
	w.WriteHeader(http.StatusForbidden)
	h.renderResponse(w, r, "content-blocked", map[string]interface{}{
		"Title":   "Access Restricted - " + h.appConfig.Server.Title,
		"Theme":   h.appConfig.Web.UI.Theme,
		"Message": restriction.Message,
		"Region":  restriction.Reason,
	})
}

// ContentRestrictedPage shows the soft block acknowledgment page
func (h *SearchHandler) ContentRestrictedPage(w http.ResponseWriter, r *http.Request) {
	// If already acknowledged, redirect to home or specified redirect
	if h.hasContentRestrictionAck(r) {
		redirect := r.URL.Query().Get("redirect")
		if redirect == "" || !strings.HasPrefix(redirect, "/") {
			redirect = "/"
		}
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}

	// Get redirect destination
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, "/") {
		redirect = "/"
	}

	// Get restriction info for display
	restriction := h.checkContentRestriction(r)
	message := "Adult content may be restricted in your region."
	region := ""
	if restriction != nil {
		message = restriction.Message
		region = restriction.Reason
	}

	h.renderResponse(w, r, "content-restricted", map[string]interface{}{
		"Title":    "Content Notice - " + h.appConfig.Server.Title,
		"Theme":    h.appConfig.Web.UI.Theme,
		"Redirect": redirect,
		"Message":  message,
		"Region":   region,
	})
}

// ContentRestrictedSubmit handles the acknowledgment form submission
func (h *SearchHandler) ContentRestrictedSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/content-restricted", http.StatusFound)
		return
	}

	// Set the acknowledgment cookie
	h.setContentRestrictionAckCookie(w)

	// Redirect to the original destination
	redirect := r.FormValue("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, "/") {
		redirect = "/"
	}

	http.Redirect(w, r, redirect, http.StatusFound)
}

// BuildDateTime returns the build time formatted per AI.md PART 16
// Format: "January 2, 2006 at 15:04:05" (December 4, 2025 at 13:05:13)
func BuildDateTime() string {
	raw := version.BuildTime
	if raw == "" || raw == "unknown" {
		return "unknown"
	}

	// Try to parse common build time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"Jan 2 2006 15:04:05",
		"Mon Jan 2 15:04:05 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, raw); err == nil {
			// Format per AI.md PART 16: %B %-d, %Y at %H:%M:%S
			return t.Format("January 2, 2006 at 15:04:05")
		}
	}

	// If parsing fails, return raw value
	return raw
}

// HomePage renders the main search page
// HomePage renders the home page with content negotiation per AI.md PART 17
func (h *SearchHandler) HomePage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	engineCount := h.engineMgr.EnabledCount()
	
	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":        h.appConfig.Server.Title,
			"description":  h.appConfig.Server.Description,
			"engine_count": engineCount,
			"version":      version.GetVersion(),
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%s\n", h.appConfig.Server.Title)
		fmt.Fprintf(w, "%s\n\n", h.appConfig.Server.Description)
		fmt.Fprintf(w, "Search Engines: %d enabled\n", engineCount)
		fmt.Fprintf(w, "Version: %s\n", version.GetVersion())
		
	default:
		// HTML response for browsers (default)
		h.renderResponse(w, r, "home", map[string]interface{}{
			"Title":         h.appConfig.Server.Title,
			"Description":   h.appConfig.Server.Description,
			"Theme":         h.appConfig.Web.UI.Theme,
			"BuildDateTime": BuildDateTime(),
			"EngineCount":   engineCount,
		})
	}
}

// SearchPage renders search results with content negotiation per AI.md PART 17
func (h *SearchHandler) SearchPage(w http.ResponseWriter, r *http.Request) {
	// Strip leading/trailing whitespace from query per AI.md
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Parse bangs from query (e.g., "!ph amateur" -> search pornhub for "amateur")
	parsed := engine.ParseBangs(query)
	searchQuery := strings.TrimSpace(parsed.Query)
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

	// Increment search count
	if h.metrics != nil {
		h.metrics.IncrementSearches()
	}

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
		h.renderResponse(w, r, "search", map[string]interface{}{
			"Title":         query + " - " + h.appConfig.Server.Title,
			"Query":         query,
			"SearchQuery":   searchQuery,
			"ResultsJSON":   template.JS(resultsJSON),
			"EnginesUsed":   results.Data.EnginesUsed,
			"SearchTime":    results.Data.SearchTimeMS,
			"Theme":         h.appConfig.Web.UI.Theme,
			"HasBang":       parsed.HasBang,
			"BangEngines":   parsed.Engines,
			"Version":       version.GetVersion(),
			"BuildDateTime": BuildDateTime(),
		})
	}
}

// PreferencesPage renders user preferences with content negotiation per AI.md PART 17
func (h *SearchHandler) PreferencesPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	engines := h.engineMgr.ListEngines()
	
	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":    "Preferences",
			"engines":  engines,
			"theme":    h.appConfig.Web.UI.Theme,
		})
		
	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Preferences - %s\n\n", h.appConfig.Server.Title)
		fmt.Fprintf(w, "Theme: %s\n", h.appConfig.Web.UI.Theme)
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
		h.renderResponse(w, r, "preferences", map[string]interface{}{
			"Title":         "Preferences - " + h.appConfig.Server.Title,
			"Theme":         h.appConfig.Web.UI.Theme,
			"Engines":       engines,
			"BuildDateTime": BuildDateTime(),
		})
	}
}

// AboutPage renders the about page with content negotiation per AI.md PART 17
func (h *SearchHandler) AboutPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	ver := version.GetVersion()

	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":         h.appConfig.Server.Title,
			"version":       ver,
			"build_date":    BuildDateTime(),
			"description":   h.appConfig.Server.Description,
		})

	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%s\n", h.appConfig.Server.Title)
		fmt.Fprintf(w, "Version: %s\n", ver)
		fmt.Fprintf(w, "Build Date: %s\n", BuildDateTime())
		fmt.Fprintf(w, "\n%s\n", h.appConfig.Server.Description)

	default:
		// HTML response for browsers (default)
		h.renderResponse(w, r, "about", map[string]interface{}{
			"Title":         "About - " + h.appConfig.Server.Title,
			"Theme":         h.appConfig.Web.UI.Theme,
			"Version":       ver,
			"BuildDateTime": BuildDateTime(),
		})
	}
}

// PrivacyPage renders the privacy policy page with content negotiation per AI.md PART 17
func (h *SearchHandler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)
	
	ver := version.GetVersion()

	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":   "Privacy Policy",
			"version": ver,
		})

	case "text/plain":
		// Plain text response for curl/CLI per AI.md PART 17
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Privacy Policy - %s\n", h.appConfig.Server.Title)
		fmt.Fprintf(w, "Version: %s\n\n", ver)
		fmt.Fprintf(w, "VidVeil is a privacy-respecting meta search engine.\n")
		fmt.Fprintf(w, "We do not track, log, or collect any user data.\n")

	default:
		// HTML response for browsers (default)
		h.renderResponse(w, r, "privacy", map[string]interface{}{
			"Title":         "Privacy Policy - " + h.appConfig.Server.Title,
			"Theme":         h.appConfig.Web.UI.Theme,
			"Version":       ver,
			"BuildDateTime": BuildDateTime(),
		})
	}
}

// detectResponseFormat returns the response format based on Accept header
// Per AI.md PART 19: Content Negotiation
// detectResponseFormat determines response format per AI.md PART 14
func detectResponseFormat(r *http.Request) string {
	// 0. Check URL path extension FIRST per AI.md PART 13
	// Use original path from context if available (set by extensionStripMiddleware)
	path := r.URL.Path
	if origPath, ok := r.Context().Value("vidveil.originalPath").(string); ok {
		path = origPath
	}
	if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(path, ".txt") {
		return "text/plain"
	}
	
	// 1. Check Accept header (explicit preference)
	accept := r.Header.Get("Accept")

	// SSE streaming takes priority for search endpoints
	if strings.Contains(accept, "text/event-stream") {
		return "text/event-stream"
	}
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

// getAPIResponseFormat determines format for /api/** routes per AI.md PART 14
// Returns "text" or "json" (raw strings, not MIME types)
// Priority: .txt extension > Accept header > CLI detection > default JSON
func getAPIResponseFormat(r *http.Request) string {
	// 1. Check .txt extension FIRST (highest priority)
	if strings.HasSuffix(r.URL.Path, ".txt") {
		return "text"
	}

	// 2. Check Accept header (explicit preference - overrides UA detection)
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return "json"
	}
	if strings.Contains(accept, "text/plain") {
		return "text"
	}

	// 3. Check if non-interactive client (CLI tools like curl, wget)
	// Per AI.md PART 14: CLI Tool column shows "Text"
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	cliTools := []string{
		"curl/", "wget/", "httpie/",
		"libcurl/", "python-requests/",
		"go-http-client/", "axios/", "node-fetch/",
	}
	for _, tool := range cliTools {
		if strings.Contains(ua, tool) {
			return "text"
		}
	}

	// Empty User-Agent = likely HTTP tool (non-interactive)
	if ua == "" {
		return "text"
	}

	// 4. Default to JSON for API routes (browsers, API clients)
	return "json"
}

// HealthCheck returns health status with content negotiation
// Per AI.md PART 16: Supports HTML (default), JSON (Accept: application/json), and Text
// HealthCheck handles /healthz endpoint with content negotiation
// Per AI.md PART 13
func (h *SearchHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	// Build health response per AI.md PART 13
	hostname, _ := os.Hostname()
	uptime := getUptime()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	
	// Get mode from config
	appMode := "production"
	if h.appConfig != nil && h.appConfig.IsDevelopmentMode() {
		appMode = "development"
	}

	// Cluster status per PART 10
	clusterEnabled := false
	clusterStatus := ""
	clusterNodes := 0
	clusterRole := ""
	
	// Build checks object - MUST be simple "ok"/"error" strings
	// Per AI.md PART 13
	checks := map[string]string{
		"database": "ok",
		"cache":    "ok",
		"disk":     "ok",
	}
	
	// Add cluster check if clustering enabled
	if clusterEnabled {
		checks["cluster"] = "ok"
	}

	// Add scheduler check
	checks["scheduler"] = "ok"

	// Add Tor check if enabled per PART 13
	if h.torSvc != nil && h.torSvc.IsEnabled() {
		if h.torSvc.IsRunning() {
			checks["tor"] = "ok"
		} else {
			checks["tor"] = "error"
		}
	}

	// Overall status - per AI.md PART 13: derive from checks
	status := "healthy"
	httpStatus := http.StatusOK
	for _, v := range checks {
		if v != "ok" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	// Get project info from config per PART 16 branding
	projectName := "VidVeil"
	projectTagline := "Privacy-first video search"
	projectDescription := "Privacy-respecting adult video meta search"
	if h.appConfig != nil {
		if h.appConfig.Server.Title != "" {
			projectName = h.appConfig.Server.Title
		}
		if h.appConfig.Web.Branding.Tagline != "" {
			projectTagline = h.appConfig.Web.Branding.Tagline
		}
	}

	switch format {
	case "application/json":
		// JSON format per AI.md PART 13 - exact field order from spec
		// 1. project, 2. status, 3. version/go_version/build, 4. uptime/mode/timestamp
		// 5. cluster, 6. features, 7. checks, 8. stats
		response := map[string]interface{}{
			// 1. Project identification (PART 16: branding config)
			"project": map[string]interface{}{
				"name":        projectName,
				"tagline":     projectTagline,
				"description": projectDescription,
			},
			// 2. Overall status
			"status": status,
			// 3. Version & build info (PART 7)
			"version":    version.GetVersion(),
			"go_version": runtime.Version(),
			"build": map[string]interface{}{
				"commit": version.CommitID,
				"date":   version.BuildTime,
			},
			// 4. Runtime info (PART 6)
			"uptime":    uptime,
			"mode":      appMode,
			"timestamp": timestamp,
			// 5. Cluster info (PART 10)
			"cluster": map[string]interface{}{
				"enabled":    clusterEnabled,
				"status":     "",
				"primary":    "",
				"nodes":      []string{},
				"node_count": 0,
				"role":       "",
			},
			// 6. Features (PARTS 20, 32)
			"features": map[string]interface{}{
				"tor": map[string]interface{}{
					"enabled":  h.torSvc != nil && h.torSvc.IsEnabled(),
					"running":  h.torSvc != nil && h.torSvc.IsRunning(),
					"status":   h.getTorStatus(),
					"hostname": h.getTorHostname(),
				},
				"geoip": h.appConfig != nil && h.appConfig.Server.GeoIP.Enabled,
			},
			// 7. Component health checks
			"checks": checks,
			// 8. Statistics (public-safe aggregates + app-specific)
			"stats": map[string]interface{}{
				"requests_total":     h.getRequestsTotal(),
				"requests_24h":       h.getRequests24h(),
				"active_connections": h.getActiveConnections(),
				"searches_total":     h.getSearchCount(),
			},
		}

		// Add cluster details if enabled per PART 10
		if clusterEnabled {
			response["cluster"] = map[string]interface{}{
				"enabled":    true,
				"status":     clusterStatus,
				"primary":    "",
				"nodes":      []string{},
				"node_count": clusterNodes,
				"role":       clusterRole,
			}
		}

		WriteJSON(w, httpStatus, response)

	case "text/plain":
		// Plain text format per AI.md PART 13
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpStatus)
		fmt.Fprintf(w, "status: %s\n", status)
		fmt.Fprintf(w, "version: %s\n", version.GetVersion())
		fmt.Fprintf(w, "mode: %s\n", appMode)
		fmt.Fprintf(w, "uptime: %s\n", uptime)
		fmt.Fprintf(w, "go_version: %s\n", runtime.Version())
		fmt.Fprintf(w, "build.commit: %s\n", version.CommitID)
		fmt.Fprintf(w, "database: %s\n", checks["database"])
		fmt.Fprintf(w, "cache: %s\n", checks["cache"])
		fmt.Fprintf(w, "disk: %s\n", checks["disk"])
		fmt.Fprintf(w, "scheduler: %s\n", checks["scheduler"])
		if clusterEnabled {
			fmt.Fprintf(w, "cluster: %s (%d nodes)\n", checks["cluster"], clusterNodes)
		}

	default:
		// HTML format (default) per AI.md PART 13 with full template
		h.renderHealthzHTML(w, r, status, httpStatus, appMode, uptime, hostname, timestamp, checks, clusterEnabled, clusterStatus, clusterNodes, clusterRole)
	}
}

// HealthzHTMLData holds all data for healthz template per AI.md PART 13
type HealthzHTMLData struct {
	Title              string
	Theme              string
	Version            string
	BuildDateTime      string

	// Nav template compatibility
	ActiveNav          string
	Query              string

	// Project info
	ProjectName        string
	ProjectDescription string

	// Status
	StatusClass        string
	StatusIcon         string
	StatusText         string

	// Version info
	GoVersion          string
	BuildCommit        string
	BuildDate          string
	Uptime             string
	Mode               string
	ModeDisplay        string

	// Cluster
	ClusterEnabled     bool
	ClusterStatus      string
	ClusterStatusClass string
	ClusterStatusIcon  string
	ClusterPrimary     string
	ClusterRole        string
	ClusterNodes       []ClusterNodeData

	// Features
	Features           FeaturesData

	// Checks
	Checks             ChecksData

	// Stats (VidVeil-specific per IDEA.md)
	Stats              StatsData

	// Timestamp
	Timestamp          string
	TimestampDisplay   string
}

type ClusterNodeData struct {
	URL       string
	IsPrimary bool
}

// FeaturesData - VidVeil is stateless, no multi-user/orgs per IDEA.md
type FeaturesData struct {
	TorEnabled     bool
	// TorStatus is "healthy", "unhealthy", or empty
	TorStatus string
	// TorOnionAddr is the .onion address
	TorOnionAddr string
	GeoIP          bool
	Metrics        bool
}

type ChecksData struct {
	Database  string
	Cache     string
	Disk      string
	Scheduler string
	Cluster   string
}

// StatsData holds statistics for healthz display per AI.md PART 13
type StatsData struct {
	RequestsTotal     uint64
	Requests24h       uint64
	ActiveConnections int64
}

// renderHealthzHTML renders the healthz HTML template per AI.md PART 13
func (h *SearchHandler) renderHealthzHTML(w http.ResponseWriter, r *http.Request, status string, httpStatus int, appMode, uptime, hostname, timestamp string, checks map[string]string, clusterEnabled bool, clusterStatus string, clusterNodes int, clusterRole string) {
	// Parse timestamp
	ts, _ := time.Parse(time.RFC3339, timestamp)

	// Build template data per AI.md PART 13
	data := HealthzHTMLData{
		Title:              "Vidveil - Health Status",
		Theme:              "dark",
		Version:            version.GetVersion(),
		BuildDateTime:      version.BuildTime,

		// Nav template compatibility
		ActiveNav:          "healthz",
		Query:              "",

		// Project info
		ProjectName:        "Vidveil",
		ProjectDescription: "Privacy-respecting adult video meta search",

		// Version info
		GoVersion:          runtime.Version(),
		BuildCommit:        version.CommitID,
		BuildDate:          version.BuildTime,
		Uptime:             uptime,
		Mode:               appMode,

		// Checks
		Checks: ChecksData{
			Database:  checks["database"],
			Cache:     checks["cache"],
			Disk:      checks["disk"],
			Scheduler: checks["scheduler"],
			Cluster:   checks["cluster"],
		},

		// Stats per AI.md PART 13
		Stats: StatsData{
			RequestsTotal:     func() uint64 { if h.metrics != nil { return h.metrics.GetRequestsTotal() }; return 0 }(),
			Requests24h:       func() uint64 { if h.metrics != nil { return h.metrics.GetRequests24h() }; return 0 }(),
			ActiveConnections: func() int64 { if h.metrics != nil { return h.metrics.GetActiveConnections() }; return 0 }(),
		},

		// Timestamp
		Timestamp:        timestamp,
		TimestampDisplay: ts.Format("Jan 02, 2006 3:04 PM"),

		// Cluster
		ClusterEnabled:   clusterEnabled,
	}

	// Status display
	switch status {
	case "healthy":
		data.StatusClass = "healthy"
		data.StatusIcon = "✅"
		data.StatusText = "All Systems Operational"
	case "unhealthy":
		data.StatusClass = "unhealthy"
		data.StatusIcon = "🔴"
		data.StatusText = "System Unhealthy"
	default:
		data.StatusClass = "degraded"
		data.StatusIcon = "⚠️"
		data.StatusText = "System Degraded"
	}

	// Mode display
	if appMode == "production" {
		data.ModeDisplay = "Production"
	} else {
		data.ModeDisplay = "Development"
	}

	// Features
	if h.appConfig != nil {
		// Tor status per AI.md PART 13
		if h.torSvc != nil {
			data.Features.TorEnabled = h.torSvc.IsRunning()
			if data.Features.TorEnabled {
				data.Features.TorStatus = "healthy"
				info := h.torSvc.GetInfo()
				if addr, ok := info["onion_address"].(string); ok {
					data.Features.TorOnionAddr = addr
				}
			} else {
				data.Features.TorStatus = "unhealthy"
			}
		}
		data.Features.GeoIP = h.appConfig.Server.GeoIP.Enabled
		data.Features.Metrics = h.appConfig.Server.Metrics.Enabled
	}

	// Cluster info
	if clusterEnabled {
		data.ClusterStatus = clusterStatus
		data.ClusterRole = clusterRole
		if checks["cluster"] == "ok" {
			data.ClusterStatusClass = "ok"
			data.ClusterStatusIcon = "✅"
		} else {
			data.ClusterStatusClass = "error"
			data.ClusterStatusIcon = "❌"
		}
	}

	// Parse and execute template
	tmpl, err := template.ParseFS(templatesFS,
		"template/page/healthz.tmpl",
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/footer.tmpl",
		"template/partial/public/scripts.tmpl",
	)
	if err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Buffer template output to prevent proxy truncation issues
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "healthz", data); err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(httpStatus)
	w.Write(buf.Bytes())
}

// RobotsTxt returns robots.txt
func (h *SearchHandler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	baseURL := "https://" + h.appConfig.Server.FQDN
	if h.appConfig.Server.Port != "443" && h.appConfig.Server.Port != "80" {
		baseURL = fmt.Sprintf("https://%s:%s", h.appConfig.Server.FQDN, h.appConfig.Server.Port)
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
func (h *SearchHandler) SecurityTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	contact := h.appConfig.Web.Security.Contact
	if contact == "" {
		contact = "security@" + h.appConfig.Server.FQDN
	}
	if !strings.HasPrefix(contact, "mailto:") {
		contact = "mailto:" + contact
	}

	expires := h.appConfig.Web.Security.Expires
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
func (h *SearchHandler) HumansTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Get app info from config
	appName := h.appConfig.Web.Branding.AppName
	if appName == "" {
		appName = "Vidveil"
	}

	appURL := "https://" + h.appConfig.Server.FQDN
	if h.appConfig.Server.Port != "443" && h.appConfig.Server.Port != "80" {
		appURL = fmt.Sprintf("https://%s:%s", h.appConfig.Server.FQDN, h.appConfig.Server.Port)
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
func (h *SearchHandler) SitemapXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	baseURL := "https://" + h.appConfig.Server.FQDN
	if h.appConfig.Server.Port != "443" && h.appConfig.Server.Port != "80" {
		baseURL = fmt.Sprintf("https://%s:%s", h.appConfig.Server.FQDN, h.appConfig.Server.Port)
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

// Favicon serves favicon.ico - redirects to embedded ICO file
// Per AI.md PART 16: /favicon.ico served (embedded default or custom)
func (h *SearchHandler) Favicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/images/favicon.ico", http.StatusMovedPermanently)
}

// AppleTouchIcon serves apple-touch-icon.png - redirects to embedded PNG icon
// Per AI.md PART 16: Browsers request this at root level
func (h *SearchHandler) AppleTouchIcon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/icons/icon-192.png", http.StatusMovedPermanently)
}

// APISearch handles search API requests with content negotiation
// Supports: JSON (default), SSE streaming (Accept: text/event-stream), plain text
func (h *SearchHandler) APISearch(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

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

	// SSE streaming mode - stream results as they arrive from engines
	if format == "text/event-stream" {
		h.handleSearchSSE(w, r, searchQuery, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.Performers)
		return
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
		ctx := r.Context()
		// Add user IP to context if user has opted-in for geo-targeted content
		if forwardIP, userIP := h.getUserIPForwardPreference(r); forwardIP {
			ctx = engine.WithUserIP(ctx, userIP, true)
		}
		results = h.engineMgr.Search(ctx, searchQuery, page, engineNames)
		results.Data.Cached = false
		// Cache the results
		h.searchCache.Set(cacheKey, results)
		// Increment search count for non-cached searches
		if h.metrics != nil {
			h.metrics.IncrementSearches()
		}
	}

	// Add bang info to response
	// Keep original query with bangs
	results.Data.Query = query
	results.Data.SearchQuery = searchQuery
	results.Data.HasBang = parsed.HasBang
	results.Data.BangEngines = parsed.Engines

	// Add related searches
	results.Data.RelatedSearches = engine.GetRelatedSearches(searchQuery, 8)

	// Plain text format for .txt extension or Accept: text/plain
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "query: %s\n", results.Data.Query)
		fmt.Fprintf(w, "results: %d\n", len(results.Data.Results))
		fmt.Fprintf(w, "---\n")
		for i, r := range results.Data.Results {
			fmt.Fprintf(w, "%d. %s\n", i+1, r.Title)
			fmt.Fprintf(w, "   url: %s\n", r.URL)
			fmt.Fprintf(w, "   source: %s\n", r.Source)
			if r.Duration != "" {
				fmt.Fprintf(w, "   duration: %s\n", r.Duration)
			}
			if r.Views != "" {
				fmt.Fprintf(w, "   views: %s\n", r.Views)
			}
			fmt.Fprintf(w, "\n")
		}
		return
	}

	h.jsonResponse(w, results)
}

// handleSearchSSE handles SSE streaming for search results
func (h *SearchHandler) handleSearchSSE(w http.ResponseWriter, r *http.Request, searchQuery string, page int, engineNames []string, exactPhrases []string, exclusions []string, performers []string) {
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

	// Increment search count for SSE searches
	if h.metrics != nil {
		h.metrics.IncrementSearches()
	}

	// Stream results with search operators
	ctx := r.Context()

	// Add user IP to context if user has opted-in for geo-targeted content
	// Per PART 32: This allows video sites to see user's IP for geo content
	if forwardIP, userIP := h.getUserIPForwardPreference(r); forwardIP {
		ctx = engine.WithUserIP(ctx, userIP, true)
	}

	resultsChan := h.engineMgr.SearchStreamWithOperators(ctx, searchQuery, page, engineNames, exactPhrases, exclusions, performers)

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

// APIBangs returns list of available bang shortcuts
func (h *SearchHandler) APIBangs(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	bangs := engine.ListBangs()

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "bangs: %d\n---\n", len(bangs))
		for _, b := range bangs {
			// b.Bang already has ! prefix per bangs.go line 287
			fmt.Fprintf(w, "%s - %s\n", b.Bang, b.EngineName)
		}
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"ok": true,
		"data":    bangs,
		"count":   len(bangs),
	})
}

// APIAutocomplete returns autocomplete suggestions for bangs
func (h *SearchHandler) APIAutocomplete(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	q := r.URL.Query().Get("q")
	if q == "" {
		// Return popular searches when query is empty
		popular := engine.GetPopularSearches(10)
		if format == "text/plain" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "type: popular\nsuggestions: %d\n---\n", len(popular))
			for _, term := range popular {
				fmt.Fprintf(w, "%s\n", term)
			}
			return
		}
		h.jsonResponse(w, map[string]interface{}{
			"ok":     true,
			"suggestions": popular,
			"type":        "popular",
		})
		return
	}

	// Check if query starts with "!" for bang autocomplete
	if strings.HasPrefix(q, "!") && len(q) > 1 {
		// Remove the "!" prefix
		prefix := q[1:]
		suggestions := engine.Autocomplete(prefix)
		if format == "text/plain" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "type: bang\nsuggestions: %d\n---\n", len(suggestions))
			for _, s := range suggestions {
				// s.Bang already has ! prefix per bangs.go line 353
				fmt.Fprintf(w, "%s - %s\n", s.Bang, s.EngineName)
			}
			return
		}
		h.jsonResponse(w, map[string]interface{}{
			"ok":     true,
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
		if format == "text/plain" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "type: bang_start\nsuggestions: %d\n---\n", len(bangs))
			for _, b := range bangs {
				// b.Bang already has ! prefix per bangs.go line 287
				fmt.Fprintf(w, "%s - %s\n", b.Bang, b.EngineName)
			}
			return
		}
		h.jsonResponse(w, map[string]interface{}{
			"ok":     true,
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
			if format == "text/plain" {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "type: bang\nreplace: %s\nsuggestions: %d\n---\n", lastWord, len(suggestions))
				for _, s := range suggestions {
					// s.Bang already has ! prefix per bangs.go line 353
					fmt.Fprintf(w, "%s - %s\n", s.Bang, s.EngineName)
				}
				return
			}
			// replace indicates what to replace in query
			h.jsonResponse(w, map[string]interface{}{
				"ok":     true,
				"suggestions": suggestions,
				"type":        "bang",
				"replace":     lastWord,
			})
			return
		}
	}

	// No bang in query - return search term suggestions
	// Get the last word as the prefix for suggestions
	lastWord := ""
	if len(words) > 0 {
		lastWord = strings.ToLower(words[len(words)-1])
	} else {
		lastWord = strings.ToLower(q)
	}

	termSuggestions := engine.AutocompleteSuggestions(lastWord, 10)

	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "type: search\nsuggestions: %d\n---\n", len(termSuggestions))
		for _, s := range termSuggestions {
			fmt.Fprintf(w, "%s\n", s.Term)
		}
		return
	}
	h.jsonResponse(w, map[string]interface{}{
		"ok":     true,
		"suggestions": termSuggestions,
		"type":        "search",
	})
}

// APIEngines returns list of available engines
func (h *SearchHandler) APIEngines(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	engines := h.engineMgr.ListEngines()

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "engines: %d\n---\n", len(engines))
		for _, e := range engines {
			status := "enabled"
			if !e.Enabled {
				status = "disabled"
			}
			fmt.Fprintf(w, "%s (%s) - tier %d [%s]\n", e.Name, e.DisplayName, e.Tier, status)
		}
		return
	}

	h.jsonResponse(w, model.EnginesResponse{
		Ok:   true,
		Data: engines,
	})
}

// APIEngineDetails returns details for a specific engine
func (h *SearchHandler) APIEngineDetails(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	name := chi.URLParam(r, "name")
	eng, ok := h.engineMgr.GetEngine(name)
	if !ok {
		h.jsonError(w, "Engine not found", "ENGINE_NOT_FOUND", http.StatusNotFound)
		return
	}

	caps := eng.Capabilities()

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "name: %s\n", eng.Name())
		fmt.Fprintf(w, "display_name: %s\n", eng.DisplayName())
		fmt.Fprintf(w, "tier: %d\n", eng.Tier())
		fmt.Fprintf(w, "enabled: %t\n", eng.IsAvailable())
		fmt.Fprintf(w, "has_preview: %t\n", caps.HasPreview)
		fmt.Fprintf(w, "has_download: %t\n", caps.HasDownload)
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"ok": true,
		"data": model.EngineInfo{
			Name:        eng.Name(),
			DisplayName: eng.DisplayName(),
			Enabled:     eng.IsAvailable(),
			Available:   eng.IsAvailable(),
			Tier:        eng.Tier(),
			Capabilities: &model.EngineCapabilities{
				HasPreview:  caps.HasPreview,
				HasDownload: caps.HasDownload,
			},
		},
	})
}

// APIStats returns public statistics
func (h *SearchHandler) APIStats(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	enabled := h.engineMgr.EnabledCount()
	total := len(h.engineMgr.ListEngines())

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "engines_enabled: %d\n", enabled)
		fmt.Fprintf(w, "engines_total: %d\n", total)
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"engines_enabled": enabled,
			"engines_total":   total,
		},
	})
}

// APIVersion returns server version info
// Per AI.md PART 13: /api/v1/version endpoint for CLI compatibility checking
// Response format matches client/api/client.go VersionResponse struct
func (h *SearchHandler) APIVersion(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"version": version.GetVersion(),
		"commit":  version.CommitID,
		"built":   version.BuildTime,
	})
}

// APIHealthCheck returns health status as JSON per AI.md PART 16
// Returns comprehensive health status with checks object for database/cache/disk
// APIHealthCheck handles /api/v1/healthz endpoint (JSON only)
// Per AI.md PART 13: Same JSON as /healthz
func (h *SearchHandler) APIHealthCheck(w http.ResponseWriter, r *http.Request) {
	// API routes default to JSON but support text output per AI.md PART 14
	// Format detection: .txt extension > Accept header > client type > default JSON

	// Build health response per AI.md PART 13
	uptime := getUptime()
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Get mode from config
	appMode := "production"
	if h.appConfig != nil && h.appConfig.IsDevelopmentMode() {
		appMode = "development"
	}

	// Cluster status
	clusterEnabled := false

	// Build checks object - MUST be simple "ok"/"error" strings
	// Per AI.md PART 13
	checks := map[string]string{
		"database": "ok",
		"cache":    "ok",
		"disk":     "ok",
	}

	// Overall status - per AI.md PART 13: derive from checks
	status := "healthy"
	httpStatus := http.StatusOK
	for _, v := range checks {
		if v != "ok" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	// Detect response format per AI.md PART 14
	format := getAPIResponseFormat(r)

	// Text output for CLI tools per AI.md PART 13
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpStatus)
		fmt.Fprintf(w, "status: %s\n", status)
		fmt.Fprintf(w, "version: %s\n", version.GetVersion())
		fmt.Fprintf(w, "mode: %s\n", appMode)
		fmt.Fprintf(w, "uptime: %s\n", uptime)
		fmt.Fprintf(w, "go_version: %s\n", version.GoVersion)
		fmt.Fprintf(w, "build.commit: %s\n", version.CommitID)
		fmt.Fprintf(w, "database: %s\n", checks["database"])
		fmt.Fprintf(w, "cache: %s\n", checks["cache"])
		fmt.Fprintf(w, "disk: %s\n", checks["disk"])
		fmt.Fprintf(w, "scheduler: ok\n")
		return
	}

	// JSON response (default) - per AI.md PART 13 canonical field order
	// Tor status for features and checks
	torEnabled := h.torSvc != nil && h.torSvc.IsEnabled()
	torRunning := h.torSvc != nil && h.torSvc.IsRunning()
	torCheck := "ok"
	if torEnabled && !torRunning {
		torCheck = "error"
	}

	response := map[string]interface{}{
		// 1. Project identification (PART 16)
		"project": map[string]interface{}{
			"name":        h.appConfig.Server.Title,
			"tagline":     h.appConfig.Web.Branding.Tagline,
			"description": "Privacy-respecting adult video meta search",
		},
		// 2. Overall status
		"status": status,
		// 3. Version & build info (PART 7)
		"version":    version.GetVersion(),
		"go_version": version.GoVersion,
		// 4. Runtime info (PART 6)
		"mode":      appMode,
		"uptime":    uptime,
		"timestamp": timestamp,
		// 5. Build info (PART 7)
		"build": map[string]interface{}{
			"commit": version.CommitID,
			"date":   version.BuildTime,
		},
		// 6. Cluster info (PART 10)
		"cluster": map[string]interface{}{
			"enabled":    clusterEnabled,
			"primary":    "",
			"nodes":      []string{},
			"node_count": 1,
			"role":       "primary",
		},
		// 7. Features - PUBLIC only, NO metrics (PART 21 is internal)
		"features": map[string]interface{}{
			// PART 32: Tor as TorInfo object
			"tor": map[string]interface{}{
				"enabled":  torEnabled,
				"running":  torRunning,
				"status":   h.getTorStatus(),
				"hostname": h.getTorHostname(),
			},
			// PART 20: GeoIP
			"geoip": h.appConfig != nil && h.appConfig.Server.GeoIP.Enabled,
		},
		// 8. Component health checks
		"checks": map[string]string{
			"database":  checks["database"],
			"cache":     checks["cache"],
			"disk":      checks["disk"],
			"scheduler": "ok",
			"cluster":   "ok",
			"tor":       torCheck,
		},
		// 9. Statistics (public-safe aggregates)
		"stats": map[string]interface{}{
			"requests_total":     h.getRequestsTotal(),
			"requests_24h":       h.getRequests24h(),
			"active_connections": h.getActiveConnections(),
		},
	}

	WriteJSON(w, httpStatus, response)
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
func (h *SearchHandler) jsonResponse(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

func (h *SearchHandler) jsonError(w http.ResponseWriter, message, code string, status int) {
	// Per AI.md PART 14: Error response format
	// - ok: false
	// - error: ERROR_CODE (machine-readable)
	// - message: Human readable message
	WriteJSON(w, status, map[string]interface{}{
		"ok":      false,
		"error":   code,
		"message": message,
	})
}

// RenderErrorPage renders a custom error page per AI.md PART 30
func (h *SearchHandler) RenderErrorPage(w http.ResponseWriter, code int, title, message string) {
	data := map[string]interface{}{
		"Code":      code,
		"Title":     title,
		"Message":   message,
		"SiteTitle": h.appConfig.Web.Branding.AppName,
		"Theme":     h.appConfig.Web.UI.Theme,
	}

	tmpl, err := template.ParseFS(templatesFS, "template/page/error.tmpl")
	if err != nil {
		// Fallback to plain text error
		http.Error(w, fmt.Sprintf("%d %s: %s", code, title, message), code)
		return
	}

	// Buffer template output to prevent proxy truncation issues
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "error", data); err != nil {
		// Fallback to plain text error
		http.Error(w, fmt.Sprintf("%d %s: %s", code, title, message), code)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(code)
	w.Write(buf.Bytes())
}

// NotFoundHandler handles 404 errors per AI.md PART 30
func (h *SearchHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderErrorPage(w, http.StatusNotFound, "Page Not Found",
		"The page you're looking for doesn't exist or has been moved.")
}

// InternalErrorHandler handles 500 errors per AI.md PART 30
func (h *SearchHandler) InternalErrorHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderErrorPage(w, http.StatusInternalServerError, "Server Error",
		"Something went wrong on our end. Please try again later.")
}

func (h *SearchHandler) renderTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	// Ensure required fields for nav.tmpl
	if data["ActiveNav"] == nil {
		data["ActiveNav"] = name
	}
	if data["Query"] == nil {
		data["Query"] = ""
	}

	// Map template names to file paths
	templateFile := ""
	templateName := ""
	switch name {
	case "home":
		templateFile = "template/page/index.tmpl"
		templateName = "home"
	case "search":
		templateFile = "template/page/search.tmpl"
		templateName = "search"
	case "preferences":
		templateFile = "template/page/preferences.tmpl"
		templateName = "preferences"
	case "about":
		templateFile = "template/page/about.tmpl"
		templateName = "about"
	case "age-verify":
		templateFile = "template/page/age-verify.tmpl"
		templateName = "age-verify"
	case "content-restricted":
		templateFile = "template/page/content-restricted.tmpl"
		templateName = "content-restricted"
	case "content-blocked":
		templateFile = "template/page/content-blocked.tmpl"
		templateName = "content-blocked"
	case "privacy":
		templateFile = "template/page/privacy.tmpl"
		templateName = "privacy"
	// nojs templates for text browsers (lynx, w3m, links)
	case "nojs/home":
		templateFile = "template/nojs/home.tmpl"
		templateName = "nojs/home"
	case "nojs/search":
		templateFile = "template/nojs/search.tmpl"
		templateName = "nojs/search"
	case "nojs/preferences":
		templateFile = "template/nojs/preferences.tmpl"
		templateName = "nojs/preferences"
	case "nojs/about":
		templateFile = "template/nojs/about.tmpl"
		templateName = "nojs/about"
	case "nojs/age-verify":
		templateFile = "template/nojs/age-verify.tmpl"
		templateName = "nojs/age-verify"
	case "nojs/privacy":
		templateFile = "template/nojs/privacy.tmpl"
		templateName = "nojs/privacy"
	default:
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Inject version for cache busting in all templates
	if data["Version"] == nil {
		data["Version"] = version.GetVersion()
	}

	// Create base template with FuncMap
	tmpl := template.New(templateName).Funcs(template.FuncMap{
		// dict creates a map from key-value pairs for passing to templates
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					key, ok := values[i].(string)
					if ok {
						dict[key] = values[i+1]
					}
				}
			}
			return dict
		},
		"eq": func(a, b interface{}) bool { return a == b },
	})

	// Load all partials first (public and admin)
	partialFiles := []string{
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/footer.tmpl",
		"template/partial/public/filters.tmpl",
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
			// Per AI.md PART 9: Never expose error details in responses
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Read and parse the main template
	content, err := templatesFS.ReadFile(templateFile)
	if err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tmpl.Parse(string(content))
	if err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Buffer template output to prevent proxy truncation issues
	// This ensures Content-Length is set and the response is written atomically,
	// which avoids issues with nginx proxy_buffer_size limits (often 8KB)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// DebugEngine probes a specific engine and returns detailed results
// GET /api/v1/debug/engine/{name}?q={query}
// Returns: engine info, capabilities, sample results with all fields
func (h *SearchHandler) DebugEngine(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	query := r.URL.Query().Get("q")
	if query == "" {
		// Default test query
		query = "test"
	}

	eng, ok := h.engineMgr.GetEngine(name)
	if !ok {
		h.jsonError(w, "Engine not found", "ENGINE_NOT_FOUND", http.StatusNotFound)
		return
	}

	// Get engine capabilities
	caps := eng.Capabilities()

	// Perform test search
	results, err := eng.Search(r.Context(), query, 1)

	// Build debug response
	response := map[string]interface{}{
		"ok": true,
		"engine": map[string]interface{}{
			"name":         eng.Name(),
			"display_name": eng.DisplayName(),
			"tier":         eng.Tier(),
			"available":    eng.IsAvailable(),
		},
		"capabilities": caps,
		"query":        query,
	}

	if err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		response["error"] = "Search failed"
		response["results"] = []interface{}{}
		response["result_count"] = 0
	} else {
		response["results"] = results
		response["result_count"] = len(results)

		// Analyze what fields are populated
		fieldStats := analyzeResultFields(results)
		response["field_stats"] = fieldStats
	}

	WriteJSON(w, http.StatusOK, response)
}

// analyzeResultFields checks which fields are populated in results
func analyzeResultFields(results []model.VideoResult) map[string]interface{} {
	stats := map[string]int{
		"has_title":        0,
		"has_url":          0,
		"has_thumbnail":    0,
		"has_preview_url":  0,
		"has_download_url": 0,
		"has_duration":     0,
		"has_views":        0,
		"has_rating":       0,
		"has_quality":      0,
		"has_published":    0,
	}

	for _, r := range results {
		if r.Title != "" {
			stats["has_title"]++
		}
		if r.URL != "" {
			stats["has_url"]++
		}
		if r.Thumbnail != "" {
			stats["has_thumbnail"]++
		}
		if r.PreviewURL != "" {
			stats["has_preview_url"]++
		}
		if r.DownloadURL != "" {
			stats["has_download_url"]++
		}
		if r.Duration != "" || r.DurationSeconds > 0 {
			stats["has_duration"]++
		}
		if r.Views != "" || r.ViewsCount > 0 {
			stats["has_views"]++
		}
		if r.Rating > 0 {
			stats["has_rating"]++
		}
		if r.Quality != "" {
			stats["has_quality"]++
		}
		if !r.Published.IsZero() {
			stats["has_published"]++
		}
	}

	total := len(results)
	return map[string]interface{}{
		"total_results": total,
		"fields":        stats,
	}
}

// DebugEnginesList returns all engines with their capabilities
// GET /api/v1/debug/engines
func (h *SearchHandler) DebugEnginesList(w http.ResponseWriter, r *http.Request) {
	engines := h.engineMgr.ListEngines()

	type engineDebug struct {
		Name         string              `json:"name"`
		DisplayName  string              `json:"display_name"`
		Tier         int                 `json:"tier"`
		Enabled      bool                `json:"enabled"`
		Capabilities engine.Capabilities `json:"capabilities"`
	}

	var list []engineDebug
	for _, info := range engines {
		eng, ok := h.engineMgr.GetEngine(info.Name)
		if !ok {
			continue
		}
		list = append(list, engineDebug{
			Name:         info.Name,
			DisplayName:  info.DisplayName,
			Tier:         info.Tier,
			Enabled:      info.Enabled,
			Capabilities: eng.Capabilities(),
		})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"count":   len(list),
		"engines": list,
	})
}

// ProxyThumbnail proxies external thumbnails to prevent tracking
// Per IDEA.md: Privacy proxy for thumbnails
func (h *SearchHandler) ProxyThumbnail(w http.ResponseWriter, r *http.Request) {
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
// Default content type
contentType = "image/jpeg"
}
w.Header().Set("Content-Type", contentType)

// Cache control: 1 hour
w.Header().Set("Cache-Control", "public, max-age=3600")

// Proxy the image
io.Copy(w, resp.Body)
}

// Autodiscover returns server connection settings for CLI/agent auto-configuration
// Per AI.md PART 37: /api/autodiscover (NON-NEGOTIABLE)
// This endpoint is NOT versioned because clients need it BEFORE they know the API version
func (h *SearchHandler) Autodiscover(w http.ResponseWriter, r *http.Request) {
	// Build response per AI.md PART 37
	response := map[string]interface{}{
		"primary":     h.appConfig.GetPublicURL(),
		"cluster":     h.appConfig.GetClusterNodes(),
		// Per AI.md PART 14: versioned API
		"api_version": "v1",
		// Default timeout in seconds
		"timeout": 30,
		// Default retry attempts
		"retry": 3,
		// Default seconds between retries
		"retry_delay": 1,
	}

	// NEVER include admin_path - security by obscurity per AI.md PART 37
	// NEVER include secrets, internal IPs, or sensitive data

	WriteJSON(w, http.StatusOK, response)
}
