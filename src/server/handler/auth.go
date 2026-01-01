// SPDX-License-Identifier: MIT
// AI.md PART 31: Route Standards - /auth/ and /user/ scopes
package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// AuthHandler handles authentication routes per AI.md PART 31
type AuthHandler struct {
	cfg      *config.Config
	adminHdl *AdminHandler
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg: cfg,
	}
}

// SetAdminHandler sets the admin handler reference for authentication
func (h *AuthHandler) SetAdminHandler(adminHdl *AdminHandler) {
	h.adminHdl = adminHdl
}

// LoginPage renders the login form and handles authentication per AI.md PART 31
func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
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

		// Authenticate admin using admin handler
		if h.adminHdl != nil {
			sessionID, err := h.adminHdl.AuthenticateAdmin(username, password)
			if err == nil && sessionID != "" {
				http.SetCookie(w, &http.Cookie{
					Name:     "vidveil_admin_session",
					Value:    sessionID,
					Path:     "/admin",
					MaxAge:   int(24 * time.Hour / time.Second),
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
				http.Redirect(w, r, "/admin", http.StatusFound)
				return
			}
		}
		errorMsg = "Invalid username or password"
	}

	h.renderLoginPage(w, errorMsg)
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
		"success": false,
		"error":   "User authentication is handled through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APILogout handles POST /api/v1/auth/logout
func (h *AuthHandler) APILogout(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// APIRegister handles POST /api/v1/auth/register
func (h *AuthHandler) APIRegister(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   "Registration is not available for this application",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIPasswordForgot handles POST /api/v1/auth/password/forgot
func (h *AuthHandler) APIPasswordForgot(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   "Password reset is managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIPasswordReset handles POST /api/v1/auth/password/reset
func (h *AuthHandler) APIPasswordReset(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   "Password reset is managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIVerify handles POST /api/v1/auth/verify
func (h *AuthHandler) APIVerify(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   "Email verification is not required",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIRefresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) APIRefresh(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
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
			"success": true,
			"data": map[string]interface{}{
				"theme":      h.cfg.Web.UI.Theme,
				"created_at": time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// PATCH
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   "Profile updates not supported",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APIPassword handles POST /api/v1/user/password
func (h *UserHandler) APIPassword(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   "Password changes are managed through the admin panel",
		"code":    "NOT_IMPLEMENTED",
	})
}

// APITokens handles GET/POST /api/v1/user/tokens
func (h *UserHandler) APITokens(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    []interface{}{},
		"message": "API tokens are managed through the admin panel",
	})
}

// APISessions handles GET /api/v1/user/sessions
func (h *UserHandler) APISessions(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    []interface{}{},
		"message": "Sessions are managed through the admin panel",
	})
}

// API2FA handles GET /api/v1/user/2fa
func (h *UserHandler) API2FA(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"enabled": false,
		},
		"message": "2FA is managed through the admin panel",
	})
}
