// SPDX-License-Identifier: MIT
// AI.md PART 14: /server/ routes
package handler

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/common/authpad"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/markdown"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/notify"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/pgp"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

// ServerHandler handles /server/ routes per AI.md PART 14
type ServerHandler struct {
	appConfig  *config.AppConfig
	torSvc     TorStatusChecker
	i2pSvc     I2PStatusChecker
	db         *sql.DB
	secretsMgr *secret.Manager
	configDir  string
	logger     *logging.AppLogger
	emailSvc   SecurityReportEmailSender
	// notifyDispatcher routes contact-form and other events to the configured
	// PART 12 webhook transports. Nil until wired via SetNotifyDispatcher.
	notifyDispatcher *notify.Dispatcher
	captchaMu        sync.Mutex
	captchaAnswers   map[string]captchaChallenge
	captchaAttempts  map[string]captchaRateWindow
	contactMu        sync.Mutex
	contactCounts    map[string]contactWindow
	contactSeen      []contactDedup
	contactStrikes   map[string][]time.Time
	contactBans      map[string]time.Time
	pathMatches      func(string) bool
}

// SecurityReportEmailSender sends the maintainer notice and optional researcher acknowledgment.
type SecurityReportEmailSender interface {
	Send(templateName, to string, vars map[string]string) error
	SendRaw(to, subject, body string) error
	SendRawAttachment(to, subject, body, filename, contentType string, payload []byte) error
}

type contactEmailSender interface {
	SendContact(to, replyTo, subject, body string) error
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

// SetI2PService wires the optional eepsite state used by public pages.
func (h *ServerHandler) SetI2PService(service I2PStatusChecker) {
	h.i2pSvc = service
}

// SetDB wires the database connection used by the coordinated-disclosure
// pipeline (AI.md PART 11 "Security Reports") to store/read security_reports rows.
func (h *ServerHandler) SetDB(db *sql.DB) {
	h.db = db
}

// templatePath adds the request-aware mount prefix to same-origin template URLs.
func templatePath(r *http.Request, value string) string {
	prefix := urlvar.GetPathPrefix(r)
	if prefix == "" || prefix == "/" {
		return value
	}
	return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(value, "/")
}

// SetSecretsManager wires the app-secrets manager used to validate the
// rotating {security_id} token (AI.md PART 11 "Security Reports").
func (h *ServerHandler) SetSecretsManager(m *secret.Manager) {
	h.secretsMgr = m
}

// SetConfigDir wires the config directory so the security pipeline can find
// the project PGP keypair under {config_dir}/security/.
func (h *ServerHandler) SetConfigDir(dir string) {
	h.configDir = dir
}

// SetLogger wires the app logger so security-report events can be recorded
// to security.log per AI.md PART 11.
func (h *ServerHandler) SetLogger(l *logging.AppLogger) {
	h.logger = l
}

// SetEmailService wires the email service used to send maintainer
// notifications and researcher acknowledgments per AI.md PART 11.
func (h *ServerHandler) SetEmailService(e SecurityReportEmailSender) {
	h.emailSvc = e
}

// SetNotifyDispatcher wires the PART 12 webhook dispatcher used to route
// non-security /server/contact submissions to the general contact role.
func (h *ServerHandler) SetNotifyDispatcher(d *notify.Dispatcher) {
	h.notifyDispatcher = d
}

func serverDocsStylesheets(r *http.Request) []string {
	switch r.URL.Path {
	case "/server/docs/swagger":
		return []string{"/static/vendor/swagger-ui/swagger-ui.css", "/static/css/swagger.css"}
	case "/server/docs/graphql":
		return []string{"/static/vendor/graphiql/graphiql.min.css", "/static/css/graphql.css"}
	default:
		return nil
	}
}

// renderServerTemplate renders a server page template with common data
func (h *ServerHandler) renderServerTemplate(w http.ResponseWriter, r *http.Request, templateName string, extraData map[string]interface{}) {
	r = withOperatorConfig(r, h.appConfig)
	// Map template names to file paths
	templateFile := ""
	switch templateName {
	case "server-about":
		templateFile = "template/page/server-about.tmpl"
	case "server-privacy":
		templateFile = "template/page/server-privacy.tmpl"
	case "server-dpo":
		templateFile = "template/page/server-dpo.tmpl"
	case "server-contact":
		templateFile = "template/page/server-contact.tmpl"
	case "server-help":
		templateFile = "template/page/server-help.tmpl"
	case "server-help-mcp":
		templateFile = "template/page/server-help-mcp.tmpl"
	case "server-docs-swagger":
		templateFile = "template/page/server-docs-swagger.tmpl"
	case "server-docs-graphql":
		templateFile = "template/page/server-docs-graphql.tmpl"
	case "server-terms":
		templateFile = "template/page/server-terms.tmpl"
	case "server-security":
		templateFile = "template/page/server-security.tmpl"
	case "server-security-policy":
		templateFile = "template/page/server-security-policy.tmpl"
	case "server-security-thanks":
		templateFile = "template/page/server-security-thanks.tmpl"
	case "server-security-report":
		templateFile = "template/page/server-security-report.tmpl"
	default:
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError, i18n.Translate(i18n.DetectLocale(r), "error.server"), i18n.Translate(i18n.DetectLocale(r), "error.generic"))
		return
	}

	// Guard against uninitialized template filesystem
	if templatesFS == nil {
		log.Printf("server template: templates filesystem not initialized")
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError, i18n.Translate(i18n.DetectLocale(r), "error.server"), i18n.Translate(i18n.DetectLocale(r), "error.generic"))
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
		"path": func(value string) string {
			return templatePath(r, value)
		},
		"docsStylesheets":           func() []string { return serverDocsStylesheets(r) },
		"operatorHTML":              func(name string) template.HTML { return operatorHTML(r, name) },
		"operatorContentConfigured": func(name string) bool { return operatorContentConfigured(r, name) },
		// asset appends the version-busting stamp per AI.md PART 9:
		// {{ asset "/static/css/app.css" }} → /static/css/app.css?v={version}-{commit}
		"asset": func(path string) string {
			return templatePath(r, path) + "?v=" + version.AssetStamp()
		},
		"trackingScript": func() template.HTML {
			return trackingScriptMarkup(h.appConfig.Server.Tracking, r)
		},
		"trackingNoScript": func() template.HTML {
			return trackingNoScriptMarkup(h.appConfig.Server.Tracking, r)
		},
		// markdownToHTML converts supported project and operator Markdown to
		// sanitized HTML before it reaches a rendered page.
		"markdownToHTML": func(source string) template.HTML {
			rendered := markdown.ToHTML(source)
			if urlvar.IsTorRequest(r) {
				return template.HTML(stripExternalAnchorLinks(string(rendered)))
			}
			return rendered
		},
	})

	// Load layout and partials first
	partialFiles := []string{
		"template/layout/nojs-server.tmpl",
		"template/layout/public.tmpl",
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/site-banner.tmpl",
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
			writeGuaranteedErrorPage(w, r, http.StatusInternalServerError, i18n.Translate(i18n.DetectLocale(r), "error.server"), i18n.Translate(i18n.DetectLocale(r), "error.generic"))
			return
		}
	}

	content, err := fs.ReadFile(templatesFS, templateFile)
	if err != nil {
		log.Printf("server template: read %s: %v", templateFile, err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError, i18n.Translate(i18n.DetectLocale(r), "error.server"), i18n.Translate(i18n.DetectLocale(r), "error.generic"))
		return
	}

	if _, err = tmpl.Parse(string(content)); err != nil {
		log.Printf("server template: parse %s: %v", templateFile, err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError, i18n.Translate(i18n.DetectLocale(r), "error.server"), i18n.Translate(i18n.DetectLocale(r), "error.generic"))
		return
	}

	// Build common template data per AI.md PART 16
	versionInfo := version.GetVersionInfo()
	appName := h.appConfig.Server.Branding.Title
	// Required fields by head.tmpl: Title
	// Required fields by nav.tmpl: ActiveNav, Query
	// Resolved per request via BuildURL (AI.md PART 12) — never frozen at
	// startup/config, so canonical/OG URLs match the Host/proto the client
	// actually used, including behind a reverse proxy.

	// Theme per AI.md PART 16 — server reads the theme cookie and falls back
	// to the configured default, then dark. Never hardcoded per request.
	theme := ""
	if c, err := r.Cookie("theme"); err == nil {
		switch c.Value {
		case "dark", "light", "auto":
			theme = c.Value
		}
	}
	if theme == "" {
		theme = h.appConfig.Web.UI.Theme
	}
	if theme == "" {
		theme = "dark"
	}

	dpoEmail := strings.TrimSpace(h.appConfig.Server.Contact.DPO.Email)
	if dpoEmail == "" {
		dpoEmail = strings.TrimSpace(h.appConfig.Server.Contact.General.Email)
	}
	data := map[string]interface{}{
		"Title":          appName,
		"AppName":        appName,
		"PageContent":    h.pageContent(templateName, locale),
		"AppTagline":     h.appConfig.Server.Branding.Tagline,
		"AppDescription": h.appConfig.Server.Branding.Description,
		"DPOName":        h.appConfig.Server.Contact.DPO.Name,
		"DPOEmail":       dpoEmail,
		"BaseURL":        urlvar.BuildURL(r, ""),
		// Versioned API prefix ("/api/{api_version}") so pages never hardcode
		// "v1" in documented routes or curl examples (AI.md PART 14)
		"APIBase":       h.appConfig.APIBasePath(),
		"Version":       versionInfo["version"],
		"BuildDateTime": versionInfo["build_time"],
		"Theme":         theme,
		// Next mode in the toggle cycle per AI.md PART 16 "Theme Cycle Logic".
		"NextTheme": nextTheme(theme),
		"ActiveNav": templateName,
		"Query":     "",
		// Consent banner gating per AI.md PART 12 — rendered only when no
		// valid cookie_consent cookie exists (zero-JS flow).
		"HasConsentCookie": hasConsentCookie(r),
		// CSRF token for the consent banner forms in footer.tmpl (AI.md PART 16)
		"CSRFToken": cSRFTokenFromRequest(r),
		// Consent banner text/links per AI.md PART 12 "Cookie Consent Banner ->
		// Implementation" (server.privacy.consent.*, server.privacy.data.sold)
		"ConsentMessage":         h.appConfig.Server.Privacy.GetConsentMessage(),
		"ConsentPolicyURL":       h.appConfig.Server.Privacy.Consent.Policy.URL,
		"ConsentPolicyText":      h.appConfig.Server.Privacy.Consent.Policy.Text,
		"ConsentDeclineText":     h.appConfig.Server.Privacy.Consent.Buttons.Decline,
		"ConsentAcceptText":      h.appConfig.Server.Privacy.Consent.Buttons.Accept,
		"ConsentPreferencesText": h.appConfig.Server.Privacy.Consent.PreferencesText,
		"ConsentDataSold":        h.appConfig.Server.Privacy.Data.Sold,
		// CurrentPath backs the return_to hidden field on the zero-JS
		// consent/theme/announcement-dismiss forms in footer/header partials
		// (AI.md PART 12 / PART 16).
		"CurrentPath": r.URL.RequestURI(),
		// Whole privacy config for the config-driven /server/privacy page
		// (AI.md PART 12 / PART 16 "/server/privacy")
		"Privacy":    h.privacyConfigForDisplay(),
		"TorRequest": urlvar.IsTorRequest(r),
		// CCPA opt-out state backing the "Do Not Sell" toggle; only ever
		// rendered when server.privacy.data.sold is true.
		"CCPAOptedOut": hasCCPAOptOut(r),
		// SEO meta per AI.md PART 16 "Robots Directive" — /server/* public
		// pages are indexable; anything else falls closed to noindex,nofollow.
		"Robots":       robotsDirective(r),
		"CanonicalURL": urlvar.BuildURL(r, r.URL.Path),
	}

	// Site banner per AI.md PART 16 "Site Banner" / PART 25 "Announcements" —
	// server filters to active, undismissed announcements so the shared
	// footer/banner partial has no flash of a dismissed or expired banner.
	data["Announcements"] = activeAnnouncements(h.appConfig, r)

	// Footer onion-address row per AI.md PART 16 — dropped entirely unless
	// Tor is both enabled and actually running.
	if h.torSvc != nil && h.torSvc.IsEnabled() && h.torSvc.IsRunning() {
		data["TorEnabled"] = true
		data["TorRunning"] = true
		if h.appConfig.Server.Tor.Expose {
			if addr, ok := h.torSvc.GetInfo()["onion_address"].(string); ok {
				data["OnionAddress"] = addr
				data["TorOnionAddr"] = addr
			}
		}
	}
	if h.i2pSvc != nil && h.i2pSvc.Enabled() && h.i2pSvc.IsRunning() {
		data["I2PEnabled"] = true
		data["I2PRunning"] = true
		if h.appConfig.Server.I2P.Expose {
			data["I2PAddress"] = h.i2pSvc.Address()
		}
	}

	// Merge extra data
	for k, v := range extraData {
		data[k] = v
	}
	contactEnabled, _ := data["ContactEnabled"].(bool)
	securityMode, _ := data["SecurityMode"].(bool)
	if templateName == "server-contact" {
		if _, exists := data["FormValues"]; !exists {
			data["FormValues"] = contactFormValues(r)
		}
	}
	if templateName == "server-contact" && (contactEnabled || securityMode) {
		for key, value := range h.captchaTemplateData(r) {
			data[key] = value
		}
		data["ContactFormToken"] = h.issueContactFormToken(r)
	}
	r = withOperatorVariables(r, data)
	injectSEOData(r, h.appConfig, data)

	// Per AI.md PART 30 inject locale + direction for <html lang="" dir="">.
	injectLocaleData(r, data)

	// Buffer template output
	var buf bytes.Buffer
	renderTemplateName := templateName
	if isTextBrowser(r) {
		renderTemplateName = "server-nojs"
	}
	if err := tmpl.ExecuteTemplate(&buf, renderTemplateName, data); err != nil {
		// Full detail server-side only (AI.md PART 11 Tier 1); the client
		// gets a minimal, content-negotiated, hardcoded fallback per PART 16
		// "Error Pages" — never a bare plain-text body.
		log.Printf("server template: execute %s: %v", renderTemplateName, err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if templateName == "server-contact" {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	}
	status := http.StatusOK
	if configuredStatus, ok := data["HTTPStatus"].(int); ok && configuredStatus >= 400 && configuredStatus <= 599 {
		status = configuredStatus
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/plain") && !strings.Contains(accept, "text/html") ||
		isHttpTool(r) && !strings.Contains(accept, "text/html") {
		text := convertHTMLToText(buf.String(), 80) + "\n"
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(text)))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(text))
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (h *ServerHandler) pageContent(templateName, locale string) string {
	switch templateName {
	case "server-help":
		return h.appConfig.Server.Pages.Help.Content.ResolveWithFallback(locale, config.DefaultHelpContent)
	default:
		return ""
	}
}

// AboutPage renders /server/about web page
func (h *ServerHandler) AboutPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-about", nil)
}

// PrivacyPage renders /server/privacy web page
func (h *ServerHandler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-privacy", nil)
}

// SwaggerDocsPage renders the interactive OpenAPI explorer inside the shared page shell.
func (h *ServerHandler) SwaggerDocsPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-docs-swagger", map[string]interface{}{
		"DocsEndpointURL": templatePath(r, h.appConfig.APIBasePath()+"/server/swagger"),
	})
}

// GraphQLDocsPage renders GraphiQL inside the shared page shell.
func (h *ServerHandler) GraphQLDocsPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-docs-graphql", map[string]interface{}{
		"DocsEndpointURL": templatePath(r, h.appConfig.APIBasePath()+"/server/graphql"),
	})
}

// DPOPage renders /server/dpo using the configured DPO/general contact fallback.
func (h *ServerHandler) DPOPage(w http.ResponseWriter, r *http.Request) {
	dpo := h.appConfig.Server.Contact.DPO
	dpo.Name = strings.TrimSpace(dpo.Name)
	dpo.Email = strings.TrimSpace(dpo.Email)
	if dpo.Email == "" {
		dpo.Email = strings.TrimSpace(h.appConfig.Server.Contact.General.Email)
	}
	h.renderServerTemplate(w, r, "server-dpo", map[string]interface{}{
		"DPOName":    dpo.Name,
		"DPOEmail":   dpo.Email,
		"PrivacyURL": "/server/privacy",
		"ContactURL": "/server/contact",
	})
}

// ConsentSubmit handles POST /server/consent per AI.md PART 12 — the zero-JS
// consent flow. Reads the choice field, sets the cookie_consent cookie
// (JSON: granular categories + timestamp), and redirects back to the
// originating page so the banner form works with JavaScript disabled.
func (h *ServerHandler) ConsentSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.invalid_input"), http.StatusBadRequest)
		return
	}

	// Accept enables configured optional preferences, decline disables them,
	// and save persists the explicit preferences-page category toggle.
	choice := r.PostFormValue("choice")
	prefs := choice == "accept" || (choice == "save" && (r.PostFormValue("preferences") == "1" || r.PostFormValue("preferences") == "on"))
	analytics := choice == "accept" || (choice == "save" && (r.PostFormValue("analytics") == "1" || r.PostFormValue("analytics") == "on"))
	value := fmt.Sprintf(`{"essential":true,"preferences":%t,"analytics":%t,"timestamp":%d}`, prefs, analytics, time.Now().Unix())

	// Secure flag when the request arrived over HTTPS (directly or via proxy)
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     "cookie_consent",
		Value:    url.QueryEscape(value),
		Path:     "/",
		MaxAge:   31536000,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect back to the originating page — path-only, so a forged Referer
	// can never turn this into an open redirect.
	target := "/"
	if ref, err := url.Parse(r.Referer()); err == nil &&
		strings.HasPrefix(ref.Path, "/") && !strings.HasPrefix(ref.Path, "//") {
		target = ref.Path
		if ref.RawQuery != "" {
			target += "?" + ref.RawQuery
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// hasCCPAOptOut reports whether the visitor has exercised the CCPA
// "Do Not Sell My Personal Information" opt-out, tracked client-side in the
// ccpa_opt_out cookie per AI.md PART 12 (no server-side visitor state).
func hasCCPAOptOut(r *http.Request) bool {
	c, err := r.Cookie("ccpa_opt_out")
	return err == nil && c.Value == "true"
}

// CCPASubmit handles POST /server/ccpa — the zero-JS CCPA opt-out toggle on
// /server/privacy. Only meaningful when server.privacy.data.sold is true;
// when data is not sold the page renders no form and the endpoint is a no-op
// redirect. Per AI.md PART 12 the choice lives in a cookie, never server-side.
func (h *ServerHandler) CCPASubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeGuaranteedErrorPage(w, r, http.StatusBadRequest)
		return
	}

	optOut := r.PostFormValue("choice") == "opt-out"
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	cookie := &http.Cookie{
		Name:     "ccpa_opt_out",
		Value:    "true",
		Path:     "/",
		MaxAge:   31536000,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	// Opting back in clears the cookie rather than storing a negative value
	if !optOut {
		cookie.Value = ""
		cookie.MaxAge = -1
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/server/privacy#ccpa-opt-out", http.StatusSeeOther)
}

// SetPathMatcher wires the application's router for local reported-URL checks.
func (h *ServerHandler) SetPathMatcher(matches func(string) bool) {
	h.pathMatches = matches
}

// ContactPage renders the report form. A valid security_id selects security mode.
func (h *ServerHandler) ContactPage(w http.ResponseWriter, r *http.Request) {
	h.contactPageWorkflow(w, r)
}

// securityIDFromRequest reads the security_id from the query string (used by
// both the GET link in security.txt and the form's action URL) or, failing
// that, a submitted form field.
func (h *ServerHandler) securityIDFromRequest(r *http.Request) string {
	if id := r.URL.Query().Get("security_id"); id != "" {
		return id
	}
	return r.FormValue("security_id")
}

// validSecurityID re-validates the security_id server-side per AI.md PART 11
// step 1 of the Submission Flow — the form value can be tampered with.
func (h *ServerHandler) validSecurityID(r *http.Request, id string) bool {
	_, valid := h.securityIDWindow(r, id)
	return valid
}

func (h *ServerHandler) securityIDWindow(r *http.Request, id string) (int64, bool) {
	if id == "" || h.secretsMgr == nil {
		return 0, false
	}
	var window int64
	now := time.Now()
	valid, err := h.secretsMgr.ValidateDerivedWithPrevious(r.Context(), secret.InstallationSecret, func(secretBytes []byte) bool {
		candidate, accepted := secreport.SecurityIDWindow(secretBytes, id, now)
		if accepted {
			window = candidate
		}
		return accepted
	})
	return window, err == nil && valid
}

// defaultDisclosureDays returns the configured default coordinated-disclosure
// window, falling back to the AI.md PART 11 spec default of 90 days.
func (h *ServerHandler) defaultDisclosureDays() int {
	if h.appConfig.Web.Security.DisclosureWindowDays > 0 {
		return h.appConfig.Web.Security.DisclosureWindowDays
	}
	return 90
}

// securityComponentOptions returns the "Affected component" dropdown options
// for the security-mode contact form, per AI.md PART 11: "Dropdown populated
// from project's IDEA.md features (auth, API, frontend, CLI, etc.)". VidVeil
// has no user accounts/auth (IDEA.md non-goals), so the list is drawn from
// IDEA.md's actual in-scope feature set instead of AI.md's generic example.
// The template always appends a final "Other" option with a free-text field.
func (h *ServerHandler) securityComponentOptions() []string {
	return []string{
		"Search / Multi-Engine Aggregation",
		"Thumbnail Proxy",
		"Video Preview",
		"Real-Time Streaming (SSE)",
		"Client-Side Preferences / Favorites / History",
		"Tor Hidden Service",
		"Geographic Content Restriction",
		"API",
		"Frontend / Web UI",
		"CLI Client",
		"Contact / Security Reporting",
	}
}

// resolveSecurityComponent resolves the "Affected component" value from the
// security-mode contact form: the select dropdown, falling back to the
// adjacent "component_other" free-text field when the dropdown value is
// "other" (AI.md PART 11: "'Other' allows free text").
func resolveSecurityComponent(r *http.Request) string {
	component := r.FormValue("component")
	if component == "other" {
		return strings.TrimSpace(r.FormValue("component_other"))
	}
	return component
}

func securityContactLengthError(r *http.Request) string {
	limits := []struct {
		field string
		max   int
	}{{"name", 100}, {"email", 254}, {"component_other", 255}, {"endpoint", 2048}, {"summary", 200}, {"steps", 10000}, {"impact", 5000}, {"suggested_fix", 5000}, {"credit_handle", 100}}
	for _, limit := range limits {
		if runeLength(strings.TrimSpace(r.FormValue(limit.field))) > limit.max {
			return limit.field
		}
	}
	gpg := strings.TrimSpace(r.FormValue("researcher_gpg"))
	maxGPG := 16384
	if strings.HasPrefix(strings.ToLower(gpg), "https://") {
		maxGPG = 2048
	}
	if runeLength(gpg) > maxGPG {
		return "researcher_gpg"
	}
	if strings.TrimSpace(r.FormValue("credit_preference")) == "handle" && strings.TrimSpace(r.FormValue("credit_handle")) == "" {
		return "credit_handle"
	}
	return ""
}

func securityContactValidationError(r *http.Request) string {
	if _, err := mail.ParseAddress(strings.TrimSpace(r.FormValue("email"))); err != nil {
		return "email"
	}
	if value := strings.ToLower(strings.TrimSpace(r.FormValue("severity"))); value != "critical" && value != "high" && value != "medium" && value != "low" && value != "informational" {
		return "severity"
	}
	switch strings.TrimSpace(r.FormValue("credit_preference")) {
	case "real_name", "handle", "anonymous", "none":
	default:
		return "credit_preference"
	}
	return ""
}

func securityCreditName(preference, researcherName, handle string) string {
	switch preference {
	case "handle":
		return strings.TrimSpace(handle)
	case "real_name":
		return strings.TrimSpace(researcherName)
	default:
		return ""
	}
}

func storedSecurityCreditPreference(preference string) string {
	switch preference {
	case "real_name":
		return "name"
	case "anonymous":
		return "anon"
	case "none":
		return "no"
	default:
		return preference
	}
}

func securityReportBody(severity, component, endpoint, summary, steps, impact, suggestedFix string) []byte {
	var body strings.Builder
	fmt.Fprintf(&body, "Severity: %s\nComponent: %s\n", severity, component)
	if endpoint != "" {
		fmt.Fprintf(&body, "Endpoint: %s\n", endpoint)
	}
	fmt.Fprintf(&body, "Summary: %s\n\nSteps to reproduce:\n%s\n\nImpact:\n%s\n", summary, steps, impact)
	if suggestedFix != "" {
		fmt.Fprintf(&body, "\nSuggested fix:\n%s\n", suggestedFix)
	}
	return []byte(body.String())
}

// logSecurityEvent writes to security.log per AI.md PART 11 when a logger is
// wired; silently no-ops otherwise (e.g. in tests that don't set one up).
func (h *ServerHandler) logSecurityEvent(event string, r *http.Request, details map[string]interface{}) {
	if h.logger == nil {
		return
	}
	// Per AI.md PART 31, Tor requests key security-log entries on the exported
	// circuit ID (or the "tor" sentinel) instead of the Tor daemon's loopback address.
	clientIP := urlvar.ResolveClientIP(r)
	if urlvar.IsTorRequest(r) {
		clientIP = urlvar.TorClientLabel(r)
	}
	h.logger.Security(event, clientIP, details)
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

func runeLength(value string) int { return utf8.RuneCountInString(value) }

func contactFieldLengthError(name, email, subject, message string) string {
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{{"name", name, 100}, {"email", email, 254}, {"subject", subject, 200}, {"message", message, 5000}} {
		if runeLength(strings.TrimSpace(field.value)) > field.max {
			return field.name
		}
	}
	return ""
}

// renderContactSuccess renders the standard contact-form acknowledgment.
func (h *ServerHandler) renderContactSuccess(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Message":        h.appConfig.Server.Pages.Contact.SuccessMessage,
		"MessageType":    "success",
	})
}

// renderContactError re-renders the contact form with a validation error.
func (h *ServerHandler) renderContactError(w http.ResponseWriter, r *http.Request, msg string) {
	_, fqdn, _ := urlvar.GetURLVars(r)
	contactType := r.FormValue("type")
	if !containsString(h.contactTypes(false), contactType) {
		contactType = ""
	}
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Robots":         "noindex,nofollow",
		"FormValues":     contactFormValues(r),
		"ContactType":    contactType,
		"ContactTypes":   h.contactTypes(false),
		"FQDN":           fqdn,
		"Message":        msg,
		"MessageType":    "error",
	})
}

func (h *ServerHandler) renderSecurityContactFeedback(w http.ResponseWriter, r *http.Request, securityID, msg, messageType string) {
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true, "SecurityMode": true, "SecurityID": securityID,
		"DefaultDisclosureDays": h.defaultDisclosureDays(), "Components": h.securityComponentOptions(),
		"Robots": "noindex,nofollow", "FormValues": contactFormValues(r),
		"Message": msg, "MessageType": messageType,
	})
}

func (h *ServerHandler) renderContactFailure(w http.ResponseWriter, r *http.Request, msg string) {
	if id := h.securityIDFromRequest(r); id != "" && h.validSecurityID(r, id) {
		h.renderSecurityContactFeedback(w, r, id, msg, "error")
		return
	}
	h.renderContactError(w, r, msg)
}

// HelpPage renders /server/help web page
func (h *ServerHandler) HelpPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-help", map[string]interface{}{
		"HelpPages": pagesForHub("help", h.appConfig),
	})
}

// TermsPage renders the /server/terms web page per AI.md PART 16. A default
// Terms of Service template is served (acceptance, acceptable use, liability,
// changes, governing law); the spec allows operator customization via API.
func (h *ServerHandler) TermsPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-terms", nil)
}

// handleSecurityReportSubmit implements the Submission Flow of AI.md PART 11
// "Security Reports — Coordinated Disclosure Pipeline" for the HTML form POST.
func (h *ServerHandler) handleSecurityReportSubmit(w http.ResponseWriter, r *http.Request, securityID string) {
	r.Body = http.MaxBytesReader(w, r.Body, contactBodyLimit)
	if err := r.ParseForm(); err != nil {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"SecurityMode":          true,
			"SecurityID":            securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(),
			"Components":            h.securityComponentOptions(),
			"Message":               "Invalid form submission.",
			"MessageType":           "error",
		})
		return
	}
	if h.contactOriginError(r) {
		h.contactBlock(r, "origin")
		h.renderSecurityContactFeedback(w, r, securityID, "Please submit the form from this site.", "error")
		return
	}
	step := strings.TrimSpace(r.FormValue("step"))
	if step == "" {
		step = "submit"
	}
	if step == "edit" {
		h.renderSecurityContactFeedback(w, r, securityID, "", "")
		return
	}
	if h.contactIsBanned(r) {
		h.contactBlock(r, "banned")
		writeContactBanned(w, false)
		return
	}
	if reason := h.contactShapeError(r, true); reason != "" {
		h.contactBlock(r, reason)
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderSecurityContactFeedback(w, r, securityID, "Invalid form submission.", "error")
		return
	}
	flags := ""
	if step == "submit" && h.contactHoneypotFilled(r) {
		flags = "d"
		if h.contactBlock(r, "honeypot") {
			writeContactBanned(w, false)
			return
		}
	} else if step == "submit" {
		if reason := h.contactFormTokenFailure(r); reason != "" {
			h.contactFormFailure(w, r, reason, false)
			return
		}
	}
	if field := securityContactLengthError(r); field != "" {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"SecurityMode": true, "SecurityID": securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(), "Components": h.securityComponentOptions(),
			"Message": "A field is too long: " + field, "MessageType": "error",
		})
		return
	}
	if field := securityContactValidationError(r); field != "" {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"SecurityMode": true, "SecurityID": securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(), "Components": h.securityComponentOptions(),
			"Message": "Invalid value for " + field + ".", "MessageType": "error",
		})
		return
	}
	if step == "submit" && flags == "" && !h.verifyCaptcha(r) {
		h.contactFormFailure(w, r, "captcha", false)
		return
	}
	if step == "confirm" {
		var valid bool
		flags, valid = h.validContactReviewToken(r)
		if !valid || r.FormValue("confirmed") != "true" {
			h.contactBlock(r, "review")
			token := h.issueContactReviewToken(r, "")
			h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
				"ContactEnabled": true, "SecurityMode": true, "SecurityID": securityID,
				"DefaultDisclosureDays": h.defaultDisclosureDays(), "Components": h.securityComponentOptions(),
				"Robots": "noindex,nofollow", "FormValues": contactFormValues(r),
				"ReviewMode": token != "", "ReviewToken": token,
				"Message": "Please read the details and confirm.", "MessageType": "error",
			})
			return
		}
	} else if step != "submit" {
		h.renderSecurityContactFeedback(w, r, securityID, "Invalid form submission.", "error")
		return
	}
	rateReason := h.checkContactRate(r, step == "confirm")
	if rateReason != "" {
		h.contactFormFailure(w, r, rateReason, false)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	component := resolveSecurityComponent(r)
	severity := r.FormValue("severity")
	summary := r.FormValue("summary")
	steps := r.FormValue("steps")
	impact := r.FormValue("impact")
	creditPref := r.FormValue("credit_preference")
	if name == "" || email == "" || component == "" || severity == "" || summary == "" ||
		steps == "" || impact == "" || creditPref == "" || r.FormValue("disclosure_agreement") == "" {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"SecurityMode":          true,
			"SecurityID":            securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(),
			"Components":            h.securityComponentOptions(),
			"Message":               "Please complete all required fields, including the coordinated disclosure agreement.",
			"MessageType":           "error",
		})
		return
	}
	if step == "submit" {
		token := h.issueContactReviewToken(r, flags)
		if token == "" {
			h.renderSecurityContactFeedback(w, r, securityID, "We could not process your report. Please try again later.", "error")
			return
		}
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"ContactEnabled": true, "SecurityMode": true, "SecurityID": securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(), "Components": h.securityComponentOptions(),
			"Robots": "noindex,nofollow", "FormValues": contactFormValues(r),
			"ReviewMode": true, "ReviewToken": token,
		})
		return
	}
	if flags == "d" {
		h.renderSecurityContactFeedback(w, r, securityID, "Thank you — your security report was received.", "success")
		return
	}

	disclosureDays := h.defaultDisclosureDays()
	if v, err := strconv.Atoi(r.FormValue("disclosure_days")); err == nil && v > 0 {
		disclosureDays = v
	}

	endpoint := strings.TrimSpace(r.FormValue("endpoint"))
	suggestedFix := strings.TrimSpace(r.FormValue("suggested_fix"))
	idWindow, validID := h.securityIDWindow(r, securityID)
	if !validID {
		h.renderSecurityContactFeedback(w, r, securityID, "Invalid form submission.", "error")
		return
	}
	body := securityReportBody(severity, component, endpoint, summary, steps, impact, suggestedFix)

	submission, err := secreport.CreateReport(r.Context(), h.db, h.configDir, h.appConfig.Server.Security.EncryptionKey, secreport.Input{
		IDWindow:                 idWindow,
		Severity:                 severity,
		Component:                component,
		Body:                     body,
		ResearcherEmail:          email,
		ResearcherGPGFingerprint: r.FormValue("researcher_gpg"),
		CVERequested:             r.FormValue("cve_requested") != "",
		DisclosureWindowDays:     disclosureDays,
		CreditPreference:         storedSecurityCreditPreference(creditPref),
		CreditName:               securityCreditName(creditPref, name, r.FormValue("credit_handle")),
		AppVersion:               version.GetVersion(),
		CommitHash:               version.GetVersionInfo()["commit"],
	})
	if err != nil {
		h.releaseContactRate(r)
		log.Printf("security report: create: %v", err)
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"SecurityMode":          true,
			"SecurityID":            securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(),
			"Components":            h.securityComponentOptions(),
			"Message":               "We could not process your report. Please try again later.",
			"MessageType":           "error",
		})
		return
	}

	h.notifyMaintainer(submission, severity, component)
	acknowledgmentSent := h.acknowledgeResearcher(r, email, r.FormValue("researcher_gpg"), submission.TrackingID, submission.ReportToken)

	h.logSecurityEvent("security.report_received", r, map[string]interface{}{
		"tracking_id": submission.TrackingID,
		"severity":    severity,
		"component":   component,
	})

	message := "Thank you — your security report was received. Tracking ID: " + submission.TrackingID + "."
	if acknowledgmentSent {
		message += " Check your email for the status-tracking link."
	}
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"Message":     message,
		"MessageType": "success",
	})
}

// notifyMaintainer sends the report ciphertext to the configured security
// contact, using PGP in the message body or AES-GCM as a ciphertext attachment.
func (h *ServerHandler) notifyMaintainer(submission *secreport.Submission, severity, component string) {
	if h.emailSvc == nil || submission == nil || len(submission.EncryptedBody) == 0 {
		return
	}
	to := h.appConfig.Server.Contact.Security.Email
	if to == "" {
		to = h.appConfig.Server.Contact.Admin.Email
	}
	if to == "" {
		return
	}
	subject := fmt.Sprintf("[Security Report] %s - %s (%s)", severity, component, submission.TrackingID)
	if submission.BodyAlgorithm == secreport.EncryptionMethodPGP {
		if err := h.emailSvc.SendRaw(to, subject, string(submission.EncryptedBody)); err != nil {
			log.Printf("security report: send PGP-encrypted maintainer notification: %v", err)
		}
		return
	}
	if submission.BodyAlgorithm != secreport.EncryptionMethodAES {
		log.Printf("security report: unknown encrypted body algorithm %q; maintainer notice skipped", submission.BodyAlgorithm)
		return
	}
	warning := "The security report is attached as AES-256-GCM ciphertext. Decrypt it with the configured server.security.encryption_key. The attachment contains no plaintext fallback."
	filename := "security-report-" + submission.TrackingID + ".aesgcm"
	if err := h.emailSvc.SendRawAttachment(to, subject, warning, filename, "application/octet-stream", submission.EncryptedBody); err != nil {
		log.Printf("security report: send AES-encrypted maintainer attachment: %v", err)
	}
}

// acknowledgeResearcher sends the researcher acknowledgment per Submission
// Flow step 5 — PGP-encrypted to the researcher's supplied key when present.
func (h *ServerHandler) acknowledgeResearcher(r *http.Request, to, rawResearcherKey, trackingID, reportToken string) bool {
	if h.emailSvc == nil || to == "" {
		return false
	}
	statusURL := urlvar.BuildURL(r, "/server/security/report/"+trackingID) + "?token=" + reportToken
	vars := map[string]string{
		"tracking_id":       trackingID,
		"report_status_url": statusURL,
	}
	if rawResearcherKey != "" {
		pub, err := secreport.ResolveResearcherKey(rawResearcherKey)
		if err != nil || len(pub) == 0 {
			log.Printf("security report: resolve researcher acknowledgment key: %v", err)
			return false
		}
		plaintext := fmt.Sprintf("Tracking ID: %s\nStatus URL: %s\n", trackingID, statusURL)
		encrypted, err := pgp.EncryptMessageToPublicKey(pub, []byte(plaintext))
		if err != nil {
			log.Printf("security report: encrypt researcher acknowledgment: %v", err)
			return false
		}
		if err := h.emailSvc.SendRaw(to, "Security report received - "+trackingID, string(encrypted)); err != nil {
			log.Printf("security report: send researcher acknowledgment: %v", err)
			return false
		}
		return true
	}
	if err := h.emailSvc.Send("security_report_researcher_ack", to, vars); err != nil {
		log.Printf("security report: send researcher acknowledgment (plaintext fallback): %v", err)
		return false
	}
	return true
}

// SecurityPage renders /server/security — human-readable rendering of
// security.txt: contact channels in RFC 9116 preference order, Expires,
// and the Encryption key when a keypair exists. Per AI.md PART 11 "Public
// Pages", rendered from live config — nothing to edit.
func (h *ServerHandler) SecurityPage(w http.ResponseWriter, r *http.Request) {
	details := h.securityOverview(r)

	h.renderServerTemplate(w, r, "server-security", map[string]interface{}{
		"Contacts":           details.contacts,
		"Expires":            details.expires,
		"HasPGPKey":          details.pgpAvailable,
		"DisclosureDays":     h.defaultDisclosureDays(),
		"ContactEnabled":     h.appConfig.Server.Pages.Contact.Enabled,
		"SecurityContactURL": details.securityContactURL,
		"ContactReportURL":   details.reportURL,
	})
}

type securityOverviewDetails struct {
	contacts           []string
	contactValues      []map[string]string
	securityContactURL string
	reportURL          string
	expires            string
	pgpAvailable       bool
	pgpURL             *string
	pgpFingerprint     *string
	policyURL          string
	thanksURL          string
}

// securityOverview resolves every public security-contact value once for the
// HTML and JSON mirrors so they cannot advertise different channels.
func (h *ServerHandler) securityOverview(r *http.Request) securityOverviewDetails {
	details := securityOverviewDetails{
		contacts: make([]string, 0, 3), contactValues: make([]map[string]string, 0, 3),
		policyURL: urlvar.BuildURL(r, "/server/security/policy"),
		thanksURL: urlvar.BuildURL(r, "/server/security/thanks"),
	}
	if reportURL := strings.TrimSpace(h.appConfig.Web.Security.ReportURL); reportURL != "" {
		details.contacts = append(details.contacts, reportURL)
		details.contactValues = append(details.contactValues, map[string]string{"type": "github", "value": reportURL})
	}
	if h.appConfig.Server.Pages.Contact.Enabled && h.secretsMgr != nil {
		if installationSecret, err := h.secretsMgr.GetInstallationSecret(r.Context()); err == nil {
			id := secreport.GenerateSecurityID(installationSecret, time.Now())
			details.securityContactURL = urlvar.BuildURL(r, "/server/contact") + "?security_id=" + id
			details.reportURL = urlvar.BuildURL(r, "/server/contact")
			details.contacts = append(details.contacts, details.securityContactURL)
			details.contactValues = append(details.contactValues, map[string]string{"type": "contact_form", "value": details.securityContactURL})
		}
	}
	mail := strings.TrimSpace(h.appConfig.Server.Contact.Security.Email)
	if urlvar.IsTorRequest(r) {
		mail = strings.TrimSpace(h.appConfig.Server.Tor.ContactEmail)
	} else if mail == "" {
		mail = strings.TrimSpace(h.appConfig.Server.Contact.Admin.Email)
	}
	mail = strings.TrimPrefix(mail, "mailto:")
	if mail != "" {
		mailto := "mailto:" + mail
		details.contacts = append(details.contacts, mailto)
		details.contactValues = append(details.contactValues, map[string]string{"type": "email", "value": mail})
	}
	details.expires = strings.TrimSpace(h.appConfig.Web.Security.Expires)
	if details.expires == "" {
		details.expires = time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	}
	if h.appConfig.Web.Security.PublishPGPKey && h.db != nil {
		meta, err := pgp.GetKeypairMeta(h.db)
		publicKeyPath := filepath.Join(h.configDir, "security", pgp.PublicKeyFile)
		if err == nil && meta != nil && !meta.Revoked && meta.Fingerprint != "" {
			if _, statErr := os.Stat(publicKeyPath); statErr == nil {
				details.pgpAvailable = true
				keyURL, fingerprint := urlvar.BuildURL(r, "/.well-known/pgp-key.asc"), strings.ToUpper(meta.Fingerprint)
				details.pgpURL, details.pgpFingerprint = &keyURL, &fingerprint
			}
		}
	}
	if h.appConfig.Server.Pages.Contact.Enabled {
		details.reportURL = urlvar.BuildURL(r, "/server/contact")
	}
	return details
}

// SecurityPolicyPage renders /server/security/policy. Content is config-file
// driven (Web.Security.PolicyText) — per AI.md PART 11 "Security
// Administration": "There are no web routes for server administration."
func (h *ServerHandler) SecurityPolicyPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-security-policy", map[string]interface{}{
		"PolicyText":     h.appConfig.Web.Security.PolicyText,
		"DisclosureDays": h.defaultDisclosureDays(),
	})
}

// SecurityThanksPage renders /server/security/thanks — researchers who opted
// into credit on disclosed reports, per AI.md PART 11 "Public Pages".
func (h *ServerHandler) SecurityThanksPage(w http.ResponseWriter, r *http.Request) {
	type credit struct {
		Name string
		Year string
	}
	var credits []credit
	if h.db != nil {
		entries, err := secreport.ListThanks(r.Context(), h.db)
		if err != nil {
			log.Printf("security thanks: load entries: %v", err)
		} else {
			for _, entry := range entries {
				year := entry.DisclosedOn
				if len(year) >= 4 {
					year = year[:4]
				}
				credits = append(credits, credit{Name: entry.Credit, Year: year})
			}
		}
	}
	h.renderServerTemplate(w, r, "server-security-thanks", map[string]interface{}{
		"Credits": credits,
	})
}

// APISecurity returns the JSON mirror of the public security overview.
func (h *ServerHandler) APISecurity(w http.ResponseWriter, r *http.Request) {
	details := h.securityOverview(r)
	pgpInfo := map[string]any{"available": details.pgpAvailable, "url": details.pgpURL, "fingerprint": details.pgpFingerprint}
	data := map[string]any{
		"contacts":   details.contactValues,
		"expires":    details.expires,
		"pgp":        pgpInfo,
		"policy_url": details.policyURL,
		"thanks_url": details.thanksURL,
	}
	if h.appConfig.Server.Pages.Contact.Enabled {
		data["report_url"] = details.reportURL
	}
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

// APISecurityPolicy returns the disclosure policy as Markdown source.
func (h *ServerHandler) APISecurityPolicy(w http.ResponseWriter, r *http.Request) {
	policy := strings.TrimSpace(h.appConfig.Web.Security.PolicyText)
	if policy == "" {
		lang := i18n.DetectLocale(r)
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", i18n.Translate(lang, "security.policy_title"))
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", i18n.Translate(lang, "security.policy_in_scope_title"), strings.Join([]string{
			i18n.Translate(lang, "security.policy_in_scope_1"),
			i18n.Translate(lang, "security.policy_in_scope_2"),
			i18n.Translate(lang, "security.policy_in_scope_3"),
		}, "\n\n"))
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", i18n.Translate(lang, "security.policy_out_of_scope_title"), strings.Join([]string{
			i18n.Translate(lang, "security.policy_out_of_scope_1"),
			i18n.Translate(lang, "security.policy_out_of_scope_2"),
			i18n.Translate(lang, "security.policy_out_of_scope_3"),
			i18n.Translate(lang, "security.policy_out_of_scope_4"),
		}, "\n\n"))
		fmt.Fprintf(&b, "## %s\n\n%s [%s](%s)%s\n", i18n.Translate(lang, "security.policy_safe_harbor_title"),
			i18n.Translate(lang, "security.policy_safe_harbor_body_pre"), i18n.Translate(lang, "security.policy_safe_harbor_link"),
			urlvar.BuildURL(r, "/server/security"), i18n.Translate(lang, "security.policy_safe_harbor_body_post"))
		policy = strings.TrimSpace(b.String())
	}
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]string{"policy": policy}})
}

// APISecurityThanks returns the ordered public hall of fame.
func (h *ServerHandler) APISecurityThanks(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]any{"entries": []map[string]string{}}})
		return
	}
	entries, err := secreport.ListThanks(r.Context(), h.db)
	if err != nil {
		SendError(w, CodeServerError, "Could not load security acknowledgments")
		return
	}
	data := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		data = append(data, map[string]string{
			"credit":   entry.Credit,
			"date":     entry.DisclosedOn,
			"severity": entry.Severity,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]any{"entries": data}})
}

// SecurityReportStatusPage renders /server/security/report/{tracking_id} —
// the researcher's one-shot status page, gated by ?token=.
func (h *ServerHandler) SecurityReportStatusPage(w http.ResponseWriter, r *http.Request) {
	// AI.md PART 11 "Identical auth response timing": pad every exit path to
	// the same floor so a valid-vs-invalid tracking_id/token can't be
	// distinguished by latency (a classic enumeration timing oracle).
	start := time.Now()
	// The one-shot token travels in the query string (delivered via a private
	// email link); prevent it leaking onward through a Referer header on any
	// outbound link/asset this page might render, and never log the URL.
	w.Header().Set("Referrer-Policy", "no-referrer")
	trackingID := chi.URLParam(r, "tracking_id")
	token := r.URL.Query().Get("token")
	if h.db == nil || trackingID == "" || token == "" {
		authpad.Wait(start)
		http.NotFound(w, r)
		return
	}
	status, err := secreport.GetReportStatus(r.Context(), h.db, trackingID, token)
	if err != nil {
		authpad.Wait(start)
		http.NotFound(w, r)
		return
	}
	authpad.Wait(start)
	h.renderServerTemplate(w, r, "server-security-report", map[string]interface{}{
		"TrackingID":         status.TrackingID,
		"Status":             status.Status,
		"MaintainerComments": status.MaintainerComments,
		"CreatedAt":          status.CreatedAt.Format("2006-01-02"),
	})
}

// API Routes per AI.md PART 14

// APIAbout handles GET {api_version}/server/about
// Per AI.md PART 14: content negotiation required on every API route.
func (h *ServerHandler) APIAbout(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"name":        h.appConfig.Server.Branding.Title,
		"tagline":     h.appConfig.Server.Branding.Tagline,
		"description": h.appConfig.Server.Branding.Description,
		"version":     version.GetVersion(),
		"features": []string{
			"Search history is stored in the visitor's browser, not as per-visitor history on the server; privacy-filtered queries may appear in public feeds",
			"Meta-search across 27 registered adult video engines with bang shortcuts",
			"Real-time SSE streaming as each engine responds",
			"Thumbnail proxy so source sites never see your IP",
			"Video preview on hover (desktop) and swipe (mobile)",
			"Cookie-based preferences, browser-local search history, and server-side favorites",
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
		_, _ = fmt.Fprintf(w, "name: %s\ntagline: %s\ndescription: %s\nversion: %s\n",
			data["name"], data["tagline"], data["description"], data["version"])
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": data})
}

// APIDPO mirrors the public DPO contact page as JSON or plain text.
func (h *ServerHandler) APIDPO(w http.ResponseWriter, r *http.Request) {
	dpo := h.appConfig.Server.Contact.DPO
	dpo.Name = strings.TrimSpace(dpo.Name)
	dpo.Email = strings.TrimSpace(dpo.Email)
	if dpo.Email == "" {
		dpo.Email = strings.TrimSpace(h.appConfig.Server.Contact.General.Email)
	}
	privacyURL := urlvar.BuildURL(r, "/server/privacy")
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "name: %s\nemail: %s\nprivacy_url: %s\n", dpo.Name, dpo.Email, privacyURL)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": map[string]string{
		"name": dpo.Name, "email": dpo.Email, "privacy_url": privacyURL,
	}})
}

// APIPrivacy handles GET {api_version}/server/privacy
// Per AI.md PART 14: content negotiation required on every API route.
func (h *ServerHandler) APIPrivacy(w http.ResponseWriter, r *http.Request) {
	// The sold/stored/sharing flags are operator policy, so they come from
	// server.privacy rather than compiled-in constants (AI.md PART 12)
	privacy := h.privacyConfigForDisplay()
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "policy_version: 1.0\nlast_updated: %s\nuser_accounts: false\nthird_party_sharing: %t\ndata_stored_on_server: %t\ndata_sold: %t\nfavorites_storage: server_side_owner_token\n",
			time.Now().Format("2006-01-02"), len(privacy.ThirdParty.Services) > 0, privacy.Data.StoredOnServer, privacy.Data.Sold)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"policy_version": "1.0",
			"last_updated":   time.Now().Format("2006-01-02"),
			// Summary block shape is fixed by AI.md PART 16 "/server/privacy"
			"summary": map[string]interface{}{
				"data_stored_on_server": privacy.Data.StoredOnServer,
				"data_sold":             privacy.Data.Sold,
				"user_control":          true,
			},
			"cookies": map[string]interface{}{
				"essential": map[string]interface{}{
					"enabled":     privacy.Cookies.Essential.Enabled,
					"description": privacy.Cookies.Essential.Description,
				},
				"preferences": map[string]interface{}{
					"enabled":     privacy.Cookies.Preferences.Enabled,
					"description": privacy.Cookies.Preferences.Description,
				},
				"analytics": map[string]interface{}{
					"enabled":     trackingConfigured(h.appConfig.Server.Tracking),
					"description": privacy.GetAnalyticsDescription(),
				},
			},
			"data": map[string]interface{}{
				"sold":             privacy.Data.Sold,
				"stored_on_server": privacy.Data.StoredOnServer,
				"sharing":          privacy.Data.Sharing,
			},
			"retention": map[string]interface{}{
				"period":             privacy.Retention.Period,
				"export_available":   privacy.Retention.ExportAvailable,
				"deletion_available": privacy.Retention.DeletionAvailable,
			},
			"third_party_services": privacy.ThirdParty.Services,
			"tracking": map[string]interface{}{
				"enabled":   trackingConfigured(h.appConfig.Server.Tracking),
				"type":      h.appConfig.Server.Tracking.Type,
				"type_name": trackingTypeName(h.appConfig.Server.Tracking.Type),
			},
			"third_party": map[string]interface{}{"services": privacy.ThirdParty.Services},
			"ccpa": map[string]interface{}{
				"applicable":     privacy.Data.Sold,
				"opt_out_url":    "/server/privacy#ccpa-opt-out",
				"user_opted_out": hasCCPAOptOut(r),
			},
			"content": map[string]interface{}{
				"consent_message": privacy.GetConsentMessage(),
				"data_usage":      privacy.GetDataUsageContent(),
			},
			"data_collection": map[string]interface{}{
				"user_accounts":       false,
				"browser_fingerprint": false,
				"user_profiles":       false,
				"third_party_sharing": len(privacy.ThirdParty.Services) > 0,
				"data_sold":           privacy.Data.Sold,
			},
			"client_side_storage": []string{
				"vidveil_history",
			},
			"server_side_storage": []string{
				"favorites (private to the owner identified by an HttpOnly owner-token cookie; no personal data or account profile)",
			},
			"cookies_used": []string{
				"theme and lang (rendering preferences)",
				"vidveil_pref_* (guest preferences)",
				"cookie_consent and ccpa_opt_out (privacy choices)",
				"age_verification (required by law)",
				"content_restriction_acknowledgment (soft-block regions only)",
				"forward_ip (opt-in only, when operator enables geo-forwarding)",
				"vidveil_owner_token_ab5213f6 (HttpOnly credential that identifies your private favorites; not an account or personal identifier)",
			},
			"thumbnail_proxy":  true,
			"tor_supported":    true,
			"third_party_sent": "Only the video engines you explicitly search receive your query.",
		},
	})
}

// APIContact handles POST {api_version}/server/contact
// Per AI.md PART 9: error codes must use standard constants from response.go.
func (h *ServerHandler) APIContact(w http.ResponseWriter, r *http.Request) {
	h.apiContactWorkflow(w, r)
}

func (h *ServerHandler) apiSecurityReportSubmit(w http.ResponseWriter, r *http.Request, securityID string) {
	if field := securityContactLengthError(r); field != "" {
		SendError(w, CodeValidation, "Field is too long: "+field)
		return
	}
	if field := securityContactValidationError(r); field != "" {
		SendError(w, CodeValidation, "Invalid value for "+field)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	component := resolveSecurityComponent(r)
	severity := r.FormValue("severity")
	summary := r.FormValue("summary")
	steps := r.FormValue("steps")
	impact := r.FormValue("impact")
	creditPref := r.FormValue("credit_preference")
	if name == "" || email == "" || component == "" || severity == "" || summary == "" ||
		steps == "" || impact == "" || creditPref == "" || r.FormValue("disclosure_agreement") == "" {
		SendError(w, CodeValidation, "name, email, component, severity, summary, steps, impact, credit_preference, and disclosure_agreement are required")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		SendError(w, CodeValidation, "A valid email address is required")
		return
	}
	if reason := h.contactRateError(r); reason != "" {
		h.contactFormFailure(w, r, reason, true)
		return
	}

	disclosureDays := h.defaultDisclosureDays()
	if v, err := strconv.Atoi(r.FormValue("disclosure_days")); err == nil && v > 0 {
		disclosureDays = v
	}

	endpoint := strings.TrimSpace(r.FormValue("endpoint"))
	suggestedFix := strings.TrimSpace(r.FormValue("suggested_fix"))
	idWindow, validID := h.securityIDWindow(r, securityID)
	if !validID {
		SendError(w, CodeValidation, "Invalid security report identifier")
		return
	}
	body := securityReportBody(severity, component, endpoint, summary, steps, impact, suggestedFix)

	submission, err := secreport.CreateReport(r.Context(), h.db, h.configDir, h.appConfig.Server.Security.EncryptionKey, secreport.Input{
		IDWindow:                 idWindow,
		Severity:                 severity,
		Component:                component,
		Body:                     body,
		ResearcherEmail:          email,
		ResearcherGPGFingerprint: r.FormValue("researcher_gpg"),
		CVERequested:             r.FormValue("cve_requested") != "",
		DisclosureWindowDays:     disclosureDays,
		CreditPreference:         storedSecurityCreditPreference(creditPref),
		CreditName:               securityCreditName(creditPref, name, r.FormValue("credit_handle")),
		AppVersion:               version.GetVersion(),
		CommitHash:               version.GetVersionInfo()["commit"],
	})
	if err != nil {
		h.releaseContactRate(r)
		log.Printf("security report: create: %v", err)
		SendError(w, CodeServerError, "Unable to process security report")
		return
	}

	h.notifyMaintainer(submission, severity, component)
	h.acknowledgeResearcher(r, email, r.FormValue("researcher_gpg"), submission.TrackingID, submission.ReportToken)

	h.logSecurityEvent("security.report_received", r, map[string]interface{}{
		"tracking_id": submission.TrackingID,
		"severity":    severity,
		"component":   component,
	})

	SendOK(w, map[string]interface{}{"tracking_id": submission.TrackingID})
}

// APIHelp handles GET {api_version}/server/help
// Per AI.md PART 14: content negotiation required; health API is at {api_version}/server/healthz.
func (h *ServerHandler) APIHelp(w http.ResponseWriter, r *http.Request) {
	locale := i18n.DetectLocale(r)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"title":   i18n.Translate(locale, "help.title"),
			"content": h.appConfig.Server.Pages.Help.Content.ResolveWithFallback(locale, config.DefaultHelpContent),
			"format":  "markdown",
			"pages":   pagePayload(pagesForHub("help", h.appConfig), locale),
		},
	})
}

// APITerms handles GET {api_version}/server/terms
// Per AI.md PART 14: content negotiation required on every API route. Serves the
// default Terms of Service; the spec allows operator customization via API.
func (h *ServerHandler) APITerms(w http.ResponseWriter, r *http.Request) {
	updated := version.GetVersionInfo()["build_time"]
	sections := []string{
		"acceptance",
		"eligibility",
		"acceptable_use",
		"third_party_content",
		"no_warranty",
		"limitation_of_liability",
		"changes",
		"governing_law",
	}
	if getAPIResponseFormat(r) == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "terms_version: 1.0\nlast_updated: %s\n", updated)
		for _, s := range sections {
			_, _ = fmt.Fprintf(w, "section: %s\n", s)
		}
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"terms_version": "1.0",
			"last_updated":  updated,
			"summary":       "By using this stateless, privacy-first meta search engine you accept these terms. You must be of legal age, use the service lawfully, and acknowledge that all video content comes from independent third-party engines the operator does not host or control.",
			"sections":      sections,
		},
	})
}

// writeGuaranteedErrorPage writes a minimal, hardcoded, content-negotiated
// error body when template rendering itself has failed — deliberately
// independent of html/template (the failing subsystem), matching the
// canonical error envelope for API/JSON clients and a themeless minimal HTML
// page for browsers, per AI.md PART 16 "Error Pages". Mirrors the
// package-server guaranteedRecoverer fallback (src/server/middleware_recover.go);
// duplicated locally rather than shared to avoid an import cycle (package
// server already imports package handler).
func writeGuaranteedErrorPage(w http.ResponseWriter, r *http.Request, status int, titleMessage ...string) {
	title := "Server Error"
	message := "Something went wrong on our end. Please try again later."
	if len(titleMessage) > 0 && titleMessage[0] != "" {
		title = titleMessage[0]
	}
	if len(titleMessage) > 1 && titleMessage[1] != "" {
		message = titleMessage[1]
	}
	accept := r.Header.Get("Accept")
	wantsJSON := strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(accept, "application/json")
	if wantsJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"ok":false,"error":"SERVER_ERROR","message":%q,"details":{}}`+"\n", message)
		return
	}
	if strings.Contains(accept, "text/plain") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, "%d %s: %s\n", status, title, message)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>%d %s</title></head><body><h1>%d %s</h1><p>%s</p></body></html>`,
		status, title, status, title, message)
}
