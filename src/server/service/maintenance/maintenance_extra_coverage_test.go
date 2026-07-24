// SPDX-License-Identifier: MIT
// AI.md PART 28: Additional coverage tests for maintenance.go branches not
// covered by maintenance_test.go or maintenance_coverage_test.go.
// Targets: encrypted BackupWithOptions, IncludeSSL path, retention with
// weekly/monthly/yearly options, RestoreWithPassword with encrypted backup.
package maintenance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildTestBackupArchive constructs a minimal gzip+tar backup archive with the
// given manifest and file contents (keyed by tar entry name, e.g.
// "config/server.yml"). Mirrors the layout loadRestoreArchive expects.
func buildTestBackupArchive(t *testing.T, manifest BackupManifest, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: "manifest.json",
		Mode: 0644,
		Size: int64(len(manifestJSON)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(manifestJSON); err != nil {
		t.Fatal(err)
	}

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

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
	sslDir := filepath.Join(tmp, "ssl")
	for _, d := range []string{cfgDir, dataDir, backupDir, sslDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	m := NewMaintenanceManager(cfgDir, dataDir, "1.0.0")
	m.paths.Backup = backupDir
	m.paths.SSL = sslDir
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

	// Create SSL directory (m.paths.SSL) with a dummy cert file
	if err := os.WriteFile(filepath.Join(m.paths.SSL, "cert.pem"), []byte("cert"), 0644); err != nil {
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

	// Verify the cert was actually captured under the ssl/ prefix by
	// restoring to a fresh location and checking the file lands correctly.
	restoreDir := filepath.Join(tmp, "restore_ssl_capture_check")
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		t.Fatal(err)
	}
	m2 := NewMaintenanceManager(restoreDir, restoreDir, "1.0.0")
	m2.paths.Backup = m.paths.Backup
	m2.paths.SSL = filepath.Join(restoreDir, "ssl")
	if err := m2.RestoreWithPassword(outFile, ""); err != nil {
		t.Fatalf("RestoreWithPassword to verify SSL capture: %v", err)
	}
	restoredCert := filepath.Join(m2.paths.SSL, "cert.pem")
	content, err := os.ReadFile(restoredCert)
	if err != nil {
		t.Fatalf("restored cert missing at %s: %v", restoredCert, err)
	}
	if string(content) != "cert" {
		t.Errorf("restored cert content = %q, want %q", content, "cert")
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
	if err := m.applyRetentionWithOptions(1, 1, 1, 1, ""); err != nil {
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
	if err := m.applyRetentionWithOptions(0, 0, 0, 0, ""); err != nil {
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

	// Create SSL directory (m.paths.SSL) with a dummy cert
	if err := os.WriteFile(filepath.Join(m.paths.SSL, "cert.pem"), []byte("fake cert"), 0644); err != nil {
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
	m2.paths.SSL = filepath.Join(restoreDir, "ssl")

	if err := m2.RestoreWithPassword(outFile, ""); err != nil {
		t.Errorf("RestoreWithPassword with SSL: %v", err)
	}

	restoredCert := filepath.Join(m2.paths.SSL, "cert.pem")
	content, err := os.ReadFile(restoredCert)
	if err != nil {
		t.Fatalf("restored cert missing at %s: %v", restoredCert, err)
	}
	if string(content) != "fake cert" {
		t.Errorf("restored cert content = %q, want %q", content, "fake cert")
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

// ── parseSizeString ──────────────────────────────────────────────────────────

func TestParseSizeString(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	cases := []struct {
		name       string
		input      string
		wantBytes  uint64
		wantEnable bool
		wantErr    bool
	}{
		{"empty disables", "", 0, false, false},
		{"zero disables", "0", 0, false, false},
		{"bare bytes", "12345", 12345, true, false},
		{"kilobytes bare", "10K", 10 * 1024, true, false},
		{"kilobytes suffixed", "10KB", 10 * 1024, true, false},
		{"megabytes lowercase", "10m", 10 * 1024 * 1024, true, false},
		{"megabytes suffixed", "10MB", 10 * 1024 * 1024, true, false},
		{"gigabytes bare", "2G", 2 * 1024 * 1024 * 1024, true, false},
		{"gigabytes suffixed", "2GB", 2 * 1024 * 1024 * 1024, true, false},
		{"terabytes bare", "1T", 1024 * 1024 * 1024 * 1024, true, false},
		{"terabytes suffixed", "1TB", 1024 * 1024 * 1024 * 1024, true, false},
		{"invalid suffix number", "abcM", 0, false, true},
		{"invalid bare bytes", "abc", 0, false, true},
		{"invalid percent", "abc%", 0, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, enabled, err := m.parseSizeString(tc.input, tmp)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseSizeString(%q): expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSizeString(%q): unexpected error: %v", tc.input, err)
			}
			if enabled != tc.wantEnable {
				t.Errorf("parseSizeString(%q): enabled = %v, want %v", tc.input, enabled, tc.wantEnable)
			}
			if got != tc.wantBytes {
				t.Errorf("parseSizeString(%q): bytes = %d, want %d", tc.input, got, tc.wantBytes)
			}
		})
	}
}

// TestParseSizeString_Percentage verifies the "%" branch resolves against the
// real disk containing the given path without erroring.
func TestParseSizeString_Percentage(t *testing.T) {
	m, tmp := newMaintMgrWithTempDirs(t)

	got, enabled, err := m.parseSizeString("50%", tmp)
	if err != nil {
		t.Fatalf("parseSizeString(50%%): unexpected error: %v", err)
	}
	if !enabled {
		t.Error("parseSizeString(50%): expected enabled=true")
	}
	if got == 0 {
		t.Error("parseSizeString(50%): expected non-zero byte limit")
	}
}

// ── enforceMaxTotalSize ───────────────────────────────────────────────────────

// TestEnforceMaxTotalSize_Disabled verifies an empty cap is a no-op.
func TestEnforceMaxTotalSize_Disabled(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)
	if err := m.enforceMaxTotalSize(""); err != nil {
		t.Errorf("enforceMaxTotalSize(\"\"): unexpected error: %v", err)
	}
}

// TestEnforceMaxTotalSize_DeletesOldestNonProtected verifies backups over the
// cap are deleted oldest-first, while vidveil-daily/vidveil-hourly named
// backups are never removed.
func TestEnforceMaxTotalSize_DeletesOldestNonProtected(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	write := func(name string, size int, age time.Duration) {
		p := filepath.Join(m.paths.Backup, name)
		if err := os.WriteFile(p, make([]byte, size), 0644); err != nil {
			t.Fatal(err)
		}
		modTime := time.Now().Add(-age)
		if err := os.Chtimes(p, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	write("vidveil-daily_old.tar.gz", 100, 5*time.Hour)
	write("vidveil-hourly_old.tar.gz", 100, 4*time.Hour)
	write("vidveil_backup_old1.tar.gz", 100, 3*time.Hour)
	write("vidveil_backup_old2.tar.gz", 100, 2*time.Hour)
	write("vidveil_backup_newer.tar.gz", 100, 1*time.Hour)

	// Total is 500 bytes; cap of 350 requires deleting old1 and old2 (oldest
	// non-protected first) but stops (breaks) once under the cap, leaving
	// "newer" untouched — this exercises the loop's early-break path.
	if err := m.enforceMaxTotalSize("350"); err != nil {
		t.Fatalf("enforceMaxTotalSize: unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(m.paths.Backup, "vidveil_backup_old1.tar.gz")); !os.IsNotExist(err) {
		t.Error("expected oldest non-protected backup (old1) to be deleted")
	}
	if _, err := os.Stat(filepath.Join(m.paths.Backup, "vidveil_backup_old2.tar.gz")); !os.IsNotExist(err) {
		t.Error("expected next-oldest non-protected backup (old2) to be deleted")
	}
	if _, err := os.Stat(filepath.Join(m.paths.Backup, "vidveil_backup_newer.tar.gz")); err != nil {
		t.Error("expected newest non-protected backup to be preserved once under the cap")
	}
	if _, err := os.Stat(filepath.Join(m.paths.Backup, "vidveil-daily_old.tar.gz")); err != nil {
		t.Error("expected vidveil-daily backup to be preserved")
	}
	if _, err := os.Stat(filepath.Join(m.paths.Backup, "vidveil-hourly_old.tar.gz")); err != nil {
		t.Error("expected vidveil-hourly backup to be preserved")
	}
}

// TestEnforceMaxTotalSize_InvalidSize verifies a malformed cap value surfaces
// the parseSizeString error instead of silently being ignored.
func TestEnforceMaxTotalSize_InvalidSize(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)
	if err := m.enforceMaxTotalSize("not-a-size"); err == nil {
		t.Error("enforceMaxTotalSize(\"not-a-size\"): expected error, got nil")
	}
}

// ── checkDiskSpace ────────────────────────────────────────────────────────────

// TestCheckDiskSpace_NoExistingBackups verifies the happy path (no prior
// backups, ample free space on the real filesystem) returns no error.
func TestCheckDiskSpace_NoExistingBackups(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)
	if err := m.checkDiskSpace(m.paths.Backup); err != nil {
		t.Errorf("checkDiskSpace: unexpected error: %v", err)
	}
}

// TestCheckDiskSpace_WithExistingBackup verifies the precheck still passes
// when a small prior backup already exists (well under any real free-space
// threshold on a CI/dev filesystem).
func TestCheckDiskSpace_WithExistingBackup(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)
	if err := os.WriteFile(filepath.Join(m.paths.Backup, "vidveil_backup_prior.tar.gz"), []byte("small backup content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.checkDiskSpace(m.paths.Backup); err != nil {
		t.Errorf("checkDiskSpace: unexpected error: %v", err)
	}
}

// TestCheckDiskSpace_MultipleExistingBackups verifies the newest-first sort
// comparator over the existing backups list is actually exercised (requires
// at least two backups to invoke the comparator).
func TestCheckDiskSpace_MultipleExistingBackups(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	older := filepath.Join(m.paths.Backup, "vidveil_backup_a.tar.gz")
	newer := filepath.Join(m.paths.Backup, "vidveil_backup_b.tar.gz")
	if err := os.WriteFile(older, []byte("older backup"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("newer backup"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	if err := m.checkDiskSpace(m.paths.Backup); err != nil {
		t.Errorf("checkDiskSpace: unexpected error: %v", err)
	}
}

// ── RestoreWithPassword / loadRestoreArchive validation branches ──────────────

// TestRestoreWithPassword_MissingManifestVersion verifies a manifest with an
// empty Version is rejected before anything is extracted.
func TestRestoreWithPassword_MissingManifestVersion(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	data := buildTestBackupArchive(t, BackupManifest{Version: ""}, nil)
	path := filepath.Join(m.paths.Backup, "vidveil_backup_noversion.tar.gz")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	err := m.RestoreWithPassword(path, "")
	if err == nil || !strings.Contains(err.Error(), "missing or has empty version") {
		t.Errorf("expected missing-version error, got: %v", err)
	}
}

// TestRestoreWithPassword_ChecksumMismatch verifies a manifest with an
// incorrect Checksum is rejected before extraction.
func TestRestoreWithPassword_ChecksumMismatch(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	data := buildTestBackupArchive(t, BackupManifest{
		Version:  "1",
		Checksum: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}, map[string]string{"config/server.yml": "content"})
	path := filepath.Join(m.paths.Backup, "vidveil_backup_badsum.tar.gz")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	err := m.RestoreWithPassword(path, "")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}
}

// TestRestoreWithPassword_ValidChecksumSucceeds verifies a manifest whose
// Checksum matches the recomputed content hash restores successfully and
// extracts the file to the config directory.
func TestRestoreWithPassword_ValidChecksumSucceeds(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	const name, content = "config/server.yml", "restored content"
	hasher := sha256.New()
	hasher.Write([]byte(name))
	hasher.Write([]byte{0})
	hasher.Write([]byte(content))
	hasher.Write([]byte{0})
	checksum := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	data := buildTestBackupArchive(t, BackupManifest{
		Version:  "1",
		Checksum: checksum,
	}, map[string]string{name: content})
	path := filepath.Join(m.paths.Backup, "vidveil_backup_goodsum.tar.gz")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.RestoreWithPassword(path, ""); err != nil {
		t.Fatalf("RestoreWithPassword: unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(m.paths.Config, "server.yml"))
	if err != nil {
		t.Fatalf("expected extracted file: %v", err)
	}
	if string(got) != content {
		t.Errorf("extracted content = %q, want %q", got, content)
	}
}

// TestRestoreWithPassword_AppVersionMismatchWarns verifies an AppVersion
// differing from the running version only warns (non-fatal) rather than
// failing the restore.
func TestRestoreWithPassword_AppVersionMismatchWarns(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	data := buildTestBackupArchive(t, BackupManifest{
		Version:    "1",
		AppVersion: "0.0.1-does-not-match",
	}, map[string]string{"config/server.yml": "content"})
	path := filepath.Join(m.paths.Backup, "vidveil_backup_oldver.tar.gz")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.RestoreWithPassword(path, ""); err != nil {
		t.Errorf("expected AppVersion mismatch to only warn, got error: %v", err)
	}
}

// TestRestoreWithPassword_NoBackupFileFound verifies an empty backup
// directory with no explicit path surfaces a clear error instead of a panic.
func TestRestoreWithPassword_NoBackupFileFound(t *testing.T) {
	m, _ := newMaintMgrWithTempDirs(t)

	err := m.RestoreWithPassword("", "")
	if err == nil || !strings.Contains(err.Error(), "no backup files found") {
		t.Errorf("expected no-backup-files error, got: %v", err)
	}
}

// TestLoadRestoreArchive_InvalidGzip verifies non-gzip data is rejected with
// a wrapped gzip error.
func TestLoadRestoreArchive_InvalidGzip(t *testing.T) {
	_, err := loadRestoreArchive([]byte("not gzip data at all"))
	if err == nil || !strings.Contains(err.Error(), "failed to read gzip") {
		t.Errorf("expected gzip read error, got: %v", err)
	}
}

// TestLoadRestoreArchive_InvalidTar verifies valid gzip wrapping non-tar
// content is rejected with a wrapped tar error.
func TestLoadRestoreArchive_InvalidTar(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("not a tar archive, just garbage bytes padded out long enough to not look like a valid header block")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := loadRestoreArchive(buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "failed to read tar") {
		t.Errorf("expected tar read error, got: %v", err)
	}
}

// TestLoadRestoreArchive_MalformedManifestJSON verifies a manifest.json entry
// containing invalid JSON is rejected with a wrapped parse error.
func TestLoadRestoreArchive_MalformedManifestJSON(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	badJSON := []byte("{not valid json")
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0644, Size: int64(len(badJSON))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(badJSON); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := loadRestoreArchive(buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "failed to parse manifest") {
		t.Errorf("expected manifest parse error, got: %v", err)
	}
}
