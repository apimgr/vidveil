// SPDX-License-Identifier: MIT
// Coverage tests for loadOrGenerateKeys, runVanityGeneration, and the
// copyFile dst-create-failure path.
// No Tor binary or network access required.
package tor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// ── loadOrGenerateKeys ────────────────────────────────────────────────────────

// TestLoadOrGenerateKeys_GeneratesKeysWhenMissing covers the key-generation
// path: no key files exist, so new ed25519 keys are created and saved.
func TestLoadOrGenerateKeys_GeneratesKeysWhenMissing(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}

	if err := s.loadOrGenerateKeys(); err != nil {
		t.Fatalf("loadOrGenerateKeys (generate): %v", err)
	}
	if len(s.privateKey) == 0 {
		t.Error("privateKey should be non-nil after key generation")
	}
	if len(s.publicKey) == 0 {
		t.Error("publicKey should be non-nil after key generation")
	}
	if _, err := os.Stat(filepath.Join(siteDir, "hs_ed25519_secret_key")); err != nil {
		t.Errorf("secret key file not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "hs_ed25519_public_key")); err != nil {
		t.Errorf("public key file not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "hostname")); err != nil {
		t.Errorf("hostname file not created: %v", err)
	}
}

// TestLoadOrGenerateKeys_Loads64ByteKey covers the 64-byte key file branch.
func TestLoadOrGenerateKeys_Loads64ByteKey(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	// 64-byte format: seed (32 bytes) + public key (32 bytes)
	secretData := make([]byte, 64)
	copy(secretData[:32], priv.Seed())
	copy(secretData[32:], pub)
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), secretData, 0600); err != nil {
		t.Fatalf("WriteFile secret key: %v", err)
	}

	if err := s.loadOrGenerateKeys(); err != nil {
		t.Fatalf("loadOrGenerateKeys (64-byte): %v", err)
	}
	if len(s.privateKey) == 0 {
		t.Error("privateKey should be non-nil after loading 64-byte key")
	}
}

// TestLoadOrGenerateKeys_Loads96ByteTorFormatKey covers the 96-byte Tor format
// key file: 32-byte header + 32-byte seed + 32-byte priv expansion.
func TestLoadOrGenerateKeys_Loads96ByteTorFormatKey(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	// 96-byte Tor format: 32-byte header + priv (64 bytes = seed + expansion)
	header := make([]byte, 32)
	copy(header, "== ed25519v1-secret: type0 ==\x00\x00\x00")
	secretData := append(header, priv.Seed()...)
	secretData = append(secretData, priv[32:]...)
	if len(secretData) != 96 {
		t.Fatalf("expected 96-byte key, got %d", len(secretData))
	}
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), secretData, 0600); err != nil {
		t.Fatalf("WriteFile secret key: %v", err)
	}

	if err := s.loadOrGenerateKeys(); err != nil {
		t.Fatalf("loadOrGenerateKeys (96-byte): %v", err)
	}
	if len(s.privateKey) == 0 {
		t.Error("privateKey should be non-nil after loading 96-byte key")
	}
}

// TestLoadOrGenerateKeys_InvalidFormatReturnsError covers the branch where the
// key file exists but its length is ≥64 and <96 (invalid format).
func TestLoadOrGenerateKeys_InvalidFormatReturnsError(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}

	// 72 bytes: ≥64 but <96 → "invalid secret key format"
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), make([]byte, 72), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := s.loadOrGenerateKeys(); err == nil {
		t.Error("loadOrGenerateKeys with 72-byte key: expected error, got nil")
	}
}

// ── runVanityGeneration ───────────────────────────────────────────────────────

// TestRunVanityGeneration_EmptyPrefix_MatchesImmediately verifies that an
// empty prefix causes runVanityGeneration to match on the first attempt and
// write key files into the vanity_pending directory.
func TestRunVanityGeneration_EmptyPrefix_MatchesImmediately(t *testing.T) {
	s := newTestService(t)
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}
	s.vanityStatus = &VanityStatus{Active: true, StartTime: time.Now()}

	// Run synchronously — empty prefix always matches, so the function returns
	// after the first successful key generation.
	s.runVanityGeneration(context.Background(), "")

	if s.vanityStatus.Active {
		t.Error("vanityStatus.Active should be false after match found")
	}
	pendingDir := filepath.Join(s.dataDir, "vanity_pending")
	if _, err := os.Stat(pendingDir); err != nil {
		t.Errorf("vanity_pending directory not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pendingDir, "hostname")); err != nil {
		t.Errorf("hostname file not written in vanity_pending: %v", err)
	}
}

// TestRunVanityGeneration_CancelledContext_ReturnsPromptly verifies that a
// pre-cancelled context causes runVanityGeneration to exit promptly.
func TestRunVanityGeneration_CancelledContext_ReturnsPromptly(t *testing.T) {
	s := newTestService(t)
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}
	s.vanityStatus = &VanityStatus{Active: true, StartTime: time.Now()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		// Use an impossible prefix so default branch never matches; cancelled
		// context must eventually win the select.
		s.runVanityGeneration(ctx, "zzzzz")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runVanityGeneration with cancelled context did not return within 10s")
	}
	if s.vanityStatus.Active {
		t.Error("vanityStatus.Active should be false after context cancellation")
	}
}

// ── copyFile — dst create failure ────────────────────────────────────────────

// TestCopyFile_DstCreateFails_ReturnsError covers the os.Create(dst) error
// path by using a directory path as the destination.
func TestCopyFile_DstCreateFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("content"), 0600); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}
	// Destination is the temp directory itself — os.Create on a directory fails.
	if err := copyFile(src, dir); err == nil {
		t.Error("copyFile with dst=directory: expected error, got nil")
	}
}

// ── RegenerateAddress ─────────────────────────────────────────────────────────

// TestRegenerateAddress_GeneratesNewKeys verifies that RegenerateAddress writes
// new key files and updates the in-memory public/private key.
func TestRegenerateAddress_GeneratesNewKeys(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}

	if err := s.RegenerateAddress(); err != nil {
		t.Fatalf("RegenerateAddress: %v", err)
	}
	if len(s.privateKey) == 0 {
		t.Error("privateKey should be set after RegenerateAddress")
	}
	if len(s.publicKey) == 0 {
		t.Error("publicKey should be set after RegenerateAddress")
	}
	if s.onionAddress == "" {
		t.Error("onionAddress should be non-empty after RegenerateAddress")
	}
	if _, err := os.Stat(filepath.Join(siteDir, "hs_ed25519_secret_key")); err != nil {
		t.Errorf("secret key file not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "hostname")); err != nil {
		t.Errorf("hostname file not written: %v", err)
	}
}

// ── ImportKeys ────────────────────────────────────────────────────────────────

// TestImportKeys_TooShortReturnsError covers the key-too-short validation path.
func TestImportKeys_TooShortReturnsError(t *testing.T) {
	s := newTestService(t)
	if err := s.ImportKeys(make([]byte, 32)); err == nil {
		t.Error("ImportKeys with 32-byte key: expected error, got nil")
	}
}

// TestImportKeys_ValidKeyReloads covers the success path: 64-byte key is
// written to siteDir and then reloaded into memory.
func TestImportKeys_ValidKeyReloads(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	// 64-byte format: seed + public key
	secretData := make([]byte, 64)
	copy(secretData[:32], priv.Seed())
	copy(secretData[32:], pub)

	if err := s.ImportKeys(secretData); err != nil {
		t.Fatalf("ImportKeys: %v", err)
	}
	if len(s.privateKey) == 0 {
		t.Error("privateKey should be loaded after ImportKeys")
	}
}

// ── ApplyVanityAddress ────────────────────────────────────────────────────────

// TestApplyVanityAddress_NoPendingReturnsError covers the path where no
// vanity_pending directory / key file exists.
func TestApplyVanityAddress_NoPendingReturnsError(t *testing.T) {
	s := newTestService(t)
	if err := s.ApplyVanityAddress(); err == nil {
		t.Error("ApplyVanityAddress with no pending key: expected error, got nil")
	}
}

// TestApplyVanityAddress_AppliesPendingKeys verifies the success path: pending
// keys are moved to siteDir, backup is created, and keys reload.
func TestApplyVanityAddress_AppliesPendingKeys(t *testing.T) {
	s := newTestService(t)
	siteDir := filepath.Join(s.dataDir, "site")
	pendingDir := filepath.Join(s.dataDir, "vanity_pending")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatalf("MkdirAll siteDir: %v", err)
	}
	if err := os.MkdirAll(pendingDir, 0700); err != nil {
		t.Fatalf("MkdirAll pendingDir: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	// Write pending keys in 64-byte format
	secretData := make([]byte, 64)
	copy(secretData[:32], priv.Seed())
	copy(secretData[32:], pub)
	if err := os.WriteFile(filepath.Join(pendingDir, "hs_ed25519_secret_key"), secretData, 0600); err != nil {
		t.Fatalf("write pending secret key: %v", err)
	}
	pubData := make([]byte, 32)
	copy(pubData, pub)
	if err := os.WriteFile(filepath.Join(pendingDir, "hs_ed25519_public_key"), pubData, 0600); err != nil {
		t.Fatalf("write pending public key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pendingDir, "hostname"), []byte("test.onion\n"), 0600); err != nil {
		t.Fatalf("write pending hostname: %v", err)
	}

	if err := s.ApplyVanityAddress(); err != nil {
		t.Fatalf("ApplyVanityAddress: %v", err)
	}
	if len(s.privateKey) == 0 {
		t.Error("privateKey should be set after ApplyVanityAddress")
	}
	// pending dir should be removed
	if _, err := os.Stat(pendingDir); !os.IsNotExist(err) {
		t.Error("vanity_pending should be removed after ApplyVanityAddress")
	}
}

// ── GetVanityStatus ───────────────────────────────────────────────────────────

// TestGetVanityStatus_NilStatusReturnsNil covers the nil vanityStatus path.
func TestGetVanityStatus_NilStatusReturnsNil(t *testing.T) {
	s := newTestService(t)
	if got := s.GetVanityStatus(); got != nil {
		t.Errorf("GetVanityStatus with nil status = %v, want nil", got)
	}
}

// TestGetVanityStatus_ReturnsACopy verifies that the returned VanityStatus is a
// copy (mutations do not affect the original).
func TestGetVanityStatus_ReturnsACopy(t *testing.T) {
	s := newTestService(t)
	s.vanityStatus = &VanityStatus{
		Active:   true,
		Prefix:   "abc",
		Attempts: 42,
	}
	got := s.GetVanityStatus()
	if got == nil {
		t.Fatal("GetVanityStatus: expected non-nil, got nil")
	}
	if got.Prefix != "abc" {
		t.Errorf("GetVanityStatus Prefix = %q, want %q", got.Prefix, "abc")
	}
	if got.Attempts != 42 {
		t.Errorf("GetVanityStatus Attempts = %d, want 42", got.Attempts)
	}
	// Mutate the copy — the original should be unaffected.
	got.Prefix = "mutated"
	if s.vanityStatus.Prefix != "abc" {
		t.Error("GetVanityStatus returned a reference, not a copy")
	}
}

// ── GenerateVanityAddress — cancel existing generation ───────────────────────

// ── GetInfo additional branches ───────────────────────────────────────────────

// TestGetInfo_OutboundConfiguredNotActive covers the "Configured but not active"
// note that appears when UseNetwork=true but dialer is nil.
func TestGetInfo_OutboundConfiguredNotActive(t *testing.T) {
	s := newTestService(t)
	cfg := &config.TorConfig{UseNetwork: true}
	s.torConfig = cfg
	// s.dialer remains nil — outboundConfigured=true, outboundActive=false

	info := s.GetInfo()
	outbound, ok := info["outbound"].(map[string]interface{})
	if !ok {
		t.Fatal("GetInfo: 'outbound' is not a map")
	}
	note, ok := outbound["note"].(string)
	if !ok {
		t.Error("GetInfo: outbound 'note' should be a string when configured but inactive")
	}
	if note != "Configured but not active (Tor not running)" {
		t.Errorf("GetInfo outbound note = %q, unexpected", note)
	}
}

// TestGetInfo_VanityGenerationActive covers the vanity_generation block that is
// included in GetInfo when vanityStatus.Active is true.
func TestGetInfo_VanityGenerationActive(t *testing.T) {
	s := newTestService(t)
	s.vanityStatus = &VanityStatus{
		Active:   true,
		Prefix:   "abc",
		Attempts: 500,
	}

	info := s.GetInfo()
	vg, ok := info["vanity_generation"].(map[string]interface{})
	if !ok {
		t.Fatal("GetInfo: 'vanity_generation' key should be present and a map when active")
	}
	if active, _ := vg["active"].(bool); !active {
		t.Error("GetInfo vanity_generation.active should be true")
	}
	if prefix, _ := vg["prefix"].(string); prefix != "abc" {
		t.Errorf("GetInfo vanity_generation.prefix = %q, want %q", prefix, "abc")
	}
}

// TestGenerateVanityAddress_CancelsExistingGeneration covers the branch where
// vanityCancel is non-nil (an existing generation is in progress).
func TestGenerateVanityAddress_CancelsExistingGeneration(t *testing.T) {
	s := newTestService(t)
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}

	// First call: starts background generation (low-probability prefix so it
	// does not finish before the second call).
	if err := s.GenerateVanityAddress("zzzzz"); err != nil {
		t.Fatalf("GenerateVanityAddress first call: %v", err)
	}

	// Second call: must cancel the first and start a new generation.
	if err := s.GenerateVanityAddress("yyyyy"); err != nil {
		t.Fatalf("GenerateVanityAddress second call: %v", err)
	}

	// Clean up: cancel the background goroutine started by the second call.
	s.mu.Lock()
	if s.vanityCancel != nil {
		s.vanityCancel()
	}
	s.mu.Unlock()
}
