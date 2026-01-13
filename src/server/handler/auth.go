// SPDX-License-Identifier: MIT
// AI.md PART 31: Route Standards - /auth/ and /user/ scopes
// AI.md PART 17: TOTP Two-Factor Authentication login flow
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

// AuthHandler handles authentication routes per AI.md PART 31
type AuthHandler struct {
	cfg         *config.Config
	adminHdl    *AdminHandler
	totpSvc     *totp.Service
	pendingAuth map[string]*PendingAuth // pending 2FA tokens
	mu          sync.RWMutex
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		totpSvc:     totp.New(cfg.Server.Title),
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

// LoginPage renders the login form and handles authentication per AI.md PART 31
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

					// Set pending auth cookie (short-lived)
					http.SetCookie(w, &http.Cookie{
						Name:     "vidveil_pending_2fa",
						Value:    pendingToken,
						Path:     "/auth",
						MaxAge:   300, // 5 minutes
						HttpOnly: true,
						SameSite: http.SameSiteStrictMode,
					})

					http.Redirect(w, r, "/auth/2fa", http.StatusFound)
					return
				}

				// No 2FA, complete login
				sessionID := h.adminHdl.CreateSessionForAdmin(adminUser)
				http.SetCookie(w, &http.Cookie{
					Name:     "vidveil_admin_session",
					Value:    sessionID,
					Path:     "/admin",
					MaxAge:   int(24 * time.Hour / time.Second),
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})

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

				// Clear pending cookie
				http.SetCookie(w, &http.Cookie{
					Name:   "vidveil_pending_2fa",
					Value:  "",
					Path:   "/auth",
					MaxAge: -1,
				})

				// Get admin info for session
				adminUser, _ := h.adminHdl.adminSvc.GetAdmin(pending.AdminID)
				if adminUser != nil {
					// Create session
					sessionID := h.adminHdl.CreateSessionForAdmin(adminUser)
					http.SetCookie(w, &http.Cookie{
						Name:     "vidveil_admin_session",
						Value:    sessionID,
						Path:     "/admin",
						MaxAge:   int(24 * time.Hour / time.Second),
						HttpOnly: true,
						SameSite: http.SameSiteLaxMode,
					})

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

					// Clear pending cookie
					http.SetCookie(w, &http.Cookie{
						Name:   "vidveil_pending_2fa",
						Value:  "",
						Path:   "/auth",
						MaxAge: -1,
					})

					// Get admin info for session
					adminUser, _ := h.adminHdl.adminSvc.GetAdmin(pending.AdminID)
					if adminUser != nil {
						// Create session
						sessionID := h.adminHdl.CreateSessionForAdmin(adminUser)
						http.SetCookie(w, &http.Cookie{
							Name:     "vidveil_admin_session",
							Value:    sessionID,
							Path:     "/admin",
							MaxAge:   int(24 * time.Hour / time.Second),
							HttpOnly: true,
							SameSite: http.SameSiteLaxMode,
						})

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

// render2FAPage renders the 2FA verification form per AI.md PART 17
func (h *AuthHandler) render2FAPage(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errorHtml := ""
	if errorMsg != "" {
		errorHtml = fmt.Sprintf(`<div class="error">%s</div>`, errorMsg)
	}
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="theme-dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Two-Factor Authentication - %s</title>
    <link rel="stylesheet" href="/static/css/common.css">
    <style>
        .login-container { max-width: 400px; margin: 100px auto; padding: 20px; }
        .login-box { background: var(--color-bg-secondary); border-radius: 8px; padding: 30px; border: 1px solid var(--color-border); }
        .login-title { text-align: center; margin-bottom: 10px; color: var(--color-text); }
        .login-subtitle { text-align: center; margin-bottom: 20px; color: var(--color-muted); font-size: 0.9rem; }
        .error { color: var(--color-error); margin-bottom: 15px; text-align: center; padding: 10px; background: var(--color-error-bg); border-radius: 4px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; color: var(--color-text); }
        .form-group input { width: 100%%; padding: 12px; border-radius: 4px; border: 2px solid var(--color-border); background: var(--color-bg-tertiary); color: var(--color-text); font-size: 1.2rem; text-align: center; letter-spacing: 0.5em; }
        .form-group input:focus { border-color: var(--color-primary); outline: none; }
        .btn-primary { width: 100%%; padding: 12px; background: var(--color-primary); color: var(--color-bg); border: none; border-radius: 4px; cursor: pointer; font-weight: 600; }
        .btn-primary:hover { opacity: 0.9; }
        .back-link { text-align: center; margin-top: 20px; }
        .back-link a { color: var(--color-muted); text-decoration: none; }
        .back-link a:hover { color: var(--color-primary); }
        .help-text { font-size: 0.85rem; color: var(--color-muted); margin-top: 10px; text-align: center; }
    </style>
</head>
<body style="background: var(--color-bg); min-height: 100vh;">
    <div class="login-container">
        <div class="login-box">
            <h1 class="login-title">Two-Factor Authentication</h1>
            <p class="login-subtitle">Enter the 6-digit code from your authenticator app</p>
            %s
            <form method="POST">
                <div class="form-group">
                    <label for="code">Verification Code</label>
                    <input type="text" id="code" name="code" maxlength="8" pattern="[0-9A-Za-z-]+" required autofocus autocomplete="one-time-code" placeholder="000000">
                </div>
                <button type="submit" class="btn-primary">Verify</button>
            </form>
            <p class="help-text">Or enter a backup code if you don't have access to your authenticator</p>
            <div class="back-link">
                <a href="/auth/login">← Back to Login</a>
            </div>
        </div>
    </div>
</body>
</html>`, h.cfg.Server.Title, errorHtml)
	w.Write([]byte(html))
}

// renderLoginPage renders the login form
func (h *AuthHandler) renderLoginPage(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errorHtml := ""
	if errorMsg != "" {
		errorHtml = fmt.Sprintf(`<div class="error">%s</div>`, errorMsg)
	}
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - %s</title>
    <link rel="stylesheet" href="/static/css/style.css">
    <style>
        .login-container { max-width: 400px; margin: 100px auto; padding: 20px; }
        .login-box { background: #1a1a2e; border-radius: 8px; padding: 30px; }
        .login-title { text-align: center; margin-bottom: 20px; }
        .error { color: #e74c3c; margin-bottom: 15px; text-align: center; padding: 10px; background: rgba(231,76,60,0.1); border-radius: 4px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; }
        .form-group input { width: 100%%; padding: 10px; border-radius: 4px; border: 1px solid #333; background: #0f0f1a; color: #fff; }
        .btn-primary { width: 100%%; padding: 12px; background: #6c5ce7; color: #fff; border: none; border-radius: 4px; cursor: pointer; }
        .btn-primary:hover { background: #5b4bc7; }
        .back-link { text-align: center; margin-top: 20px; }
        .back-link a { color: #888; text-decoration: none; }
        .back-link a:hover { color: #6c5ce7; }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="login-box">
            <h1 class="login-title">Login</h1>
            %s
            <form method="POST">
                <div class="form-group">
                    <label for="username">Username</label>
                    <input type="text" id="username" name="username" required autofocus>
                </div>
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" required>
                </div>
                <button type="submit" class="btn-primary">Login</button>
            </form>
            <div class="back-link">
                <a href="/">← Back to Search</a>
            </div>
        </div>
    </div>
</body>
</html>`, h.cfg.Server.Title, errorHtml)
	w.Write([]byte(html))
}

// LogoutPage handles logout (web route)
func (h *AuthHandler) LogoutPage(w http.ResponseWriter, r *http.Request) {
	// Clear any user session cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "user_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// RegisterPage renders registration form
func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	// Registration not implemented for this project - redirect to home
	http.Redirect(w, r, "/", http.StatusFound)
}

// PasswordForgotPage renders password forgot form
func (h *AuthHandler) PasswordForgotPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Password Reset</title></head>
<body>
<h1>Password Reset</h1>
<p>Password reset functionality is managed through the admin panel.</p>
<a href="/">Back to Home</a>
</body></html>`))
}

// PasswordResetPage handles password reset with token
func (h *AuthHandler) PasswordResetPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Password Reset</title></head>
<body>
<h1>Password Reset</h1>
<p>Invalid or expired reset token.</p>
<a href="/">Back to Home</a>
</body></html>`))
}

// VerifyPage handles email verification
func (h *AuthHandler) VerifyPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Email Verification</title></head>
<body>
<h1>Email Verification</h1>
<p>Email verification is not required for this application.</p>
<a href="/">Back to Home</a>
</body></html>`))
}

// API Routes per AI.md PART 31

// APILogin handles POST /api/v1/auth/login
func (h *AuthHandler) APILogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// This project uses admin panel authentication, not user auth
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "User authentication is handled through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APILogout handles POST /api/v1/auth/logout
func (h *AuthHandler) APILogout(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"message": "Logged out successfully",
	})
}

// APIRegister handles POST /api/v1/auth/register
func (h *AuthHandler) APIRegister(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Registration is not available for this application",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIPasswordForgot handles POST /api/v1/auth/password/forgot
func (h *AuthHandler) APIPasswordForgot(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Password reset is managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIPasswordReset handles POST /api/v1/auth/password/reset
func (h *AuthHandler) APIPasswordReset(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Password reset is managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIVerify handles POST /api/v1/auth/verify
func (h *AuthHandler) APIVerify(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Email verification is not required",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIRefresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) APIRefresh(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Token refresh is managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// UserHandler handles /user/ routes per AI.md PART 31
type UserHandler struct {
	cfg *config.Config
}

// NewUserHandler creates a new user handler
func NewUserHandler(cfg *config.Config) *UserHandler {
	return &UserHandler{
		cfg: cfg,
	}
}

// ProfilePage renders user profile (web route)
func (h *UserHandler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	// Redirect to preferences page for this project
	http.Redirect(w, r, "/preferences", http.StatusFound)
}

// SettingsPage renders user settings (web route)
func (h *UserHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/preferences", http.StatusFound)
}

// TokensPage renders API tokens management
func (h *UserHandler) TokensPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head><title>API Tokens</title></head>
<body>
<h1>API Tokens</h1>
<p>API token management is available in the admin panel.</p>
<a href="/admin">Go to Admin Panel</a>
</body></html>`))
}

// SecurityPage renders security settings
func (h *UserHandler) SecurityPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Security Settings</title></head>
<body>
<h1>Security Settings</h1>
<p>Security settings are managed through the admin panel.</p>
<a href="/admin">Go to Admin Panel</a>
</body></html>`))
}

// API Routes per AI.md PART 31

// APIProfile handles GET/PATCH /api/v1/user/profile
func (h *UserHandler) APIProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		// Return basic profile (no user system in this project)
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true,
			"data": map[string]interface{}{
				"theme":      h.cfg.Web.UI.Theme,
				"created_at": time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// PATCH
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Profile updates not supported",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIPassword handles POST /api/v1/user/password
func (h *UserHandler) APIPassword(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": false,
		"error":   "Password changes are managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APITokens handles GET/POST /api/v1/user/tokens
func (h *UserHandler) APITokens(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data":    []interface{}{},
		"message": "API tokens are managed through the admin panel",
	})
}

// APISessions handles GET /api/v1/user/sessions
func (h *UserHandler) APISessions(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data":    []interface{}{},
		"message": "Sessions are managed through the admin panel",
	})
}

// API2FA handles GET /api/v1/user/2fa
func (h *UserHandler) API2FA(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"enabled": false,
		},
		"message": "2FA is managed through the admin panel",
	})
}
