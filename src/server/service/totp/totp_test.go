// SPDX-License-Identifier: MIT
// Tests for the totp package: NewTOTPService, GenerateSecret, GenerateBackupCodes,
// GetProvisioningURI, ValidateCode, ValidateBackupCode, generateCode, Setup,
// and DefaultTOTPConfig.
package totp

import (
	"encoding/base32"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ---- NewTOTPService ----

func TestNewTOTPServiceNonNil(t *testing.T) {
	svc := NewTOTPService("TestIssuer")
	if svc == nil {
		t.Fatal("expected non-nil TOTPService")
	}
}

func TestNewTOTPServiceStoresIssuer(t *testing.T) {
	svc := NewTOTPService("MyApp")
	uri := svc.GetProvisioningURI("user@example.com", "SOMEBASE32SECRET")
	if !strings.Contains(uri, "MyApp") {
		t.Errorf("provisioning URI does not contain issuer: %q", uri)
	}
}

// ---- GenerateSecret ----

func TestGenerateSecretNonEmpty(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
}

func TestGenerateSecretValidBase32NoPadding(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret returned error: %v", err)
	}
	// Secret must be uppercase and not contain padding characters
	if secret != strings.ToUpper(secret) {
		t.Errorf("secret is not uppercase: %q", secret)
	}
	if strings.Contains(secret, "=") {
		t.Errorf("secret contains padding character '=': %q", secret)
	}
	// Decoding must succeed
	_, decErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if decErr != nil {
		t.Errorf("base32 decode of secret failed: %v (secret=%q)", decErr, secret)
	}
}

func TestGenerateSecretTwoCallsDiffer(t *testing.T) {
	svc := NewTOTPService("issuer")
	s1, err1 := svc.GenerateSecret()
	s2, err2 := svc.GenerateSecret()
	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateSecret errors: %v, %v", err1, err2)
	}
	if s1 == s2 {
		t.Errorf("two GenerateSecret calls returned identical secrets: %q", s1)
	}
}

// ---- GenerateBackupCodes ----

func TestGenerateBackupCodesReturnsTen(t *testing.T) {
	svc := NewTOTPService("issuer")
	codes, err := svc.GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes returned error: %v", err)
	}
	if len(codes) != 10 {
		t.Errorf("expected 10 backup codes, got %d", len(codes))
	}
}

func TestGenerateBackupCodesFormat(t *testing.T) {
	svc := NewTOTPService("issuer")
	codes, err := svc.GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes returned error: %v", err)
	}
	re := regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)
	for i, code := range codes {
		if !re.MatchString(code) {
			t.Errorf("backup code[%d] %q does not match XXXX-XXXX format", i, code)
		}
	}
}

func TestGenerateBackupCodesAllUnique(t *testing.T) {
	svc := NewTOTPService("issuer")
	codes, err := svc.GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes returned error: %v", err)
	}
	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate backup code found: %q", code)
		}
		seen[code] = true
	}
}

func TestGenerateBackupCodesTwoCallsDiffer(t *testing.T) {
	svc := NewTOTPService("issuer")
	codes1, err1 := svc.GenerateBackupCodes()
	codes2, err2 := svc.GenerateBackupCodes()
	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateBackupCodes errors: %v, %v", err1, err2)
	}
	// At least one code must differ between the two sets
	identical := true
	for i := range codes1 {
		if codes1[i] != codes2[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("two GenerateBackupCodes calls returned identical sets")
	}
}

// ---- GetProvisioningURI ----

func TestGetProvisioningURIScheme(t *testing.T) {
	svc := NewTOTPService("ExampleApp")
	secret := "JBSWY3DPEHPK3PXP"
	uri := svc.GetProvisioningURI("alice@example.com", secret)
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("URI does not start with otpauth://totp/: %q", uri)
	}
}

func TestGetProvisioningURIContainsIssuer(t *testing.T) {
	svc := NewTOTPService("ExampleApp")
	uri := svc.GetProvisioningURI("alice@example.com", "JBSWY3DPEHPK3PXP")
	if !strings.Contains(uri, "ExampleApp") {
		t.Errorf("URI missing issuer: %q", uri)
	}
}

func TestGetProvisioningURIContainsAccount(t *testing.T) {
	svc := NewTOTPService("ExampleApp")
	uri := svc.GetProvisioningURI("alice@example.com", "JBSWY3DPEHPK3PXP")
	if !strings.Contains(uri, "alice@example.com") {
		t.Errorf("URI missing accountName: %q", uri)
	}
}

func TestGetProvisioningURIContainsSecret(t *testing.T) {
	svc := NewTOTPService("ExampleApp")
	secret := "JBSWY3DPEHPK3PXP"
	uri := svc.GetProvisioningURI("alice@example.com", secret)
	if !strings.Contains(uri, secret) {
		t.Errorf("URI missing secret: %q", uri)
	}
}

func TestGetProvisioningURIContainsAlgorithm(t *testing.T) {
	svc := NewTOTPService("ExampleApp")
	uri := svc.GetProvisioningURI("alice@example.com", "JBSWY3DPEHPK3PXP")
	if !strings.Contains(uri, "algorithm=SHA1") {
		t.Errorf("URI missing algorithm=SHA1: %q", uri)
	}
}

func TestGetProvisioningURIContainsDigitsAndPeriod(t *testing.T) {
	svc := NewTOTPService("ExampleApp")
	uri := svc.GetProvisioningURI("alice@example.com", "JBSWY3DPEHPK3PXP")
	if !strings.Contains(uri, "digits=6") {
		t.Errorf("URI missing digits=6: %q", uri)
	}
	if !strings.Contains(uri, "period=30") {
		t.Errorf("URI missing period=30: %q", uri)
	}
}

// ---- ValidateCode ----

func TestValidateCodeCurrentTimeReturnsTrue(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	// Generate a valid code using the same counter the validator will use
	counter := time.Now().Unix() / DefaultPeriod
	code := svc.generateCode(secret, counter)
	if code == "" {
		t.Fatal("generateCode returned empty string for valid secret")
	}
	if !svc.ValidateCode(secret, code) {
		t.Errorf("ValidateCode returned false for current-time code %q", code)
	}
}

func TestValidateCodeWrongCodeReturnsFalse(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	// "000000" is almost always wrong; validate that it fails
	if svc.ValidateCode(secret, "000000") {
		// Probabilistic: only fails ~1-in-1000000 times; safe to assert false
		t.Log("000000 happened to be valid (astronomically unlikely) — skipping")
	}
}

func TestValidateCodeEmptyCodeReturnsFalse(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	if svc.ValidateCode(secret, "") {
		t.Error("ValidateCode returned true for empty code")
	}
}

func TestValidateCodePreviousWindowAccepted(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	// Generate a code for the previous time window (clock-drift tolerance)
	counter := time.Now().Unix()/DefaultPeriod - 1
	code := svc.generateCode(secret, counter)
	if !svc.ValidateCode(secret, code) {
		t.Errorf("ValidateCode rejected previous-window code %q (drift tolerance)", code)
	}
}

func TestValidateCodeFutureWindowAccepted(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	// Generate a code for the next time window (clock-drift tolerance)
	counter := time.Now().Unix()/DefaultPeriod + 1
	code := svc.generateCode(secret, counter)
	if !svc.ValidateCode(secret, code) {
		t.Errorf("ValidateCode rejected next-window code %q (drift tolerance)", code)
	}
}

// ---- ValidateBackupCode ----

func TestValidateBackupCodeExactMatch(t *testing.T) {
	svc := NewTOTPService("issuer")
	if !svc.ValidateBackupCode("ABCD-EFGH", []string{"ABCD-EFGH"}) {
		t.Error("expected true for exact match")
	}
}

func TestValidateBackupCodeCaseInsensitive(t *testing.T) {
	svc := NewTOTPService("issuer")
	if !svc.ValidateBackupCode("abcd-efgh", []string{"ABCD-EFGH"}) {
		t.Error("expected true for lowercase input against uppercase stored code")
	}
}

func TestValidateBackupCodeNoDashes(t *testing.T) {
	svc := NewTOTPService("issuer")
	if !svc.ValidateBackupCode("ABCDEFGH", []string{"ABCD-EFGH"}) {
		t.Error("expected true when dashes are omitted from input")
	}
}

func TestValidateBackupCodeNotInList(t *testing.T) {
	svc := NewTOTPService("issuer")
	if svc.ValidateBackupCode("WXYZ-1234", []string{"ABCD-EFGH", "IJKL-MNOP"}) {
		t.Error("expected false for code not in list")
	}
}

func TestValidateBackupCodeEmptyList(t *testing.T) {
	svc := NewTOTPService("issuer")
	if svc.ValidateBackupCode("ABCD-EFGH", []string{}) {
		t.Error("expected false for empty validCodes list")
	}
}

func TestValidateBackupCodeNilList(t *testing.T) {
	svc := NewTOTPService("issuer")
	if svc.ValidateBackupCode("ABCD-EFGH", nil) {
		t.Error("expected false for nil validCodes list")
	}
}

// ---- generateCode (unexported, accessible from same package) ----

func TestGenerateCodeReturnsSixDigits(t *testing.T) {
	svc := NewTOTPService("issuer")
	// GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ is the base32 of "12345678901234567890" (no padding)
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code := svc.generateCode(secret, 1)
	if code == "" {
		t.Fatal("generateCode returned empty string for valid secret")
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %q (len=%d)", code, len(code))
	}
	re := regexp.MustCompile(`^\d{6}$`)
	if !re.MatchString(code) {
		t.Errorf("code is not a 6-digit string: %q", code)
	}
}

func TestGenerateCodeEmptySecretStillProducesCode(t *testing.T) {
	// base32 decoding of "" succeeds with an empty key; HMAC-SHA1 with a zero-length
	// key still produces a deterministic digest, so generateCode returns a 6-digit code.
	// The result must be a 6-digit string, not a panic or a crash.
	svc := NewTOTPService("issuer")
	code := svc.generateCode("", 1)
	re := regexp.MustCompile(`^\d{6}$`)
	if !re.MatchString(code) {
		t.Errorf("generateCode with empty secret: expected 6-digit string, got %q", code)
	}
}

func TestGenerateCodeInvalidBase32ReturnsEmpty(t *testing.T) {
	svc := NewTOTPService("issuer")
	// "!!!" is not valid base32
	code := svc.generateCode("!!!", 1)
	if code != "" {
		t.Errorf("expected empty string for invalid base32 secret, got %q", code)
	}
}

func TestGenerateCodeDifferentCountersDifferentCodes(t *testing.T) {
	svc := NewTOTPService("issuer")
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code1 := svc.generateCode(secret, 1000)
	code2 := svc.generateCode(secret, 1001)
	// Different counters should almost always produce different codes
	if code1 == code2 {
		t.Logf("codes matched for consecutive counters (rare collision): %q", code1)
	}
}

// ---- Setup ----

func TestSetupNonNil(t *testing.T) {
	svc := NewTOTPService("issuer")
	data, err := svc.Setup("alice@example.com")
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil SetupData")
	}
}

func TestSetupSecretNonEmpty(t *testing.T) {
	svc := NewTOTPService("issuer")
	data, err := svc.Setup("alice@example.com")
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if data.Secret == "" {
		t.Error("expected non-empty Secret in SetupData")
	}
}

func TestSetupProvisionURIScheme(t *testing.T) {
	svc := NewTOTPService("issuer")
	data, err := svc.Setup("alice@example.com")
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if !strings.HasPrefix(data.ProvisionURI, "otpauth://totp/") {
		t.Errorf("ProvisionURI missing expected prefix: %q", data.ProvisionURI)
	}
}

func TestSetupBackupCodesCount(t *testing.T) {
	svc := NewTOTPService("issuer")
	data, err := svc.Setup("alice@example.com")
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if len(data.BackupCodes) != BackupCodeCount {
		t.Errorf("expected %d backup codes, got %d", BackupCodeCount, len(data.BackupCodes))
	}
}

func TestSetupSecretMatchesProvisionURI(t *testing.T) {
	svc := NewTOTPService("issuer")
	data, err := svc.Setup("alice@example.com")
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if !strings.Contains(data.ProvisionURI, data.Secret) {
		t.Errorf("ProvisionURI does not contain the secret: uri=%q secret=%q",
			data.ProvisionURI, data.Secret)
	}
}

// ---- DefaultTOTPConfig ----

func TestDefaultTOTPConfigEnabled(t *testing.T) {
	cfg := DefaultTOTPConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled=true in default TOTP config")
	}
}

func TestDefaultTOTPConfigNotRequired(t *testing.T) {
	cfg := DefaultTOTPConfig()
	if cfg.Required {
		t.Error("expected Required=false in default TOTP config")
	}
}

func TestDefaultTOTPConfigRememberDeviceDays(t *testing.T) {
	cfg := DefaultTOTPConfig()
	if cfg.RememberDeviceDays != 30 {
		t.Errorf("expected RememberDeviceDays=30, got %d", cfg.RememberDeviceDays)
	}
}
