// SPDX-License-Identifier: MIT
// AI.md PART 17: Email & Notifications
package email

import (
	"bytes"
	"crypto/tls"
	"embed"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

//go:embed template/*.txt
var embeddedTemplates embed.FS

// Default templates per AI.md PART 17.
// Only system/operator notification templates — no account-related emails.
// VidVeil has no user accounts (PARTS 34-36 not implemented).
var defaultTemplates = map[string]string{
	"security_alert": `Subject: Security Alert - {app_name}
---
SECURITY ALERT

From: {app_name} ({fqdn})
Time: {timestamp}

{event}

Details:
  Source IP: {ip}
  {details}

────────────────────────────────────────────────────────────────────────
{app_name}
{app_url}`,

	"backup_complete": `Subject: Backup Complete - {app_name}
---
BACKUP COMPLETE

From: {app_name} ({fqdn})
Time: {timestamp}

Your backup completed successfully.

Filename: {filename}
Size: {size}

--
{app_name}
{app_url}`,

	"backup_failed": `Subject: Backup Failed - {app_name}
---
BACKUP FAILED

From: {app_name} ({fqdn})
Time: {timestamp}

A backup operation failed.

Error: {error}

Please check your server logs.

--
{app_name}
{app_url}`,

	"ssl_expiring": `Subject: SSL Certificate Expiring - {app_name}
---
Hello,

Your SSL certificate for {domain} is expiring soon.

Expires in: {expires_in} days
Expiry date: {expiry_date}

Please renew your certificate.

--
{app_name}
{app_url}`,

	"ssl_renewed": `Subject: SSL Certificate Renewed - {app_name}
---
Hello,

Your SSL certificate for {domain} has been renewed.

Valid until: {valid_until}

--
{app_name}
{app_url}`,

	"ssl_renewal_failed": `Subject: SSL Renewal Failed - {app_name}
---
SSL CERTIFICATE RENEWAL FAILED

From: {app_name} ({fqdn})
Time: {timestamp}

Automatic SSL certificate renewal failed for domain: {fqdn}

Error: {error}

Current certificate expires in {expires_in} days ({expiry_date}).
The system will retry automatically: {next_retry}

--
{app_name}
{app_url}`,

	"scheduler_error": `Subject: Scheduled Task Failed - {app_name}
---
SCHEDULED TASK FAILED

From: {app_name} ({fqdn})
Time: {timestamp}

The scheduled task "{task_name}" failed.

Error: {error}
Next run: {next_run}

--
{app_name}
{app_url}`,

	"test": `Subject: Test Email - {app_name}
---
Hello,

This is a test email from {app_name}.

If you received this, your email settings are configured correctly.

Time: {timestamp}

--
{app_name}
{app_url}`,
}

// EmailService provides email sending functionality
type EmailService struct {
	appConfig   *config.AppConfig
	templateDir string
}

// NewEmailService creates a new email service
func NewEmailService(appConfig *config.AppConfig) *EmailService {
	paths := config.GetAppPaths("", "")
	templateDir := filepath.Join(paths.Config, "templates", "email")

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

// getTemplate returns template content, preferring custom over embedded
func (s *EmailService) getTemplate(name string) (string, error) {
	// Check for custom template first
	customPath := filepath.Join(s.templateDir, name+".txt")
	if data, err := os.ReadFile(customPath); err == nil {
		return string(data), nil
	}

	// Check embedded templates
	if data, err := embeddedTemplates.ReadFile("template/" + name + ".txt"); err == nil {
		return string(data), nil
	}

	// Fall back to default templates
	if tmpl, ok := defaultTemplates[name]; ok {
		return tmpl, nil
	}

	return "", fmt.Errorf("template not found: %s", name)
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

// applyVars replaces {var} placeholders with values
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
		"notification_reply_to": "",
		"admin_email":           s.appConfig.Server.Admin.Email,
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
	if v := os.Getenv("SMTP_HOST"); v != "" {
		host = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}
	if v := os.Getenv("SMTP_USERNAME"); v != "" {
		username = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		password = v
	}
	if v := os.Getenv("SMTP_TLS"); v != "" {
		tlsMode = v
	}
	if v := os.Getenv("SMTP_FROM_NAME"); v != "" {
		fromName = v
	}
	if v := os.Getenv("SMTP_FROM_EMAIL"); v != "" {
		fromAddr = v
	}

	return
}

// sendEmail sends the actual email via SMTP
func (s *EmailService) sendEmail(to, subject, body string) error {
	host, port, username, password, fromAddr, fromName, tlsMode := s.effectiveEmailConfig()

	// Try autodetect if host is empty (per AI.md PART 17: autodetect is always enabled)
	if host == "" {
		h, p := s.autodetectSMTP()
		if h != "" {
			host, port = h, p
		}
	}

	if host == "" {
		return fmt.Errorf("no SMTP server configured")
	}

	// Build RFC 5321-compliant From header: "Name <email>" or just "email"
	from := fromAddr
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromAddr)
	}

	// Build message
	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Determine auth (uses env-var-resolved username/password)
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	// Resolve TLS mode per PART 17: auto, starttls, tls, none
	if tlsMode == "" || tlsMode == "auto" {
		if port == 465 {
			tlsMode = "tls"
		} else {
			tlsMode = "starttls"
		}
	}

	if tlsMode == "tls" {
		// Implicit TLS (port 465)
		return s.sendTLS(addr, host, auth, from, to, msg.Bytes())
	}

	// Standard SMTP with optional STARTTLS (tlsMode == "starttls" or "none")
	return smtp.SendMail(addr, auth, from, []string{to}, msg.Bytes())
}

// sendTLS sends email over implicit TLS
func (s *EmailService) sendTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(from); err != nil {
		return err
	}

	if err := client.Rcpt(to); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// buildSMTPAutodetectHosts builds the SMTP autodetect host list per AI.md PART 17 priority order:
// 1: 127.0.0.1, 2: Docker bridge 172.17.0.1, 3: default gateway, 4: FQDN,
// 5: global IPv4, 6: mail.{fqdn}, 7: smtp.{fqdn}.
func buildSMTPAutodetectHosts(fqdn string) []string {
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
		conn.Close()
	}
	if fqdn != "" && fqdn != "localhost" {
		hosts = append(hosts, "mail."+fqdn, "smtp."+fqdn)
	}
	return hosts
}

// autodetectSMTP tries to find an SMTP server per AI.md PART 17.
// Hosts are always built from the spec priority list; ports are always {25, 465, 587}.
func (s *EmailService) autodetectSMTP() (string, int) {
	hosts := buildSMTPAutodetectHosts(s.appConfig.Server.FQDN)
	return AutodetectSMTP(hosts, nil)
}

// AutodetectSMTP tries to find an SMTP server per AI.md PART 17
// It performs actual SMTP EHLO handshake (not just TCP connect)
// Returns host, port on success; empty string, 0 on failure
func AutodetectSMTP(customHosts []string, customPorts []int) (string, int) {
	hosts := customHosts
	if len(hosts) == 0 {
		// Per AI.md PART 17: full priority list (no FQDN available here)
		hosts = []string{"127.0.0.1", "172.17.0.1"}
		if gw := getGatewayIP(); gw != "" {
			hosts = append(hosts, gw)
		}
	}

	ports := customPorts
	if len(ports) == 0 {
		ports = []int{25, 587, 465}
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
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Set read/write deadline
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// For port 465 (SMTPS), wrap in TLS first
	if port == 465 {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: host,
			// Allow self-signed for local servers
			InsecureSkipVerify: true,
		})
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
	defer client.Quit()

	// EHLO handshake - this is what the spec requires
	if err := client.Hello("localhost"); err != nil {
		return false
	}

	return true
}

// TestSMTPConfig tests a specific SMTP configuration per AI.md PART 17
// Returns nil on success, error on failure
func TestSMTPConfig(host string, port int) error {
	if host == "" {
		return fmt.Errorf("no SMTP host configured")
	}
	if !testSMTPConnection(host, port) {
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
	defer conn.Close()

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
	return s.Send("test", to, nil)
}

// GetTemplateList returns list of available templates
func (s *EmailService) GetTemplateList() []string {
	templates := make([]string, 0, len(defaultTemplates))
	for name := range defaultTemplates {
		templates = append(templates, name)
	}
	return templates
}

// GetTemplate returns a template's content
func (s *EmailService) GetTemplate(name string) (string, error) {
	return s.getTemplate(name)
}

// SaveTemplate saves a custom template
func (s *EmailService) SaveTemplate(name, content string) error {
	if err := os.MkdirAll(s.templateDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.templateDir, name+".txt")
	return os.WriteFile(path, []byte(content), 0644)
}

// ResetTemplate deletes a custom template (falls back to default)
func (s *EmailService) ResetTemplate(name string) error {
	path := filepath.Join(s.templateDir, name+".txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

// IsCustomTemplate checks if a template is customized
func (s *EmailService) IsCustomTemplate(name string) bool {
	path := filepath.Join(s.templateDir, name+".txt")
	_, err := os.Stat(path)
	return err == nil
}
