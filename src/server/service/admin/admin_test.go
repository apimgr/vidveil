// SPDX-License-Identifier: MIT
// Tests for the admin package: service construction, token generation,
// token hashing, password hashing/verification, and AdminUser JSON round-trip.
package admin

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

// ---- NewAdminService construction ----

func TestNewAdminServiceNilDB(t *testing.T) {
	svc := NewAdminService(nil)
	if svc == nil {
		t.Fatal("NewAdminService(nil) returned nil")
	}
}

func TestGetDBReturnsNilWhenConstructedWithNil(t *testing.T) {
	svc := NewAdminService(nil)
	if svc.GetDB() != nil {
		t.Errorf("GetDB() = %v, want nil", svc.GetDB())
	}
}

// ---- Zero-value state reads ----

func TestIsFirstRunFalseByDefault(t *testing.T) {
	svc := NewAdminService(nil)
	if svc.IsFirstRun() {
		t.Error("IsFirstRun() should be false before Initialize() is called")
	}
}

func TestGetSetupTokenEmptyByDefault(t *testing.T) {
	svc := NewAdminService(nil)
	if svc.GetSetupToken() != "" {
		t.Errorf("GetSetupToken() = %q, want empty string", svc.GetSetupToken())
	}
}

// ---- generateSecureToken ----

func TestGenerateSecureTokenNonEmpty(t *testing.T) {
	token, err := generateSecureToken(32)
	if err != nil {
		t.Fatalf("generateSecureToken(32) error: %v", err)
	}
	if token == "" {
		t.Error("generateSecureToken(32) returned empty string")
	}
}

func TestGenerateSecureTokenLength32Returns64HexChars(t *testing.T) {
	token, err := generateSecureToken(32)
	if err != nil {
		t.Fatalf("generateSecureToken(32) error: %v", err)
	}
	// 32 random bytes encoded as hex = 64 characters
	if len(token) != 64 {
		t.Errorf("generateSecureToken(32) length = %d, want 64", len(token))
	}
}

func TestGenerateSecureTokenIsLowercaseHex(t *testing.T) {
	token, err := generateSecureToken(32)
	if err != nil {
		t.Fatalf("generateSecureToken(32) error: %v", err)
	}
	re := regexp.MustCompile(`^[0-9a-f]+$`)
	if !re.MatchString(token) {
		t.Errorf("generateSecureToken(32) = %q, not lowercase hex", token)
	}
}

func TestGenerateSecureTokenTwoCallsDiffer(t *testing.T) {
	t1, err1 := generateSecureToken(32)
	t2, err2 := generateSecureToken(32)
	if err1 != nil || err2 != nil {
		t.Fatalf("generateSecureToken errors: %v, %v", err1, err2)
	}
	if t1 == t2 {
		t.Errorf("two generateSecureToken calls returned identical tokens: %q", t1)
	}
}

func TestGenerateSecureTokenLength1(t *testing.T) {
	token, err := generateSecureToken(1)
	if err != nil {
		t.Fatalf("generateSecureToken(1) error: %v", err)
	}
	// 1 byte = 2 hex chars
	if len(token) != 2 {
		t.Errorf("generateSecureToken(1) length = %d, want 2", len(token))
	}
}

// ---- hashToken ----

func TestHashTokenNonEmpty(t *testing.T) {
	h := hashToken("sometoken")
	if h == "" {
		t.Error("hashToken returned empty string")
	}
}

func TestHashTokenIs64HexChars(t *testing.T) {
	h := hashToken("sometoken")
	// SHA-256 produces 32 bytes = 64 hex characters
	if len(h) != 64 {
		t.Errorf("hashToken length = %d, want 64 (hex=%q)", len(h), h)
	}
	re := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !re.MatchString(h) {
		t.Errorf("hashToken = %q, not lowercase hex", h)
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	input := "deterministic_input"
	h1 := hashToken(input)
	h2 := hashToken(input)
	if h1 != h2 {
		t.Errorf("hashToken not deterministic: %q != %q", h1, h2)
	}
}

func TestHashTokenDifferentInputsDifferentHashes(t *testing.T) {
	h1 := hashToken("token_a")
	h2 := hashToken("token_b")
	if h1 == h2 {
		t.Error("hashToken: different inputs produced the same hash")
	}
}

func TestHashTokenEmptyStringNonEmpty(t *testing.T) {
	h := hashToken("")
	if h == "" {
		t.Error("hashToken(\"\") returned empty string; SHA-256 always produces output")
	}
	if len(h) != 64 {
		t.Errorf("hashToken(\"\") length = %d, want 64", len(h))
	}
}

// ---- hashPassword / verifyPassword round-trips ----

func TestHashPasswordReturnsNonEmptyPHCString(t *testing.T) {
	hash, err := hashPassword("TestPass123!")
	if err != nil {
		t.Fatalf("hashPassword error: %v", err)
	}
	if hash == "" {
		t.Error("hashPassword returned empty string")
	}
	// PHC format starts with $argon2id$
	if len(hash) < 10 {
		t.Errorf("hashPassword returned suspiciously short string: %q", hash)
	}
}

func TestHashPasswordProducesDifferentHashesForSameInput(t *testing.T) {
	// Each call uses a fresh random salt, so hashes must differ.
	h1, err1 := hashPassword("SamePass99!")
	h2, err2 := hashPassword("SamePass99!")
	if err1 != nil || err2 != nil {
		t.Fatalf("hashPassword errors: %v, %v", err1, err2)
	}
	if h1 == h2 {
		t.Error("two hashPassword calls with the same input produced identical hashes (same salt is a bug)")
	}
}

func TestVerifyPasswordCorrectPasswordReturnsTrue(t *testing.T) {
	password := "CorrectPass42!"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword error: %v", err)
	}
	ok, err := verifyPassword(password, hash)
	if err != nil {
		t.Fatalf("verifyPassword error: %v", err)
	}
	if !ok {
		t.Error("verifyPassword returned false for the correct password")
	}
}

func TestVerifyPasswordWrongPasswordReturnsFalse(t *testing.T) {
	hash, err := hashPassword("CorrectPass42!")
	if err != nil {
		t.Fatalf("hashPassword error: %v", err)
	}
	ok, err := verifyPassword("WrongPass99!", hash)
	if err != nil {
		t.Fatalf("verifyPassword error: %v", err)
	}
	if ok {
		t.Error("verifyPassword returned true for an incorrect password")
	}
}

func TestVerifyPasswordEmptyPasswordAgainstRealHashReturnsFalse(t *testing.T) {
	hash, err := hashPassword("SomePass1!")
	if err != nil {
		t.Fatalf("hashPassword error: %v", err)
	}
	ok, err := verifyPassword("", hash)
	if err != nil {
		t.Fatalf("verifyPassword error: %v", err)
	}
	if ok {
		t.Error("verifyPassword returned true for an empty password")
	}
}

func TestVerifyPasswordInvalidHashFormatReturnsError(t *testing.T) {
	_, err := verifyPassword("any", "notvalidhash")
	if err == nil {
		t.Error("verifyPassword should return an error for an invalid hash format")
	}
}

func TestVerifyPasswordUnsupportedAlgorithmReturnsError(t *testing.T) {
	// Construct a PHC string that claims bcrypt, which verifyPassword rejects.
	badHash := "$bcrypt$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	_, err := verifyPassword("any", badHash)
	if err == nil {
		t.Error("verifyPassword should return an error for an unsupported algorithm")
	}
}

// ---- AdminUser struct ----

func TestAdminUserConstruction(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	login := now.Add(-1 * time.Hour)
	u := AdminUser{
		ID:          42,
		Username:    "primary_admin",
		TOTPEnabled: true,
		CreatedAt:   now,
		LastLogin:   &login,
		LoginCount:  5,
		IsPrimary:   true,
	}
	if u.ID != 42 {
		t.Errorf("ID = %d, want 42", u.ID)
	}
	if u.Username != "primary_admin" {
		t.Errorf("Username = %q, want %q", u.Username, "primary_admin")
	}
	if !u.TOTPEnabled {
		t.Error("TOTPEnabled should be true")
	}
	if !u.IsPrimary {
		t.Error("IsPrimary should be true")
	}
	if u.LoginCount != 5 {
		t.Errorf("LoginCount = %d, want 5", u.LoginCount)
	}
}

func TestAdminUserJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	login := now.Add(-2 * time.Hour)
	original := AdminUser{
		ID:          7,
		Username:    "admin_user",
		TOTPEnabled: false,
		CreatedAt:   now,
		LastLogin:   &login,
		LoginCount:  12,
		IsPrimary:   false,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("json.Marshal returned empty bytes")
	}

	var decoded AdminUser
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Username != original.Username {
		t.Errorf("Username: got %q, want %q", decoded.Username, original.Username)
	}
	if decoded.TOTPEnabled != original.TOTPEnabled {
		t.Errorf("TOTPEnabled: got %v, want %v", decoded.TOTPEnabled, original.TOTPEnabled)
	}
	if decoded.LoginCount != original.LoginCount {
		t.Errorf("LoginCount: got %d, want %d", decoded.LoginCount, original.LoginCount)
	}
	if decoded.IsPrimary != original.IsPrimary {
		t.Errorf("IsPrimary: got %v, want %v", decoded.IsPrimary, original.IsPrimary)
	}
	if decoded.LastLogin == nil {
		t.Error("LastLogin: got nil, want non-nil")
	}
}

func TestAdminUserJSONOmitsLastLoginWhenNil(t *testing.T) {
	u := AdminUser{
		ID:       1,
		Username: "no_login",
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// last_login is omitempty — must be absent when nil
	if _, exists := m["last_login"]; exists {
		t.Error("last_login field should be omitted when nil")
	}
}

func TestAdminUserJSONFieldNamesMatchTags(t *testing.T) {
	login := time.Now().UTC()
	u := AdminUser{
		ID:          99,
		Username:    "tagcheck",
		TOTPEnabled: true,
		CreatedAt:   login,
		LastLogin:   &login,
		LoginCount:  3,
		IsPrimary:   true,
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"id", "username", "totp_enabled", "created_at", "last_login", "login_count", "is_primary"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}
