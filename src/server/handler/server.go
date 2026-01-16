// SPDX-License-Identifier: MIT
// AI.md PART 31: /server/ routes
package handler

import (
	"net/http"
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
	return &ServerHandler{
		appConfig: appConfig,
	}
}

// AboutPage renders /server/about web page
func (h *ServerHandler) AboutPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>About - ` + h.appConfig.Server.Title + `</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body class="dark-theme">
    <div class="container">
        <header>
            <h1>About ` + h.appConfig.Server.Title + `</h1>
        </header>
        <main>
            <section>
                <h2>What is ` + h.appConfig.Server.Title + `?</h2>
                <p>` + h.appConfig.Server.Description + `</p>
            </section>
            <section>
                <h2>Features</h2>
                <ul>
                    <li>Privacy-focused video meta-search</li>
                    <li>No tracking or personal data collection</li>
                    <li>Aggregates results from multiple sources</li>
                    <li>Open source and self-hostable</li>
                </ul>
            </section>
            <section>
                <h2>Version</h2>
                <p>Version 0.2.0</p>
            </section>
        </main>
        <footer>
            <a href="/">Home</a> |
            <a href="/server/privacy">Privacy</a> |
            <a href="/server/contact">Contact</a>
        </footer>
    </div>
</body>
</html>`))
}

// PrivacyPage renders /server/privacy web page
func (h *ServerHandler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Privacy Policy - ` + h.appConfig.Server.Title + `</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body class="dark-theme">
    <div class="container">
        <header>
            <h1>Privacy Policy</h1>
        </header>
        <main>
            <section>
                <h2>Data Collection</h2>
                <p>` + h.appConfig.Server.Title + ` is designed with privacy in mind:</p>
                <ul>
                    <li>We do not track your searches</li>
                    <li>We do not store your IP address</li>
                    <li>We do not use cookies for tracking</li>
                    <li>We do not share any data with third parties</li>
                </ul>
            </section>
            <section>
                <h2>Cookies</h2>
                <p>We only use essential cookies for:</p>
                <ul>
                    <li>Age verification (required by law)</li>
                    <li>User preferences (theme, safe search settings)</li>
                </ul>
            </section>
            <section>
                <h2>Third Party Services</h2>
                <p>Search queries are forwarded to third-party video sites.
                   Please refer to their respective privacy policies for information
                   about how they handle your data.</p>
            </section>
        </main>
        <footer>
            <a href="/">Home</a> |
            <a href="/server/about">About</a> |
            <a href="/server/contact">Contact</a>
        </footer>
    </div>
</body>
</html>`))
}

// ContactPage renders /server/contact web page
func (h *ServerHandler) ContactPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Handle contact form submission
		h.handleContactSubmit(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Contact - ` + h.appConfig.Server.Title + `</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body class="dark-theme">
    <div class="container">
        <header>
            <h1>Contact Us</h1>
        </header>
        <main>
            <section>
                <p>For security issues, please see our
                   <a href="/.well-known/security.txt">security.txt</a> file.</p>
            </section>
            <section>
                <h2>Contact Form</h2>
                <form method="POST" action="/server/contact">
                    <div class="form-group">
                        <label for="email">Email (optional)</label>
                        <input type="email" id="email" name="email" placeholder="your@email.com">
                    </div>
                    <div class="form-group">
                        <label for="subject">Subject</label>
                        <input type="text" id="subject" name="subject" required>
                    </div>
                    <div class="form-group">
                        <label for="message">Message</label>
                        <textarea id="message" name="message" rows="5" required></textarea>
                    </div>
                    <button type="submit">Send Message</button>
                </form>
            </section>
        </main>
        <footer>
            <a href="/">Home</a> |
            <a href="/server/about">About</a> |
            <a href="/server/privacy">Privacy</a>
        </footer>
    </div>
</body>
</html>`))
}

// handleContactSubmit handles contact form submission
func (h *ServerHandler) handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Message Sent - ` + h.appConfig.Server.Title + `</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body class="dark-theme">
    <div class="container">
        <header>
            <h1>Message Sent</h1>
        </header>
        <main>
            <p>Thank you for your message. We will get back to you if needed.</p>
            <p><a href="/">Return to Home</a></p>
        </main>
    </div>
</body>
</html>`))
}

// HelpPage renders /server/help web page
func (h *ServerHandler) HelpPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Help - ` + h.appConfig.Server.Title + `</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body class="dark-theme">
    <div class="container">
        <header>
            <h1>Help</h1>
        </header>
        <main>
            <section>
                <h2>How to Search</h2>
                <p>Enter your search terms in the search box on the home page and press Enter or click Search.</p>
            </section>
            <section>
                <h2>Search Tips</h2>
                <ul>
                    <li>Use specific keywords for better results</li>
                    <li>You can select specific engines in Preferences</li>
                    <li>Results are aggregated from multiple sources</li>
                </ul>
            </section>
            <section>
                <h2>Preferences</h2>
                <p>Visit the <a href="/preferences">Preferences</a> page to customize:</p>
                <ul>
                    <li>Enable/disable specific search engines</li>
                    <li>Change theme settings</li>
                </ul>
            </section>
            <section>
                <h2>API Access</h2>
                <p>API documentation is available at <a href="/openapi">/openapi</a></p>
            </section>
        </main>
        <footer>
            <a href="/">Home</a> |
            <a href="/server/about">About</a> |
            <a href="/server/contact">Contact</a>
        </footer>
    </div>
</body>
</html>`))
}

// API Routes per AI.md PART 31

// APIAbout handles GET /api/v1/server/about
func (h *ServerHandler) APIAbout(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"name":        h.appConfig.Server.Title,
			"description": h.appConfig.Server.Description,
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
