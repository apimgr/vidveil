// SPDX-License-Identifier: MIT
// AI.md PART 17: Email & Notifications
package email

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/netip"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	emailtemplates "github.com/apimgr/vidveil/src/server/template/email"
)

// EmailService provides email sending functionality
type EmailService struct {
	appConfig   *config.AppConfig
	templateDir string
}

// TemplatePreview contains the fully substituted subject and body for CLI validation.
type TemplatePreview struct {
	Name    string
	Subject string
	Body    string
}

// NewEmailService creates a new email service
func NewEmailService(appConfig *config.AppConfig) *EmailService {
	paths := config.GetAppPaths("", "")
	return NewEmailServiceAt(appConfig, paths.Config)
}

// NewEmailServiceAt uses an explicit config directory for CLI and isolated setups.
func NewEmailServiceAt(appConfig *config.AppConfig, configDir string) *EmailService {
	templateDir := filepath.Join(configDir, "template", "email")
	return &EmailService{
		appConfig:   appConfig,
		templateDir: templateDir,
	}
}

// Send sends an email using a template
func (s *EmailService) Send(templateName string, to string, vars map[string]string) error {
	if !s.appConfig.Server.Notifications.Email.Enabled {
		return fmt.Errorf("email is not enabled")
	}

	// Get template content
	template, err := s.getTemplate(templateName)
	if err != nil {
		return err
	}

	// Parse template
	subject, body := s.parseTemplate(template)

	// Apply variables
	subject = s.applyVars(subject, vars)
	body = s.applyVars(body, vars)

	// Apply global variables
	globalVars := s.getGlobalVars()
	subject = s.applyVars(subject, globalVars)
	body = s.applyVars(body, globalVars)

	return s.sendEmail(to, subject, body)
}

// SendRaw sends a pre-built subject/body pair directly, bypassing the named
// template lookup. Used by the coordinated-disclosure pipeline (AI.md PART 11)
// to deliver PGP-encrypted bodies, which must not be run through applyVars.
func (s *EmailService) SendRaw(to, subject, body string) error {
	if !s.appConfig.Server.Notifications.Email.Enabled {
		return fmt.Errorf("email is not enabled")
	}
	return s.sendEmail(to, subject, body)
}

// SendRawAttachment sends an untemplated text message with one MIME-safe
// attachment. The caller supplies already-encrypted bytes; this method never
// transforms or logs the attachment payload.
func (s *EmailService) SendRawAttachment(to, subject, body, filename, contentType string, payload []byte) error {
	if !s.appConfig.Server.Notifications.Email.Enabled {
		return fmt.Errorf("email is not enabled")
	}
	_, _, _, _, fromAddr, fromName, _ := s.effectiveEmailConfig()
	message, err := BuildRawAttachmentMessage(fromName, fromAddr, to, subject, body, filename, contentType, payload)
	if err != nil {
		return err
	}
	return s.sendPrepared(to, message)
}

// BuildRawAttachmentMessage returns a MIME-safe message with one encrypted
// attachment. It is shared by SMTP delivery and the security export command.
func BuildRawAttachmentMessage(fromName, fromAddr, to, subject, body, filename, contentType string, payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("email attachment is empty")
	}
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "." || filename == "" || strings.ContainsAny(filename, "\r\n\x00") {
		return nil, fmt.Errorf("invalid email attachment filename")
	}
	if contentType == "" || strings.ContainsAny(contentType, "\r\n") {
		return nil, fmt.Errorf("invalid email attachment content type")
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return nil, fmt.Errorf("invalid email attachment content type: %w", err)
	}
	if err := validateMessageFields(fromName, fromAddr, to, subject, body); err != nil {
		return nil, err
	}
	messageID, err := newMessageID(fromAddr)
	if err != nil {
		return nil, err
	}
	boundary := "vidveil-" + strings.Trim(messageID, "<>")
	alternativeBoundary := boundary + "-alternative"
	var msg bytes.Buffer
	writeMessageHeaders(&msg, fromName, fromAddr, to, subject, messageID, "multipart/mixed; boundary=\""+boundary+"\"")
	fmt.Fprintf(&msg, "--%s\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary, alternativeBoundary)
	writeAlternativeBody(&msg, alternativeBoundary, body)
	fmt.Fprintf(&msg, "--%s--\r\n", alternativeBoundary)
	params := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	fmt.Fprintf(&msg, "--%s\r\nContent-Type: %s\r\nContent-Transfer-Encoding: base64\r\nContent-Disposition: %s\r\n\r\n", boundary, contentType, params)
	for offset := 0; offset < len(payload); offset += 57 {
		end := offset + 57
		if end > len(payload) {
			end = len(payload)
		}
		msg.WriteString(base64.StdEncoding.EncodeToString(payload[offset:end]))
		msg.WriteString("\r\n")
	}
	fmt.Fprintf(&msg, "--%s--\r\n", boundary)
	return msg.Bytes(), nil
}

func validateMessageFields(fromName, fromAddr, to, subject, body string) error {
	for name, value := range map[string]string{"from name": fromName, "from address": fromAddr, "recipient": to, "subject": subject} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("email %s contains a line break", name)
		}
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("email body cannot be empty")
	}
	for name, value := range map[string]string{"from address": fromAddr, "recipient": to} {
		parsed, err := mail.ParseAddress(value)
		if err != nil || parsed.Address != value {
			return fmt.Errorf("email %s is not a valid addr-spec", name)
		}
	}
	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("email subject cannot be empty")
	}
	return nil
}

func smtpTLSConfig(host string) *tls.Config {
	config := &tls.Config{ServerName: strings.Trim(host, "[]")}
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return config
	}
	private := address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast()
	if address.Is4() {
		private = private || netip.MustParsePrefix("100.64.0.0/10").Contains(address)
	}
	if private {
		config.InsecureSkipVerify = true
	}
	return config
}

// SMTPTLSVerificationSkipped reports whether the literal relay address uses the
// PART 17 private-network TLS exception.
func SMTPTLSVerificationSkipped(host string) bool {
	return smtpTLSConfig(host).InsecureSkipVerify
}

func newMessageID(fromAddr string) (string, error) {
	parsed, err := mail.ParseAddress(fromAddr)
	if err != nil {
		return "", fmt.Errorf("parse sender address: %w", err)
	}
	_, domain, ok := strings.Cut(parsed.Address, "@")
	if !ok || domain == "" {
		return "", fmt.Errorf("sender address has no domain")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate message id: %w", err)
	}
	return "<" + hex.EncodeToString(random[:]) + "@" + domain + ">", nil
}

func buildEmailMessage(fromName, fromAddr, to, subject, body, replyTo string) ([]byte, error) {
	if err := validateMessageFields(fromName, fromAddr, to, subject, body); err != nil {
		return nil, err
	}
	if strings.ContainsAny(replyTo, "\r\n") {
		return nil, fmt.Errorf("email reply-to contains a line break")
	}
	if replyTo = strings.TrimSpace(replyTo); replyTo != "" {
		parsed, err := mail.ParseAddress(replyTo)
		if err != nil || parsed.Address != replyTo {
			return nil, fmt.Errorf("email reply-to is not a valid addr-spec")
		}
	}
	messageID, err := newMessageID(fromAddr)
	if err != nil {
		return nil, err
	}
	boundary := "vidveil-" + strings.Trim(messageID, "<>")
	var message bytes.Buffer
	writeMessageHeaders(&message, fromName, fromAddr, to, subject, messageID, "multipart/alternative; boundary=\""+boundary+"\"")
	if replyTo != "" {
		message.WriteString("Reply-To: " + replyTo + "\r\n")
	}
	message.WriteString("\r\n")
	writeAlternativeBody(&message, boundary, body)
	return message.Bytes(), nil
}

func writeMessageHeaders(msg *bytes.Buffer, fromName, fromAddr, to, subject, messageID, contentType string) {
	from := fromAddr
	if fromName != "" {
		from = (&mail.Address{Name: fromName, Address: fromAddr}).String()
	}
	fmt.Fprintf(msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nAuto-Submitted: auto-generated\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n", from, to, mime.QEncoding.Encode("UTF-8", subject), time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"), messageID, contentType)
}

func writeAlternativeBody(msg *bytes.Buffer, boundary, body string) {
	fmt.Fprintf(msg, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary)
	plain := quotedprintable.NewWriter(msg)
	_, _ = plain.Write([]byte(body + "\r\n"))
	_ = plain.Close()
	fmt.Fprintf(msg, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary)
	htmlBody := "<!doctype html><html><body><pre>" + html.EscapeString(body) + "</pre></body></html>\r\n"
	encoded := quotedprintable.NewWriter(msg)
	_, _ = encoded.Write([]byte(htmlBody))
	_ = encoded.Close()
	fmt.Fprintf(msg, "--%s--\r\n", boundary)
}

func (s *EmailService) sendPrepared(to string, message []byte) error {
	host, port, username, password, fromAddr, _, tlsMode := s.effectiveEmailConfig()
	if host == "" {
		return fmt.Errorf("no SMTP server configured")
	}
	return s.deliverSMTP(host, port, username, password, fromAddr, to, tlsMode, message)
}

// SendContact sends a plain-text contact message from the configured server
// identity and uses the validated submitter address only as Reply-To.
func (s *EmailService) SendContact(to, replyTo, subject, body string) error {
	if !s.appConfig.Server.Notifications.Email.Enabled {
		return fmt.Errorf("email is not enabled")
	}
	appName := strings.TrimSpace(s.appConfig.Server.Branding.Title)
	if appName == "" {
		appName = "vidveil"
	}
	return s.sendEmailWithReplyTo(to, "["+appName+" contact] "+subject, body, replyTo)
}

// getTemplate returns template content, preferring custom over embedded
func (s *EmailService) getTemplate(name string) (string, error) {
	if !validTemplateName(name) {
		return "", fmt.Errorf("invalid email template name")
	}
	// Check for custom template first
	customPath := filepath.Join(s.templateDir, name+".txt")
	if data, err := os.ReadFile(customPath); err == nil {
		if validationErr := ValidateTemplate(string(data)); validationErr == nil {
			return string(data), nil
		} else {
			log.Printf("WARN invalid custom email template %s: %v; using embedded default", name, validationErr)
		}
	}

	if data, err := emailtemplates.Files.ReadFile(name + ".txt"); err == nil {
		return string(data), nil
	}

	return "", fmt.Errorf("template not found: %s", name)
}

func validTemplateName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && !strings.ContainsAny(name, "/\\")
}

var knownTemplateVariables = map[string]struct{}{
	"app_name": {}, "app_url": {}, "fqdn": {}, "onion_url": {}, "onion_address": {}, "i2p_url": {}, "i2p_address": {},
	"notification_reply_to": {}, "timestamp": {}, "year": {}, "event": {}, "ip": {}, "details": {}, "filename": {}, "size": {},
	"error": {}, "domain": {}, "expires_in": {}, "expiry_date": {}, "valid_until": {}, "next_retry": {}, "task_name": {}, "next_run": {}, "hostname": {},
	"version": {}, "mode": {}, "reason": {}, "path": {}, "used_percent": {}, "free": {}, "threshold_percent": {}, "since": {},
	"current_version": {}, "new_version": {}, "channel": {}, "previous_version": {}, "tracking_id": {}, "severity": {}, "component": {},
	"endpoint": {}, "summary": {}, "report_status_url": {},
}

var templateVariablePattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ValidateTemplate rejects malformed overrides before they replace embedded defaults.
func ValidateTemplate(content string) error {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if strings.Contains(content, "\r") {
		return fmt.Errorf("invalid template syntax: bare carriage return")
	}
	parts := strings.SplitN(content, "\n---\n", 2)
	if len(parts) != 2 || !strings.HasPrefix(strings.TrimSpace(parts[0]), "Subject:") {
		return fmt.Errorf("invalid template syntax: expected Subject line and --- separator")
	}
	if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "Subject:")) == "" {
		return fmt.Errorf("subject cannot be empty")
	}
	if strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("body cannot be empty")
	}
	for _, match := range templateVariablePattern.FindAllStringSubmatch(content, -1) {
		if _, ok := knownTemplateVariables[match[1]]; !ok {
			return fmt.Errorf("unknown variable: {%s}", match[1])
		}
	}
	return nil
}

// DryRunTemplates validates every built-in template and override, then renders previews.
func (s *EmailService) DryRunTemplates() ([]TemplatePreview, []error) {
	previews := make([]TemplatePreview, 0)
	errors := make([]error, 0)
	sample := s.getGlobalVars()
	for key, value := range map[string]string{
		"ip": "192.168.1.100", "event": "Sample security event", "details": "Sample details", "filename": "backup.tar.gz", "size": "1.2 MB",
		"domain": "example.com", "expires_in": "7", "expiry_date": "2030-01-01", "valid_until": "2031-01-01", "task_name": "sample_task",
		"next_run": "2030-01-02T03:04:05Z", "next_retry": "2030-01-02T03:04:05Z", "hostname": "server.example.com", "version": "1.2.3", "mode": "production", "reason": "signal",
		"path": "/var/lib/vidveil", "used_percent": "86", "free": "14 GB", "threshold_percent": "85", "since": "2030-01-01T00:00:00Z",
		"current_version": "1.2.2", "new_version": "1.2.3", "channel": "stable", "previous_version": "1.2.2", "tracking_id": "sec_0123456789abcdef",
		"severity": "medium", "component": "API", "endpoint": "/api/v1/search", "summary": "Sample report", "report_status_url": "https://example.com/status",
	} {
		sample[key] = value
	}
	for _, name := range s.GetTemplateList() {
		content, err := os.ReadFile(filepath.Join(s.templateDir, name+".txt"))
		if err != nil {
			content, err = emailtemplates.Files.ReadFile(name + ".txt")
		}
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if err := ValidateTemplate(string(content)); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", name, err))
			continue
		}
		subject, body := s.parseTemplate(string(content))
		previews = append(previews, TemplatePreview{Name: name, Subject: s.applyVars(s.applyVars(subject, sample), s.getGlobalVars()), Body: s.applyVars(s.applyVars(body, sample), s.getGlobalVars())})
	}
	return previews, errors
}

// parseTemplate extracts subject and body from template
func (s *EmailService) parseTemplate(template string) (subject, body string) {
	parts := strings.SplitN(template, "\n---\n", 2)
	if len(parts) != 2 {
		return "Notification", template
	}

	// Extract subject from first line
	subjectLine := strings.TrimSpace(parts[0])
	if strings.HasPrefix(subjectLine, "Subject: ") {
		subject = strings.TrimPrefix(subjectLine, "Subject: ")
	} else {
		subject = "Notification"
	}

	body = strings.TrimSpace(parts[1])
	return
}

// applyVars replaces template variables. Header values are rejected at message
// construction instead of silently changing submitted content.
func (s *EmailService) applyVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}

// getGlobalVars returns global template variables per AI.md PART 17 §Global Variables.
func (s *EmailService) getGlobalVars() map[string]string {
	// Build app_url respecting SSL config per AI.md PART 15
	scheme := "http"
	if s.appConfig.Server.SSL.Enabled || s.appConfig.Server.SSL.LetsEncrypt.Enabled {
		scheme = "https"
	}
	port := s.appConfig.Server.Port
	fqdn := s.appConfig.Server.FQDN
	appURL := fmt.Sprintf("%s://%s", scheme, fqdn)
	if port != "" && port != "80" && port != "443" {
		appURL = fmt.Sprintf("%s://%s:%s", scheme, fqdn, port)
	}

	// Onion and I2P addresses are runtime values managed by their respective
	// service packages; the email service does not hold references to them.
	// Variables default to "" so templates expand the placeholder to empty
	// string rather than displaying the raw "{onion_url}" literal.

	return map[string]string{
		"app_name":              s.appConfig.Server.Branding.Title,
		"app_url":               appURL,
		"fqdn":                  fqdn,
		"onion_url":             "",
		"onion_address":         "",
		"i2p_url":               "",
		"i2p_address":           "",
		"notification_reply_to": s.appConfig.Server.Notifications.Email.ReplyTo,
		"admin_email":           s.appConfig.Server.Contact.Admin.Email,
		"timestamp":             time.Now().Format(time.RFC3339),
		"year":                  fmt.Sprintf("%d", time.Now().Year()),
	}
}

// effectiveEmailConfig returns the email config with SMTP_* env var overrides
// applied per AI.md PART 17: "SMTP_* env vars override config file settings."
func (s *EmailService) effectiveEmailConfig() (host string, port int, username, password, fromAddr, fromName, tlsMode string) {
	notif := s.appConfig.Server.Notifications.Email

	host = notif.SMTP.Host
	port = notif.SMTP.Port
	username = notif.SMTP.Username
	password = notif.SMTP.Password
	tlsMode = notif.SMTP.TLS

	// From address: use configured From.Email, then default
	fromAddr = notif.From.Email
	if fromAddr == "" {
		fromAddr = "no-reply@" + s.appConfig.Server.FQDN
	}

	// From name: use configured From.Name, then app title
	fromName = notif.From.Name
	if fromName == "" {
		fromName = s.appConfig.Server.Branding.Title
	}

	// SMTP_* env var overrides (PART 17)
	host, port, tlsMode, _ = ResolveSMTPSettings(host, port, tlsMode)
	if v := os.Getenv("SMTP_USERNAME"); v != "" {
		username = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		password = v
	}
	if v := os.Getenv("SMTP_FROM_NAME"); v != "" {
		fromName = v
	}
	if v := os.Getenv("SMTP_FROM_EMAIL"); v != "" {
		fromAddr = v
	}

	return
}

// ResolveSMTPSettings applies environment overrides and the PART 17 defaults.
func ResolveSMTPSettings(host string, port int, tlsMode string) (string, int, string, []string) {
	warnings := make([]string, 0)
	if value := os.Getenv("SMTP_HOST"); value != "" {
		host = value
	}
	if value := os.Getenv("SMTP_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 65535 {
			warnings = append(warnings, "invalid SMTP_PORT; using configured/default port")
		} else {
			port = parsed
		}
	}
	if port < 1 || port > 65535 {
		warnings = append(warnings, "invalid SMTP port; using default 587")
		port = 587
	}
	if value := os.Getenv("SMTP_TLS"); value != "" {
		tlsMode = value
	}
	tlsMode = strings.ToLower(strings.TrimSpace(tlsMode))
	if tlsMode == "" {
		tlsMode = "auto"
	}
	switch tlsMode {
	case "auto", "starttls", "tls", "none":
	default:
		warnings = append(warnings, "invalid SMTP TLS mode; using auto")
		tlsMode = "auto"
	}
	return strings.TrimSpace(host), port, tlsMode, warnings
}

// sendEmail sends the actual email via SMTP
func (s *EmailService) sendEmail(to, subject, body string) error {
	return s.sendEmailWithReplyTo(to, subject, body, s.appConfig.Server.Notifications.Email.ReplyTo)
}

func (s *EmailService) sendEmailWithReplyTo(to, subject, body, replyTo string) error {
	host, port, username, password, fromAddr, fromName, tlsMode := s.effectiveEmailConfig()
	if host == "" {
		return fmt.Errorf("no SMTP server configured")
	}

	msg, err := buildEmailMessage(fromName, fromAddr, to, subject, body, replyTo)
	if err != nil {
		return err
	}

	return s.deliverSMTP(host, port, username, password, fromAddr, to, tlsMode, msg)
}

func (s *EmailService) deliverSMTP(host string, port int, username, password, from, to, tlsMode string, message []byte) error {
	if tlsMode == "" || tlsMode == "auto" {
		if port == 465 {
			tlsMode = "tls"
		} else {
			tlsMode = "auto-starttls"
		}
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if tlsMode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, smtpTLSConfig(host))
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	tlsEstablished := tlsMode == "tls"
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if tlsMode == "starttls" || tlsMode == "auto-starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok && tlsMode == "starttls" {
			return fmt.Errorf("SMTP server does not offer required STARTTLS")
		} else if ok {
			if err := client.StartTLS(smtpTLSConfig(host)); err != nil {
				return err
			}
			tlsEstablished = true
		}
	}
	if username != "" {
		if !tlsEstablished {
			return fmt.Errorf("SMTP authentication requires an established TLS connection")
		}
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// sendTLS sends email over implicit TLS
// buildSMTPAutodetectHosts builds the SMTP autodetect host list per AI.md PART 17 priority order:
// 1: 127.0.0.1, 2: Docker bridge 172.17.0.1, 3: default gateway, 4: FQDN,
// 5: global IPv4, 6: mail.{fqdn}, 7: smtp.{fqdn}.
func SMTPAutodetectHosts(fqdn string) []string {
	hosts := []string{"127.0.0.1", "172.17.0.1"}
	if gw := getGatewayIP(); gw != "" {
		hosts = append(hosts, gw)
	}
	if fqdn != "" && fqdn != "localhost" {
		hosts = append(hosts, fqdn)
	}
	// Global IPv4 via outbound route probe (priority 5)
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		if udp, ok := conn.LocalAddr().(*net.UDPAddr); ok && !udp.IP.IsLoopback() && !udp.IP.IsPrivate() {
			hosts = append(hosts, udp.IP.String())
		}
		_ = conn.Close()
	}
	if fqdn != "" && fqdn != "localhost" {
		hosts = append(hosts, "mail."+fqdn, "smtp."+fqdn)
	}
	return hosts
}

// AutodetectSMTP tries to find an SMTP server per AI.md PART 17
// It performs actual SMTP EHLO handshake (not just TCP connect)
// Returns host, port on success; empty string, 0 on failure
func AutodetectSMTP(customHosts []string, customPorts []int) (string, int) {
	hosts := customHosts
	if len(hosts) == 0 {
		// Per AI.md PART 17: full priority list (no FQDN available here)
		hosts = SMTPAutodetectHosts("")
	}

	ports := customPorts
	if len(ports) == 0 {
		ports = []int{25, 465, 587}
	}

	for _, host := range hosts {
		for _, port := range ports {
			if testSMTPConnection(host, port) {
				return host, port
			}
		}
	}

	return "", 0
}

// testSMTPConnection tests an SMTP server with EHLO handshake per AI.md PART 17
func testSMTPConnection(host string, port int) bool {
	return testSMTPConnectionMode(host, port, "auto")
}

func testSMTPConnectionMode(host string, port int, tlsMode string) bool {
	if tlsMode == "" || tlsMode == "auto" {
		if port == 465 {
			tlsMode = "tls"
		} else {
			tlsMode = "auto-starttls"
		}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()

	// Set read/write deadline
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return false
	}

	// For port 465 (SMTPS), wrap in TLS first
	if tlsMode == "tls" {
		tlsConn := tls.Client(conn, smtpTLSConfig(host))
		if err := tlsConn.Handshake(); err != nil {
			return false
		}
		conn = tlsConn
	}

	// Create SMTP client and do EHLO handshake
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return false
	}
	defer func() { _ = client.Quit() }()

	// EHLO handshake - this is what the spec requires
	if err := client.Hello("localhost"); err != nil {
		return false
	}
	if tlsMode == "starttls" || tlsMode == "auto-starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok && tlsMode == "starttls" {
			return false
		} else if ok {
			if err := client.StartTLS(smtpTLSConfig(host)); err != nil {
				return false
			}
		}
	}

	return true
}

// TestSMTPConfig tests a specific SMTP configuration per AI.md PART 17
// Returns nil on success, error on failure
func TestSMTPConfig(host string, port int) error {
	return TestSMTPConfigMode(host, port, "auto")
}

// TestSMTPConfigMode verifies the greeting and configured TLS requirements.
func TestSMTPConfigMode(host string, port int, tlsMode string) error {
	if host == "" {
		return fmt.Errorf("no SMTP host configured")
	}
	host, port, tlsMode, _ = ResolveSMTPSettings(host, port, tlsMode)
	if !testSMTPConnectionMode(host, port, tlsMode) {
		return fmt.Errorf("SMTP connection failed to %s:%d", host, port)
	}
	return nil
}

// getGatewayIP attempts to determine the default gateway IP
func getGatewayIP() string {
	// Try to connect to an external address to find the gateway
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	// Gateway is typically at .1 of the same subnet
	ip := localAddr.IP.To4()
	if ip == nil {
		return ""
	}
	ip[3] = 1
	return ip.String()
}

// SendTest sends a test email
func (s *EmailService) SendTest(to string) error {
	if !s.appConfig.Server.Notifications.Email.Enabled {
		return fmt.Errorf("email is not enabled")
	}
	template, err := s.getTemplate("test")
	if err != nil {
		return err
	}
	subject, body := s.parseTemplate(template)
	vars := s.getGlobalVars()
	return s.sendEmail(to, "[TEST] "+s.applyVars(subject, vars), s.applyVars(body, vars))
}

// GetTemplateList returns list of available templates
func (s *EmailService) GetTemplateList() []string {
	templates := emailtemplates.Names()
	sort.Strings(templates)
	return templates
}

// GetTemplate returns a template's content
func (s *EmailService) GetTemplate(name string) (string, error) {
	return s.getTemplate(name)
}

// SaveTemplate saves a custom template
func (s *EmailService) SaveTemplate(name, content string) error {
	if !validTemplateName(name) {
		return fmt.Errorf("invalid email template name")
	}
	if err := ValidateTemplate(content); err != nil {
		return err
	}
	if err := os.MkdirAll(s.templateDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.templateDir, name+".txt")
	content = strings.TrimRight(content, "\r\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// ResetTemplate deletes a custom template (falls back to default)
func (s *EmailService) ResetTemplate(name string) error {
	if !validTemplateName(name) {
		return fmt.Errorf("invalid email template name")
	}
	path := filepath.Join(s.templateDir, name+".txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

// IsCustomTemplate checks if a template is customized
func (s *EmailService) IsCustomTemplate(name string) bool {
	if !validTemplateName(name) {
		return false
	}
	path := filepath.Join(s.templateDir, name+".txt")
	_, err := os.Stat(path)
	return err == nil
}
