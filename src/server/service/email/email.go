// SPDX-License-Identifier: MIT
// AI.md PART 18: Email & Notifications
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
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

//go:embed template/*.txt
var embeddedTemplates embed.FS

// Default templates per AI.md PART 16
var defaultTemplates = map[string]string{
	"welcome": `Subject: Welcome to {app_name}
---
Hello,

Welcome to {app_name}!

Your admin panel is available at: {admin_url}
Username: {admin_username}

--
{app_name}
{app_url}`,

	"password_reset": `Subject: Password Reset Request - {app_name}
---
Hello,

A password reset was requested for your account.

Click the link below to reset your password:
{reset_link}

This link expires in 1 hour.

Request IP: {ip}

If you did not request this, please ignore this email.

--
{app_name}
{app_url}`,

	"backup_complete": `Subject: Backup Complete - {app_name}
---
Hello,

Your backup completed successfully.

Filename: {filename}
Size: {size}
Time: {timestamp}

--
{app_name}
{app_url}`,

	"backup_failed": `Subject: Backup Failed - {app_name}
---
Hello,

A backup operation failed.

Error: {error}
Time: {timestamp}

Please check your server configuration.

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

	"login_alert": `Subject: New Login - {app_name}
---
Hello,

A new login to your {app_name} admin panel was detected.

IP Address: {ip}
Location: {location}
Device: {device}
Time: {time}

If this wasn't you, please change your password immediately.

--
{app_name}
{app_url}`,

	"security_alert": `Subject: Security Alert - {app_name}
---
Hello,

A security event was detected:

Event: {event}
Source IP: {ip}
Details: {details}
Time: {timestamp}

Please review your security logs.

--
{app_name}
{app_url}`,

	"scheduler_error": `Subject: Scheduled Task Failed - {app_name}
---
Hello,

A scheduled task failed to complete.

Task: {task_name}
Error: {error}
Next run: {next_run}

Please check your server logs.

--
{app_name}
{app_url}`,

	"test": `Subject: Test Email from {app_name}
---
Hello,

This is a test email from {app_name}.

If you received this, your email settings are configured correctly.

Time: {timestamp}

--
{app_name}
{app_url}`,
}

// Service provides email sending functionality
type Service struct {
	cfg         *config.Config
	templateDir string
}

// New creates a new email service
func New(cfg *config.Config) *Service {
	paths := config.GetPaths("", "")
	templateDir := filepath.Join(paths.Config, "templates", "email")

	return &Service{
		cfg:         cfg,
		templateDir: templateDir,
	}
}

// Send sends an email using a template
func (s *Service) Send(templateName string, to string, vars map[string]string) error {
	if !s.cfg.Server.Email.Enabled {
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
func (s *Service) getTemplate(name string) (string, error) {
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
func (s *Service) parseTemplate(template string) (subject, body string) {
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
func (s *Service) applyVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}

// getGlobalVars returns global template variables
func (s *Service) getGlobalVars() map[string]string {
	return map[string]string{
		"app_name":    s.cfg.Server.Title,
		"app_url":     fmt.Sprintf("http://%s:%s", s.cfg.Server.FQDN, s.cfg.Server.Port),
		"admin_email": s.cfg.Server.Admin.Email,
		"timestamp":   time.Now().Format(time.RFC3339),
		"year":        fmt.Sprintf("%d", time.Now().Year()),
	}
}

// sendEmail sends the actual email via SMTP
func (s *Service) sendEmail(to, subject, body string) error {
	emailCfg := s.cfg.Server.Email

	// Try autodetect if enabled
	host, port := emailCfg.Host, emailCfg.Port
	if emailCfg.Autodetect && host == "" {
		h, p := s.autodetectSMTP()
		if h != "" {
			host, port = h, p
		}
	}

	if host == "" {
		return fmt.Errorf("no SMTP server configured")
	}

	from := emailCfg.From
	if from == "" {
		from = "noreply@" + s.cfg.Server.FQDN
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

	addr := fmt.Sprintf("%s:%d", host, port)

	// Determine auth
	var auth smtp.Auth
	if emailCfg.Username != "" {
		auth = smtp.PlainAuth("", emailCfg.Username, emailCfg.Password, host)
	}

	// Handle TLS
	tlsMode := emailCfg.TLS
	if tlsMode == "" || tlsMode == "auto" {
		if port == 465 {
			tlsMode = "required"
		} else {
			tlsMode = "starttls"
		}
	}

	if tlsMode == "required" {
		// Implicit TLS (port 465)
		return s.sendTLS(addr, host, auth, from, to, msg.Bytes())
	}

	// Standard SMTP with optional STARTTLS
	return smtp.SendMail(addr, auth, from, []string{to}, msg.Bytes())
}

// sendTLS sends email over implicit TLS
func (s *Service) sendTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
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

// autodetectSMTP tries to find an SMTP server per AI.md PART 31 lines 10267-10284
func (s *Service) autodetectSMTP() (string, int) {
	hosts := s.cfg.Server.Email.AutodetectHost
	if len(hosts) == 0 {
		// Per AI.md PART 31: Check localhost, 127.0.0.1, Docker host, gateway
		hosts = []string{"localhost", "127.0.0.1", "172.17.0.1"}
		// Try to get gateway IP
		if gw := getGatewayIP(); gw != "" {
			hosts = append(hosts, gw)
		}
	}

	ports := s.cfg.Server.Email.AutodetectPort
	if len(ports) == 0 {
		ports = []int{25, 587, 465}
	}

	for _, host := range hosts {
		for _, port := range ports {
			addr := fmt.Sprintf("%s:%d", host, port)
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err == nil {
				conn.Close()
				return host, port
			}
		}
	}

	return "", 0
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
func (s *Service) SendTest(to string) error {
	return s.Send("test", to, nil)
}

// GetTemplateList returns list of available templates
func (s *Service) GetTemplateList() []string {
	templates := make([]string, 0, len(defaultTemplates))
	for name := range defaultTemplates {
		templates = append(templates, name)
	}
	return templates
}

// GetTemplate returns a template's content
func (s *Service) GetTemplate(name string) (string, error) {
	return s.getTemplate(name)
}

// SaveTemplate saves a custom template
func (s *Service) SaveTemplate(name, content string) error {
	if err := os.MkdirAll(s.templateDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.templateDir, name+".txt")
	return os.WriteFile(path, []byte(content), 0644)
}

// ResetTemplate deletes a custom template (falls back to default)
func (s *Service) ResetTemplate(name string) error {
	path := filepath.Join(s.templateDir, name+".txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

// IsCustomTemplate checks if a template is customized
func (s *Service) IsCustomTemplate(name string) bool {
	path := filepath.Join(s.templateDir, name+".txt")
	_, err := os.Stat(path)
	return err == nil
}
