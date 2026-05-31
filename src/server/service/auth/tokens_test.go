// SPDX-License-Identifier: MIT
// Tests for the auth tokens package: constants, ExpirationOptions, GenerateToken,
// HashToken, GetTokenPrefix, ValidateTokenFormat, GetTokenType, and IsAgentToken.
package auth

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// ---- Constants ----

func TestPrefixAdminValue(t *testing.T) {
	if PrefixAdmin != "adm_" {
		t.Errorf("PrefixAdmin = %q, want %q", PrefixAdmin, "adm_")
	}
}

func TestPrefixAdminAgtValue(t *testing.T) {
	if PrefixAdminAgt != "adm_agt_" {
		t.Errorf("PrefixAdminAgt = %q, want %q", PrefixAdminAgt, "adm_agt_")
	}
}

func TestScopeGlobalValue(t *testing.T) {
	if ScopeGlobal != "global" {
		t.Errorf("ScopeGlobal = %q, want %q", ScopeGlobal, "global")
	}
}

func TestScopeReadWriteValue(t *testing.T) {
	if ScopeReadWrite != "read-write" {
		t.Errorf("ScopeReadWrite = %q, want %q", ScopeReadWrite, "read-write")
	}
}

func TestScopeReadValue(t *testing.T) {
	if ScopeRead != "read" {
		t.Errorf("ScopeRead = %q, want %q", ScopeRead, "read")
	}
}

// ---- ExpirationOptions ----

func TestExpirationOptionNever(t *testing.T) {
	if ExpirationOptions["never"] != 0 {
		t.Errorf("ExpirationOptions[never] = %v, want 0", ExpirationOptions["never"])
	}
}

func TestExpirationOption7Days(t *testing.T) {
	want := 7 * 24 * time.Hour
	if ExpirationOptions["7days"] != want {
		t.Errorf("ExpirationOptions[7days] = %v, want %v", ExpirationOptions["7days"], want)
	}
}

func TestExpirationOption1Year(t *testing.T) {
	want := 365 * 24 * time.Hour
	if ExpirationOptions["1year"] != want {
		t.Errorf("ExpirationOptions[1year] = %v, want %v", ExpirationOptions["1year"], want)
	}
}

// ---- GenerateToken ----

func TestGenerateTokenAdminPrefixAndLength(t *testing.T) {
	token, err := GenerateToken(PrefixAdmin)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if !strings.HasPrefix(token, "adm_") {
		t.Errorf("token does not start with adm_: %q", token)
	}
	// "adm_" is 4 chars, payload is 32, total 36
	if len(token) != 36 {
		t.Errorf("expected length 36, got %d (token=%q)", len(token), token)
	}
}

func TestGenerateTokenAgentPrefixAndLength(t *testing.T) {
	token, err := GenerateToken(PrefixAdminAgt)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if !strings.HasPrefix(token, "adm_agt_") {
		t.Errorf("token does not start with adm_agt_: %q", token)
	}
	// "adm_agt_" is 8 chars, payload is 32, total 40
	if len(token) != 40 {
		t.Errorf("expected length 40, got %d (token=%q)", len(token), token)
	}
}

func TestGenerateTokenTwoCallsDiffer(t *testing.T) {
	t1, err1 := GenerateToken(PrefixAdmin)
	t2, err2 := GenerateToken(PrefixAdmin)
	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateToken errors: %v, %v", err1, err2)
	}
	if t1 == t2 {
		t.Errorf("two GenerateToken calls returned identical tokens: %q", t1)
	}
}

func TestGenerateTokenPayloadURLSafe(t *testing.T) {
	token, err := GenerateToken(PrefixAdmin)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	payload := token[len(PrefixAdmin):]
	// base64 URL encoding uses [A-Za-z0-9_-]; no + or /
	if strings.ContainsAny(payload, "+/") {
		t.Errorf("token payload contains non-URL-safe chars: %q", payload)
	}
	re := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !re.MatchString(payload) {
		t.Errorf("token payload contains unexpected characters: %q", payload)
	}
}

// ---- HashToken ----

func TestHashTokenNonEmpty(t *testing.T) {
	h := HashToken("adm_sometoken")
	if h == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashTokenIs64HexChars(t *testing.T) {
	h := HashToken("adm_sometoken")
	// SHA-256 produces 32 bytes = 64 hex characters
	if len(h) != 64 {
		t.Errorf("expected 64 hex chars, got %d (hash=%q)", len(h), h)
	}
	re := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !re.MatchString(h) {
		t.Errorf("hash is not lowercase hex: %q", h)
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	token := "adm_deterministic_token"
	h1 := HashToken(token)
	h2 := HashToken(token)
	if h1 != h2 {
		t.Errorf("HashToken is not deterministic: %q != %q", h1, h2)
	}
}

func TestHashTokenDifferentInputsDifferentHashes(t *testing.T) {
	h1 := HashToken("adm_token_a")
	h2 := HashToken("adm_token_b")
	if h1 == h2 {
		t.Error("expected different hashes for different tokens")
	}
}

// ---- GetTokenPrefix ----

func TestGetTokenPrefixLongTokenAppendsDots(t *testing.T) {
	// A token longer than 8 chars must return first 8 chars + "..."
	token := "adm_a1b2c3d4e5"
	got := GetTokenPrefix(token)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected trailing ..., got %q", got)
	}
	if !strings.HasPrefix(got, "adm_a1b2") {
		t.Errorf("expected prefix adm_a1b2, got %q", got)
	}
}

func TestGetTokenPrefixShortTokenNoDots(t *testing.T) {
	// Tokens with fewer than 8 characters must be returned as-is
	token := "abc"
	got := GetTokenPrefix(token)
	if got != "abc" {
		t.Errorf("expected %q, got %q", "abc", got)
	}
	if strings.Contains(got, "...") {
		t.Errorf("short token should not have trailing ...: %q", got)
	}
}

func TestGetTokenPrefixExactlyEightCharsAppendsDots(t *testing.T) {
	// A token with exactly 8 characters: first 8 chars + "..."
	token := "abcdefgh"
	got := GetTokenPrefix(token)
	if got != "abcdefgh..." {
		t.Errorf("expected %q, got %q", "abcdefgh...", got)
	}
}

// ---- ValidateTokenFormat ----

func TestValidateTokenFormatValidAdminToken(t *testing.T) {
	// adm_ + 32 alphanumeric chars
	token := "adm_" + strings.Repeat("a", 32)
	if !ValidateTokenFormat(token) {
		t.Errorf("expected valid admin token: %q", token)
	}
}

func TestValidateTokenFormatValidAgentToken(t *testing.T) {
	// adm_agt_ + 32 alphanumeric chars
	token := "adm_agt_" + strings.Repeat("b", 32)
	if !ValidateTokenFormat(token) {
		t.Errorf("expected valid agent token: %q", token)
	}
}

func TestValidateTokenFormatAdminTooShort(t *testing.T) {
	// adm_ + 31 chars is invalid
	token := "adm_" + strings.Repeat("a", 31)
	if ValidateTokenFormat(token) {
		t.Errorf("expected invalid: admin token with 31-char payload: %q", token)
	}
}

func TestValidateTokenFormatAdminTooLong(t *testing.T) {
	// adm_ + 33 chars is invalid
	token := "adm_" + strings.Repeat("a", 33)
	if ValidateTokenFormat(token) {
		t.Errorf("expected invalid: admin token with 33-char payload: %q", token)
	}
}

func TestValidateTokenFormatUnknownPrefix(t *testing.T) {
	token := "tok_" + strings.Repeat("a", 32)
	if ValidateTokenFormat(token) {
		t.Errorf("expected invalid for unknown prefix: %q", token)
	}
}

func TestValidateTokenFormatEmptyString(t *testing.T) {
	if ValidateTokenFormat("") {
		t.Error("expected invalid for empty string")
	}
}

// ---- GetTokenType ----

func TestGetTokenTypeAdminAgent(t *testing.T) {
	token := "adm_agt_" + strings.Repeat("x", 32)
	got := GetTokenType(token)
	if got != "admin_agent" {
		t.Errorf("GetTokenType = %q, want %q", got, "admin_agent")
	}
}

func TestGetTokenTypeAdmin(t *testing.T) {
	token := "adm_" + strings.Repeat("x", 32)
	got := GetTokenType(token)
	if got != "admin" {
		t.Errorf("GetTokenType = %q, want %q", got, "admin")
	}
}

func TestGetTokenTypeUnknown(t *testing.T) {
	token := "unknown_" + strings.Repeat("x", 32)
	got := GetTokenType(token)
	if got != "unknown" {
		t.Errorf("GetTokenType = %q, want %q", got, "unknown")
	}
}

func TestGetTokenTypeEmptyString(t *testing.T) {
	got := GetTokenType("")
	if got != "unknown" {
		t.Errorf("GetTokenType(\"\") = %q, want %q", got, "unknown")
	}
}

// ---- IsAgentToken ----

func TestIsAgentTokenAgentPrefix(t *testing.T) {
	token := "adm_agt_" + strings.Repeat("x", 32)
	if !IsAgentToken(token) {
		t.Errorf("expected true for agent token: %q", token)
	}
}

func TestIsAgentTokenAdminPrefix(t *testing.T) {
	token := "adm_" + strings.Repeat("x", 32)
	if IsAgentToken(token) {
		t.Errorf("expected false for admin (non-agent) token: %q", token)
	}
}

func TestIsAgentTokenEmptyString(t *testing.T) {
	if IsAgentToken("") {
		t.Error("expected false for empty string")
	}
}
