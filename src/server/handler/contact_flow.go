// SPDX-License-Identifier: MIT
package handler

import (
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/apimgr/vidveil/src/notify"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

func (h *ServerHandler) contactPageWorkflow(w http.ResponseWriter, r *http.Request) {
	securityID := h.securityIDFromRequest(r)
	securityMode := securityID != "" && h.validSecurityID(r, securityID)
	if securityID != "" && !securityMode {
		h.logSecurityEvent("security.security_id_invalid", r, map[string]interface{}{"supplied_id_hash": contactSecurityIDHash(securityID)})
		securityID = ""
	}
	if !h.appConfig.Server.Pages.Contact.Enabled {
		http.NotFound(w, r)
		return
	}
	if securityMode && r.Method == http.MethodPost {
		h.handleSecurityReportSubmit(w, r, securityID)
		return
	}
	if r.Method == http.MethodPost {
		h.handleContactReportSubmit(w, r)
		return
	}
	data := map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Robots":         "noindex,nofollow",
		"ContactTypes":   h.contactTypes(securityMode),
	}
	_, fqdn, _ := urlvar.GetURLVars(r)
	data["FQDN"] = fqdn
	contactType := strings.TrimSpace(r.URL.Query().Get("type"))
	if containsString(h.contactTypes(securityMode), contactType) {
		data["ContactType"] = contactType
	}
	if securityMode {
		data["SecurityMode"] = true
		data["SecurityID"] = securityID
		data["DefaultDisclosureDays"] = h.defaultDisclosureDays()
		data["Components"] = h.securityComponentOptions()
	}
	h.renderServerTemplate(w, r, "server-contact", data)
}

func (h *ServerHandler) contactTypes(securityMode bool) []string {
	types := []string{"abuse", "dmca", "privacy"}
	if securityMode {
		types = append(types, "security")
	}
	return types
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func contactSecurityIDHash(id string) string { return shortContactDigest(id) }

func (h *ServerHandler) handleContactReportSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, contactBodyLimit)
	if err := r.ParseForm(); err != nil {
		h.renderContactFailure(withResponseStatus(w, http.StatusBadRequest), r, "Invalid form submission.")
		return
	}
	if h.contactOriginError(r) {
		h.contactBlock(r, "origin")
		h.renderContactFailure(w, r, "Please submit the form from this site.")
		return
	}
	step := strings.TrimSpace(r.FormValue("step"))
	if step == "" {
		step = "submit"
	}
	if step == "edit" {
		h.renderContactFormValues(w, r, "", "")
		return
	}
	if h.contactIsBanned(r) {
		h.contactBlock(r, "banned")
		writeContactBanned(w, false)
		return
	}
	if reason := h.contactShapeError(r, false); reason != "" {
		h.contactBlock(r, reason)
		h.renderContactFailure(withResponseStatus(w, http.StatusBadRequest), r, "Invalid form submission.")
		return
	}
	if field := reportContactFieldError(r); field != "" {
		h.contactBlock(r, "shape")
		h.renderContactFailure(withResponseStatus(w, http.StatusBadRequest), r, "Invalid form submission.")
		return
	}
	if step == "submit" {
		flags := ""
		if h.contactHoneypotFilled(r) {
			flags = "d"
			if h.contactBlock(r, "honeypot") {
				writeContactBanned(w, false)
				return
			}
		} else {
			if reason := h.contactFormTokenFailure(r); reason != "" {
				h.contactFormFailure(w, r, reason, false)
				return
			}
			if !h.verifyCaptcha(r) {
				h.contactFormFailure(w, r, "captcha", false)
				return
			}
			if reason := h.contactContentError(r); reason != "" {
				h.contactFormFailure(w, r, reason, false)
				return
			}
			if (r.FormValue("type") == "abuse" || r.FormValue("type") == "dmca") && !h.validReportedContactURL(r, r.FormValue("reported_url")) {
				h.contactFormFailure(w, r, "content", false)
				return
			}
		}
		if reason := h.checkContactRate(r, false); reason != "" {
			h.contactFormFailure(w, r, reason, false)
			return
		}
		token := h.issueContactReviewToken(r, flags)
		if token == "" {
			h.renderContactFailure(w, r, "We could not process your message. Please try again later.")
			return
		}
		h.renderContactFormValues(w, r, token, "")
		return
	}
	if step != "confirm" {
		h.contactBlock(r, "shape")
		h.renderContactFailure(w, r, "Invalid form submission.")
		return
	}
	flags, valid := h.validContactReviewToken(r)
	if !valid || r.FormValue("confirmed") != "true" {
		h.contactBlock(r, "review")
		h.renderContactFormValues(w, r, h.issueContactReviewToken(r, ""), "Please read the details and confirm.")
		return
	}
	if reason := h.contactRateError(r); reason != "" {
		h.contactFormFailure(w, r, reason, false)
		return
	}
	if flags == "d" {
		h.renderContactSuccess(w, r)
		return
	}
	if h.duplicateContact(r) {
		if h.contactBlock(r, "duplicate") {
			writeContactBanned(w, false)
			return
		}
		h.renderContactSuccess(w, r)
		return
	}
	name, email := strings.TrimSpace(r.FormValue("name")), strings.TrimSpace(r.FormValue("email"))
	subject, message := strings.TrimSpace(r.FormValue("subject")), strings.TrimSpace(r.FormValue("message"))
	if _, err := mail.ParseAddress(email); err != nil {
		h.renderContactFailure(w, r, "Please provide a valid email address.")
		return
	}
	if field := contactFieldLengthError(name, email, subject, message); field != "" {
		h.renderContactFailure(w, r, "The "+field+" field is too long. Please shorten it and try again.")
		return
	}
	if err := h.sendReportEmail(r); err != nil {
		h.releaseContactRate(r)
		h.forgetContactDuplicate(r)
		h.renderContactFailure(w, r, "We could not send your report. Please try again later.")
		return
	}
	if h.notifyDispatcher != nil {
		role := notify.RoleGeneral
		if r.FormValue("type") == "abuse" || r.FormValue("type") == "dmca" {
			role = notify.RoleAbuse
		}
		body := fmt.Sprintf("Type: %s\nName: %s\nEmail: %s\n\n%s", r.FormValue("type"), r.FormValue("name"), r.FormValue("email"), r.FormValue("message"))
		h.notifyDispatcher.Send(r.Context(), role, notify.Payload{Event: "contact." + r.FormValue("type"), Subject: r.FormValue("subject"), Body: body, Severity: notify.SeverityInfo})
	}
	h.renderContactSuccess(w, r)
}

func (h *ServerHandler) apiContactWorkflow(w http.ResponseWriter, r *http.Request) {
	if !h.appConfig.Server.Pages.Contact.Enabled {
		SendError(w, CodeNotFound, MsgNotFound)
		return
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Cache-Control", "no-store")
		securityMode := false
		if id := r.URL.Query().Get("security_id"); id != "" {
			securityMode = h.validSecurityID(r, id)
			if !securityMode {
				h.logSecurityEvent("security.security_id_invalid", r, map[string]interface{}{"supplied_id_hash": contactSecurityIDHash(id)})
			}
		}
		types := []string{"abuse", "dmca", "privacy"}
		fields := reportContactFields()
		if securityMode {
			types = append(types, "security")
			fields = securityContactAPIFields()
		}
		WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": map[string]interface{}{
			"fields": fields, "types": types, "captcha": h.apiCaptchaDescriptor(r),
			"form_token":   h.issueContactFormToken(r),
			"security_url": urlvar.BuildURL(r, "/server/security"),
			"abuse_email":  h.publicAbuseEmail(),
		}})
		return
	}
	if r.Method != http.MethodPost {
		SendError(w, CodeMethodNotAllowed, MsgMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, contactBodyLimit)
	if err := r.ParseForm(); err != nil {
		SendError(w, CodeBadRequest, "Invalid form data")
		return
	}
	securityID := h.securityIDFromRequest(r)
	securityMode := securityID != "" && h.validSecurityID(r, securityID)
	if securityID != "" && !securityMode {
		h.logSecurityEvent("security.security_id_invalid", r, map[string]interface{}{"supplied_id_hash": contactSecurityIDHash(securityID)})
	}
	if h.contactIsBanned(r) {
		h.contactBlock(r, "banned")
		writeContactBanned(w, true)
		return
	}
	if reason := h.contactShapeError(r, securityMode); reason != "" {
		h.contactBlock(r, reason)
		SendError(w, CodeValidation, "Invalid form data")
		return
	}
	if !securityMode {
		if field := reportContactFieldError(r); field != "" {
			h.contactBlock(r, "shape")
			SendError(w, CodeValidation, "Invalid form data")
			return
		}
	}
	step := strings.TrimSpace(r.FormValue("step"))
	if step == "" {
		step = "submit"
	}
	if step == "submit" {
		flags := ""
		if h.contactHoneypotFilled(r) {
			flags = "d"
			if h.contactBlock(r, "honeypot") {
				writeContactBanned(w, true)
				return
			}
		} else {
			if reason := h.contactFormTokenFailure(r); reason != "" {
				h.contactFormFailure(w, r, reason, true)
				return
			}
			if !h.verifyCaptcha(r) {
				h.contactFormFailure(w, r, "captcha", true)
				return
			}
			if !securityMode {
				if reason := h.contactContentError(r); reason != "" || (r.FormValue("type") == "abuse" || r.FormValue("type") == "dmca") && !h.validReportedContactURL(r, r.FormValue("reported_url")) {
					h.contactFormFailure(w, r, "content", true)
					return
				}
			}
		}
		if reason := h.checkContactRate(r, false); reason != "" {
			h.contactFormFailure(w, r, reason, true)
			return
		}
		token := h.issueContactReviewToken(r, flags)
		if token == "" {
			SendError(w, CodeServerError, "Unable to create review")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": map[string]interface{}{
			"review":                map[string]string{"type": r.FormValue("type"), "email": r.FormValue("email"), "subject": r.FormValue("subject")},
			"review_token":          token,
			"form_token":            h.issueContactFormToken(r),
			"confirm_after_seconds": h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds,
		}})
		return
	}
	if step != "confirm" {
		SendError(w, CodeValidation, "Invalid form step")
		return
	}
	flags, valid := h.validContactReviewToken(r)
	if !valid || r.FormValue("confirmed") != "true" {
		h.contactBlock(r, "review")
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": CodeValidation, "message": "Please read the details and confirm", "details": map[string]interface{}{
			"field": "review", "review_token": h.issueContactReviewToken(r, ""),
			"confirm_after_seconds": h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds,
		}})
		return
	}
	if flags == "d" {
		if reason := h.contactRateError(r); reason != "" {
			h.contactFormFailure(w, r, reason, true)
			return
		}
		SendOK(w, map[string]interface{}{"message": "Report received successfully"})
		return
	}
	if securityMode {
		h.apiSecurityReportSubmit(w, r, securityID)
		return
	}
	if reason := h.contactRateError(r); reason != "" {
		h.contactFormFailure(w, r, reason, true)
		return
	}
	if h.duplicateContact(r) {
		if h.contactBlock(r, "duplicate") {
			writeContactBanned(w, true)
			return
		}
		SendOK(w, map[string]interface{}{"message": "Report received successfully"})
		return
	}
	if err := h.sendReportEmail(r); err != nil {
		h.releaseContactRate(r)
		h.forgetContactDuplicate(r)
		SendError(w, CodeServerError, "Unable to send report")
		return
	}
	SendOK(w, map[string]interface{}{"message": "Report received successfully"})
}

func reportContactFields() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "name", "type": "text", "required": true, "max_length": 100},
		{"name": "email", "type": "email", "required": true, "max_length": 254},
		{"name": "subject", "type": "text", "required": true, "max_length": 200},
		{"name": "message", "type": "textarea", "required": true, "max_length": 5000},
		{"name": "type", "type": "enum", "required": true, "values": []string{"abuse", "dmca", "privacy"}},
		{"name": "reported_url", "type": "url", "required": false, "required_when": map[string]string{"field": "type", "equals": "abuse,dmca"}, "max_length": 2048},
		{"name": "reason", "type": "enum", "required": false, "required_when": map[string]string{"field": "type", "equals": "abuse"}, "values": []string{"illegal_content", "terms_violation", "harassment", "malware_phishing", "other"}},
		{"name": "copyrighted_work", "type": "textarea", "required": false, "required_when": map[string]string{"field": "type", "equals": "dmca"}, "max_length": 2000},
		{"name": "good_faith", "type": "checkbox", "required": false, "required_when": map[string]string{"field": "type", "equals": "dmca"}},
		{"name": "accuracy", "type": "checkbox", "required": false, "required_when": map[string]string{"field": "type", "equals": "dmca"}},
		{"name": "signature", "type": "text", "required": false, "required_when": map[string]string{"field": "type", "equals": "dmca"}, "max_length": 100},
		{"name": "request_kind", "type": "enum", "required": false, "required_when": map[string]string{"field": "type", "equals": "privacy"}, "values": []string{"access", "export", "deletion", "correction", "opt_out", "other"}},
		{"name": "website", "type": "text", "required": false, "hidden": true, "max_length": 200},
		{"name": "form_token", "type": "hidden", "required": true, "max_length": 100},
		{"name": "step", "type": "hidden", "required": false, "values": []string{"submit", "confirm", "edit"}},
		{"name": "review_token", "type": "hidden", "required": false, "max_length": 300},
		{"name": "confirmed", "type": "checkbox", "required": false},
	}
}

func reportContactFieldError(r *http.Request) string {
	for _, field := range []struct {
		name  string
		max   int
		value string
	}{{"reported_url", 2048, r.FormValue("reported_url")}, {"copyrighted_work", 2000, r.FormValue("copyrighted_work")}, {"signature", 100, r.FormValue("signature")}} {
		if runeLength(strings.TrimSpace(field.value)) > field.max {
			return field.name
		}
	}
	switch r.FormValue("reason") {
	case "", "illegal_content", "terms_violation", "harassment", "malware_phishing", "other":
	default:
		return "reason"
	}
	switch r.FormValue("request_kind") {
	case "", "access", "export", "deletion", "correction", "opt_out", "other":
	default:
		return "request_kind"
	}
	if r.FormValue("type") == "dmca" && (r.FormValue("good_faith") != "true" || r.FormValue("accuracy") != "true") {
		return "dmca_statements"
	}
	return ""
}

func securityContactAPIFields() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "name", "type": "text", "required": true, "max_length": 100},
		{"name": "email", "type": "email", "required": true, "max_length": 254},
		{"name": "type", "type": "enum", "required": true, "values": []string{"security"}},
		{"name": "researcher_gpg", "type": "text_or_url", "required": false, "max_length": 16384, "max_length_when": []map[string]interface{}{{"prefix": "https://", "max_length": 2048}}},
		{"name": "component", "type": "select_or_text", "required": true, "max_length": 255},
		{"name": "endpoint", "type": "text", "required": false, "max_length": 2048},
		{"name": "severity", "type": "enum", "required": true, "values": []string{"critical", "high", "medium", "low", "informational"}},
		{"name": "summary", "type": "text", "required": true, "max_length": 200},
		{"name": "steps", "type": "textarea", "required": true, "max_length": 10000},
		{"name": "impact", "type": "textarea", "required": true, "max_length": 5000},
		{"name": "suggested_fix", "type": "textarea", "required": false, "max_length": 5000},
		{"name": "cve_requested", "type": "checkbox", "required": false},
		{"name": "disclosure_days", "type": "number", "required": false},
		{"name": "credit_preference", "type": "enum", "required": true, "values": []string{"real_name", "handle", "anonymous", "none"}},
		{"name": "credit_handle", "type": "text", "required": false, "required_when": map[string]string{"field": "credit_preference", "equals": "handle"}, "max_length": 100},
		{"name": "disclosure_agreement", "type": "checkbox", "required": true},
		{"name": "website", "type": "text", "required": false, "hidden": true, "max_length": 200},
		{"name": "form_token", "type": "hidden", "required": true, "max_length": 100},
		{"name": "step", "type": "hidden", "required": false, "values": []string{"submit", "confirm", "edit"}},
		{"name": "review_token", "type": "hidden", "required": false, "max_length": 300},
		{"name": "confirmed", "type": "checkbox", "required": false},
	}
}

func (h *ServerHandler) renderContactFormValues(w http.ResponseWriter, r *http.Request, reviewToken, message string) {
	contactType := strings.TrimSpace(r.FormValue("type"))
	if !containsString(h.contactTypes(false), contactType) {
		contactType = ""
	}
	_, fqdn, _ := urlvar.GetURLVars(r)
	h.renderServerTemplate(w, r, "server-contact", map[string]interface{}{
		"ContactEnabled": true,
		"AbuseEmail":     h.publicAbuseEmail(),
		"Robots":         "noindex,nofollow",
		"ReviewMode":     reviewToken != "",
		"ReviewToken":    reviewToken,
		"FormValues":     contactFormValues(r),
		"Message":        message,
		"MessageType":    "error",
		"ContactType":    contactType,
		"ContactTypes":   h.contactTypes(false),
		"FQDN":           fqdn,
	})
}

func contactFormValues(r *http.Request) map[string]string {
	values := make(map[string]string, len(r.Form))
	for key, entries := range r.Form {
		if len(entries) > 0 {
			values[key] = entries[0]
		}
	}
	return values
}

func (h *ServerHandler) sendReportEmail(r *http.Request) error {
	sender, ok := h.emailSvc.(contactEmailSender)
	if !ok {
		return fmt.Errorf("contact email sender is unavailable")
	}
	typeName := strings.ToLower(strings.TrimSpace(r.FormValue("type")))
	var recipient string
	switch typeName {
	case "abuse", "dmca":
		recipient = strings.TrimSpace(h.appConfig.Server.Contact.Abuse.Email)
	case "privacy":
		recipient = strings.TrimSpace(h.appConfig.Server.Contact.DPO.Email)
	default:
		return fmt.Errorf("unsupported report type")
	}
	if recipient == "" {
		recipient = strings.TrimSpace(h.appConfig.Server.Contact.General.Email)
	}
	if recipient == "" {
		recipient = strings.TrimSpace(h.appConfig.Server.Contact.Admin.Email)
	}
	if recipient == "" {
		return fmt.Errorf("report recipient is unavailable")
	}
	name, email := strings.TrimSpace(r.FormValue("name")), strings.TrimSpace(r.FormValue("email"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	body := fmt.Sprintf("Type: %s\nName: %s\nEmail: %s\n", typeName, name, email)
	for _, key := range []string{"reported_url", "reason", "copyrighted_work", "good_faith", "accuracy", "signature", "request_kind"} {
		if value := strings.TrimSpace(r.FormValue(key)); value != "" {
			body += fmt.Sprintf("%s: %s\n", strings.ReplaceAll(key, "_", " "), value)
		}
	}
	body += "\n" + strings.TrimSpace(r.FormValue("message"))
	return sender.SendContact(recipient, email, subject, body)
}

func (h *ServerHandler) validReportedContactURL(r *http.Request, value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if !h.allowedContactLinkHost(r, u.Hostname()) {
		return false
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if path == "/" || path == "/server/contact" || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/server/admin") || h.pathMatches == nil {
		return false
	}
	return h.pathMatches(u.Path)
}
