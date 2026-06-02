// SPDX-License-Identifier: MIT
// Coverage tests for tor service paths not covered by service_test.go.
// Targets: buildTorrc, ensureTorrc, copyFile, GetInfo, GetPublicKeyHex.
// No Tor binary or network access required — all pure-logic or filesystem.
package tor

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// ── copyFile ─────────────────────────────────────────────────────────────────

func TestCopyFile_ContentMatches(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	want := "hello tor"
	if err := os.WriteFile(src, []byte(want), 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != want {
		t.Errorf("dst content = %q, want %q", string(data), want)
	}
}

func TestCopyFile_MissingSrcReturnsError(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "nonexistent.txt"), filepath.Join(dir, "dst.txt"))
	if err == nil {
		t.Error("copyFile with missing src: expected error, got nil")
	}
}

func TestCopyFile_EmptyFileSucceeds(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "empty.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte{}, 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Errorf("copyFile empty: %v", err)
	}
}

// ── buildTorrc ────────────────────────────────────────────────────────────────

func TestBuildTorrc_NilConfigDefaultsAccepted(t *testing.T) {
	got := buildTorrc(nil)
	if got == "" {
		t.Error("buildTorrc(nil): expected non-empty string")
	}
}

func TestBuildTorrc_UseNetworkEnablesSocks(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.UseNetwork = true
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "SocksPort auto") {
		t.Errorf("buildTorrc with UseNetwork=true: expected SocksPort auto, got:\n%s", got)
	}
}

func TestBuildTorrc_NoNetworkDisablesSocks(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.UseNetwork = false
	cfg.AllowUserPreference = false
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "SocksPort 0") {
		t.Errorf("buildTorrc with UseNetwork=false: expected SocksPort 0, got:\n%s", got)
	}
}

func TestBuildTorrc_AllowUserPreferenceEnablesSocks(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.UseNetwork = false
	cfg.AllowUserPreference = true
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "SocksPort auto") {
		t.Errorf("buildTorrc with AllowUserPreference=true: expected SocksPort auto, got:\n%s", got)
	}
}

func TestBuildTorrc_SafeLoggingTrue(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.SafeLogging = true
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "SafeLogging 1") {
		t.Errorf("buildTorrc SafeLogging=true: expected SafeLogging 1, got:\n%s", got)
	}
}

func TestBuildTorrc_SafeLoggingFalse(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.SafeLogging = false
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "SafeLogging 0") {
		t.Errorf("buildTorrc SafeLogging=false: expected SafeLogging 0, got:\n%s", got)
	}
}

func TestBuildTorrc_BandwidthLimitIncluded(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.MaxMonthlyBandwidth = "10 GB"
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "AccountingMax 10 GB") {
		t.Errorf("buildTorrc with bandwidth limit: expected AccountingMax 10 GB, got:\n%s", got)
	}
}

func TestBuildTorrc_UnlimitedBandwidthOmitsAccounting(t *testing.T) {
	cfg := config.DefaultTorConfig()
	cfg.MaxMonthlyBandwidth = "unlimited"
	got := buildTorrc(&cfg)
	if strings.Contains(got, "AccountingMax") {
		t.Errorf("buildTorrc with unlimited bandwidth: unexpected AccountingMax, got:\n%s", got)
	}
}

func TestBuildTorrc_NoRelayOrExit(t *testing.T) {
	cfg := config.DefaultTorConfig()
	got := buildTorrc(&cfg)
	if !strings.Contains(got, "ExitRelay 0") {
		t.Errorf("buildTorrc: expected ExitRelay 0, got:\n%s", got)
	}
	if !strings.Contains(got, "ORPort 0") {
		t.Errorf("buildTorrc: expected ORPort 0, got:\n%s", got)
	}
}

// ── ensureTorrc ───────────────────────────────────────────────────────────────

func TestEnsureTorrc_CreatesFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "torrc")
	created, err := ensureTorrc(path, "# test torrc\n")
	if err != nil {
		t.Fatalf("ensureTorrc: %v", err)
	}
	if !created {
		t.Error("ensureTorrc: expected created=true for new file")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("ensureTorrc: file was not created on disk")
	}
}

func TestEnsureTorrc_ContentWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "torrc")
	content := "# test content\nSocksPort 0\n"
	if _, err := ensureTorrc(path, content); err != nil {
		t.Fatalf("ensureTorrc: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("ensureTorrc content = %q, want %q", string(data), content)
	}
}

func TestEnsureTorrc_ExistingFileNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "torrc")
	original := "# original\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatalf("pre-write: %v", err)
	}
	created, err := ensureTorrc(path, "# new content\n")
	if err != nil {
		t.Fatalf("ensureTorrc: %v", err)
	}
	if created {
		t.Error("ensureTorrc: expected created=false for existing file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Errorf("ensureTorrc overwrote existing file: got %q, want %q", string(data), original)
	}
}

func TestEnsureTorrc_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "deeper", "torrc")
	if _, err := ensureTorrc(path, "# test\n"); err != nil {
		t.Fatalf("ensureTorrc with nested dirs: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("ensureTorrc: file not created in nested dir")
	}
}

// ── GetPublicKeyHex ───────────────────────────────────────────────────────────

func TestGetPublicKeyHex_EmptyWhenNoKey(t *testing.T) {
	s := newTestService(t)
	got := s.GetPublicKeyHex()
	if got != "" {
		t.Errorf("GetPublicKeyHex with no key = %q, want empty string", got)
	}
}

func TestGetPublicKeyHex_EncodesPublicKey(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	s := newTestService(t)
	s.publicKey = pub
	got := s.GetPublicKeyHex()
	want := hex.EncodeToString(pub)
	if got != want {
		t.Errorf("GetPublicKeyHex = %q, want %q", got, want)
	}
}

// ── GetInfo ───────────────────────────────────────────────────────────────────

func TestGetInfo_DisabledStatusReturnsMap(t *testing.T) {
	s := newTestService(t)
	info := s.GetInfo()
	if info == nil {
		t.Fatal("GetInfo returned nil")
	}
}

func TestGetInfo_StatusKeyPresent(t *testing.T) {
	s := newTestService(t)
	info := s.GetInfo()
	if _, ok := info["status"]; !ok {
		t.Error("GetInfo: missing 'status' key")
	}
}

func TestGetInfo_EnabledKeyPresent(t *testing.T) {
	s := newTestService(t)
	info := s.GetInfo()
	if _, ok := info["enabled"]; !ok {
		t.Error("GetInfo: missing 'enabled' key")
	}
}

func TestGetInfo_OutboundKeyPresent(t *testing.T) {
	s := newTestService(t)
	info := s.GetInfo()
	if _, ok := info["outbound"]; !ok {
		t.Error("GetInfo: missing 'outbound' key")
	}
}

func TestGetInfo_DisabledEnabledFalse(t *testing.T) {
	s := newTestService(t)
	info := s.GetInfo()
	enabled, ok := info["enabled"].(bool)
	if !ok {
		t.Fatal("GetInfo: 'enabled' is not bool")
	}
	if enabled {
		t.Error("GetInfo: disabled service should have enabled=false")
	}
}

func TestGetInfo_ConnectedEnabledTrue(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusConnected
	s.startTime = s.startTime.Add(0)
	info := s.GetInfo()
	enabled, ok := info["enabled"].(bool)
	if !ok {
		t.Fatal("GetInfo: 'enabled' is not bool")
	}
	if !enabled {
		t.Error("GetInfo: connected service should have enabled=true")
	}
}

func TestGetInfo_NoTorBinaryEnabledTrue(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusNoTorBinary
	info := s.GetInfo()
	enabled, ok := info["enabled"].(bool)
	if !ok {
		t.Fatal("GetInfo: 'enabled' is not bool")
	}
	if !enabled {
		t.Error("GetInfo: NoTorBinary status should have enabled=true")
	}
}

func TestGetInfo_OutboundNotActiveWhenDialerNil(t *testing.T) {
	s := newTestService(t)
	info := s.GetInfo()
	outbound, ok := info["outbound"].(map[string]interface{})
	if !ok {
		t.Fatal("GetInfo: 'outbound' is not map")
	}
	active, ok := outbound["active"].(bool)
	if !ok {
		t.Fatal("GetInfo: outbound 'active' is not bool")
	}
	if active {
		t.Error("GetInfo: outbound should not be active when dialer is nil")
	}
}
