// SPDX-License-Identifier: MIT
// AI.md PART 17: Admin authentication and TOTP Two-Factor Authentication
// VidVeil is stateless - no PART 34 (Multi-User), only Server Admin auth
package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/totp"
)

// PendingAuth represents a pending 2FA authentication per AI.md PART 17
type PendingAuth struct {
	AdminID    int64
	Username   string
	RemoteAddr string
	UserAgent  string
	CreatedAt  time.Time
}

// AuthHandler handles admin authentication routes per AI.md PART 17
type AuthHandler struct {
	appConfig   *config.AppConfig
	adminHdl    *AdminHandler
	totpSvc     *totp.TOTPService
	// pendingAuth stores pending 2FA authentication tokens
	pendingAuth map[string]*PendingAuth
	mu          sync.RWMutex
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(appConfig *config.AppConfig) *AuthHandler {
	return &AuthHandler{
		appConfig:   appConfig,
		totpSvc:     totp.NewTOTPService(appConfig.Server.Branding.Title),
		pendingAuth: make(map[string]*PendingAuth),
	}
}

// SetAdminHandler sets the admin handler reference for authentication
func (h *AuthHandler) SetAdminHandler(adminHdl *AdminHandler) {
	h.adminHdl = adminHdl
}

// generatePendingToken creates a random token for pending 2FA auth
func (h *AuthHandler) generatePendingToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// storePendingAuth stores a pending 2FA authentication
func (h *AuthHandler) storePendingAuth(token string, pending *PendingAuth) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pendingAuth[token] = pending
}

// getPendingAuth retrieves and removes a pending 2FA authentication
func (h *AuthHandler) getPendingAuth(token string) (*PendingAuth, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	pending, ok := h.pendingAuth[token]
	if ok {
		delete(h.pendingAuth, token)
	}
	return pending, ok
}

// cleanupPendingAuth removes expired pending auths (older than 5 minutes)
func (h *AuthHandler) cleanupPendingAuth() {
	h.mu.Lock()
	defer h.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	for token, pending := range h.pendingAuth {
		if pending.CreatedAt.Before(cutoff) {
			delete(h.pendingAuth, token)
		}
	}
}

// LoginPage renders the admin login form and handles authentication per AI.md PART 17
// Per AI.md PART 17: If 2FA enabled, show 2FA prompt after password validation
func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Clean up expired pending auths
	h.cleanupPendingAuth()

	// Check if admin already logged in
	if h.adminHdl != nil {
		if cookie, err := r.Cookie("vidveil_admin_session"); err == nil {
			if h.adminHdl.validateSession(cookie.Value) {
				http.Redirect(w, r, "/admin", http.StatusFound)
				return
			}
		}
	}

	errorMsg := ""
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Authenticate admin using admin handler with context for audit logging per AI.md PART 11
		if h.adminHdl != nil {
			// First check credentials (returns admin info including 2FA status)
			adminUser, err := h.adminHdl.CheckCredentials(username, password)
			if err == nil && adminUser != nil {
				// Check if 2FA is enabled
				if adminUser.TOTPEnabled {
					// Store pending auth and redirect to 2FA page per AI.md PART 17
					pendingToken := h.generatePendingToken()
					h.storePendingAuth(pendingToken, &PendingAuth{
						AdminID:    adminUser.ID,
						Username:   adminUser.Username,
						RemoteAddr: r.RemoteAddr,
						UserAgent:  r.UserAgent(),
						CreatedAt:  time.Now(),
					})

					// Set pending auth cookie (short-lived) per AI.md PART 11
					http.SetCookie(w, NewSecureCookieStrict(
						"vidveil_pending_2fa",
						pendingToken,
						"/auth",
						// 5 minutes
					300,
						h.appConfig.Server.SSL.Enabled,
					))

					http.Redirect(w, r, "/auth/2fa", http.StatusFound)
					return
				}

				// No 2FA, complete login
				sessionID := h.adminHdl.CreateSessionForAdmin(adminUser)
				http.SetCookie(w, NewSecureCookie(
					"vidveil_admin_session",
					sessionID,
					"/admin",
					int(24*time.Hour/time.Second),
					h.appConfig.Server.SSL.Enabled,
				))

				// Log successful login
				if h.adminHdl.logger != nil {
					h.adminHdl.logger.Audit("admin.login", adminUser.Username, "auth", map[string]interface{}{
						"ip":         r.RemoteAddr,
						"user_agent": r.UserAgent(),
						"mfa_used":   false,
					})
				}

				http.Redirect(w, r, "/admin", http.StatusFound)
				return
			}
		}
		errorMsg = "Invalid username or password"
	}

	h.renderLoginPage(w, errorMsg)
}

// TwoFactorPage handles the 2FA verification step per AI.md PART 17
func (h *AuthHandler) TwoFactorPage(w http.ResponseWriter, r *http.Request) {
	// Get pending auth token from cookie
	cookie, err := r.Cookie("vidveil_pending_2fa")
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	errorMsg := ""

	if r.Method == http.MethodPost {
		code := r.FormValue("code")

		// Get pending auth (without removing yet)
		h.mu.RLock()
		pending, ok := h.pendingAuth[cookie.Value]
		h.mu.RUnlock()

		if !ok || pending == nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Get admin's TOTP secret
		secret, err := h.adminHdl.adminSvc.GetTOTPSecret(pending.AdminID)
		if err != nil {
			errorMsg = "2FA configuration error"
		} else {
			// Validate TOTP code
			if h.totpSvc.ValidateCode(secret, code) {
				// Remove pending auth
				h.getPendingAuth(cookie.Value)

				// Clear pending cookie per AI.md PART 11
				http.SetCookie(w, DeleteCookie("vidveil_pending_2fa", "/auth"))

				// Get admin info for session
				adminUser, _ := h.adminHdl.adminSvc.GetAdmin(pending.AdminID)
				if adminUser != nil {
					// Create session per AI.md PART 11
					sessionID := h.adminHdl.CreateSessionForAdmin(adminUser)
					http.SetCookie(w, NewSecureCookie(
						"vidveil_admin_session",
						sessionID,
						"/admin",
						int(24*time.Hour/time.Second),
						h.appConfig.Server.SSL.Enabled,
					))

					// Log successful login with MFA
					if h.adminHdl.logger != nil {
						h.adminHdl.logger.Audit("admin.login", pending.Username, "auth", map[string]interface{}{
							"ip":         pending.RemoteAddr,
							"user_agent": pending.UserAgent,
							"mfa_used":   true,
						})
					}

					http.Redirect(w, r, "/admin", http.StatusFound)
					return
				}
			} else {
				// Check if it's a backup code
				valid, _ := h.adminHdl.adminSvc.UseBackupCode(pending.AdminID, code)
				if valid {
					// Remove pending auth
					h.getPendingAuth(cookie.Value)

					// Clear pending cookie per AI.md PART 11
					http.SetCookie(w, DeleteCookie("vidveil_pending_2fa", "/auth"))

					// Get admin info for session
					adminUser, _ := h.adminHdl.adminSvc.GetAdmin(pending.AdminID)
					if adminUser != nil {
						// Create session per AI.md PART 11
						sessionID := h.adminHdl.CreateSessionForAdmin(adminUser)
						http.SetCookie(w, NewSecureCookie(
							"vidveil_admin_session",
							sessionID,
							"/admin",
							int(24*time.Hour/time.Second),
							h.appConfig.Server.SSL.Enabled,
						))

						// Log successful login with backup code
						if h.adminHdl.logger != nil {
							h.adminHdl.logger.Audit("admin.login", pending.Username, "auth", map[string]interface{}{
								"ip":           pending.RemoteAddr,
								"user_agent":   pending.UserAgent,
								"mfa_used":     true,
								"backup_code":  true,
							})
						}

						http.Redirect(w, r, "/admin", http.StatusFound)
						return
					}
				}
				errorMsg = "Invalid verification code"
			}
		}
	}

	h.render2FAPage(w, errorMsg)
}

// render2FAPage renders the 2FA verification form using common.css per AI.md PART 16
func (h *AuthHandler) render2FAPage(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errorHtml := ""
	if errorMsg != "" {
		errorHtml = fmt.Sprintf(`<div class="alert alert-error">%s</div>`, errorMsg)
	}
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="theme-dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Two-Factor Authentication - %s</title>
    <link rel="stylesheet" href="/static/css/common.css">
</head>
<body class="centered-page-body">
    <div class="setup-container">
        <div class="setup-card">
            <div class="setup-header">
                <h1>Two-Factor Authentication</h1>
                <p>Enter the 6-digit code from your authenticator app</p>
            </div>
            %s
            <form method="POST" id="2fa-form">
                <div class="form-group">
                    <label for="code">Verification Code</label>
                    <input type="text" id="code" name="code" maxlength="8" pattern="[0-9A-Za-z-]+" required autofocus autocomplete="one-time-code" placeholder="000000" class="text-center" style="letter-spacing: 0.5em; font-size: 1.2rem;">
                </div>
                <button type="submit" class="btn btn-primary" style="width: 100%%;">Verify</button>
            </form>
            <p class="help-text text-center mt-md">Or enter a backup code if you don't have access to your authenticator</p>
            <p class="help-text text-center mt-lg">
                <a href="/auth/login">← Back to Login</a>
            </p>
        </div>
    </div>
</body>
</html>`, h.appConfig.Server.Branding.Title, errorHtml)
	w.Write([]byte(html))
}

// renderLoginPage renders the admin login form using common.css styles per AI.md PART 16
func (h *AuthHandler) renderLoginPage(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errorHtml := ""
	if errorMsg != "" {
		errorHtml = fmt.Sprintf(`<div class="alert alert-error">%s</div>`, errorMsg)
	}
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="theme-dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Login - %s</title>
    <link rel="stylesheet" href="/static/css/common.css">
</head>
<body class="centered-page-body">
    <div class="setup-container">
        <div class="setup-card">
            <div class="setup-header">
                <h1>Admin Login</h1>
                <p>Sign in to the admin panel</p>
            </div>
            %s
            <form method="POST" id="login-form">
                <div class="form-group">
                    <label for="username">Username</label>
                    <input type="text" id="username" name="username" required autofocus placeholder="Enter your username">
                </div>
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" required placeholder="Enter your password">
                </div>
                <button type="submit" class="btn btn-primary" style="width: 100%%;">Login</button>
            </form>
            <p class="help-text text-center mt-lg">
                <a href="/">← Back to Search</a>
            </p>
        </div>
    </div>
</body>
</html>`, h.appConfig.Server.Branding.Title, errorHtml)
	w.Write([]byte(html))
}

// LogoutPage handles admin logout (web route)
func (h *AuthHandler) LogoutPage(w http.ResponseWriter, r *http.Request) {
	// Clear admin session cookie per AI.md PART 11
	http.SetCookie(w, DeleteCookie("vidveil_admin_session", "/admin"))
	http.Redirect(w, r, "/", http.StatusFound)
}

// PasswordForgotPage renders password forgot form using common.css per AI.md PART 16
func (h *AuthHandler) PasswordForgotPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="theme-dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Reset - %s</title>
    <link rel="stylesheet" href="/static/css/common.css">
</head>
<body class="centered-page-body">
    <div class="setup-container">
        <div class="setup-card">
            <div class="setup-header">
                <h1>Password Reset</h1>
                <p>Admin password reset is managed through the server CLI</p>
            </div>
            <p class="help-text text-center mt-lg">
                <a href="/auth/login">← Back to Login</a>
            </p>
        </div>
    </div>
</body>
</html>`, h.appConfig.Server.Branding.Title)
	w.Write([]byte(html))
}

// PasswordResetPage handles password reset with token using common.css per AI.md PART 16
func (h *AuthHandler) PasswordResetPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="theme-dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Reset - %s</title>
    <link rel="stylesheet" href="/static/css/common.css">
</head>
<body class="centered-page-body">
    <div class="setup-container">
        <div class="setup-card">
            <div class="setup-header">
                <h1>Password Reset</h1>
                <p>Invalid or expired reset token</p>
            </div>
            <div class="alert alert-error">The reset token is invalid or has expired</div>
            <p class="help-text text-center mt-lg">
                <a href="/auth/login">← Back to Login</a>
            </p>
        </div>
    </div>
</body>
</html>`, h.appConfig.Server.Branding.Title)
	w.Write([]byte(html))
}
