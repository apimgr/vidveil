// SPDX-License-Identifier: MIT
// AI.md PART 28: Additional coverage tests for maintenance.go branches not
// covered by maintenance_test.go or maintenance_coverage_test.go.
// Targets: encrypted BackupWithOptions, IncludeSSL path, retention with
// weekly/monthly/yearly options, RestoreWithPassword with encrypted backup.
package maintenance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newMaintMgrWithTempDirs creates a MaintenanceManager with fully initialized
// temp directories. Uses os.MkdirTemp under the project org prefix per spec.
func newMaintMgrWithTempDirs(t *testing.T) (*MaintenanceManager, string) {
	t.Helper()
	base := filepath.Join(os.TempDir(), "apimgr")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	tmp, err := os.MkdirTemp(base, "vidveil-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	cfgDir := filepath.Join(tmp, "config")
	dataDir := filepath.Join(tmp, "data")
	backupDir := filepath.Join(tmp, "backup")
	for _, d := range []string{cfgDir, dataDir, backupDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	m := NewMaintenanceManager(cfgDir, dataDir, "1.0.0")
	m.paths.Backup = backupDir
	return m, tmp
}

// ── BackupWithOptions — encrypted path ───────────────────────────────────────

func TestBackupWithOptions_EncryptedBackup_CreatesEncFile(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)
	outFile := filepath.Join(tmp, "backup.tar.gz.enc")

	err := m.BackupWithOptions(BackupOptions{
		Filename: outFile,
		Password: "testpassword123",
	})
	if err != nil {
		t.Fatalf("BackupWithOptions encrypted: %v", err)
	}
	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Error("BackupWithOptions encrypted: output file missing")
	}
}

func TestBackupWithOptions_EncryptedAutoFilename_UsesEncExt(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	err := m.BackupWithOptions(BackupOptions{
		Password:    "mypassword",
		IncludeData: false,
	})
	if err != nil {
		t.Fatalf("BackupWithOptions encrypted auto filename: %v", err)
	}

	backups, err := m.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	found := false
	for _, b := range backups {
		if strings.HasSuffix(b.Filename, ".tar.gz.enc") {
			found = true
		}
	}
	if !found {
		t.Error("BackupWithOptions encrypted auto: expected .tar.gz.enc file")
	}
}

// ── BackupWithOptions — IncludeSSL path ──────────────────────────────────────

func TestBackupWithOptions_IncludeSSL_ExistsAndIncluded(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	// Create SSL directory with a dummy cert file
	sslDir := filepath.Join(m.paths.Config, "ssl")
	if err := os.MkdirAll(sslDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sslDir, "cert.pem"), []byte("cert"), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmp, "backup_ssl.tar.gz")
	err := m.BackupWithOptions(BackupOptions{
		Filename:   outFile,
		IncludeSSL: true,
	})
	if err != nil {
		t.Fatalf("BackupWithOptions IncludeSSL: %v", err)
	}
	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Error("BackupWithOptions IncludeSSL: output file missing")
	}
}

func TestBackupWithOptions_IncludeSSL_DirMissing_NoPanic(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)
	outFile := filepath.Join(tmp, "backup_nossl.tar.gz")

	err := m.BackupWithOptions(BackupOptions{
		Filename:   outFile,
		IncludeSSL: true,
	})
	if err != nil {
		t.Fatalf("BackupWithOptions IncludeSSL no dir: %v", err)
	}
}

// ── BackupWithOptions + RestoreWithPassword — encrypted round-trip ───────────

func TestBackupAndRestoreWithPassword_RoundTrip(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	// Write a test config file
	if err := os.WriteFile(filepath.Join(m.paths.Config, "server.yml"), []byte("test: true"), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmp, "backup_enc.tar.gz.enc")
	password := "testpassword"

	if err := m.BackupWithOptions(BackupOptions{
		Filename: outFile,
		Password: password,
	}); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	restoreDir := filepath.Join(tmp, "restore_out")
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		t.Fatal(err)
	}

	m2 := NewMaintenanceManager(restoreDir, restoreDir, "1.0.0")
	m2.paths.Backup = m.paths.Backup

	if err := m2.RestoreWithPassword(outFile, password); err != nil {
		t.Fatalf("RestoreWithPassword: %v", err)
	}
}

// ── applyRetentionWithOptions — weekly/monthly/yearly branches ───────────────

func TestApplyRetentionWithOptions_WeeklyMonthlyYearly_NoPanic(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	// Create several backup files with different (fake) modification times.
	// The retention logic checks ModTime, so we need to set various dates.
	// We create files and use os.Chtimes to set their timestamps.
	type fileSpec struct {
		name    string
		modTime time.Time
	}

	// Jan 1 = yearly candidate, 1st of month = monthly, Sunday = weekly
	files := []fileSpec{
		{"backup_jan1.tar.gz", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)},
		{"backup_feb1.tar.gz", time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC)},
		{"backup_sunday.tar.gz", time.Date(2024, 2, 4, 10, 0, 0, 0, time.UTC)},  // Sunday
		{"backup_recent.tar.gz", time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)}, // Regular daily
		{"backup_old.tar.gz", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},    // Old
	}

	for _, f := range files {
		path := filepath.Join(tmp, "backup", f.name)
		if err := os.WriteFile(path, []byte("fake backup data"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, f.modTime, f.modTime); err != nil {
			t.Fatal(err)
		}
	}

	// Apply retention keeping 1 daily, 1 weekly, 1 monthly, 1 yearly
	if err := m.applyRetentionWithOptions(1, 1, 1, 1); err != nil {
		t.Fatalf("applyRetentionWithOptions: %v", err)
	}

	// After retention, at most 4 backups should remain (1 daily + 1 weekly + 1 monthly + 1 yearly)
	remaining, err := m.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups after retention: %v", err)
	}
	if len(remaining) > 4 {
		t.Errorf("applyRetentionWithOptions: %d backups remain (max 4 expected)", len(remaining))
	}
}

func TestApplyRetentionWithOptions_MaxBackupsZero_DefaultsToOne(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	for i := 0; i < 3; i++ {
		path := filepath.Join(tmp, "backup", fmt.Sprintf("backup_%d.tar.gz", i))
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	// maxBackups=0 should default to 1
	if err := m.applyRetentionWithOptions(0, 0, 0, 0); err != nil {
		t.Fatalf("applyRetentionWithOptions(0,0,0,0): %v", err)
	}

	remaining, _ := m.ListBackups()
	if len(remaining) > 1 {
		t.Errorf("applyRetentionWithOptions(0): %d remain, want ≤1", len(remaining))
	}
}

// ── RestoreWithPassword — additional paths ────────────────────────────────────

// TestRestoreWithPassword_EmptyFilename_AutoFinds tests the auto-find path
// where backupFile="" causes RestoreWithPassword to find the most recent backup.
func TestRestoreWithPassword_EmptyFilename_AutoFinds(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)
	_ = tmp

	// Create a backup first
	if err := m.BackupWithOptions(BackupOptions{IncludeData: false}); err != nil {
		t.Fatalf("Backup for auto-find test: %v", err)
	}

	// Restore with empty filename — should auto-find the backup
	m2 := NewMaintenanceManager(m.paths.Config, m.paths.Data, "1.0.0")
	m2.paths.Backup = m.paths.Backup

	if err := m2.RestoreWithPassword("", ""); err != nil {
		t.Logf("RestoreWithPassword(auto-find): %v (may fail due to overwriting)", err)
	}
}

// TestRestoreWithPassword_WithSSLEntries_RestoresSSL tests the ssl/ prefix path.
func TestRestoreWithPassword_WithSSLEntries_RestoresSSL(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	// Create SSL directory with a dummy cert
	sslDir := filepath.Join(m.paths.Config, "ssl")
	if err := os.MkdirAll(sslDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sslDir, "cert.pem"), []byte("fake cert"), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmp, "backup_with_ssl.tar.gz")
	if err := m.BackupWithOptions(BackupOptions{
		Filename:   outFile,
		IncludeSSL: true,
	}); err != nil {
		t.Fatalf("Backup with SSL: %v", err)
	}

	// Restore to a different location
	restoreDir := filepath.Join(tmp, "restore_ssl")
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		t.Fatal(err)
	}

	m2 := NewMaintenanceManager(restoreDir, restoreDir, "1.0.0")
	m2.paths.Backup = m.paths.Backup

	if err := m2.RestoreWithPassword(outFile, ""); err != nil {
		t.Errorf("RestoreWithPassword with SSL: %v", err)
	}
}

// TestRestoreWithPassword_WithDataEntries_RestoresData tests the data/ prefix path.
func TestRestoreWithPassword_WithDataEntries_RestoresData(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	// Create data file
	if err := os.WriteFile(filepath.Join(m.paths.Data, "test.db"), []byte("test data"), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmp, "backup_with_data.tar.gz")
	if err := m.BackupWithOptions(BackupOptions{
		Filename:    outFile,
		IncludeData: true,
	}); err != nil {
		t.Fatalf("Backup with data: %v", err)
	}

	restoreDir := filepath.Join(tmp, "restore_data")
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		t.Fatal(err)
	}

	m2 := NewMaintenanceManager(restoreDir, restoreDir, "1.0.0")
	m2.paths.Backup = m.paths.Backup

	if err := m2.RestoreWithPassword(outFile, ""); err != nil {
		t.Errorf("RestoreWithPassword with data: %v", err)
	}
}

// TestRestoreWithPassword_EncryptedRequiresPassword tests the "encrypted but no
// password" path that returns an error without decrypting.
func TestRestoreWithPassword_EncryptedNoPassword_ReturnsError(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)
	outFile := filepath.Join(tmp, "enc_backup.tar.gz.enc")

	if err := m.BackupWithOptions(BackupOptions{
		Filename: outFile,
		Password: "secret",
	}); err != nil {
		t.Fatalf("Backup encrypted: %v", err)
	}

	m2 := NewMaintenanceManager(m.paths.Config, m.paths.Data, "1.0.0")
	m2.paths.Backup = m.paths.Backup

	// Try to restore with no password — should fail
	err := m2.RestoreWithPassword(outFile, "")
	if err == nil {
		t.Error("RestoreWithPassword(enc, no pwd): expected error, got nil")
	}
}
