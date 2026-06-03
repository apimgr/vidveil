// SPDX-License-Identifier: MIT
// Coverage tests for maintenance package functions not covered by maintenance_test.go:
// compareVersions, formatBytes, SetUpdateBranch, GetUpdateBranch, ListBackups,
// applyRetention, applyRetentionWithOptions, Backup, BackupIncremental,
// BackupWithOptions, verifyBackup, RestoreWithPassword, Restore, addDirToTar.
package maintenance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newManagerWithDirs creates a MaintenanceManager whose Config/Data/Backup
// all live under temp directories so no test writes to real system paths.
func newManagerWithDirs(t *testing.T) (*MaintenanceManager, string, string) {
	t.Helper()
	configDir := t.TempDir()
	dataDir := t.TempDir()
	m := NewMaintenanceManager(configDir, dataDir, "1.0.0")
	return m, configDir, dataDir
}

// ── compareVersions ───────────────────────────────────────────────────────────

func TestCompareVersions_Equal(t *testing.T) {
	if got := compareVersions("1.2.3", "1.2.3"); got != 0 {
		t.Errorf("compareVersions equal = %d, want 0", got)
	}
}

func TestCompareVersions_AGreater(t *testing.T) {
	if got := compareVersions("2.0.0", "1.9.9"); got != 1 {
		t.Errorf("compareVersions a>b = %d, want 1", got)
	}
}

func TestCompareVersions_BGreater(t *testing.T) {
	if got := compareVersions("1.0.0", "2.0.0"); got != -1 {
		t.Errorf("compareVersions a<b = %d, want -1", got)
	}
}

func TestCompareVersions_MinorDiff(t *testing.T) {
	if got := compareVersions("1.2.0", "1.3.0"); got != -1 {
		t.Errorf("compareVersions minor a<b = %d, want -1", got)
	}
}

func TestCompareVersions_PatchDiff(t *testing.T) {
	if got := compareVersions("1.0.5", "1.0.4"); got != 1 {
		t.Errorf("compareVersions patch a>b = %d, want 1", got)
	}
}

// ── formatBytes ──────────────────────────────────────────────────────────────

func TestFormatBytes_Bytes(t *testing.T) {
	got := formatBytes(512)
	if got != "512 B" {
		t.Errorf("formatBytes(512) = %q, want %q", got, "512 B")
	}
}

func TestFormatBytes_Kilobytes(t *testing.T) {
	got := formatBytes(2048)
	if got != "2.0 KB" {
		t.Errorf("formatBytes(2048) = %q, want %q", got, "2.0 KB")
	}
}

func TestFormatBytes_Megabytes(t *testing.T) {
	got := formatBytes(1024 * 1024)
	if got != "1.0 MB" {
		t.Errorf("formatBytes(1MB) = %q, want %q", got, "1.0 MB")
	}
}

func TestFormatBytes_Gigabytes(t *testing.T) {
	got := formatBytes(1024 * 1024 * 1024)
	if got != "1.0 GB" {
		t.Errorf("formatBytes(1GB) = %q, want %q", got, "1.0 GB")
	}
}

// ── SetUpdateBranch / GetUpdateBranch ─────────────────────────────────────────

func TestSetUpdateBranch_StableWritesFile(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.SetUpdateBranch("stable"); err != nil {
		t.Fatalf("SetUpdateBranch stable: %v", err)
	}
	if got := m.GetUpdateBranch(); got != "stable" {
		t.Errorf("GetUpdateBranch = %q, want %q", got, "stable")
	}
}

func TestSetUpdateBranch_Beta(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.SetUpdateBranch("beta"); err != nil {
		t.Fatalf("SetUpdateBranch beta: %v", err)
	}
	if got := m.GetUpdateBranch(); got != "beta" {
		t.Errorf("GetUpdateBranch = %q, want %q", got, "beta")
	}
}

func TestSetUpdateBranch_Daily(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.SetUpdateBranch("daily"); err != nil {
		t.Fatalf("SetUpdateBranch daily: %v", err)
	}
	if got := m.GetUpdateBranch(); got != "daily" {
		t.Errorf("GetUpdateBranch = %q, want %q", got, "daily")
	}
}

func TestSetUpdateBranch_InvalidReturnsError(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.SetUpdateBranch("nightly"); err == nil {
		t.Error("SetUpdateBranch invalid: expected error, got nil")
	}
}

func TestGetUpdateBranch_DefaultsToStable(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if got := m.GetUpdateBranch(); got != "stable" {
		t.Errorf("GetUpdateBranch default = %q, want %q", got, "stable")
	}
}

// ── ListBackups ───────────────────────────────────────────────────────────────

func TestListBackups_EmptyDir(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	backups, err := m.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups empty: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("ListBackups empty: count = %d, want 0", len(backups))
	}
}

func TestListBackups_WithTarGzFiles(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	f1 := filepath.Join(backupDir, "vidveil_backup_2026-01-01_120000.tar.gz")
	f2 := filepath.Join(backupDir, "vidveil_backup_2026-01-02_120000.tar.gz")
	os.WriteFile(f1, []byte("fake"), 0644)
	time.Sleep(2 * time.Millisecond)
	os.WriteFile(f2, []byte("fake2"), 0644)

	backups, err := m.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups with files: %v", err)
	}
	if len(backups) != 2 {
		t.Errorf("ListBackups with files: count = %d, want 2", len(backups))
	}
}

func TestListBackups_FiltersNonBackupFiles(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	os.WriteFile(filepath.Join(backupDir, "readme.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(backupDir, "vidveil_backup_2026-01-01_120000.tar.gz"), []byte("b"), 0644)

	backups, err := m.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups filter: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("ListBackups filter: count = %d, want 1 (.tar.gz only)", len(backups))
	}
}

func TestListBackups_IncludesEncFile(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	os.WriteFile(filepath.Join(backupDir, "vidveil_backup_2026-01-01_120000.tar.gz.enc"), []byte("enc"), 0644)

	backups, err := m.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups enc: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("ListBackups enc: count = %d, want 1", len(backups))
	}
}

// ── applyRetention / applyRetentionWithOptions ───────────────────────────────

func TestApplyRetention_KeepOne(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	for i := 0; i < 4; i++ {
		name := filepath.Join(backupDir, "vidveil_backup_2026-01-0"+string(rune('1'+i))+"_120000.tar.gz")
		os.WriteFile(name, []byte("data"), 0644)
		time.Sleep(2 * time.Millisecond)
	}

	if err := m.applyRetention(1); err != nil {
		t.Fatalf("applyRetention: %v", err)
	}

	backups, _ := m.ListBackups()
	if len(backups) > 1 {
		t.Errorf("applyRetention keepCount=1: %d backups remain, want ≤1", len(backups))
	}
}

func TestApplyRetentionWithOptions_KeepTwo(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	for i := 0; i < 5; i++ {
		name := filepath.Join(backupDir, "vidveil_backup_2026-01-0"+string(rune('1'+i))+"_120000.tar.gz")
		os.WriteFile(name, []byte("data"), 0644)
		time.Sleep(2 * time.Millisecond)
	}

	if err := m.applyRetentionWithOptions(2, 0, 0, 0); err != nil {
		t.Fatalf("applyRetentionWithOptions: %v", err)
	}

	backups, _ := m.ListBackups()
	if len(backups) > 2 {
		t.Errorf("applyRetentionWithOptions keep=2: %d backups remain, want ≤2", len(backups))
	}
}

func TestApplyRetention_EmptyDirNoPanic(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	if err := m.applyRetention(1); err != nil {
		t.Errorf("applyRetention empty dir: %v", err)
	}
}

// ── BackupWithOptions / Backup / BackupIncremental ───────────────────────────

func TestBackupWithOptions_CreatesFile(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)

	os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644)

	outFile := filepath.Join(backupDir, "test_backup.tar.gz")
	err := m.BackupWithOptions(BackupOptions{
		Filename:    outFile,
		IncludeData: true,
		MaxBackups:  1,
	})
	if err != nil {
		t.Fatalf("BackupWithOptions: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("BackupWithOptions: output file missing")
	}
}

func TestBackup_CreatesFile(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	outFile := filepath.Join(backupDir, "backup.tar.gz")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("Backup: output file missing")
	}
}

func TestBackupIncremental_CreatesFile(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	outFile := filepath.Join(backupDir, "hourly.tar.gz")
	if err := m.BackupIncremental(outFile); err != nil {
		t.Fatalf("BackupIncremental: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("BackupIncremental: output file missing")
	}
}

// ── verifyBackup ──────────────────────────────────────────────────────────────

func TestVerifyBackup_ValidBackup(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	outFile := filepath.Join(backupDir, "verify_test.tar.gz")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup for verify: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	sum := sha256.Sum256(data)
	checksum := "sha256:" + hex.EncodeToString(sum[:])

	if err := m.verifyBackup(outFile, checksum, ""); err != nil {
		t.Errorf("verifyBackup valid: %v", err)
	}
}

func TestVerifyBackup_MissingFile(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.verifyBackup("/tmp/nonexistent_backup_xyz.tar.gz", "sha256:abc", ""); err == nil {
		t.Error("verifyBackup missing: expected error, got nil")
	}
}

func TestVerifyBackup_WrongChecksum(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	outFile := filepath.Join(backupDir, "chk_test.tar.gz")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup for checksum test: %v", err)
	}

	if err := m.verifyBackup(outFile, "sha256:wrongchecksum", ""); err == nil {
		t.Error("verifyBackup wrong checksum: expected error, got nil")
	}
}

// ── RestoreWithPassword / Restore ─────────────────────────────────────────────

func TestRestoreWithPassword_RestoresFromBackup(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)

	os.WriteFile(filepath.Join(configDir, "restore_test.yml"), []byte("key: value"), 0644)

	outFile := filepath.Join(backupDir, "restore.tar.gz")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup for restore: %v", err)
	}

	if err := m.RestoreWithPassword(outFile, ""); err != nil {
		t.Errorf("RestoreWithPassword: %v", err)
	}
}

func TestRestore_WrapsRestoreWithPassword(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	outFile := filepath.Join(backupDir, "restore2.tar.gz")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup for restore: %v", err)
	}

	if err := m.Restore(outFile); err != nil {
		t.Errorf("Restore: %v", err)
	}
}

func TestRestoreWithPassword_MissingFileReturnsError(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.RestoreWithPassword("/tmp/no_such_backup_xyz.tar.gz", ""); err == nil {
		t.Error("RestoreWithPassword missing: expected error, got nil")
	}
}

func TestRestoreWithPassword_EncryptedWrongPasswordReturnsError(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)

	outFile := filepath.Join(backupDir, "enc.tar.gz.enc")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup for enc restore: %v", err)
	}

	if err := m.RestoreWithPassword(outFile, "wrongpass"); err == nil {
		t.Error("RestoreWithPassword encrypted wrong pass: expected error, got nil")
	}
}

// ── ResetAdminCredentials ─────────────────────────────────────────────────────

// TestResetAdminCredentials_WritesTokenAndFlagFiles verifies that
// ResetAdminCredentials generates a non-empty setup token, writes it to
// <data>/setup_token, and creates <data>/admin_reset.flag.
func TestResetAdminCredentials_WritesTokenAndFlagFiles(t *testing.T) {
	m, _, dataDir := newManagerWithDirs(t)

	token, err := m.ResetAdminCredentials()
	if err != nil {
		t.Fatalf("ResetAdminCredentials() error: %v", err)
	}
	if token == "" {
		t.Error("ResetAdminCredentials() returned empty token")
	}

	tokenFile := filepath.Join(dataDir, "setup_token")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("failed to read setup_token: %v", err)
	}
	if string(data) != token {
		t.Errorf("setup_token content = %q, want %q", string(data), token)
	}

	resetFlag := filepath.Join(dataDir, "admin_reset.flag")
	if _, err := os.Stat(resetFlag); os.IsNotExist(err) {
		t.Error("admin_reset.flag was not created")
	}
}

// ── SetMaintenanceMode disable-when-absent branch ─────────────────────────────

// TestSetMaintenanceMode_DisableWhenNotEnabled verifies that calling
// SetMaintenanceMode(false) when no flag file exists returns nil (not an error).
func TestSetMaintenanceMode_DisableWhenNotEnabled(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)

	if err := m.SetMaintenanceMode(false); err != nil {
		t.Fatalf("SetMaintenanceMode(false) on missing flag: got %v, want nil", err)
	}
}

// ── SetUpdateBranch invalid branch ────────────────────────────────────────────

// TestSetUpdateBranch_InvalidBranch verifies that SetUpdateBranch rejects
// unknown branch names with a non-nil error.
func TestSetUpdateBranch_InvalidBranch(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)

	if err := m.SetUpdateBranch("nightly"); err == nil {
		t.Error("SetUpdateBranch(nightly): expected error for invalid branch, got nil")
	}
}
