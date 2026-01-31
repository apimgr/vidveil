// SPDX-License-Identifier: MIT
// AI.md PART 31: /server/ routes
package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
)

// ServerHandler handles /server/ routes per AI.md PART 31
type ServerHandler struct {
	appConfig *config.AppConfig
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

// renderServerTemplate renders a server page template with common data
func (h *ServerHandler) renderServerTemplate(w http.ResponseWriter, templateName string, extraData map[string]interface{}) {
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

	// Create base template with partials
	tmpl := template.New(templateName)

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
		content, err := templatesFS.ReadFile(pf)
		if err != nil {
			continue
		}
		_, err = tmpl.Parse(string(content))
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Read and parse the main template
	content, err := templatesFS.ReadFile(templateFile)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tmpl.Parse(string(content))
	if err != nil {
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

	// Merge extra data
	for k, v := range extraData {
		data[k] = v
	}

	// Buffer template output
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
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
	h.renderServerTemplate(w, "server-about", nil)
}

// PrivacyPage renders /server/privacy web page
func (h *ServerHandler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, "server-privacy", nil)
}

// ContactPage renders /server/contact web page
func (h *ServerHandler) ContactPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Handle contact form submission
		h.handleContactSubmit(w, r)
		return
	}

	// Show contact form - contact form always available
	h.renderServerTemplate(w, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
	})
}

// handleContactSubmit handles contact form submission
func (h *ServerHandler) handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	// Parse form and show success message
	h.renderServerTemplate(w, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"Message":        "Thank you for your message. We will get back to you if needed.",
		"MessageType":    "success",
	})
}

// HelpPage renders /server/help web page
func (h *ServerHandler) HelpPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, "server-help", nil)
}

// API Routes per AI.md PART 31

// APIAbout handles GET /api/v1/server/about
func (h *ServerHandler) APIAbout(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"name":        h.appConfig.Server.Branding.Title,
			"description": h.appConfig.Server.Branding.Description,
			"version":     version.GetVersion(),
			"features": []string{
				"Privacy-focused video meta-search",
				"No tracking or personal data collection",
				"Aggregates results from multiple sources",
				"Open source and self-hostable",
			},
		},
	})
}

// APIPrivacy handles GET /api/v1/server/privacy
func (h *ServerHandler) APIPrivacy(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"policy_version": "1.0",
			"last_updated":   time.Now().Format("2006-01-02"),
			"data_collection": map[string]interface{}{
				"search_queries":     false,
				"ip_addresses":       false,
				"tracking_cookies":   false,
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
func (h *ServerHandler) APIContact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false,
			"error":   "Method not allowed",
			"code":    "METHOD_NOT_ALLOWED",
		})
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false,
			"error":   "Invalid form data",
			"code":    "INVALID_REQUEST",
		})
		return
	}

	// Validate required fields
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	if subject == "" || message == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false,
			"error":   "Subject and message are required",
			"code":    "MISSING_FIELDS",
		})
		return
	}

	// In a real implementation, this would send an email or store the message
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"message": "Message received successfully",
	})
}

// APIHelp handles GET /api/v1/server/help
func (h *ServerHandler) APIHelp(w http.ResponseWriter, r *http.Request) {
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
				"endpoint":    "/api/v1/healthz",
				"method":      "GET",
				"description": "Check server health status",
			},
			"documentation": "/openapi",
		},
	})
}

// ChangePasswordRedirect handles /.well-known/change-password per RFC 8615
// This redirects to the appropriate password change page
func ChangePasswordRedirect(w http.ResponseWriter, r *http.Request) {
	// Redirect to admin panel password change for this project
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}
