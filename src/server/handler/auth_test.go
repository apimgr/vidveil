// SPDX-License-Identifier: MIT
// AI.md PART 23: Test coverage for auth handlers
package handler

import (
	"encoding/json"
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

	if h.cfg != cfg {
		t.Error("AuthHandler should store config reference")
	}
}

func TestNewUserHandler(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	if h == nil {
		t.Fatal("NewUserHandler should return non-nil handler")
	}

	if h.cfg != cfg {
		t.Error("UserHandler should store config reference")
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

	// Check that session cookie was cleared
	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "user_session" && c.MaxAge != -1 {
			t.Error("LogoutPage should clear user_session cookie")
		}
	}
}

func TestAuthHandler_RegisterPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/register", nil)
	rr := httptest.NewRecorder()

	h.RegisterPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("RegisterPage returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	if location != "/" {
		t.Errorf("RegisterPage should redirect to /, got %s", location)
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

func TestAuthHandler_VerifyPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/verify", nil)
	rr := httptest.NewRecorder()

	h.VerifyPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("VerifyPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Email Verification") {
		t.Error("VerifyPage should contain 'Email Verification'")
	}
}

func TestAuthHandler_APILogin(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	rr := httptest.NewRecorder()

	h.APILogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APILogin returned status %d, want %d", rr.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APILogin returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != false {
		t.Error("APILogin should return ok: false (not implemented)")
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APILogin should return code: NOT_IMPLEMENTED")
	}
}

func TestAuthHandler_APILogout(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rr := httptest.NewRecorder()

	h.APILogout(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APILogout returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != true {
		t.Error("APILogout should return ok: true")
	}
}

func TestAuthHandler_APIRegister(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/register", nil)
	rr := httptest.NewRecorder()

	h.APIRegister(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIRegister returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != false {
		t.Error("APIRegister should return ok: false")
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APIRegister should return code: NOT_IMPLEMENTED")
	}
}

func TestAuthHandler_APIPasswordForgot(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/password/forgot", nil)
	rr := httptest.NewRecorder()

	h.APIPasswordForgot(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIPasswordForgot returned invalid JSON: %v", err)
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APIPasswordForgot should return code: NOT_IMPLEMENTED")
	}
}

func TestAuthHandler_APIPasswordReset(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/password/reset", nil)
	rr := httptest.NewRecorder()

	h.APIPasswordReset(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIPasswordReset returned invalid JSON: %v", err)
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APIPasswordReset should return code: NOT_IMPLEMENTED")
	}
}

func TestAuthHandler_APIVerify(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/verify", nil)
	rr := httptest.NewRecorder()

	h.APIVerify(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIVerify returned invalid JSON: %v", err)
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APIVerify should return code: NOT_IMPLEMENTED")
	}
}

func TestAuthHandler_APIRefresh(t *testing.T) {
	cfg := createTestConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	rr := httptest.NewRecorder()

	h.APIRefresh(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIRefresh returned invalid JSON: %v", err)
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APIRefresh should return code: NOT_IMPLEMENTED")
	}
}

func TestUserHandler_ProfilePage(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/user/profile", nil)
	rr := httptest.NewRecorder()

	h.ProfilePage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("ProfilePage returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	if location != "/preferences" {
		t.Errorf("ProfilePage should redirect to /preferences, got %s", location)
	}
}

func TestUserHandler_SettingsPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/user/settings", nil)
	rr := httptest.NewRecorder()

	h.SettingsPage(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("SettingsPage returned status %d, want %d (redirect)", rr.Code, http.StatusFound)
	}
}

func TestUserHandler_TokensPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/user/tokens", nil)
	rr := httptest.NewRecorder()

	h.TokensPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("TokensPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "API Tokens") {
		t.Error("TokensPage should contain 'API Tokens'")
	}
}

func TestUserHandler_SecurityPage(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/user/security", nil)
	rr := httptest.NewRecorder()

	h.SecurityPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SecurityPage returned status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Security Settings") {
		t.Error("SecurityPage should contain 'Security Settings'")
	}
}

func TestUserHandler_APIProfile_GET(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	rr := httptest.NewRecorder()

	h.APIProfile(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIProfile returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != true {
		t.Error("APIProfile GET should return ok: true")
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Error("APIProfile should return data object")
	}

	if data["theme"] != "dark" {
		t.Errorf("APIProfile theme = %v, want 'dark'", data["theme"])
	}
}

func TestUserHandler_APIProfile_PATCH(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("PATCH", "/api/v1/user/profile", nil)
	rr := httptest.NewRecorder()

	h.APIProfile(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIProfile returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != false {
		t.Error("APIProfile PATCH should return ok: false (not supported)")
	}
}

func TestUserHandler_APIPassword(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/user/password", nil)
	rr := httptest.NewRecorder()

	h.APIPassword(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APIPassword returned invalid JSON: %v", err)
	}

	if response["code"] != "NOT_IMPLEMENTED" {
		t.Error("APIPassword should return code: NOT_IMPLEMENTED")
	}
}

func TestUserHandler_APITokens(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/user/tokens", nil)
	rr := httptest.NewRecorder()

	h.APITokens(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APITokens returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != true {
		t.Error("APITokens should return ok: true")
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Error("APITokens should return data array")
	}

	if len(data) != 0 {
		t.Error("APITokens should return empty array")
	}
}

func TestUserHandler_APISessions(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/user/sessions", nil)
	rr := httptest.NewRecorder()

	h.APISessions(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("APISessions returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != true {
		t.Error("APISessions should return ok: true")
	}
}

func TestUserHandler_API2FA(t *testing.T) {
	cfg := createTestConfig()
	h := NewUserHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/user/2fa", nil)
	rr := httptest.NewRecorder()

	h.API2FA(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("API2FA returned invalid JSON: %v", err)
	}

	// Per AI.md PART 14: API uses "ok" field, not "success"
	if response["ok"] != true {
		t.Error("API2FA should return ok: true")
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Error("API2FA should return data object")
	}

	if data["enabled"] != false {
		t.Error("API2FA enabled should be false")
	}
}
