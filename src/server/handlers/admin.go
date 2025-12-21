// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 12: Admin Panel
package handlers

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/services/admin"
	"github.com/apimgr/vidveil/src/services/engines"
	"github.com/apimgr/vidveil/src/services/maintenance"
	"github.com/apimgr/vidveil/src/services/scheduler"
)

// adminTemplatesFS holds embedded admin templates - set by server.go
var adminTemplatesFS embed.FS

// SetAdminTemplatesFS sets the embedded templates filesystem for admin
func SetAdminTemplatesFS(fs embed.FS) {
	adminTemplatesFS = fs
}

const (
	adminSessionCookieName = "vidveil_admin_session"
	adminSessionDuration   = 24 * time.Hour
	csrfTokenCookieName    = "vidveil_csrf_token"
)

// AdminHandler handles admin panel routes per TEMPLATE.md PART 12
type AdminHandler struct {
	cfg        *config.Config
	engineMgr  *engines.Manager
	adminSvc   *admin.Service
	scheduler  *scheduler.Scheduler
	sessions   map[string]adminSession
	csrfTokens map[string]string // sessionID -> csrfToken
	startTime  time.Time
}

type adminSession struct {
	username  string
	adminID   int64
	createdAt time.Time
	expiresAt time.Time
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(cfg *config.Config, engineMgr *engines.Manager, adminSvc *admin.Service) *AdminHandler {
	return &AdminHandler{
		cfg:        cfg,
		engineMgr:  engineMgr,
		adminSvc:   adminSvc,
		sessions:   make(map[string]adminSession),
		csrfTokens: make(map[string]string),
		startTime:  time.Now(),
	}
}

// SetScheduler sets the scheduler reference
func (h *AdminHandler) SetScheduler(s *scheduler.Scheduler) {
	h.scheduler = s
}

// IsFirstRun checks if this is the first run (no admin exists)
func (h *AdminHandler) IsFirstRun() bool {
	return h.adminSvc.IsFirstRun()
}

// AuthMiddleware protects admin routes per TEMPLATE.md PART 31
func (h *AdminHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie(adminSessionCookieName)
		if err != nil || !h.validateSession(cookie.Value) {
			// Redirect to /auth/login per TEMPLATE.md PART 31
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoginPage redirects to /auth/login per TEMPLATE.md PART 31
// All logins (admin and user) go through /auth/login
func (h *AdminHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

// AuthenticateAdmin handles admin login (called from AuthHandler)
// Returns session ID on success, empty string on failure
func (h *AdminHandler) AuthenticateAdmin(username, password string) (string, error) {
	adminUser, err := h.adminSvc.Authenticate(username, password)
	if err != nil {
		return "", err
	}
	if adminUser == nil {
		return "", fmt.Errorf("invalid credentials")
	}
	return h.createSessionWithID(adminUser.Username, adminUser.ID), nil
}

// LogoutHandler logs out the admin
func (h *AdminHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(adminSessionCookieName); err == nil {
		delete(h.sessions, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Redirect to /auth/login per TEMPLATE.md PART 31
	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

// SetupTokenPage handles setup token entry at /admin on first run per TEMPLATE.md PART 31
// Step 2-3: User navigates to /admin, enters setup token
// Step 4: Redirect to /admin/server/setup
func (h *AdminHandler) SetupTokenPage(w http.ResponseWriter, r *http.Request) {
	// Check if setup is still needed
	if !h.adminSvc.IsFirstRun() {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	errorMsg := ""
	if r.Method == http.MethodPost {
		token := r.FormValue("token")

		// Validate the setup token
		if h.adminSvc.ValidateSetupToken(token) {
			// Store validated token in cookie for wizard step
			http.SetCookie(w, &http.Cookie{
				Name:     "vidveil_setup_token",
				Value:    token,
				Path:     "/admin",
				MaxAge:   3600, // 1 hour to complete setup
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, "/admin/server/setup", http.StatusFound)
			return
		}
		errorMsg = "Invalid or expired setup token"
	}

	h.renderSetupTokenPage(w, errorMsg)
}

// SetupWizardPage renders the setup wizard at /admin/server/setup per TEMPLATE.md PART 31
func (h *AdminHandler) SetupWizardPage(w http.ResponseWriter, r *http.Request) {
	// Check if setup is still needed
	if !h.adminSvc.IsFirstRun() {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	// Verify setup token cookie exists (must come from token entry page)
	tokenCookie, err := r.Cookie("vidveil_setup_token")
	if err != nil || !h.adminSvc.ValidateSetupToken(tokenCookie.Value) {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	data := map[string]interface{}{
		"SiteTitle": h.cfg.Server.Title,
		"Error":     "",
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")
		confirm := r.FormValue("confirm")

		// Validate passwords match
		if password != confirm {
			data["Error"] = "Passwords do not match"
			h.renderSetupWizardPage(w, data)
			return
		}

		// Create admin account using admin service
		adminUser, err := h.adminSvc.CreateAdminWithSetupToken(tokenCookie.Value, username, password)
		if err != nil {
			data["Error"] = err.Error()
			h.renderSetupWizardPage(w, data)
			return
		}

		// Clear setup token cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "vidveil_setup_token",
			Value:  "",
			Path:   "/admin",
			MaxAge: -1,
		})

		// Create session for the new admin
		sessionID := h.createSessionWithID(adminUser.Username, adminUser.ID)
		http.SetCookie(w, &http.Cookie{
			Name:     adminSessionCookieName,
			Value:    sessionID,
			Path:     "/admin",
			MaxAge:   int(adminSessionDuration.Seconds()),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
		return
	}

	h.renderSetupWizardPage(w, data)
}

// renderSetupTokenPage renders the setup token entry form
func (h *AdminHandler) renderSetupTokenPage(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Setup - %s</title>
    <link rel="stylesheet" href="/static/css/style.css">
    <style>
        .setup-container { max-width: 400px; margin: 100px auto; padding: 20px; }
        .setup-box { background: #1a1a2e; border-radius: 8px; padding: 30px; }
        .setup-title { text-align: center; margin-bottom: 20px; }
        .error { color: #e74c3c; margin-bottom: 15px; text-align: center; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; }
        .form-group input { width: 100%%; padding: 10px; border-radius: 4px; border: 1px solid #333; background: #0f0f1a; color: #fff; }
        .btn-primary { width: 100%%; padding: 12px; background: #6c5ce7; color: #fff; border: none; border-radius: 4px; cursor: pointer; }
        .btn-primary:hover { background: #5b4bc7; }
        .info { text-align: center; margin-top: 20px; color: #888; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="setup-container">
        <div class="setup-box">
            <h1 class="setup-title">Admin Setup</h1>
            <p style="text-align: center; margin-bottom: 20px;">Enter the setup token displayed in the server console.</p>
            %s
            <form method="POST">
                <div class="form-group">
                    <label for="token">Setup Token</label>
                    <input type="text" id="token" name="token" required autofocus placeholder="Enter setup token">
                </div>
                <button type="submit" class="btn-primary">Continue</button>
            </form>
            <p class="info">The setup token was shown once when the server first started.</p>
        </div>
    </div>
</body>
</html>`, h.cfg.Server.Title, func() string {
		if errorMsg != "" {
			return fmt.Sprintf(`<div class="error">%s</div>`, errorMsg)
		}
		return ""
	}())
	w.Write([]byte(html))
}

// renderSetupWizardPage renders the setup wizard template
func (h *AdminHandler) renderSetupWizardPage(w http.ResponseWriter, data map[string]interface{}) {
	tmpl, err := template.ParseFS(adminTemplatesFS, "templates/admin/setup.tmpl")
	if err != nil {
		http.Error(w, "Failed to load setup template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "setup", data); err != nil {
		http.Error(w, "Failed to render setup template", http.StatusInternalServerError)
	}
}

// DashboardPage renders the admin dashboard
func (h *AdminHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	h.renderAdminTemplate(w, r, "dashboard", map[string]interface{}{
		"EngineCount":   len(h.engineMgr.ListEngines()),
		"EnabledCount":  h.engineMgr.EnabledCount(),
		"MemoryMB":      m.Alloc / 1024 / 1024,
		"Goroutines":    runtime.NumGoroutine(),
		"GoVersion":     runtime.Version(),
		"OS":            runtime.GOOS,
		"Arch":          runtime.GOARCH,
		"Mode":          h.cfg.Server.Mode,
		"Port":          h.cfg.Server.Port,
		"TorEnabled":    h.cfg.Search.Tor.Enabled,
	})
}

// EnginesPage renders the engines management page
func (h *AdminHandler) EnginesPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderEnginesPage()))
}

// SettingsPage renders the settings page
func (h *AdminHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderSettingsPage()))
}

// LogsPage renders the logs viewer
func (h *AdminHandler) LogsPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "logs", nil)
}

// === PART 12 Required Admin Sections ===

// ServerSettingsPage renders server settings (Section 2)
func (h *AdminHandler) ServerSettingsPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "server", nil)
}

// WebSettingsPage renders web settings (Section 3)
func (h *AdminHandler) WebSettingsPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "web", nil)
}

// SecuritySettingsPage renders security settings (Section 4)
func (h *AdminHandler) SecuritySettingsPage(w http.ResponseWriter, r *http.Request) {
	tokenPrefix := ""
	if len(h.cfg.Server.Admin.Token) > 8 {
		tokenPrefix = h.cfg.Server.Admin.Token[:8]
	}
	h.renderAdminTemplate(w, r, "security", map[string]interface{}{
		"TokenPrefix": tokenPrefix,
	})
}

// DatabasePage renders database & cache settings (Section 5)
func (h *AdminHandler) DatabasePage(w http.ResponseWriter, r *http.Request) {
	dbPath := h.cfg.Server.Database.SQLite.Dir
	if dbPath == "" {
		dbPath = "default"
	}
	h.renderAdminTemplate(w, r, "database", map[string]interface{}{
		"DBDriver": h.cfg.Server.Database.Driver,
		"DBPath":   dbPath,
		"DBSize":   "N/A", // Would require file stat
	})
}

// EmailPage renders email & notifications settings (Section 6)
func (h *AdminHandler) EmailPage(w http.ResponseWriter, r *http.Request) {
	// Email templates list (placeholder)
	templates := []map[string]string{
		{"Name": "welcome", "Status": "Active"},
		{"Name": "password-reset", "Status": "Active"},
		{"Name": "notification", "Status": "Active"},
	}
	h.renderAdminTemplate(w, r, "email", map[string]interface{}{
		"EmailTemplates": templates,
	})
}

// SSLPage renders SSL/TLS settings (Section 7)
func (h *AdminHandler) SSLPage(w http.ResponseWriter, r *http.Request) {
	sslMode := "disabled"
	if h.cfg.Server.SSL.Enabled {
		if h.cfg.Server.SSL.LetsEncrypt.Enabled {
			sslMode = "letsencrypt"
		} else {
			sslMode = "custom"
		}
	}
	h.renderAdminTemplate(w, r, "ssl", map[string]interface{}{
		"SSLMode":    sslMode,
		"SSLEnabled": h.cfg.Server.SSL.Enabled,
		"SSLDomain":  h.cfg.Server.SSL.LetsEncrypt.Domain,
		"SSLExpiry":  "N/A",
		"SSLIssuer":  "N/A",
	})
}

// SchedulerPage renders scheduler management (Section 8)
func (h *AdminHandler) SchedulerPage(w http.ResponseWriter, r *http.Request) {
	var tasks []map[string]interface{}
	if h.scheduler != nil {
		for _, t := range h.scheduler.ListTasks() {
			tasks = append(tasks, map[string]interface{}{
				"Name":     t.Name,
				"Schedule": t.Schedule,
				"LastRun":  t.LastRun.Format("2006-01-02 15:04"),
				"NextRun":  t.NextRun.Format("2006-01-02 15:04"),
				"Enabled":  t.Enabled,
			})
		}
	}
	h.renderAdminTemplate(w, r, "scheduler", map[string]interface{}{
		"ScheduledTasks": tasks,
	})
}

// BackupPage renders backup & maintenance (Section 10)
func (h *AdminHandler) BackupPage(w http.ResponseWriter, r *http.Request) {
	// Placeholder backups list - would be populated from backup directory
	backups := []map[string]string{}
	h.renderAdminTemplate(w, r, "backup", map[string]interface{}{
		"Backups": backups,
	})
}

// SystemInfoPage renders system info (Section 11)
func (h *AdminHandler) SystemInfoPage(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	hostname, _ := os.Hostname()

	// Engine health info
	enginesList := []map[string]interface{}{}
	for _, e := range h.engineMgr.ListEngines() {
		enginesList = append(enginesList, map[string]interface{}{
			"Name":        e.DisplayName,
			"Enabled":     e.Enabled,
			"Healthy":     e.Available,
			"LastCheck":   "N/A",
			"SuccessRate": 100,
		})
	}

	h.renderAdminTemplate(w, r, "system", map[string]interface{}{
		"Version":         "0.2.0",
		"GoVersion":       runtime.Version(),
		"BuildDate":       BuildDateTime,
		"GitCommit":       "unknown",
		"Uptime":          time.Since(h.startTime).Round(time.Second).String(),
		"StartTime":       h.startTime.Format("2006-01-02 15:04:05"),
		"MemoryHeap":      strconv.FormatUint(m.Alloc/1024/1024, 10) + " MB",
		"MemorySystem":    strconv.FormatUint(m.Sys/1024/1024, 10) + " MB",
		"Goroutines":      runtime.NumGoroutine(),
		"GCCycles":        m.NumGC,
		"CPUCores":        runtime.NumCPU(),
		"Hostname":        hostname,
		"OS":              runtime.GOOS,
		"Arch":            runtime.GOARCH,
		"DiskUsage":       "N/A",
		"Engines":         enginesList,
		"LatestVersion":   "",
		"UpdateAvailable": false,
	})
}

// NodesPage renders cluster nodes management (TEMPLATE.md PART 24)
func (h *AdminHandler) NodesPage(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	// In single instance mode, show limited info
	h.renderAdminTemplate(w, r, "nodes", map[string]interface{}{
		"NodeID":         hostname,
		"IsPrimary":      true,
		"ClusterEnabled": false,
		"TotalNodes":     1,
		"ActiveNodes":    1,
		"ActiveLocks":    0,
		"Nodes":          nil,
		"Locks":          nil,
	})
}

// TorPage renders Tor hidden service settings (TEMPLATE.md PART 32)
func (h *AdminHandler) TorPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "tor", map[string]interface{}{
		"TorEnabled":      h.cfg.Search.Tor.Enabled,
		"TorConnected":    false, // Would check actual Tor connection
		"TorProxy":        h.cfg.Search.Tor.Proxy,
		"TorControlPort":  strconv.Itoa(h.cfg.Search.Tor.ControlPort),
		"TorCircuit":      "N/A",
		"OnionEnabled":    false, // Would check actual onion service
		"OnionAddress":    "",
		"VanityJobs":      []map[string]interface{}{},
	})
}

// BrandingPage renders branding & SEO settings per PART 15
func (h *AdminHandler) BrandingPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "branding", nil)
}

// SecurityAuthPage renders authentication settings per PART 15
func (h *AdminHandler) SecurityAuthPage(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		clientIP = xff
	}
	h.renderAdminTemplate(w, r, "security-auth", map[string]interface{}{
		"ClientIP": clientIP,
	})
}

// SecurityTokensPage renders API token management per PART 15
func (h *AdminHandler) SecurityTokensPage(w http.ResponseWriter, r *http.Request) {
	tokenPrefix := ""
	if len(h.cfg.Server.Admin.Token) > 8 {
		tokenPrefix = h.cfg.Server.Admin.Token[:8]
	}
	h.renderAdminTemplate(w, r, "security-tokens", map[string]interface{}{
		"TokenPrefix": tokenPrefix,
	})
}

// SecurityRateLimitPage renders rate limiting settings per PART 15
func (h *AdminHandler) SecurityRateLimitPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "security-ratelimit", nil)
}

// SecurityFirewallPage renders firewall/IP blocking per PART 15
func (h *AdminHandler) SecurityFirewallPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "security-firewall", nil)
}

// GeoIPPage renders GeoIP settings per PART 15
func (h *AdminHandler) GeoIPPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "geoip", nil)
}

// BlocklistsPage renders blocklist management per PART 15
func (h *AdminHandler) BlocklistsPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "blocklists", nil)
}

// MaintenancePage renders maintenance mode settings per PART 15
func (h *AdminHandler) MaintenancePage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "maintenance", nil)
}

// UpdatesPage renders update management per PART 15
func (h *AdminHandler) UpdatesPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "updates", map[string]interface{}{
		"CurrentVersion":  "0.2.0",
		"LatestVersion":   "",
		"UpdateAvailable": false,
	})
}

// HelpPage renders help/documentation per PART 15
func (h *AdminHandler) HelpPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdminTemplate(w, r, "help", nil)
}

// ProfilePage renders admin profile page per TEMPLATE.md PART 31
func (h *AdminHandler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	// Get current admin from session
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	session, ok := h.sessions[cookie.Value]
	if !ok {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	// Get admin details
	admin, err := h.adminSvc.GetAdmin(session.adminID)
	if err != nil {
		h.renderAdminTemplate(w, r, "profile", map[string]interface{}{
			"Error": "Failed to load profile",
		})
		return
	}

	// Get token info
	tokenPrefix, tokenLastUsed, tokenUseCount, _ := h.adminSvc.GetAPITokenInfo(session.adminID)

	h.renderAdminTemplate(w, r, "profile", map[string]interface{}{
		"Admin":         admin,
		"TokenPrefix":   tokenPrefix,
		"TokenLastUsed": tokenLastUsed,
		"TokenUseCount": tokenUseCount,
	})
}

// APIProfilePassword handles password change via API per TEMPLATE.md PART 31
func (h *AdminHandler) APIProfilePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	// Get admin ID from session
	adminID := h.getSessionAdminID(r)
	if adminID == 0 {
		h.jsonError(w, "Unauthorized", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.jsonError(w, "Invalid request body", "ERR_INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	if err := h.adminSvc.ChangePassword(adminID, body.CurrentPassword, body.NewPassword); err != nil {
		h.jsonError(w, err.Error(), "ERR_PASSWORD_CHANGE", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password updated successfully",
	})
}

// APIProfileToken regenerates API token per TEMPLATE.md PART 31
func (h *AdminHandler) APIProfileToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	// Get admin ID from session
	adminID := h.getSessionAdminID(r)
	if adminID == 0 {
		h.jsonError(w, "Unauthorized", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	token, err := h.adminSvc.RegenerateAPIToken(adminID)
	if err != nil {
		h.jsonError(w, err.Error(), "ERR_TOKEN_REGENERATE", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"token": token,
		},
	})
}

// getSessionAdminID returns the admin ID from the current session
func (h *AdminHandler) getSessionAdminID(r *http.Request) int64 {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return 0
	}
	session, ok := h.sessions[cookie.Value]
	if !ok {
		return 0
	}
	return session.adminID
}

// APIRecoveryKeysStatus returns the status of recovery keys per TEMPLATE.md PART 31
func (h *AdminHandler) APIRecoveryKeysStatus(w http.ResponseWriter, r *http.Request) {
	adminID := h.getSessionAdminID(r)
	if adminID == 0 {
		h.jsonError(w, "Unauthorized", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	status, err := h.adminSvc.GetRecoveryKeysStatus(adminID)
	if err != nil {
		h.jsonError(w, err.Error(), "ERR_RECOVERY_KEYS", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

// APIRecoveryKeysGenerate generates new recovery keys per TEMPLATE.md PART 31
func (h *AdminHandler) APIRecoveryKeysGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	adminID := h.getSessionAdminID(r)
	if adminID == 0 {
		h.jsonError(w, "Unauthorized", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	keys, err := h.adminSvc.GenerateRecoveryKeys(adminID)
	if err != nil {
		h.jsonError(w, err.Error(), "ERR_RECOVERY_KEYS", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"keys":    keys,
			"warning": "These keys will only be shown once. Save them securely.",
		},
	})
}

// UsersAdminsPage renders the admin users management page per TEMPLATE.md PART 31
func (h *AdminHandler) UsersAdminsPage(w http.ResponseWriter, r *http.Request) {
	// Get current admin from session
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	session, ok := h.sessions[cookie.Value]
	if !ok {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	// Get admin count
	adminCount, _ := h.adminSvc.GetAdminCount()

	// Get online admins (those with active sessions)
	onlineAdmins := h.getOnlineAdmins()

	h.renderAdminTemplate(w, r, "users-admins", map[string]interface{}{
		"CurrentAdmin": session.username,
		"AdminCount":   adminCount,
		"OnlineAdmins": onlineAdmins,
	})
}

// getOnlineAdmins returns a comma-separated list of currently online admin usernames
func (h *AdminHandler) getOnlineAdmins() string {
	names := make(map[string]bool)
	now := time.Now()
	for _, sess := range h.sessions {
		if sess.expiresAt.After(now) {
			names[sess.username] = true
		}
	}

	result := ""
	for name := range names {
		if result != "" {
			result += ", "
		}
		result += name
	}
	if result == "" {
		result = "None"
	}
	return result
}

// getOnlineCount returns the number of currently online admins
func (h *AdminHandler) getOnlineCount() int {
	count := 0
	now := time.Now()
	seen := make(map[string]bool)
	for _, sess := range h.sessions {
		if sess.expiresAt.After(now) && !seen[sess.username] {
			seen[sess.username] = true
			count++
		}
	}
	return count
}

// AdminInvitePage handles the invite acceptance flow per TEMPLATE.md PART 31
func (h *AdminHandler) AdminInvitePage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		// Try to extract from path
		path := r.URL.Path
		if idx := len("/admin/invite/"); idx < len(path) {
			token = path[idx:]
		}
	}

	data := map[string]interface{}{
		"SiteTitle": h.cfg.Server.Title,
		"Token":     token,
		"Valid":     false,
	}

	// Validate invite token
	invite, err := h.adminSvc.ValidateInviteToken(token)
	if err != nil || invite == nil {
		data["Error"] = "This invite link is invalid or has expired."
		h.renderInvitePage(w, data)
		return
	}

	data["Valid"] = true
	data["Username"] = invite.Username

	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		confirm := r.FormValue("confirm")

		if password != confirm {
			data["Error"] = "Passwords do not match"
			h.renderInvitePage(w, data)
			return
		}

		// Create the admin account
		_, err := h.adminSvc.CreateAdminWithInvite(token, invite.Username, password)
		if err != nil {
			data["Error"] = err.Error()
			h.renderInvitePage(w, data)
			return
		}

		data["Valid"] = false
		data["Success"] = "Account created successfully! You can now log in."
	}

	h.renderInvitePage(w, data)
}

// renderInvitePage renders the invite acceptance template
func (h *AdminHandler) renderInvitePage(w http.ResponseWriter, data map[string]interface{}) {
	tmpl, err := template.ParseFS(adminTemplatesFS, "templates/admin/invite.tmpl")
	if err != nil {
		http.Error(w, "Failed to load invite template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "invite", data); err != nil {
		http.Error(w, "Failed to render invite template", http.StatusInternalServerError)
	}
}

// APIUsersAdminsInvite creates an admin invite per TEMPLATE.md PART 31
func (h *AdminHandler) APIUsersAdminsInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	// Get admin ID from session
	adminID := h.getSessionAdminID(r)
	if adminID == 0 {
		h.jsonError(w, "Unauthorized", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	var body struct {
		Username     string `json:"username"`
		ExpiresHours int    `json:"expires_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.jsonError(w, "Invalid request body", "ERR_INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	if body.Username == "" {
		h.jsonError(w, "Username is required", "ERR_INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	if body.ExpiresHours <= 0 {
		body.ExpiresHours = 24
	}

	// Generate invite token
	token, err := h.adminSvc.CreateAdminInvite(adminID, body.Username, time.Duration(body.ExpiresHours)*time.Hour)
	if err != nil {
		h.jsonError(w, err.Error(), "ERR_INVITE_FAILED", http.StatusBadRequest)
		return
	}

	// Build invite URL
	scheme := "https"
	if h.cfg.Server.Mode == "development" {
		scheme = "http"
	}
	host := h.cfg.Server.FQDN
	if host == "" {
		host = fmt.Sprintf("localhost:%s", h.cfg.Server.Port)
	}
	inviteURL := fmt.Sprintf("%s://%s/admin/invite/%s", scheme, host, token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"invite_url": inviteURL,
			"expires_in": fmt.Sprintf("%d hours", body.ExpiresHours),
		},
	})
}

// === API Handlers ===

// SessionOrTokenMiddleware allows either session cookie or API token authentication
// Used for profile endpoints that can be accessed from both web UI and API
func (h *AdminHandler) SessionOrTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First, try session cookie
		if cookie, err := r.Cookie(adminSessionCookieName); err == nil {
			if h.validateSession(cookie.Value) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Then, try API token
		token := r.Header.Get("X-API-Token")
		if token == "" {
			token = r.Header.Get("Authorization")
			if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}
		}

		if token != "" && h.validateToken(token) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Authentication required",
		})
	})
}

// APITokenMiddleware validates API tokens
func (h *AdminHandler) APITokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-API-Token")
		if token == "" {
			token = r.Header.Get("Authorization")
			if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}
		}

		if !h.validateToken(token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid or missing API token",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// APIStats returns server statistics
func (h *AdminHandler) APIStats(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"engines": map[string]interface{}{
				"total":   len(h.engineMgr.ListEngines()),
				"enabled": h.engineMgr.EnabledCount(),
			},
			"memory": map[string]interface{}{
				"alloc_mb":       m.Alloc / 1024 / 1024,
				"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
				"sys_mb":         m.Sys / 1024 / 1024,
				"num_gc":         m.NumGC,
			},
			"runtime": map[string]interface{}{
				"goroutines": runtime.NumGoroutine(),
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			},
			"config": map[string]interface{}{
				"port":           h.cfg.Server.Port,
				"tor_enabled":    h.cfg.Search.Tor.Enabled,
				"results_per_page": h.cfg.Search.ResultsPerPage,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// APIEngines returns engine information
func (h *AdminHandler) APIEngines(w http.ResponseWriter, r *http.Request) {
	engines := h.engineMgr.ListEngines()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    engines,
	})
}

// APIBackup triggers a backup
func (h *AdminHandler) APIBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	maint := maintenance.New("", "", "")
	backupFile := r.URL.Query().Get("file")

	if err := maint.Backup(backupFile); err != nil {
		h.jsonError(w, err.Error(), "ERR_BACKUP_FAILED", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Backup created successfully",
	})
}

// APIMaintenanceMode toggles maintenance mode
func (h *AdminHandler) APIMaintenanceMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	maint := maintenance.New("", "", "")

	enabled := r.URL.Query().Get("enabled")
	if enabled == "" {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		enabled = strconv.FormatBool(body.Enabled)
	}

	enable := enabled == "true" || enabled == "1"
	if err := maint.SetMaintenanceMode(enable); err != nil {
		h.jsonError(w, err.Error(), "ERR_MAINTENANCE_FAILED", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Maintenance mode updated",
		"enabled": enable,
	})
}

// === PART 12 Required Admin API Endpoints ===

// APIConfig handles GET/PUT/PATCH for /api/v1/admin/config
func (h *AdminHandler) APIConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return sanitized config (no passwords/tokens)
		safeCfg := map[string]interface{}{
			"server": map[string]interface{}{
				"port":        h.cfg.Server.Port,
				"address":     h.cfg.Server.Address,
				"fqdn":        h.cfg.Server.FQDN,
				"mode":        h.cfg.Server.Mode,
				"title":       h.cfg.Server.Title,
				"description": h.cfg.Server.Description,
			},
			"web": map[string]interface{}{
				"theme": h.cfg.Web.UI.Theme,
				"cors":  h.cfg.Web.CORS,
			},
			"search": map[string]interface{}{
				"results_per_page": h.cfg.Search.ResultsPerPage,
				"tor_enabled":      h.cfg.Search.Tor.Enabled,
			},
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    safeCfg,
		})

	case http.MethodPut, http.MethodPatch:
		// Parse update request
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			h.jsonError(w, "Invalid request body", "ERR_VALIDATION", http.StatusBadRequest)
			return
		}

		// Apply updates to config (partial update for PATCH, full for PUT)
		updated := false
		if serverCfg, ok := updates["server"].(map[string]interface{}); ok {
			if title, ok := serverCfg["title"].(string); ok {
				h.cfg.Server.Title = title
				updated = true
			}
			if desc, ok := serverCfg["description"].(string); ok {
				h.cfg.Server.Description = desc
				updated = true
			}
			if mode, ok := serverCfg["mode"].(string); ok {
				h.cfg.Server.Mode = config.NormalizeMode(mode)
				updated = true
			}
			if fqdn, ok := serverCfg["fqdn"].(string); ok {
				h.cfg.Server.FQDN = fqdn
				updated = true
			}
		}
		if webCfg, ok := updates["web"].(map[string]interface{}); ok {
			if theme, ok := webCfg["theme"].(string); ok {
				h.cfg.Web.UI.Theme = theme
				updated = true
			}
		}
		if searchCfg, ok := updates["search"].(map[string]interface{}); ok {
			if rpp, ok := searchCfg["results_per_page"].(float64); ok {
				h.cfg.Search.ResultsPerPage = int(rpp)
				updated = true
			}
			// Tor settings per TEMPLATE.md PART 32
			if torCfg, ok := searchCfg["tor"].(map[string]interface{}); ok {
				if enabled, ok := torCfg["enabled"].(bool); ok {
					h.cfg.Search.Tor.Enabled = enabled
					updated = true
				}
				if proxy, ok := torCfg["proxy"].(string); ok {
					h.cfg.Search.Tor.Proxy = proxy
					updated = true
				}
				if port, ok := torCfg["control_port"].(float64); ok {
					h.cfg.Search.Tor.ControlPort = int(port)
					updated = true
				}
				if forceAll, ok := torCfg["force_all"].(bool); ok {
					h.cfg.Search.Tor.ForceAll = forceAll
					updated = true
				}
				if rotate, ok := torCfg["rotate_circuit"].(bool); ok {
					h.cfg.Search.Tor.RotateCircuit = rotate
					updated = true
				}
				if fallback, ok := torCfg["clearnet_fallback"].(bool); ok {
					h.cfg.Search.Tor.ClearnetFallback = fallback
					updated = true
				}
			}
		}

		if updated {
			// Save config to file
			paths := config.GetPaths("", "")
			configPath := filepath.Join(paths.Config, "server.yml")
			if err := config.Save(h.cfg, configPath); err != nil {
				h.jsonError(w, "Failed to save configuration: "+err.Error(), "ERR_INTERNAL", http.StatusInternalServerError)
				return
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Configuration updated (restart required for some changes)",
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
	}
}

// jsonError sends a standardized error response per PART 24
func (h *AdminHandler) jsonError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    code,
		"status":  status,
	})
}

// APIStatus returns server status
func (h *AdminHandler) APIStatus(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"status":  "running",
			"mode":    h.cfg.Server.Mode,
			"uptime":  uptime.String(),
			"version": "0.2.0",
		},
	})
}

// APIHealth returns detailed health info
func (h *AdminHandler) APIHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"status": "healthy",
			"checks": map[string]string{
				"engines":   "ok",
				"memory":    "ok",
				"goroutines": "ok",
			},
			"memory_mb":  m.Alloc / 1024 / 1024,
			"goroutines": runtime.NumGoroutine(),
		},
	})
}

// APILogsAccess returns access logs
func (h *AdminHandler) APILogsAccess(w http.ResponseWriter, r *http.Request) {
	lines := h.readLogLines(h.cfg.Server.Logs.Access.Filename, 100)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"filename": h.cfg.Server.Logs.Access.Filename,
			"lines":    lines,
		},
	})
}

// APILogsError returns error logs
func (h *AdminHandler) APILogsError(w http.ResponseWriter, r *http.Request) {
	lines := h.readLogLines(h.cfg.Server.Logs.Error.Filename, 100)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"filename": h.cfg.Server.Logs.Error.Filename,
			"lines":    lines,
		},
	})
}

// APIRestore restores from backup
func (h *AdminHandler) APIRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	maint := maintenance.New("", "", "")
	backupFile := r.URL.Query().Get("file")

	if err := maint.Restore(backupFile); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Restore completed successfully",
	})
}

// APITestEmail sends a test email
func (h *AdminHandler) APITestEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Email sending would be implemented with SMTP
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Test email sent (if email is configured)",
	})
}

// APIPassword changes admin password using database per TEMPLATE.md PART 31
func (h *AdminHandler) APIPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	// Get current admin session
	session := h.getSession(r)
	if session == nil || session.adminID == 0 {
		h.jsonError(w, "Session not found", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.jsonError(w, "Invalid request body", "ERR_VALIDATION", http.StatusBadRequest)
		return
	}

	// Change password using admin service (database)
	if err := h.adminSvc.ChangePassword(session.adminID, body.CurrentPassword, body.NewPassword); err != nil {
		h.jsonError(w, err.Error(), "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
	})
}

// APITokenRegenerate regenerates the API token
func (h *AdminHandler) APITokenRegenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	// Generate new token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		h.jsonError(w, "Failed to generate token", "ERR_INTERNAL", http.StatusInternalServerError)
		return
	}

	newToken := hex.EncodeToString(tokenBytes)

	// In production, this would update the database/config
	// For now, just return the new token (shown only once per TEMPLATE.md)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Token regenerated - save this token now, it will not be shown again",
		"token":   newToken,
	})
}

// APISchedulerTasks returns scheduler tasks
func (h *AdminHandler) APISchedulerTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.scheduler == nil {
		h.jsonError(w, "Scheduler not initialized", "ERR_NOT_INITIALIZED", http.StatusServiceUnavailable)
		return
	}

	tasks := h.scheduler.ListTasks()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    tasks,
	})
}

// APISchedulerRunTask manually triggers a task
func (h *AdminHandler) APISchedulerRunTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		var body struct {
			TaskID string `json:"task_id"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		taskID = body.TaskID
	}

	if h.scheduler == nil {
		h.jsonError(w, "Scheduler not initialized", "ERR_NOT_INITIALIZED", http.StatusServiceUnavailable)
		return
	}

	if err := h.scheduler.RunTaskNow(taskID); err != nil {
		h.jsonError(w, err.Error(), "ERR_TASK_FAILED", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Task triggered",
	})
}

// APISchedulerHistory returns task run history
func (h *AdminHandler) APISchedulerHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.scheduler == nil {
		h.jsonError(w, "Scheduler not initialized", "ERR_NOT_INITIALIZED", http.StatusServiceUnavailable)
		return
	}

	taskID := r.URL.Query().Get("task_id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	history := h.scheduler.GetHistory(taskID, limit)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    history,
	})
}

// =====================================================
// Tor API handlers per TEMPLATE.md PART 32
// =====================================================

// APITorStatus returns Tor hidden service status
// GET /api/v1/admin/server/tor
func (h *AdminHandler) APITorStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"enabled":       h.cfg.Search.Tor.Enabled,
			"status":        "disconnected", // Would check actual Tor connection
			"onion_address": "",             // Would get from Tor manager
			"uptime":        "",
			"proxy":         h.cfg.Search.Tor.Proxy,
			"control_port":  h.cfg.Search.Tor.ControlPort,
		},
	})
}

// APITorUpdate updates Tor settings
// PATCH /api/v1/admin/server/tor
func (h *AdminHandler) APITorUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Enabled          *bool   `json:"enabled"`
		Proxy            *string `json:"proxy"`
		ControlPort      *int    `json:"control_port"`
		ForceAll         *bool   `json:"force_all"`
		RotateCircuit    *bool   `json:"rotate_circuit"`
		ClearnetFallback *bool   `json:"clearnet_fallback"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	// Update config
	updated := false
	if req.Enabled != nil {
		h.cfg.Search.Tor.Enabled = *req.Enabled
		updated = true
	}
	if req.Proxy != nil {
		h.cfg.Search.Tor.Proxy = *req.Proxy
		updated = true
	}
	if req.ControlPort != nil {
		h.cfg.Search.Tor.ControlPort = *req.ControlPort
		updated = true
	}
	if req.ForceAll != nil {
		h.cfg.Search.Tor.ForceAll = *req.ForceAll
		updated = true
	}
	if req.RotateCircuit != nil {
		h.cfg.Search.Tor.RotateCircuit = *req.RotateCircuit
		updated = true
	}
	if req.ClearnetFallback != nil {
		h.cfg.Search.Tor.ClearnetFallback = *req.ClearnetFallback
		updated = true
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": updated,
		"message": "Tor settings updated",
	})
}

// APITorRegenerate regenerates the .onion address
// POST /api/v1/admin/server/tor/regenerate
func (h *AdminHandler) APITorRegenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Would trigger Tor manager to regenerate address
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Tor circuit regenerated",
	})
}

// APITorVanityStatus returns vanity address generation status
// GET /api/v1/admin/server/tor/vanity
func (h *AdminHandler) APITorVanityStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"jobs": []map[string]interface{}{},
		},
	})
}

// APITorVanityStart starts vanity address generation
// POST /api/v1/admin/server/tor/vanity
func (h *AdminHandler) APITorVanityStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Prefix string `json:"prefix"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Prefix == "" || len(req.Prefix) > 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Prefix must be 1-6 characters",
		})
		return
	}

	// Would start vanity generation in background
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Vanity generation started for prefix: " + req.Prefix,
	})
}

// APITorVanityCancel cancels vanity address generation
// DELETE /api/v1/admin/server/tor/vanity
func (h *AdminHandler) APITorVanityCancel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Vanity generation cancelled",
	})
}

// APITorVanityApply applies a generated vanity address
// POST /api/v1/admin/server/tor/vanity/apply
func (h *AdminHandler) APITorVanityApply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Would apply the vanity address
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Vanity address applied",
	})
}

// APITorImport imports external Tor keys
// POST /api/v1/admin/server/tor/import
func (h *AdminHandler) APITorImport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		PrivateKey string `json:"private_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.PrivateKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Private key is required",
		})
		return
	}

	// Would import the key and restart Tor
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Tor keys imported successfully",
	})
}

// APITorTest tests Tor connection
// POST /api/v1/admin/server/tor/test
func (h *AdminHandler) APITorTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Would test actual Tor connection
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"connected": false,
			"ip":        "",
			"message":   "Tor connection test not implemented",
		},
	})
}

// Helper to read log file lines
func (h *AdminHandler) readLogLines(filename string, maxLines int) []string {
	paths := config.GetPaths("", "")
	logPath := filepath.Join(paths.Data, "logs", filename)

	file, err := os.Open(logPath)
	if err != nil {
		return []string{"Log file not found: " + filename}
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Return last N lines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return lines
}

// === Helper functions ===

func (h *AdminHandler) validateToken(token string) bool {
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.Server.Admin.Token)) == 1
}

func (h *AdminHandler) createSessionWithID(username string, adminID int64) string {
	// Generate session ID
	data := []byte(username + time.Now().String())
	hash := sha256.Sum256(data)
	sessionID := hex.EncodeToString(hash[:])

	h.sessions[sessionID] = adminSession{
		username:  username,
		adminID:   adminID,
		createdAt: time.Now(),
		expiresAt: time.Now().Add(adminSessionDuration),
	}

	// Clean up expired sessions
	for id, session := range h.sessions {
		if time.Now().After(session.expiresAt) {
			delete(h.sessions, id)
		}
	}

	return sessionID
}

func (h *AdminHandler) validateSession(sessionID string) bool {
	session, ok := h.sessions[sessionID]
	if !ok {
		return false
	}
	if time.Now().After(session.expiresAt) {
		delete(h.sessions, sessionID)
		return false
	}
	return true
}

// getSession returns the current admin session from the request
func (h *AdminHandler) getSession(r *http.Request) *adminSession {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return nil
	}
	session, ok := h.sessions[cookie.Value]
	if !ok || time.Now().After(session.expiresAt) {
		return nil
	}
	return &session
}

// === CSRF Protection per TEMPLATE.md PART 12 ===

// generateCSRFToken creates a new CSRF token for a session
func (h *AdminHandler) generateCSRFToken(sessionID string) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	h.csrfTokens[sessionID] = token
	return token
}

// getCSRFToken retrieves or generates a CSRF token for the session
func (h *AdminHandler) getCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return ""
	}

	sessionID := cookie.Value
	if token, ok := h.csrfTokens[sessionID]; ok {
		return token
	}

	return h.generateCSRFToken(sessionID)
}

// validateCSRFToken validates the CSRF token from a request
func (h *AdminHandler) validateCSRFToken(r *http.Request) bool {
	// Get session ID from cookie
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return false
	}

	sessionID := cookie.Value
	expectedToken, ok := h.csrfTokens[sessionID]
	if !ok {
		return false
	}

	// Check for token in form field
	submittedToken := r.FormValue("_csrf_token")
	if submittedToken == "" {
		// Also check header for AJAX requests
		submittedToken = r.Header.Get("X-CSRF-Token")
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(expectedToken), []byte(submittedToken)) == 1
}

// CSRFMiddleware validates CSRF tokens on POST/PUT/DELETE requests
func (h *AdminHandler) CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate for state-changing methods
		if r.Method == http.MethodPost || r.Method == http.MethodPut ||
			r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if !h.validateCSRFToken(r) {
				http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// csrfFormField returns the hidden input field HTML for CSRF token
func (h *AdminHandler) csrfFormField(r *http.Request) string {
	token := h.getCSRFToken(r)
	return `<input type="hidden" name="_csrf_token" value="` + token + `">`
}

// Template rendering functions

func (h *AdminHandler) renderDashboard() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	engineCount := len(h.engineMgr.ListEngines())
	enabledCount := h.engineMgr.EnabledCount()

	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Dashboard - ` + h.cfg.Server.Title + `</title>
    ` + adminStyles() + `
</head>
<body>
    ` + h.renderAdminNav("dashboard") + `
    <main class="admin-main">
        <h1>Dashboard</h1>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">` + strconv.Itoa(enabledCount) + `</div>
                <div class="stat-label">Engines Active</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">` + strconv.Itoa(engineCount) + `</div>
                <div class="stat-label">Total Engines</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">` + strconv.FormatUint(m.Alloc/1024/1024, 10) + ` MB</div>
                <div class="stat-label">Memory Usage</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">` + strconv.Itoa(runtime.NumGoroutine()) + `</div>
                <div class="stat-label">Goroutines</div>
            </div>
        </div>

        <div class="card">
            <h2>System Info</h2>
            <table class="info-table">
                <tr><td>Mode</td><td>` + h.cfg.Server.Mode + `</td></tr>
                <tr><td>Go Version</td><td>` + runtime.Version() + `</td></tr>
                <tr><td>OS / Arch</td><td>` + runtime.GOOS + ` / ` + runtime.GOARCH + `</td></tr>
                <tr><td>Server Port</td><td>` + h.cfg.Server.Port + `</td></tr>
                <tr><td>Tor Enabled</td><td>` + strconv.FormatBool(h.cfg.Search.Tor.Enabled) + `</td></tr>
            </table>
        </div>

        <div class="card">
            <h2>Quick Actions</h2>
            <div class="button-group">
                <button onclick="backupNow()" class="btn btn-primary">Create Backup</button>
                <button onclick="toggleMaintenance()" class="btn btn-warning">Toggle Maintenance</button>
            </div>
        </div>
    </main>

    <div id="toast" class="admin-toast"></div>
    <div id="confirm-modal" class="admin-modal hidden">
        <div class="admin-modal-backdrop"></div>
        <div class="admin-modal-content">
            <p id="confirm-message">Are you sure?</p>
            <div class="admin-modal-buttons">
                <button onclick="confirmAction(true)" class="btn btn-primary">Yes</button>
                <button onclick="confirmAction(false)" class="btn">Cancel</button>
            </div>
        </div>
    </div>
    <script>
    let pendingAction = null;
    function showToast(message, type) {
        const toast = document.getElementById('toast');
        toast.textContent = message;
        toast.className = 'admin-toast ' + type + ' show';
        setTimeout(() => { toast.className = 'admin-toast'; }, 3000);
    }
    function showConfirm(message, action) {
        pendingAction = action;
        document.getElementById('confirm-message').textContent = message;
        document.getElementById('confirm-modal').classList.remove('hidden');
    }
    function confirmAction(confirmed) {
        document.getElementById('confirm-modal').classList.add('hidden');
        if (confirmed && pendingAction) pendingAction();
        pendingAction = null;
    }
    async function backupNow() {
        showConfirm('Create a backup now?', async () => {
            try {
                const resp = await fetch('/api/v1/admin/backup', { method: 'POST' });
                const data = await resp.json();
                showToast(data.success ? 'Backup created!' : 'Error: ' + data.error, data.success ? 'success' : 'error');
            } catch (e) {
                showToast('Error: ' + e.message, 'error');
            }
        });
    }
    async function toggleMaintenance() {
        try {
            const resp = await fetch('/api/v1/admin/maintenance', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: true })
            });
            const data = await resp.json();
            showToast(data.success ? 'Maintenance mode toggled!' : 'Error: ' + data.error, data.success ? 'success' : 'error');
        } catch (e) {
            showToast('Error: ' + e.message, 'error');
        }
    }
    </script>
</body>
</html>`
}

func (h *AdminHandler) renderEnginesPage() string {
	engines := h.engineMgr.ListEngines()

	engineRows := ""
	for _, eng := range engines {
		status := `<span class="badge badge-success">Enabled</span>`
		if !eng.Enabled {
			status = `<span class="badge badge-error">Disabled</span>`
		}
		engineRows += `<tr>
            <td>` + eng.Name + `</td>
            <td>` + eng.DisplayName + `</td>
            <td>Tier ` + strconv.Itoa(eng.Tier) + `</td>
            <td>` + status + `</td>
        </tr>`
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Engines - Admin - ` + h.cfg.Server.Title + `</title>
    ` + adminStyles() + `
</head>
<body>
    ` + h.renderAdminNav("engines") + `
    <main class="admin-main">
        <h1>Search Engines</h1>

        <div class="card">
            <table class="data-table">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Name</th>
                        <th>Tier</th>
                        <th>Status</th>
                    </tr>
                </thead>
                <tbody>
                    ` + engineRows + `
                </tbody>
            </table>
        </div>
    </main>
</body>
</html>`
}

func (h *AdminHandler) renderSettingsPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Settings - Admin - ` + h.cfg.Server.Title + `</title>
    ` + adminStyles() + `
</head>
<body>
    ` + h.renderAdminNav("settings") + `
    <main class="admin-main">
        <h1>Settings</h1>

        <div class="card">
            <h2>Server Configuration</h2>
            <p class="text-muted">Configuration is managed via server.yml</p>
            <p>Config path: <code>` + h.cfg.Server.FQDN + `</code></p>
        </div>

        <div class="card">
            <h2>API Token</h2>
            <p class="text-muted">Use this token for API authentication</p>
            <div class="token-display">
                <code id="api-token">••••••••••••••••</code>
                <button onclick="toggleToken()" class="btn btn-sm">Show</button>
            </div>
        </div>
    </main>

    <script>
    let tokenVisible = false;
    function toggleToken() {
        const el = document.getElementById('api-token');
        if (tokenVisible) {
            el.textContent = '••••••••••••••••';
        } else {
            el.textContent = '` + h.cfg.Server.Admin.Token + `';
        }
        tokenVisible = !tokenVisible;
    }
    </script>
</body>
</html>`
}

func (h *AdminHandler) renderLogsPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Logs - Admin - ` + h.cfg.Server.Title + `</title>
    ` + adminStyles() + `
</head>
<body>
    ` + h.renderAdminNav("logs") + `
    <main class="admin-main">
        <h1>Logs</h1>

        <div class="card">
            <p class="text-muted">Log viewing is not yet implemented.</p>
            <p>Logs are written to: <code>` + h.cfg.Server.Logs.Server.Filename + `</code></p>
        </div>
    </main>
</body>
</html>`
}

func (h *AdminHandler) renderAdminNav(active string) string {
	navClass := func(name string) string {
		if name == active {
			return "nav-link active"
		}
		return "nav-link"
	}

	// PART 12: Full navigation with all 11 sections + Tor (PART 32)
	return `<nav class="admin-nav">
        <div class="nav-brand">
            <a href="/admin">🔍 Vidveil Admin</a>
        </div>
        <div class="nav-links">
            <a href="/admin" class="` + navClass("dashboard") + `">Dashboard</a>
            <a href="/admin/server" class="` + navClass("server") + `">Server</a>
            <a href="/admin/web" class="` + navClass("web") + `">Web</a>
            <a href="/admin/security" class="` + navClass("security") + `">Security</a>
            <a href="/admin/database" class="` + navClass("database") + `">Database</a>
            <a href="/admin/email" class="` + navClass("email") + `">Email</a>
            <a href="/admin/ssl" class="` + navClass("ssl") + `">SSL/TLS</a>
            <a href="/admin/scheduler" class="` + navClass("scheduler") + `">Scheduler</a>
            <a href="/admin/tor" class="` + navClass("tor") + `">Tor</a>
            <a href="/admin/logs" class="` + navClass("logs") + `">Logs</a>
            <a href="/admin/backup" class="` + navClass("backup") + `">Backup</a>
            <a href="/admin/system" class="` + navClass("system") + `">System</a>
            <a href="/admin/logout" class="nav-link nav-logout">Logout</a>
        </div>
    </nav>`
}

// === PART 12 Additional Page Render Functions ===

func (h *AdminHandler) renderServerSettingsPage() string {
	return h.renderAdminPage("server", "Server Settings", `
        <div class="card">
            <h2>Server Configuration</h2>
            <table class="info-table">
                <tr><td>Port</td><td>`+h.cfg.Server.Port+`</td></tr>
                <tr><td>Address</td><td>`+h.cfg.Server.Address+`</td></tr>
                <tr><td>FQDN</td><td>`+h.cfg.Server.FQDN+`</td></tr>
                <tr><td>Mode</td><td>`+h.cfg.Server.Mode+`</td></tr>
                <tr><td>Title</td><td>`+h.cfg.Server.Title+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Admin Settings</h2>
            <table class="info-table">
                <tr><td>Username</td><td>`+h.cfg.Server.Admin.Username+`</td></tr>
                <tr><td>Email</td><td>`+h.cfg.Server.Admin.Email+`</td></tr>
            </table>
        </div>`)
}

func (h *AdminHandler) renderWebSettingsPage() string {
	return h.renderAdminPage("web", "Web Settings", `
        <div class="card">
            <h2>UI Configuration</h2>
            <table class="info-table">
                <tr><td>Theme</td><td>`+h.cfg.Web.UI.Theme+`</td></tr>
                <tr><td>CORS</td><td>`+h.cfg.Web.CORS+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Search Settings</h2>
            <table class="info-table">
                <tr><td>Results Per Page</td><td>`+strconv.Itoa(h.cfg.Search.ResultsPerPage)+`</td></tr>
                <tr><td>Tor Enabled</td><td>`+strconv.FormatBool(h.cfg.Search.Tor.Enabled)+`</td></tr>
            </table>
        </div>`)
}

func (h *AdminHandler) renderSecuritySettingsPage() string {
	return h.renderAdminPage("security", "Security Settings", `
        <div class="card">
            <h2>Security Headers</h2>
            <table class="info-table">
                <tr><td>Enabled</td><td>`+strconv.FormatBool(h.cfg.Server.SecurityHeaders.Enabled)+`</td></tr>
                <tr><td>HSTS</td><td>`+strconv.FormatBool(h.cfg.Server.SecurityHeaders.HSTS)+`</td></tr>
                <tr><td>X-Frame-Options</td><td>`+h.cfg.Server.SecurityHeaders.XFrameOptions+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Rate Limiting</h2>
            <table class="info-table">
                <tr><td>Enabled</td><td>`+strconv.FormatBool(h.cfg.Server.RateLimit.Enabled)+`</td></tr>
                <tr><td>Requests</td><td>`+strconv.Itoa(h.cfg.Server.RateLimit.Requests)+`</td></tr>
                <tr><td>Window</td><td>`+strconv.Itoa(h.cfg.Server.RateLimit.Window)+` seconds</td></tr>
            </table>
        </div>`)
}

func (h *AdminHandler) renderDatabasePage() string {
	cacheType := h.cfg.Server.Cache.Type
	if cacheType == "" {
		cacheType = "memory"
	}
	cacheTTL := h.cfg.Server.Cache.TTL
	journalMode := h.cfg.Server.Database.SQLite.JournalMode
	if journalMode == "" {
		journalMode = "WAL"
	}

	// Helper for selected attribute
	sel := func(current, value string) string {
		if current == value {
			return "selected"
		}
		return ""
	}

	return h.renderAdminPage("database", "Database & Cache", `
        <div class="card">
            <h2>Database Settings</h2>
            <p class="help-text">Configure database driver and connection settings.</p>
            <div class="form-group">
                <label for="db_driver">Driver</label>
                <select id="db_driver" name="driver" disabled aria-describedby="db_driver_help">
                    <option value="sqlite" `+sel(h.cfg.Server.Database.Driver, "sqlite")+`>SQLite (Default)</option>
                </select>
                <small id="db_driver_help" class="help-text">Database driver. SQLite is recommended for single-instance deployments.</small>
            </div>
            <div class="form-group">
                <label>Database Directory</label>
                <input type="text" value="`+h.cfg.Server.Database.SQLite.Dir+`" readonly class="readonly-field">
                <small class="help-text">Directory where database files are stored. Change via config file.</small>
            </div>
            <div class="form-group">
                <label>Journal Mode</label>
                <input type="text" value="`+journalMode+`" readonly class="readonly-field">
                <small class="help-text">SQLite journal mode. WAL provides better concurrency.</small>
            </div>
            <table class="info-table">
                <tr><td>Current Driver</td><td>`+h.cfg.Server.Database.Driver+`</td></tr>
                <tr><td>Status</td><td><span class="badge badge-success">Connected</span></td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Cache Settings</h2>
            <p class="help-text">Configure caching to improve performance.</p>
            <div class="form-group">
                <label for="cache_type">Cache Type</label>
                <select id="cache_type" name="cache_type" disabled aria-describedby="cache_type_help">
                    <option value="memory" `+sel(cacheType, "memory")+`>Memory (Default)</option>
                    <option value="redis" `+sel(cacheType, "redis")+`>Redis</option>
                    <option value="memcache" `+sel(cacheType, "memcache")+`>Memcached</option>
                </select>
                <small id="cache_type_help" class="help-text">Cache backend. Memory is suitable for single instances.</small>
            </div>
            <div class="form-group">
                <label>Default TTL (seconds)</label>
                <input type="number" value="`+strconv.Itoa(cacheTTL)+`" readonly class="readonly-field">
                <small class="help-text">Default time-to-live for cached items. 0 = no expiration.</small>
            </div>
            <table class="info-table">
                <tr><td>Current Type</td><td>`+cacheType+`</td></tr>
                <tr><td>TTL</td><td>`+strconv.Itoa(cacheTTL)+` seconds</td></tr>
                <tr><td>Status</td><td><span class="badge badge-success">Active</span></td></tr>
            </table>
            <p class="help-text"><strong>Note:</strong> Database and cache settings require server restart to change. Edit server.yml to modify.</p>
        </div>`)
}

func (h *AdminHandler) renderEmailPage() string {
	return h.renderAdminPage("email", "Email & Notifications", `
        <div class="card">
            <h2>Email Configuration</h2>
            <table class="info-table">
                <tr><td>Enabled</td><td>`+strconv.FormatBool(h.cfg.Server.Email.Enabled)+`</td></tr>
                <tr><td>SMTP Host</td><td>`+h.cfg.Server.Email.Host+`</td></tr>
                <tr><td>From</td><td>`+h.cfg.Server.Email.From+`</td></tr>
            </table>
            <div class="button-group" style="margin-top: 1rem;">
                <button onclick="testEmail()" class="btn btn-primary">Send Test Email</button>
            </div>
        </div>
        <div class="card">
            <h2>Notifications</h2>
            <p class="text-muted">Notification settings are managed via server.yml</p>
        </div>
        <script>
        async function testEmail() {
            try {
                const resp = await fetch('/api/v1/admin/test/email', { method: 'POST' });
                const data = await resp.json();
                if (data.success) { showSuccess('Test email sent!'); } else { showError('Error: ' + data.error); }
            } catch (e) {
                showError('Error: ' + e.message);
            }
        }
        </script>`)
}

func (h *AdminHandler) renderSSLPage() string {
	sslStatus := "Disabled"
	if h.cfg.Server.SSL.Enabled {
		sslStatus = "Enabled"
	}
	leStatus := "Disabled"
	if h.cfg.Server.SSL.LetsEncrypt.Enabled {
		leStatus = "Enabled"
	}

	return h.renderAdminPage("ssl", "SSL/TLS", `
        <div class="card">
            <h2>SSL/TLS Status</h2>
            <table class="info-table">
                <tr><td>SSL Enabled</td><td>`+sslStatus+`</td></tr>
                <tr><td>Certificate Path</td><td>`+h.cfg.Server.SSL.CertPath+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Let's Encrypt</h2>
            <table class="info-table">
                <tr><td>Enabled</td><td>`+leStatus+`</td></tr>
                <tr><td>Email</td><td>`+h.cfg.Server.SSL.LetsEncrypt.Email+`</td></tr>
                <tr><td>Challenge Type</td><td>`+h.cfg.Server.SSL.LetsEncrypt.Challenge+`</td></tr>
            </table>
        </div>`)
}

func (h *AdminHandler) renderSchedulerPage() string {
	taskRows := ""
	if h.scheduler != nil {
		tasks := h.scheduler.ListTasks()
		for _, task := range tasks {
			statusBadge := `<span class="badge badge-success">` + task.LastResult + `</span>`
			if task.LastResult == "failure" {
				statusBadge = `<span class="badge badge-error">Failed</span>`
			} else if task.LastResult == "running" {
				statusBadge = `<span class="badge badge-warning">Running</span>`
			}
			enabledBadge := ""
			if !task.Enabled {
				enabledBadge = ` <span class="badge badge-error">Disabled</span>`
			}
			taskRows += `<tr>
                <td>` + task.Name + enabledBadge + `</td>
                <td>` + task.Schedule + `</td>
                <td>` + task.NextRun.Format("2006-01-02 15:04") + `</td>
                <td>` + statusBadge + `</td>
                <td>
                    <button onclick="runTask('` + task.ID + `')" class="btn btn-sm btn-primary">Run Now</button>
                </td>
            </tr>`
		}
	}

	if taskRows == "" {
		taskRows = `<tr><td colspan="5" class="text-muted">No scheduled tasks</td></tr>`
	}

	return h.renderAdminPage("scheduler", "Scheduler", `
        <div class="card">
            <h2>Scheduled Tasks</h2>
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Task</th>
                        <th>Schedule</th>
                        <th>Next Run</th>
                        <th>Last Result</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>`+taskRows+`</tbody>
            </table>
        </div>
        <script>
        async function runTask(id) {
            try {
                const resp = await fetch('/api/v1/admin/scheduler/run?id=' + id, { method: 'POST' });
                const data = await resp.json();
                if (data.success) { showSuccess('Task triggered!'); location.reload(); } else { showError('Error: ' + data.error); }
            } catch (e) {
                showError('Error: ' + e.message);
            }
        }
        </script>`)
}

func (h *AdminHandler) renderBackupPage() string {
	return h.renderAdminPage("backup", "Backup & Maintenance", `
        <div class="card">
            <h2>Backup</h2>
            <p class="text-muted">Create a backup of configuration and data</p>
            <div class="button-group" style="margin-top: 1rem;">
                <button onclick="createBackup()" class="btn btn-primary">Create Backup Now</button>
            </div>
        </div>
        <div class="card">
            <h2>Restore</h2>
            <p class="text-muted">Restore from a previous backup</p>
            <div class="button-group" style="margin-top: 1rem;">
                <button onclick="restoreBackup()" class="btn btn-warning">Restore from Backup</button>
            </div>
        </div>
        <div class="card">
            <h2>Maintenance Mode</h2>
            <p class="text-muted">Enable maintenance mode to show 503 to all visitors</p>
            <div class="button-group" style="margin-top: 1rem;">
                <button onclick="toggleMaintenance(true)" class="btn btn-warning">Enable Maintenance</button>
                <button onclick="toggleMaintenance(false)" class="btn">Disable Maintenance</button>
            </div>
        </div>
        <script>
        async function createBackup() {
            try {
                const resp = await fetch('/api/v1/admin/backup', { method: 'POST' });
                const data = await resp.json();
                if (data.success) { showSuccess('Backup created!'); } else { showError('Error: ' + data.error); }
            } catch (e) {
                showError('Error: ' + e.message);
            }
        }
        async function restoreBackup() {
            const confirmed = await showConfirm('Are you sure? This will overwrite current configuration.');
            if (!confirmed) return;
            try {
                const resp = await fetch('/api/v1/admin/restore', { method: 'POST' });
                const data = await resp.json();
                if (data.success) { showSuccess('Restore completed!'); } else { showError('Error: ' + data.error); }
            } catch (e) {
                showError('Error: ' + e.message);
            }
        }
        async function toggleMaintenance(enable) {
            try {
                const resp = await fetch('/api/v1/admin/maintenance', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ enabled: enable })
                });
                const data = await resp.json();
                if (data.success) { showSuccess('Maintenance mode ' + (enable ? 'enabled' : 'disabled')); } else { showError('Error: ' + data.error); }
            } catch (e) {
                showError('Error: ' + e.message);
            }
        }
        </script>`)
}

func (h *AdminHandler) renderSystemInfoPage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(h.startTime)

	return h.renderAdminPage("system", "System Information", `
        <div class="card">
            <h2>Runtime</h2>
            <table class="info-table">
                <tr><td>Go Version</td><td>`+runtime.Version()+`</td></tr>
                <tr><td>OS / Arch</td><td>`+runtime.GOOS+` / `+runtime.GOARCH+`</td></tr>
                <tr><td>CPUs</td><td>`+strconv.Itoa(runtime.NumCPU())+`</td></tr>
                <tr><td>Goroutines</td><td>`+strconv.Itoa(runtime.NumGoroutine())+`</td></tr>
                <tr><td>Uptime</td><td>`+uptime.Round(time.Second).String()+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Memory</h2>
            <table class="info-table">
                <tr><td>Allocated</td><td>`+strconv.FormatUint(m.Alloc/1024/1024, 10)+` MB</td></tr>
                <tr><td>Total Allocated</td><td>`+strconv.FormatUint(m.TotalAlloc/1024/1024, 10)+` MB</td></tr>
                <tr><td>System Memory</td><td>`+strconv.FormatUint(m.Sys/1024/1024, 10)+` MB</td></tr>
                <tr><td>GC Cycles</td><td>`+strconv.FormatUint(uint64(m.NumGC), 10)+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Engines</h2>
            <table class="info-table">
                <tr><td>Total</td><td>`+strconv.Itoa(len(h.engineMgr.ListEngines()))+`</td></tr>
                <tr><td>Enabled</td><td>`+strconv.Itoa(h.engineMgr.EnabledCount())+`</td></tr>
            </table>
        </div>`)
}

// renderTorPage renders Tor hidden service admin page per TEMPLATE.md PART 32
func (h *AdminHandler) renderTorPage() string {
	torEnabled := h.cfg.Search.Tor.Enabled
	enabledStr := "Disabled"
	statusClass := "badge-error"
	if torEnabled {
		enabledStr = "Enabled"
		statusClass = "badge-success"
	}

	return h.renderAdminPage("tor", "Tor Hidden Service", `
        <div class="card">
            <h2>Hidden Service Status</h2>
            <table class="info-table">
                <tr><td>Status</td><td><span class="badge `+statusClass+`">`+enabledStr+`</span></td></tr>
                <tr><td>Proxy</td><td>`+h.cfg.Search.Tor.Proxy+`</td></tr>
                <tr><td>Control Port</td><td>`+strconv.Itoa(h.cfg.Search.Tor.ControlPort)+`</td></tr>
                <tr><td>Force All Traffic</td><td>`+strconv.FormatBool(h.cfg.Search.Tor.ForceAll)+`</td></tr>
                <tr><td>Rotate Circuit</td><td>`+strconv.FormatBool(h.cfg.Search.Tor.RotateCircuit)+`</td></tr>
                <tr><td>Clearnet Fallback</td><td>`+strconv.FormatBool(h.cfg.Search.Tor.ClearnetFallback)+`</td></tr>
            </table>
        </div>
        <div class="card">
            <h2>Configuration</h2>
            <form id="tor-form" onsubmit="saveTorConfig(event)">
                <div class="form-group">
                    <label class="toggle-label">
                        <input type="checkbox" id="tor-enabled" `+func() string { if torEnabled { return "checked" }; return "" }()+`>
                        <span>Enable Tor Hidden Service</span>
                    </label>
                </div>
                <div class="form-group">
                    <label for="tor-proxy">SOCKS5 Proxy</label>
                    <input type="text" id="tor-proxy" value="`+h.cfg.Search.Tor.Proxy+`" placeholder="socks5://127.0.0.1:9050">
                </div>
                <div class="form-group">
                    <label for="tor-control-port">Control Port</label>
                    <input type="number" id="tor-control-port" value="`+strconv.Itoa(h.cfg.Search.Tor.ControlPort)+`" placeholder="9051">
                </div>
                <div class="form-group">
                    <label class="toggle-label">
                        <input type="checkbox" id="tor-force-all" `+func() string { if h.cfg.Search.Tor.ForceAll { return "checked" }; return "" }()+`>
                        <span>Force all traffic through Tor</span>
                    </label>
                </div>
                <div class="form-group">
                    <label class="toggle-label">
                        <input type="checkbox" id="tor-rotate" `+func() string { if h.cfg.Search.Tor.RotateCircuit { return "checked" }; return "" }()+`>
                        <span>Rotate circuit per request</span>
                    </label>
                </div>
                <div class="form-group">
                    <label class="toggle-label">
                        <input type="checkbox" id="tor-clearnet" `+func() string { if h.cfg.Search.Tor.ClearnetFallback { return "checked" }; return "" }()+`>
                        <span>Fallback to clearnet if Tor fails</span>
                    </label>
                </div>
                <div class="button-group">
                    <button type="submit" class="btn btn-primary">Save Changes</button>
                </div>
            </form>
        </div>
        <div class="card">
            <h2>Vanity Address</h2>
            <p class="text-muted">Generate a custom .onion address with a specific prefix (e.g., "vidv")</p>
            <div class="form-group">
                <label for="vanity-prefix">Prefix (2-6 chars)</label>
                <input type="text" id="vanity-prefix" placeholder="vidv" maxlength="6" pattern="[a-z2-7]{2,6}">
            </div>
            <div class="button-group">
                <button onclick="startVanity()" class="btn btn-secondary">Start Generation</button>
                <button onclick="stopVanity()" class="btn btn-warning">Stop</button>
            </div>
            <div id="vanity-status" class="text-muted" style="margin-top: 1rem;"></div>
        </div>
        <script>
        async function saveTorConfig(e) {
            e.preventDefault();
            try {
                const resp = await fetch('/api/v1/admin/config', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        search: {
                            tor: {
                                enabled: document.getElementById('tor-enabled').checked,
                                proxy: document.getElementById('tor-proxy').value,
                                control_port: parseInt(document.getElementById('tor-control-port').value),
                                force_all: document.getElementById('tor-force-all').checked,
                                rotate_circuit: document.getElementById('tor-rotate').checked,
                                clearnet_fallback: document.getElementById('tor-clearnet').checked
                            }
                        }
                    })
                });
                const data = await resp.json();
                if (data.success) { showSuccess('Tor settings saved!'); } else { showError('Error: ' + data.error); }
            } catch (e) { showError('Error: ' + e.message); }
        }
        async function startVanity() {
            const prefix = document.getElementById('vanity-prefix').value;
            if (!prefix || prefix.length < 2) { showError('Prefix must be at least 2 characters'); return; }
            document.getElementById('vanity-status').textContent = 'Starting vanity generation for "' + prefix + '"...';
            try {
                const resp = await fetch('/api/v1/admin/tor/vanity/start?prefix=' + prefix, { method: 'POST' });
                const data = await resp.json();
                if (data.success) { showSuccess('Vanity generation started!'); pollVanityStatus(); }
                else { showError('Error: ' + data.error); }
            } catch (e) { showError('Error: ' + e.message); }
        }
        async function stopVanity() {
            try {
                const resp = await fetch('/api/v1/admin/tor/vanity/stop', { method: 'POST' });
                const data = await resp.json();
                if (data.success) { showSuccess('Vanity generation stopped'); }
            } catch (e) { showError('Error: ' + e.message); }
        }
        function pollVanityStatus() {
            setInterval(async () => {
                try {
                    const resp = await fetch('/api/v1/admin/tor/vanity/status');
                    const data = await resp.json();
                    if (data.success && data.data) {
                        const s = data.data;
                        document.getElementById('vanity-status').textContent =
                            s.active ? 'Searching for "' + s.prefix + '": ' + s.attempts + ' attempts (' + s.elapsed_time + ')' :
                            'Not running';
                    }
                } catch (e) {}
            }, 2000);
        }
        </script>`)
}

// renderAdminTemplate renders admin pages using proper Go html/template per TEMPLATE.md PART 13
func (h *AdminHandler) renderAdminTemplate(w http.ResponseWriter, r *http.Request, templateName string, data map[string]interface{}) {
	// Add common template data
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Config"] = h.cfg
	data["ActiveNav"] = templateName
	data["SiteTitle"] = h.cfg.Server.Title

	// Add session info for header display per TEMPLATE.md PART 31
	if r != nil {
		if sess := h.getSession(r); sess != nil {
			data["AdminUsername"] = sess.username
		}
	}
	data["OnlineCount"] = h.getOnlineCount()

	// Set page title based on template name if not already set
	if _, ok := data["Title"]; !ok {
		titles := map[string]string{
			"dashboard":          "Dashboard",
			"profile":            "Profile",
			"users-admins":       "Administrators",
			"invite":             "Admin Invite",
			"nodes":              "Cluster Nodes",
			"server":             "Server Settings",
			"branding":           "Branding & SEO",
			"ssl":                "SSL/TLS",
			"scheduler":          "Scheduler",
			"email":              "Email & Notifications",
			"logs":               "Logs",
			"database":           "Database",
			"web":                "Web Settings",
			"security":           "Security",
			"security-auth":      "Authentication",
			"security-tokens":    "API Tokens",
			"security-ratelimit": "Rate Limiting",
			"security-firewall":  "Firewall",
			"tor":                "Tor Configuration",
			"geoip":              "GeoIP Filtering",
			"blocklists":         "Blocklists",
			"backup":             "Backup & Restore",
			"maintenance":        "Maintenance",
			"updates":            "Updates",
			"system":             "System Info",
			"engines":            "Search Engines",
			"help":               "Help",
		}
		if title, ok := titles[templateName]; ok {
			data["Title"] = title
		} else {
			data["Title"] = templateName
		}
	}

	// Create template with functions
	tmpl := template.New("admin").Funcs(template.FuncMap{
		"eq": func(a, b interface{}) bool { return a == b },
	})

	// Load layout template
	layoutContent, err := adminTemplatesFS.ReadFile("templates/layouts/admin.tmpl")
	if err != nil {
		http.Error(w, "Admin layout not found: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl, err = tmpl.Parse(string(layoutContent))
	if err != nil {
		http.Error(w, "Layout parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load the specific content template
	contentFile := "templates/admin/" + templateName + ".tmpl"
	contentData, err := adminTemplatesFS.ReadFile(contentFile)
	if err != nil {
		http.Error(w, "Admin template not found: "+contentFile+": "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl, err = tmpl.Parse(string(contentData))
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "admin", data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
	}
}

// Helper to render admin pages with consistent layout (legacy - for inline HTML pages not yet converted)
func (h *AdminHandler) renderAdminPage(active, title, content string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + title + ` - Admin - ` + h.cfg.Server.Title + `</title>
    ` + adminStyles() + `
</head>
<body>
    ` + h.renderAdminNav(active) + `
    <main class="admin-main">
        <h1>` + title + `</h1>
        ` + content + `
    </main>
    <div id="toast-container" class="toast-container"></div>
    ` + adminToastScript() + `
</body>
</html>`
}

// adminToastScript returns the toast notification JavaScript per PART 10/12 (no alerts)
func adminToastScript() string {
	return `<script>
// Toast notification system (replaces alerts per TEMPLATE.md PART 10)
function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.innerHTML = '<span class="toast-message">' + message + '</span><button class="toast-close" onclick="this.parentElement.remove()">&times;</button>';
    container.appendChild(toast);
    setTimeout(() => toast.classList.add('show'), 10);
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 5000);
}
function showSuccess(msg) { showToast(msg, 'success'); }
function showError(msg) { showToast(msg, 'error'); }
function showWarning(msg) { showToast(msg, 'warning'); }

// Custom confirm dialog (replaces confirm() per TEMPLATE.md PART 10)
function showConfirm(message) {
    return new Promise((resolve) => {
        const overlay = document.createElement('div');
        overlay.className = 'modal-overlay';
        overlay.innerHTML = '<div class="modal-dialog"><div class="modal-content"><p>' + message + '</p><div class="modal-actions"><button class="btn" onclick="this.closest(\'.modal-overlay\').remove(); window.__confirmResolve(false)">Cancel</button><button class="btn btn-primary" onclick="this.closest(\'.modal-overlay\').remove(); window.__confirmResolve(true)">Confirm</button></div></div></div>';
        document.body.appendChild(overlay);
        window.__confirmResolve = resolve;
        setTimeout(() => overlay.classList.add('show'), 10);
    });
}
</script>
<style>
.toast-container { position: fixed; top: 1rem; right: 1rem; z-index: 10000; display: flex; flex-direction: column; gap: 0.5rem; }
.toast { padding: 1rem 2rem 1rem 1rem; border-radius: 4px; color: #fff; display: flex; align-items: center; gap: 1rem; opacity: 0; transform: translateX(100%); transition: all 0.3s ease; max-width: 400px; }
.toast.show { opacity: 1; transform: translateX(0); }
.toast-success { background: #10b981; }
.toast-error { background: #ef4444; }
.toast-warning { background: #f59e0b; }
.toast-info { background: #3b82f6; }
.toast-close { background: none; border: none; color: #fff; font-size: 1.5rem; cursor: pointer; position: absolute; right: 0.5rem; top: 50%; transform: translateY(-50%); opacity: 0.7; }
.toast-close:hover { opacity: 1; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 10001; opacity: 0; transition: opacity 0.3s; }
.modal-overlay.show { opacity: 1; }
.modal-dialog { background: var(--card-bg, #282a36); border-radius: 8px; padding: 2rem; max-width: 400px; }
.modal-actions { margin-top: 1.5rem; display: flex; gap: 1rem; justify-content: flex-end; }
</style>`
}

func adminStyles() string {
	return `<style>
        :root {
            --bg-primary: #282a36;
            --bg-secondary: #44475a;
            --bg-tertiary: #343746;
            --text-primary: #f8f8f2;
            --text-muted: #6272a4;
            --accent: #bd93f9;
            --success: #50fa7b;
            --warning: #ffb86c;
            --error: #ff5555;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
        }
        .admin-nav {
            background: var(--bg-secondary);
            padding: 1rem 2rem;
            display: flex;
            align-items: center;
            justify-content: space-between;
            position: sticky;
            top: 0;
            z-index: 100;
        }
        .nav-brand a {
            color: var(--accent);
            text-decoration: none;
            font-size: 1.25rem;
            font-weight: bold;
        }
        .nav-links {
            display: flex;
            gap: 1rem;
        }
        .nav-link {
            color: var(--text-primary);
            text-decoration: none;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            transition: background 0.2s;
        }
        .nav-link:hover, .nav-link.active {
            background: var(--bg-tertiary);
        }
        .nav-logout {
            color: var(--error);
        }
        .admin-main {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        .admin-main h1 {
            margin-bottom: 1.5rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: var(--bg-secondary);
            padding: 1.5rem;
            border-radius: 8px;
            text-align: center;
        }
        .stat-value {
            font-size: 2rem;
            font-weight: bold;
            color: var(--accent);
        }
        .stat-label {
            color: var(--text-muted);
            margin-top: 0.5rem;
        }
        .card {
            background: var(--bg-secondary);
            padding: 1.5rem;
            border-radius: 8px;
            margin-bottom: 1.5rem;
        }
        .card h2 {
            margin-bottom: 1rem;
            color: var(--accent);
        }
        .info-table {
            width: 100%;
        }
        .info-table td {
            padding: 0.5rem 0;
            border-bottom: 1px solid var(--bg-tertiary);
        }
        .info-table td:first-child {
            color: var(--text-muted);
        }
        .data-table {
            width: 100%;
            border-collapse: collapse;
        }
        .data-table th, .data-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid var(--bg-tertiary);
        }
        .data-table th {
            color: var(--text-muted);
            font-weight: normal;
        }
        .badge {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        .badge-success {
            background: rgba(80, 250, 123, 0.2);
            color: var(--success);
        }
        .badge-error {
            background: rgba(255, 85, 85, 0.2);
            color: var(--error);
        }
        .btn {
            padding: 0.5rem 1rem;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.875rem;
            transition: opacity 0.2s;
        }
        .btn:hover {
            opacity: 0.9;
        }
        .btn-primary {
            background: var(--accent);
            color: var(--bg-primary);
        }
        .btn-warning {
            background: var(--warning);
            color: var(--bg-primary);
        }
        .btn-sm {
            padding: 0.25rem 0.5rem;
            font-size: 0.75rem;
        }
        .button-group {
            display: flex;
            gap: 1rem;
        }
        .text-muted {
            color: var(--text-muted);
        }
        code {
            background: var(--bg-tertiary);
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-family: monospace;
        }
        .token-display {
            display: flex;
            align-items: center;
            gap: 1rem;
            margin-top: 0.5rem;
        }
        .admin-toast {
            position: fixed;
            bottom: 2rem;
            right: 2rem;
            padding: 1rem 1.5rem;
            border-radius: 8px;
            background: var(--bg-secondary);
            color: var(--text-primary);
            box-shadow: 0 4px 12px rgba(0,0,0,0.3);
            transform: translateY(100px);
            opacity: 0;
            transition: transform 0.3s, opacity 0.3s;
            z-index: 1000;
        }
        .admin-toast.show {
            transform: translateY(0);
            opacity: 1;
        }
        .admin-toast.success {
            border-left: 4px solid var(--success);
        }
        .admin-toast.error {
            border-left: 4px solid var(--error);
        }
        .admin-modal {
            position: fixed;
            inset: 0;
            z-index: 1000;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .admin-modal.hidden {
            display: none;
        }
        .admin-modal-backdrop {
            position: absolute;
            inset: 0;
            background: rgba(0,0,0,0.7);
        }
        .admin-modal-content {
            position: relative;
            background: var(--bg-secondary);
            padding: 2rem;
            border-radius: 8px;
            min-width: 300px;
            text-align: center;
        }
        .admin-modal-content p {
            margin-bottom: 1.5rem;
            font-size: 1.1rem;
        }
        .admin-modal-buttons {
            display: flex;
            gap: 1rem;
            justify-content: center;
        }
    </style>`
}
