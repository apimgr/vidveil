// SPDX-License-Identifier: MIT
// AI.md PART 23: Test coverage for auth handlers
// VidVeil is stateless - no PART 34 (Multi-User), only Server Admin auth
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAuthHandler(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	if h == nil {
		t.Fatal("NewAuthHandler should return non-nil handler")
	}

	if h.appConfig != cfg {
		t.Error("AuthHandler should store config reference")
	}
}

func TestAuthHandler_LoginPage_GET(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	rr := httptest.NewRecorder()

	h.LoginPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("LoginPage GET returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Login") {
		t.Error("LoginPage should contain 'Login'")
	}

	if !strings.Contains(body, "<form") {
		t.Error("LoginPage should contain a form")
	}

	if !strings.Contains(body, "username") {
		t.Error("LoginPage should contain username field")
	}

	if !strings.Contains(body, "password") {
		t.Error("LoginPage should contain password field")
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("LoginPage Content-Type = %s, want text/html", contentType)
	}
}

func TestAuthHandler_LoginPage_POST_NoAdmin(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	// POST without admin handler should show error
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader("username=test&password=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.LoginPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("LoginPage POST returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Invalid") {
		t.Error("LoginPage POST should show error message")
	}
}

func TestAuthHandler_LogoutPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/logout", nil)
	rr := httptest.NewRecorder()

	h.LogoutPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("LogoutPage returned status %d, want %d", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	if location != "/" {
		t.Errorf("LogoutPage should redirect to /, got %s", location)
	}

	// Check that admin session cookie was cleared
	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "vidveil_admin_session" && c.MaxAge != -1 {
			t.Error("LogoutPage should clear vidveil_admin_session cookie")
		}
	}
}

func TestAuthHandler_PasswordForgotPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/password/forgot", nil)
	rr := httptest.NewRecorder()

	h.PasswordForgotPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PasswordForgotPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Password Reset") {
		t.Error("PasswordForgotPage should contain 'Password Reset'")
	}
}

func TestAuthHandler_PasswordResetPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/password/reset", nil)
	rr := httptest.NewRecorder()

	h.PasswordResetPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("PasswordResetPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Password Reset") {
		t.Error("PasswordResetPage should contain 'Password Reset'")
	}
}
