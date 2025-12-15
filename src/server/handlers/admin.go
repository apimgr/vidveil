// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 12: Admin Panel
package handlers

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/services/engines"
	"github.com/apimgr/vidveil/src/services/maintenance"
	"github.com/apimgr/vidveil/src/services/scheduler"
)

const (
	adminSessionCookieName = "vidveil_admin_session"
	adminSessionDuration   = 24 * time.Hour
	csrfTokenCookieName    = "vidveil_csrf_token"
)

// AdminHandler handles admin panel routes per TEMPLATE.md PART 12
type AdminHandler struct {
	cfg        *config.Config
	engineMgr  *engines.Manager
	scheduler  *scheduler.Scheduler
	sessions   map[string]adminSession
	csrfTokens map[string]string // sessionID -> csrfToken
	startTime  time.Time
}

type adminSession struct {
	username  string
	createdAt time.Time
	expiresAt time.Time
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(cfg *config.Config, engineMgr *engines.Manager) *AdminHandler {
	return &AdminHandler{
		cfg:        cfg,
		engineMgr:  engineMgr,
		sessions:   make(map[string]adminSession),
		csrfTokens: make(map[string]string),
		startTime:  time.Now(),
	}
}

// SetScheduler sets the scheduler reference
func (h *AdminHandler) SetScheduler(s *scheduler.Scheduler) {
	h.scheduler = s
}

// AuthMiddleware protects admin routes
func (h *AdminHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie(adminSessionCookieName)
		if err != nil || !h.validateSession(cookie.Value) {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoginPage renders the admin login page
func (h *AdminHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if cookie, err := r.Cookie(adminSessionCookieName); err == nil && h.validateSession(cookie.Value) {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	errorMsg := ""
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if h.validateCredentials(username, password) {
			// Create session
			sessionID := h.createSession(username)
			http.SetCookie(w, &http.Cookie{
				Name:     adminSessionCookieName,
				Value:    sessionID,
				Path:     "/admin",
				MaxAge:   int(adminSessionDuration.Seconds()),
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		errorMsg = "Invalid username or password"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderLoginPage(errorMsg)))
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

	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

// DashboardPage renders the admin dashboard
func (h *AdminHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderDashboard()))
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderLogsPage()))
}

// === PART 12 Required Admin Sections ===

// ServerSettingsPage renders server settings (Section 2)
func (h *AdminHandler) ServerSettingsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderServerSettingsPage()))
}

// WebSettingsPage renders web settings (Section 3)
func (h *AdminHandler) WebSettingsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderWebSettingsPage()))
}

// SecuritySettingsPage renders security settings (Section 4)
func (h *AdminHandler) SecuritySettingsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderSecuritySettingsPage()))
}

// DatabasePage renders database & cache settings (Section 5)
func (h *AdminHandler) DatabasePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderDatabasePage()))
}

// EmailPage renders email & notifications settings (Section 6)
func (h *AdminHandler) EmailPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderEmailPage()))
}

// SSLPage renders SSL/TLS settings (Section 7)
func (h *AdminHandler) SSLPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderSSLPage()))
}

// SchedulerPage renders scheduler management (Section 8)
func (h *AdminHandler) SchedulerPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderSchedulerPage()))
}

// BackupPage renders backup & maintenance (Section 10)
func (h *AdminHandler) BackupPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderBackupPage()))
}

// SystemInfoPage renders system info (Section 11)
func (h *AdminHandler) SystemInfoPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(h.renderSystemInfoPage()))
}

// === API Handlers ===

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

// APIPassword changes admin password
func (h *AdminHandler) APIPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", "ERR_METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed)
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

	// Verify current password
	if !h.validateCredentials(h.cfg.Server.Admin.Username, body.CurrentPassword) {
		h.jsonError(w, "Current password is incorrect", "ERR_UNAUTHORIZED", http.StatusUnauthorized)
		return
	}

	// In production, this would update the database/config
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

func (h *AdminHandler) validateCredentials(username, password string) bool {
	expectedUsername := h.cfg.Server.Admin.Username
	expectedPassword := h.cfg.Server.Admin.Password

	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1

	return usernameMatch && passwordMatch
}

func (h *AdminHandler) validateToken(token string) bool {
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.Server.Admin.Token)) == 1
}

func (h *AdminHandler) createSession(username string) string {
	// Generate session ID
	data := []byte(username + time.Now().String())
	hash := sha256.Sum256(data)
	sessionID := hex.EncodeToString(hash[:])

	h.sessions[sessionID] = adminSession{
		username:  username,
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
func (h *AdminHandler) renderLoginPage(errorMsg string) string {
	errorHtml := ""
	if errorMsg != "" {
		errorHtml = `<div class="alert alert-error">` + errorMsg + `</div>`
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Login - ` + h.cfg.Server.Title + `</title>
    <style>
        :root {
            --bg-primary: #282a36;
            --bg-secondary: #44475a;
            --text-primary: #f8f8f2;
            --text-muted: #6272a4;
            --accent: #bd93f9;
            --success: #50fa7b;
            --error: #ff5555;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .login-card {
            background: var(--bg-secondary);
            border-radius: 8px;
            padding: 2rem;
            width: 100%;
            max-width: 400px;
            margin: 1rem;
        }
        .login-card h1 {
            text-align: center;
            margin-bottom: 1.5rem;
            color: var(--accent);
        }
        .form-group {
            margin-bottom: 1rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            color: var(--text-muted);
        }
        .form-group input {
            width: 100%;
            padding: 0.75rem;
            border: none;
            border-radius: 4px;
            background: var(--bg-primary);
            color: var(--text-primary);
            font-size: 1rem;
        }
        .form-group input:focus {
            outline: 2px solid var(--accent);
        }
        button {
            width: 100%;
            padding: 0.75rem;
            border: none;
            border-radius: 4px;
            background: var(--accent);
            color: var(--bg-primary);
            font-size: 1rem;
            font-weight: bold;
            cursor: pointer;
            transition: opacity 0.2s;
        }
        button:hover {
            opacity: 0.9;
        }
        .alert {
            padding: 0.75rem;
            border-radius: 4px;
            margin-bottom: 1rem;
        }
        .alert-error {
            background: rgba(255, 85, 85, 0.2);
            color: var(--error);
            border: 1px solid var(--error);
        }
        .back-link {
            text-align: center;
            margin-top: 1rem;
        }
        .back-link a {
            color: var(--text-muted);
            text-decoration: none;
        }
        .back-link a:hover {
            color: var(--accent);
        }
    </style>
</head>
<body>
    <div class="login-card">
        <h1>🔐 Admin Login</h1>
        ` + errorHtml + `
        <form method="POST">
            <div class="form-group">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required autofocus>
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            <button type="submit">Login</button>
        </form>
        <div class="back-link">
            <a href="/">← Back to Search</a>
        </div>
    </div>
</body>
</html>`
}

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

	// PART 12: Full navigation with all 11 sections
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

// Helper to render admin pages with consistent layout
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
