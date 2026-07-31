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
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/notify"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/pgp"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secrets"
	"github.com/apimgr/vidveil/src/server/service/urlvars"
)

// ServerHandler handles /server/ routes per AI.md PART 14
type ServerHandler struct {
	appConfig  *config.AppConfig
	torSvc     TorStatusChecker
	db         *sql.DB
	secretsMgr *secrets.Manager
	configDir  string
	logger     *logging.AppLogger
	emailSvc   *email.EmailService
	// notifyDispatcher routes contact-form and other events to the configured
	// PART 12 webhook transports. Nil until wired via SetNotifyDispatcher.
	notifyDispatcher *notify.Dispatcher
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

// SetDB wires the database connection used by the coordinated-disclosure
// pipeline (AI.md PART 11 "Security Reports") to store/read security_reports rows.
func (h *ServerHandler) SetDB(db *sql.DB) {
	h.db = db
}

// SetSecretsManager wires the app-secrets manager used to validate the
// rotating {security_id} token (AI.md PART 11 "Security Reports").
func (h *ServerHandler) SetSecretsManager(m *secrets.Manager) {
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
func (h *ServerHandler) SetEmailService(e *email.EmailService) {
	h.emailSvc = e
}

// SetNotifyDispatcher wires the PART 12 webhook dispatcher used to route
// non-security /server/contact submissions to the general contact role.
func (h *ServerHandler) SetNotifyDispatcher(d *notify.Dispatcher) {
	h.notifyDispatcher = d
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

// ContactPage renders /server/contact web page. Per AI.md PART 11
// "/server/contact?security_id={id} — Mode Switch": a valid, current-or-
// previous-window security_id switches the form into coordinated-disclosure
// mode; an invalid/expired one silently falls back to the standard form.
func (h *ServerHandler) ContactPage(w http.ResponseWriter, r *http.Request) {
	securityID := h.securityIDFromRequest(r)
	securityMode := false
	if securityID != "" {
		if h.validSecurityID(r, securityID) {
			securityMode = true
		} else {
			h.logSecurityEvent("security.security_id_invalid", r, map[string]interface{}{"supplied_id": securityID})
			securityID = ""
		}
	}

	if r.Method == http.MethodPost {
		if securityMode {
			h.handleSecurityReportSubmit(w, r, securityID)
			return
		}
		h.handleContactSubmit(w, r)
		return
	}

	if securityMode {
		h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
			"SecurityMode":          true,
			"SecurityID":            securityID,
			"DefaultDisclosureDays": h.defaultDisclosureDays(),
			"Components":            h.securityComponentOptions(),
		})
		return
	}

	// Show contact form - contact form always available
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
	})
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
	if id == "" || h.secretsMgr == nil {
		return false
	}
	secret, err := h.secretsMgr.GetInstallationSecret(r.Context())
	if err != nil {
		return false
	}
	return secreport.ValidateSecurityID(secret, id, time.Now())
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

// logSecurityEvent writes to security.log per AI.md PART 11 when a logger is
// wired; silently no-ops otherwise (e.g. in tests that don't set one up).
func (h *ServerHandler) logSecurityEvent(event string, r *http.Request, details map[string]interface{}) {
	if h.logger == nil {
		return
	}
	h.logger.Security(event, r.RemoteAddr, details)
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

// maxContactFieldLen bounds each contact-form field so a submission cannot be
// used to flood downstream notification transports.
const maxContactFieldLen = 8000

// handleContactSubmit handles a non-security /server/contact submission and
// dispatches it to the general contact role per AI.md PART 12.
func (h *ServerHandler) handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderContactError(w, r, "Invalid form submission.")
		return
	}

	// Honeypot: legitimate browsers leave the hidden field empty. A filled
	// value indicates a bot — respond with the normal success page so the
	// bot cannot distinguish rejection from acceptance.
	if strings.TrimSpace(r.FormValue("contact_hp")) != "" {
		h.renderContactSuccess(w, r)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	message := strings.TrimSpace(r.FormValue("message"))

	if subject == "" || message == "" {
		h.renderContactError(w, r, "Please provide a subject and a message.")
		return
	}
	if len(email) > maxContactFieldLen || len(subject) > maxContactFieldLen || len(message) > maxContactFieldLen {
		h.renderContactError(w, r, "Your message is too long. Please shorten it and try again.")
		return
	}

	if h.notifyDispatcher != nil {
		from := "anonymous"
		if email != "" {
			from = email
		}
		body := fmt.Sprintf("From: %s\n\n%s", from, message)
		h.notifyDispatcher.Send(r.Context(), notify.RoleGeneral, notify.Payload{
			Event:    "contact.general",
			Subject:  subject,
			Body:     body,
			Severity: notify.SeverityInfo,
		})
	}

	h.renderContactSuccess(w, r)
}

// renderContactSuccess renders the standard contact-form acknowledgment.
func (h *ServerHandler) renderContactSuccess(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Message":        "Thank you for your message. We will get back to you if needed.",
		"MessageType":    "success",
	})
}

// renderContactError re-renders the contact form with a validation error.
func (h *ServerHandler) renderContactError(w http.ResponseWriter, r *http.Request, msg string) {
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Message":        msg,
		"MessageType":    "error",
	})
}

// HelpPage renders /server/help web page
func (h *ServerHandler) HelpPage(w http.ResponseWriter, r *http.Request) {
	h.renderServerTemplate(w, r, "server-help", nil)
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
	if err := r.ParseForm(); err != nil {
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

	email := r.FormValue("email")
	component := resolveSecurityComponent(r)
	severity := r.FormValue("severity")
	summary := r.FormValue("summary")
	steps := r.FormValue("steps")
	impact := r.FormValue("impact")
	creditPref := r.FormValue("credit_preference")
	if email == "" || component == "" || severity == "" || summary == "" ||
		steps == "" || impact == "" || creditPref == "" || r.FormValue("disclosure_agreement") == "" {
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

	disclosureDays := h.defaultDisclosureDays()
	if v, err := strconv.Atoi(r.FormValue("disclosure_days")); err == nil && v > 0 {
		disclosureDays = v
	}

	body := "Steps to reproduce:\n" + steps + "\n\nImpact:\n" + impact
	if fix := r.FormValue("suggested_fix"); fix != "" {
		body += "\n\nSuggested fix:\n" + fix
	}

	submission, err := secreport.CreateReport(r.Context(), h.db, h.configDir, h.appConfig.Server.Security.EncryptionKey, secreport.Input{
		Severity:                 severity,
		Component:                component,
		Endpoint:                 r.FormValue("endpoint"),
		Summary:                  summary,
		Body:                     []byte(body),
		ResearcherEmail:          email,
		ResearcherGPGFingerprint: r.FormValue("researcher_gpg"),
		CVERequested:             r.FormValue("cve_requested") != "",
		DisclosureWindowDays:     disclosureDays,
		CreditPreference:         creditPref,
		CreditName:               r.FormValue("credit_name"),
		AppVersion:               version.GetVersion(),
		CommitHash:               version.GetVersionInfo()["commit"],
	})
	if err != nil {
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

	h.notifyMaintainer(submission.TrackingID, severity, component, r.FormValue("endpoint"), summary)
	h.acknowledgeResearcher(r, email, r.FormValue("researcher_gpg"), submission.TrackingID, submission.ReportToken)

	h.logSecurityEvent("security.report_received", r, map[string]interface{}{
		"tracking_id": submission.TrackingID,
		"severity":    severity,
		"component":   component,
	})

	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"Message":     "Thank you — your security report was received. Tracking ID: " + submission.TrackingID + ". Check your email for a confirmation with a status-tracking link.",
		"MessageType": "success",
	})
}

// notifyMaintainer sends the maintainer notification per Submission Flow
// step 4 — PGP-encrypted to server.contact.security.email when a project
// keypair exists, otherwise a plaintext fallback with a warning.
func (h *ServerHandler) notifyMaintainer(trackingID, severity, component, endpoint, summary string) {
	if h.emailSvc == nil {
		return
	}
	to := h.appConfig.Server.Contact.Security.Email
	if to == "" {
		to = h.appConfig.Server.Contact.Admin.Email
	}
	if to == "" {
		return
	}
	vars := map[string]string{
		"tracking_id": trackingID,
		"severity":    severity,
		"component":   component,
		"endpoint":    endpoint,
		"summary":     summary,
	}
	if pub, err := pgp.LoadPublicKey(h.configDir); err == nil && len(pub) > 0 {
		plaintext := fmt.Sprintf("Tracking ID: %s\nSeverity: %s\nComponent: %s\nEndpoint: %s\nSummary: %s\n",
			trackingID, severity, component, endpoint, summary)
		if encrypted, encErr := pgp.EncryptMessageToPublicKey(pub, []byte(plaintext)); encErr == nil {
			subject := fmt.Sprintf("[Security Report] %s - %s (%s)", severity, component, trackingID)
			if err := h.emailSvc.SendRaw(to, subject, string(encrypted)); err != nil {
				log.Printf("security report: send maintainer notification: %v", err)
			}
			return
		}
	}
	if err := h.emailSvc.Send("security_report_maintainer", to, vars); err != nil {
		log.Printf("security report: send maintainer notification (plaintext fallback): %v", err)
	}
}

// acknowledgeResearcher sends the researcher acknowledgment per Submission
// Flow step 5 — PGP-encrypted to the researcher's supplied key when present.
func (h *ServerHandler) acknowledgeResearcher(r *http.Request, to, rawResearcherKey, trackingID, reportToken string) {
	if h.emailSvc == nil || to == "" {
		return
	}
	statusURL := urlvars.BuildURL(r, "/server/security/report/"+trackingID) + "?token=" + reportToken
	vars := map[string]string{
		"tracking_id":       trackingID,
		"report_status_url": statusURL,
	}
	if pub, err := secreport.ResolveResearcherKey(rawResearcherKey); err == nil && len(pub) > 0 {
		plaintext := fmt.Sprintf("Tracking ID: %s\nStatus URL: %s\n", trackingID, statusURL)
		if encrypted, encErr := pgp.EncryptMessageToPublicKey(pub, []byte(plaintext)); encErr == nil {
			if err := h.emailSvc.SendRaw(to, "Security report received - "+trackingID, string(encrypted)); err != nil {
				log.Printf("security report: send researcher acknowledgment: %v", err)
			}
			return
		}
	}
	if err := h.emailSvc.Send("security_report_researcher_ack", to, vars); err != nil {
		log.Printf("security report: send researcher acknowledgment (plaintext fallback): %v", err)
	}
}

// SecurityPage renders /server/security — human-readable rendering of
// security.txt: contact channels in RFC 9116 preference order, Expires,
// and the Encryption key when a keypair exists. Per AI.md PART 11 "Public
// Pages", rendered from live config — nothing to edit.
func (h *ServerHandler) SecurityPage(w http.ResponseWriter, r *http.Request) {
	var contacts []string
	if reportURL := h.appConfig.Web.Security.ReportURL; reportURL != "" {
		contacts = append(contacts, reportURL)
	}
	if h.secretsMgr != nil {
		if secret, err := h.secretsMgr.GetInstallationSecret(r.Context()); err == nil {
			id := secreport.GenerateSecurityID(secret, time.Now())
			contacts = append(contacts, urlvars.BuildURL(r, "/server/contact")+"?security_id="+id)
		}
	}
	mailto := h.appConfig.Web.Security.Contact
	if mailto == "" {
		mailto = h.appConfig.Server.Contact.Security.Email
	}
	if mailto == "" {
		mailto = "security@" + h.appConfig.Server.FQDN
	}
	contacts = append(contacts, "mailto:"+strings.TrimPrefix(mailto, "mailto:"))

	h.renderServerTemplate(w, r, "server-security", map[string]interface{}{
		"Contacts":       contacts,
		"Expires":        h.appConfig.Web.Security.Expires,
		"HasPGPKey":      h.appConfig.Web.Security.PublishPGPKey,
		"DisclosureDays": h.defaultDisclosureDays(),
	})
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
		rows, err := h.db.QueryContext(r.Context(), `
			SELECT credit_preference, credit_name, created_at
			FROM security_reports
			WHERE disclosed = 1 AND credit_preference != 'none'
			ORDER BY created_at DESC`)
		if err == nil {
			defer rows.Close()
			n := 0
			for rows.Next() {
				var pref, name string
				var createdAt time.Time
				if err := rows.Scan(&pref, &name, &createdAt); err != nil {
					continue
				}
				n++
				display := name
				if pref == "anonymous" || display == "" {
					display = fmt.Sprintf("Anonymous Researcher #%d", n)
				}
				credits = append(credits, credit{Name: display, Year: createdAt.Format("2006")})
			}
		}
	}
	h.renderServerTemplate(w, r, "server-security-thanks", map[string]interface{}{
		"Credits": credits,
	})
}

// SecurityReportStatusPage renders /server/security/report/{tracking_id} —
// the researcher's one-shot status page, gated by ?token=.
func (h *ServerHandler) SecurityReportStatusPage(w http.ResponseWriter, r *http.Request) {
	// The one-shot token travels in the query string (delivered via a private
	// email link); prevent it leaking onward through a Referer header on any
	// outbound link/asset this page might render, and never log the URL.
	w.Header().Set("Referrer-Policy", "no-referrer")
	trackingID := chi.URLParam(r, "tracking_id")
	token := r.URL.Query().Get("token")
	if h.db == nil || trackingID == "" || token == "" {
		http.NotFound(w, r)
		return
	}
	status, err := secreport.GetReportStatus(r.Context(), h.db, trackingID, token)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderServerTemplate(w, r, "server-security-report", map[string]interface{}{
		"TrackingID":         status.TrackingID,
		"Status":             status.Status,
		"MaintainerComments": status.MaintainerComments,
		"CreatedAt":          status.CreatedAt.Format("2006-01-02"),
	})
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

	securityID := h.securityIDFromRequest(r)
	if securityID != "" {
		if h.validSecurityID(r, securityID) {
			h.apiSecurityReportSubmit(w, r, securityID)
			return
		}
		h.logSecurityEvent("security.security_id_invalid", r, map[string]interface{}{"supplied_id": securityID})
	}

	subject := r.FormValue("subject")
	message := r.FormValue("message")

	if subject == "" || message == "" {
		SendError(w, CodeValidation, "Subject and message are required")
		return
	}

	SendOK(w, map[string]interface{}{"message": "Message received successfully"})
}

// apiSecurityReportSubmit implements the Submission Flow of AI.md PART 11
// for POST /api/{api_version}/server/contact with a valid security_id.
func (h *ServerHandler) apiSecurityReportSubmit(w http.ResponseWriter, r *http.Request, securityID string) {
	email := r.FormValue("email")
	component := resolveSecurityComponent(r)
	severity := r.FormValue("severity")
	summary := r.FormValue("summary")
	steps := r.FormValue("steps")
	impact := r.FormValue("impact")
	creditPref := r.FormValue("credit_preference")
	if email == "" || component == "" || severity == "" || summary == "" ||
		steps == "" || impact == "" || creditPref == "" || r.FormValue("disclosure_agreement") == "" {
		SendError(w, CodeValidation, "email, component, severity, summary, steps, impact, credit_preference, and disclosure_agreement are required")
		return
	}

	disclosureDays := h.defaultDisclosureDays()
	if v, err := strconv.Atoi(r.FormValue("disclosure_days")); err == nil && v > 0 {
		disclosureDays = v
	}

	body := "Steps to reproduce:\n" + steps + "\n\nImpact:\n" + impact
	if fix := r.FormValue("suggested_fix"); fix != "" {
		body += "\n\nSuggested fix:\n" + fix
	}

	submission, err := secreport.CreateReport(r.Context(), h.db, h.configDir, h.appConfig.Server.Security.EncryptionKey, secreport.Input{
		Severity:                 severity,
		Component:                component,
		Endpoint:                 r.FormValue("endpoint"),
		Summary:                  summary,
		Body:                     []byte(body),
		ResearcherEmail:          email,
		ResearcherGPGFingerprint: r.FormValue("researcher_gpg"),
		CVERequested:             r.FormValue("cve_requested") != "",
		DisclosureWindowDays:     disclosureDays,
		CreditPreference:         creditPref,
		CreditName:               r.FormValue("credit_name"),
		AppVersion:               version.GetVersion(),
		CommitHash:               version.GetVersionInfo()["commit"],
	})
	if err != nil {
		log.Printf("security report: create: %v", err)
		SendError(w, CodeServerError, "Unable to process security report")
		return
	}

	h.notifyMaintainer(submission.TrackingID, severity, component, r.FormValue("endpoint"), summary)
	h.acknowledgeResearcher(r, email, r.FormValue("researcher_gpg"), submission.TrackingID, submission.ReportToken)

	h.logSecurityEvent("security.report_received", r, map[string]interface{}{
		"tracking_id": submission.TrackingID,
		"severity":    severity,
		"component":   component,
	})

	SendOK(w, map[string]interface{}{"tracking_id": submission.TrackingID})
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

// APITerms handles GET /api/v1/server/terms
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
		fmt.Fprintf(w, "terms_version: 1.0\nlast_updated: %s\n", updated)
		for _, s := range sections {
			fmt.Fprintf(w, "section: %s\n", s)
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
