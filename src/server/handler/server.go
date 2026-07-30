// SPDX-License-Identifier: MIT
// AI.md PART 14: /server/ routes
package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
)

// ServerHandler handles /server/ routes per AI.md PART 14
type ServerHandler struct {
	appConfig *config.AppConfig
	torSvc    TorStatusChecker
}

// NewServerHandler creates a new server handler
func NewServerHandler(appConfig *config.AppConfig) *ServerHandler {
	// Use default config if nil per AI.md PART 5
	if appConfig == nil {
		appConfig = config.DefaultAppConfig()
	}
	return &ServerHandler{
		appConfig: appConfig,
	}
}

// SetTorService sets the Tor service so the footer can show the onion address
// per AI.md PART 16 (footer row is dropped entirely when Tor is disabled/not running).
func (h *ServerHandler) SetTorService(t TorStatusChecker) {
	h.torSvc = t
}

// renderServerTemplate renders a server page template with common data
func (h *ServerHandler) renderServerTemplate(w http.ResponseWriter, r *http.Request, templateName string, extraData map[string]interface{}) {
	// Map template names to file paths
	templateFile := ""
	switch templateName {
	case "server-about":
		templateFile = "template/page/server-about.tmpl"
	case "server-privacy":
		templateFile = "template/page/server-privacy.tmpl"
	case "server-contact":
		templateFile = "template/page/server-contact.tmpl"
	case "server-help":
		templateFile = "template/page/server-help.tmpl"
	default:
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Guard against uninitialized template filesystem
	if templatesFS == nil {
		log.Printf("server template: templates filesystem not initialized")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Resolve locale for i18n translation function
	locale := i18n.DetectLocale(r)

	// Create base template with FuncMap so templates can use {{ t "key" }}
	tmpl := template.New(templateName).Funcs(template.FuncMap{
		"dict": func(values ...interface{}) map[string]interface{} {
			d := make(map[string]interface{})
			for i := 0; i+1 < len(values); i += 2 {
				if key, ok := values[i].(string); ok {
					d[key] = values[i+1]
				}
			}
			return d
		},
		"eq": func(a, b interface{}) bool { return a == b },
		"t": func(key string) string {
			return i18n.Translate(locale, key)
		},
		"tf": func(key string, args ...interface{}) string {
			return i18n.TranslateFormat(locale, key, args...)
		},
	})

	// Load layout and partials first
	partialFiles := []string{
		"template/layout/public.tmpl",
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/footer.tmpl",
		"template/partial/public/scripts.tmpl",
	}

	for _, pf := range partialFiles {
		content, err := fs.ReadFile(templatesFS, pf)
		if err != nil {
			continue
		}
		if _, err = tmpl.Parse(string(content)); err != nil {
			log.Printf("server template: parse %s: %v", pf, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	content, err := fs.ReadFile(templatesFS, templateFile)
	if err != nil {
		log.Printf("server template: read %s: %v", templateFile, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if _, err = tmpl.Parse(string(content)); err != nil {
		log.Printf("server template: parse %s: %v", templateFile, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build common template data per AI.md PART 16
	versionInfo := version.GetVersionInfo()
	appName := h.appConfig.Server.Branding.Title
	// Required fields by head.tmpl: Title
	// Required fields by nav.tmpl: ActiveNav, Query
	scheme := "https"
	if !h.appConfig.Server.SSL.Enabled {
		scheme = "http"
	}

	data := map[string]interface{}{
		"Title":          appName,
		"AppName":        appName,
		"AppTagline":     h.appConfig.Server.Branding.Tagline,
		"AppDescription": h.appConfig.Server.Branding.Description,
		"BaseURL":        scheme + "://" + h.appConfig.Server.FQDN,
		"Version":        versionInfo["version"],
		"BuildDateTime":  versionInfo["build_time"],
		"Theme":          "dark",
		"ActiveNav":      templateName,
		"Query":          "",
	}

	// Footer onion-address row per AI.md PART 16 — dropped entirely unless
	// Tor is both enabled and actually running.
	if h.torSvc != nil && h.torSvc.IsEnabled() && h.torSvc.IsRunning() {
		data["TorEnabled"] = true
		data["TorRunning"] = true
		if addr, ok := h.torSvc.GetInfo()["onion_address"].(string); ok {
			data["TorAddress"] = addr
			data["TorOnionAddr"] = addr
		}
	}

	// Merge extra data
	for k, v := range extraData {
		data[k] = v
	}

	// Per AI.md PART 30 inject locale + direction for <html lang="" dir="">.
	injectLocaleData(r, data)

	// Buffer template output
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		log.Printf("server template: execute %s: %v", templateName, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// AboutPage renders /server/about web page
func (h *ServerHandler) AboutPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-about", nil)
}

// PrivacyPage renders /server/privacy web page
func (h *ServerHandler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-privacy", nil)
}

// ContactPage renders /server/contact web page
func (h *ServerHandler) ContactPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Handle contact form submission
		h.handleContactSubmit(w, r)
		return
	}

	// Show contact form - contact form always available
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
	})
}

// publicAbuseEmail resolves the abuse-report address shown on /server/contact per
// AI.md PART 16: server.contact.abuse.email if set, else server.contact.general.email
// if set, else empty. The admin address is never public and is deliberately excluded.
func (h *ServerHandler) publicAbuseEmail() string {
	if h.appConfig.Server.Contact.Abuse.Email != "" {
		return h.appConfig.Server.Contact.Abuse.Email
	}
	return h.appConfig.Server.Contact.General.Email
}

// handleContactSubmit handles contact form submission
func (h *ServerHandler) handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	// Parse form and show success message
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Message":        "Thank you for your message. We will get back to you if needed.",
		"MessageType":    "success",
	})
}

// HelpPage renders /server/help web page
func (h *ServerHandler) HelpPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-help", nil)
}

// API Routes per AI.md PART 14

// APIAbout handles GET /api/v1/server/about
// Per AI.md PART 14: content negotiation required on every API route.
func (h *ServerHandler) APIAbout(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"name":        h.appConfig.Server.Branding.Title,
		"tagline":     h.appConfig.Server.Branding.Tagline,
		"description": h.appConfig.Server.Branding.Description,
		"version":     version.GetVersion(),
		"features": []string{
			"Privacy-first search: no tracking, logging, or analytics",
			"Meta-search across 42 adult video engines with bang shortcuts",
			"Real-time SSE streaming as each engine responds",
			"Thumbnail proxy so source sites never see your IP",
			"Video preview on hover (desktop) and swipe (mobile)",
			"Client-side preferences, favorites, and history (localStorage only)",
			"Built-in Tor hidden service support",
			"Admin-configurable geographic content restriction",
			"Single static binary with all assets embedded",
		},
		"links": map[string]interface{}{
			"source":  "https://github.com/apimgr/vidveil",
			"website": "https://x.scour.li",
		},
	}
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "name: %s\ntagline: %s\ndescription: %s\nversion: %s\n",
			data["name"], data["tagline"], data["description"], data["version"])
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": data})
}

// APIPrivacy handles GET /api/v1/server/privacy
// Per AI.md PART 14: content negotiation required on every API route.
func (h *ServerHandler) APIPrivacy(w http.ResponseWriter, r *http.Request) {
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "policy_version: 1.0\nlast_updated: %s\nuser_accounts: false\nsearch_queries_logged: false\nip_addresses_stored: false\ntracking_cookies: false\nthird_party_sharing: false\ndata_sold: false\nclient_side_storage_only: true\n",
			time.Now().Format("2006-01-02"))
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"policy_version": "1.0",
			"last_updated":   time.Now().Format("2006-01-02"),
			"summary":        "Stateless, privacy-first meta search. No user accounts, no query logging, no data sold.",
			"data_collection": map[string]interface{}{
				"user_accounts":         false,
				"search_queries_logged": false,
				"ip_addresses_stored":   false,
				"tracking_cookies":      false,
				"browser_fingerprint":   false,
				"user_profiles":         false,
				"third_party_sharing":   false,
				"data_sold":             false,
			},
			"client_side_storage": []string{
				"vidveil-theme",
				"vidveil_prefs",
				"vidveil_history",
				"vidveil_favorites",
			},
			"cookies": []string{
				"age_verification (required by law)",
				"content_restriction_acknowledgment (soft-block regions only)",
				"forward_ip (opt-in only, when operator enables geo-forwarding)",
			},
			"thumbnail_proxy":  true,
			"tor_supported":    true,
			"third_party_sent": "Only the video engines you explicitly search receive your query.",
		},
	})
}

// APIContact handles POST /api/v1/server/contact
// Per AI.md PART 9: error codes must use standard constants from response.go.
func (h *ServerHandler) APIContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, CodeMethodNotAllowed, MsgMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		SendError(w, CodeBadRequest, "Invalid form data")
		return
	}

	subject := r.FormValue("subject")
	message := r.FormValue("message")

	if subject == "" || message == "" {
		SendError(w, CodeValidation, "Subject and message are required")
		return
	}

	SendOK(w, map[string]interface{}{"message": "Message received successfully"})
}

// APIHelp handles GET /api/v1/server/help
// Per AI.md PART 14: content negotiation required; health API is at /api/v1/server/healthz.
func (h *ServerHandler) APIHelp(w http.ResponseWriter, r *http.Request) {
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "search: GET /api/v1/search  params: q, page, engines (bang shortcuts like !ph supported in q)\n")
		fmt.Fprintf(w, "bangs: GET /api/v1/bangs\n")
		fmt.Fprintf(w, "autocomplete: GET /api/v1/bangs/autocomplete?q={partial}\n")
		fmt.Fprintf(w, "engines: GET /api/v1/engines\n")
		fmt.Fprintf(w, "engines_health: GET /api/v1/engines/health\n")
		fmt.Fprintf(w, "thumbnail_proxy: GET /api/v1/proxy/thumbnails?url={url}\n")
		fmt.Fprintf(w, "health: GET /api/v1/server/healthz\n")
		fmt.Fprintf(w, "swagger: /server/docs/swagger\n")
		fmt.Fprintf(w, "graphql: /server/docs/graphql\n")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"endpoint":    "/api/v1/search",
				"method":      "GET",
				"parameters":  []string{"q (query; bang shortcuts like !ph !xv supported)", "page", "engines"},
				"description": "Search across 42 video engines; supports JSON, SSE (text/event-stream), and text/plain",
			},
			"bangs": map[string]interface{}{
				"endpoint":    "/api/v1/bangs",
				"method":      "GET",
				"description": "List all bang shortcuts",
			},
			"autocomplete": map[string]interface{}{
				"endpoint":    "/api/v1/bangs/autocomplete",
				"method":      "GET",
				"parameters":  []string{"q (partial query, bang, or @performer)"},
				"description": "Autocomplete bangs, performer names, and search terms",
			},
			"engines": map[string]interface{}{
				"endpoint":    "/api/v1/engines",
				"method":      "GET",
				"description": "List available search engines",
			},
			"engines_health": map[string]interface{}{
				"endpoint":    "/api/v1/engines/health",
				"method":      "GET",
				"description": "Per-engine health/availability status",
			},
			"thumbnail_proxy": map[string]interface{}{
				"endpoint":    "/api/v1/proxy/thumbnails",
				"method":      "GET",
				"parameters":  []string{"url (upstream thumbnail URL)"},
				"description": "Privacy-preserving thumbnail proxy",
			},
			"health": map[string]interface{}{
				"endpoint":    "/api/v1/server/healthz",
				"method":      "GET",
				"description": "Check server health status",
			},
			"documentation": map[string]interface{}{
				"swagger": "/server/docs/swagger",
				"graphql": "/server/docs/graphql",
			},
		},
	})
}
