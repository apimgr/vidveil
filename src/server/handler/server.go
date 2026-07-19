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
	data := map[string]interface{}{
		"Title":          appName,
		"AppName":        appName,
		"AppDescription": h.appConfig.Server.Branding.Description,
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
	})
}

// handleContactSubmit handles contact form submission
func (h *ServerHandler) handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	// Parse form and show success message
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
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
		"description": h.appConfig.Server.Branding.Description,
		"version":     version.GetVersion(),
		"features": []string{
			"Privacy-focused video meta-search",
			"No tracking or personal data collection",
			"Aggregates results from multiple sources",
			"Open source and self-hostable",
		},
	}
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "name: %s\ndescription: %s\nversion: %s\n",
			data["name"], data["description"], data["version"])
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": data})
}

// APIPrivacy handles GET /api/v1/server/privacy
// Per AI.md PART 14: content negotiation required on every API route.
func (h *ServerHandler) APIPrivacy(w http.ResponseWriter, r *http.Request) {
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "policy_version: 1.0\nlast_updated: %s\nsearch_queries: false\nip_addresses: false\ntracking_cookies: false\nthird_party_sharing: false\n",
			time.Now().Format("2006-01-02"))
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"policy_version": "1.0",
			"last_updated":   time.Now().Format("2006-01-02"),
			"data_collection": map[string]interface{}{
				"search_queries":      false,
				"ip_addresses":        false,
				"tracking_cookies":    false,
				"third_party_sharing": false,
			},
			"cookies": []string{
				"age_verification (required)",
				"user_preferences (optional)",
			},
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
		fmt.Fprintf(w, "search: GET /search or /api/v1/search  params: q, page, engines\n")
		fmt.Fprintf(w, "engines: GET /api/v1/engines\n")
		fmt.Fprintf(w, "health: GET /api/v1/server/healthz\n")
		fmt.Fprintf(w, "documentation: /server/docs/swagger\n")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"endpoint":    "/search or /api/v1/search",
				"method":      "GET",
				"parameters":  []string{"q (query)", "page", "engines"},
				"description": "Search across multiple video sources",
			},
			"engines": map[string]interface{}{
				"endpoint":    "/api/v1/engines",
				"method":      "GET",
				"description": "List available search engines",
			},
			"health": map[string]interface{}{
				"endpoint":    "/api/v1/server/healthz",
				"method":      "GET",
				"description": "Check server health status",
			},
			"documentation": "/server/docs/swagger",
		},
	})
}
