// SPDX-License-Identifier: MIT
// Coverage tests for maintenance package functions not covered by maintenance_test.go:
// compareVersions, formatBytes, SetUpdateBranch, GetUpdateBranch, ListBackups,
// applyRetention, applyRetentionWithOptions, Backup, BackupIncremental,
// BackupWithOptions, verifyBackup, RestoreWithPassword, Restore, addDirToTar.
package maintenance

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/service/pgp"
)

// newManagerWithDirs creates a MaintenanceManager whose Config/Data/Backup
// all live under temp directories so no test writes to real system paths.
func newManagerWithDirs(t *testing.T) (*MaintenanceManager, string, string) {
	t.Helper()
	configDir := t.TempDir()
	dataDir := t.TempDir()
	m := NewMaintenanceManager(configDir, dataDir, "1.0.0")
	attachTestApplicationSecrets(t, m)
	setMaintenanceTestDiskSpace(m)
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

func TestSetUpdateBranch_AliasesNormalizeToDevel(t *testing.T) {
	for _, alias := range []string{"devel", "daily", "rolling", "dev", "development", "DeVeLoPmEnT"} {
		t.Run(alias, func(t *testing.T) {
			m, _, _ := newManagerWithDirs(t)
			if err := m.SetUpdateBranch(alias); err != nil {
				t.Fatalf("SetUpdateBranch(%q): %v", alias, err)
			}
			if got := m.GetUpdateBranch(); got != "devel" {
				t.Errorf("GetUpdateBranch = %q, want devel", got)
			}
		})
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
	if err := os.WriteFile(f1, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := os.WriteFile(f2, []byte("fake2"), 0644); err != nil {
		t.Fatal(err)
	}

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

	if err := os.WriteFile(filepath.Join(backupDir, "readme.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "vidveil_backup_2026-01-01_120000.tar.gz"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

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

	if err := os.WriteFile(filepath.Join(backupDir, "vidveil_backup_2026-01-01_120000.tar.gz.enc"), []byte("enc"), 0644); err != nil {
		t.Fatal(err)
	}

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
		if err := os.WriteFile(name, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
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
		if err := os.WriteFile(name, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	if err := m.applyRetentionWithOptions(2, 0, 0, 0, ""); err != nil {
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

	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}

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

func TestBackupAlwaysSnapshotsSQLiteDatabase(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, dataDir := newManagerWithDirs(t)
	databaseDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(databaseDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := m.appDB.Write(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec("CREATE TABLE snapshot_test (value TEXT)"); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO snapshot_test(value) VALUES ('captured')")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	backupFile := filepath.Join(backupDir, "database.tar.gz")
	if err := m.BackupWithOptions(BackupOptions{Filename: backupFile, IncludeData: false}); err != nil {
		t.Fatalf("backup without optional data: %v", err)
	}
	archiveData, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := loadRestoreArchive(archiveData)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.files {
		if file.name == "data/db/server.db" {
			if err := verifySQLiteIntegrity(file.content); err != nil {
				t.Fatalf("snapshot database: %v", err)
			}
			return
		}
	}
	t.Fatal("backup omitted the SQLite database when optional data was disabled")
}

func TestTestBackupVerifiesArchivedPGPKeypair(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)
	secret := []byte("test installation secret with enough entropy")
	keypair, err := pgp.GenerateKeypair("VidVeil Security", "security@example.test", 0)
	if err != nil {
		t.Fatal(err)
	}
	encryptedPrivate, err := pgp.EncryptPrivateKey(keypair.PrivateArmored, secret)
	if err != nil {
		t.Fatal(err)
	}
	securityDir := filepath.Join(configDir, "security")
	if err := os.MkdirAll(securityDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(securityDir, pgp.PublicKeyFile), keypair.PublicArmored, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(securityDir, pgp.PrivateKeyFile), encryptedPrivate, 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.appDB.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE app_secrets SET value = ? WHERE key = ?", base64.StdEncoding.EncodeToString(secret), "installation_secret")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	backupFile := filepath.Join(backupDir, "operator-test.tar.gz")
	if err := m.BackupWithOptions(BackupOptions{Filename: backupFile}); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	result, err := m.TestBackup(backupFile, "")
	if err != nil {
		t.Fatalf("test backup: %v", err)
	}
	if !result.ChecksumVerified || !result.ManifestVerified || !result.DatabaseVerified || !result.PGPKeypairVerified {
		t.Fatalf("backup test checks incomplete: %+v", result)
	}
}

func TestBackupIncremental_CreatesFile(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	if err := m.BackupWithOptions(BackupOptions{Filename: filepath.Join(backupDir, "vidveil_backup_2026-01-01.tar.gz")}); err != nil {
		t.Fatalf("create full backup base: %v", err)
	}

	outFile := filepath.Join(backupDir, "hourly.tar.gz")
	if err := m.BackupIncremental(outFile); err != nil {
		t.Fatalf("BackupIncremental: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("BackupIncremental: output file missing")
	}
}

func TestDefaultBackupTestFilesSelectsLatestFullAndIncrementals(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	files := map[string]time.Time{
		"vidveil_backup_2026-01-02.tar.gz":        time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		"vidveil_backup_2026-01-03.tar.gz.enc":    time.Date(2026, 1, 3, 8, 0, 0, 0, time.UTC),
		"vidveil_backup_2026-01-03_120000.tar.gz": time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC),
		"vidveil-daily.tar.gz.enc":                time.Date(2026, 1, 3, 13, 0, 0, 0, time.UTC),
		"vidveil-hourly.tar.gz":                   time.Date(2026, 1, 3, 14, 0, 0, 0, time.UTC),
		"vidveil_backup_2026-01-04.tar.gz":        time.Date(2026, 1, 4, 8, 0, 0, 0, time.UTC),
	}
	for name, modified := range files {
		file := filepath.Join(backupDir, name)
		if err := os.WriteFile(file, []byte("archive"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	selected, err := m.DefaultBackupTestFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(backupDir, "vidveil_backup_2026-01-04.tar.gz"),
		filepath.Join(backupDir, "vidveil-daily.tar.gz.enc"),
		filepath.Join(backupDir, "vidveil-hourly.tar.gz"),
	}
	if len(selected) != len(want) {
		t.Fatalf("selected %v, want %v", selected, want)
	}
	for i := range want {
		if selected[i] != want[i] {
			t.Errorf("selected[%d] = %q, want %q", i, selected[i], want[i])
		}
	}
}

func TestBackupIncrementalWithPassword_UsesEncryptedBase(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	password := "backup-secret"
	baseFile := filepath.Join(backupDir, "vidveil_backup_2026-01-01.tar.gz.enc")
	if err := m.BackupWithOptions(BackupOptions{Filename: baseFile, Password: password}); err != nil {
		t.Fatalf("create encrypted full base: %v", err)
	}
	outFile := filepath.Join(backupDir, "vidveil-hourly.tar.gz.enc")
	if err := m.BackupIncrementalWithPassword(outFile, password); err != nil {
		t.Fatalf("create encrypted incremental: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("encrypted incremental missing: %v", err)
	}
	if err := m.BackupIncrementalWithPassword(filepath.Join(backupDir, "wrong-password.tar.gz.enc"), "wrong-password"); err == nil {
		t.Fatal("wrong password unexpectedly created an incremental")
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

func TestVerifyBackup_IncrementalChecksItsBase(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	baseFile := filepath.Join(backupDir, "vidveil_backup_2026-01-01.tar.gz")
	if err := m.BackupWithOptions(BackupOptions{Filename: baseFile}); err != nil {
		t.Fatalf("create full base: %v", err)
	}
	incrementalFile := filepath.Join(backupDir, "vidveil-hourly.tar.gz")
	if err := m.BackupIncremental(incrementalFile); err != nil {
		t.Fatalf("create incremental: %v", err)
	}
	data, err := os.ReadFile(incrementalFile)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	checksum := "sha256:" + hex.EncodeToString(sum[:])
	if err := m.verifyBackup(incrementalFile, checksum, ""); err != nil {
		t.Fatalf("verify incremental with base: %v", err)
	}
	if err := os.Remove(baseFile); err != nil {
		t.Fatal(err)
	}
	if err := m.verifyBackup(incrementalFile, checksum, ""); err == nil || !strings.Contains(err.Error(), "ERR_BASE_MISSING") {
		t.Fatalf("verify incremental without base error = %v, want ERR_BASE_MISSING", err)
	}
}

// ── RestoreWithPassword / Restore ─────────────────────────────────────────────

func TestRestoreWithPassword_RestoresFromBackup(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)

	if err := os.WriteFile(filepath.Join(configDir, "restore_test.yml"), []byte("key: value"), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(backupDir, "restore.tar.gz")
	if err := m.Backup(outFile); err != nil {
		t.Fatalf("Backup for restore: %v", err)
	}

	if err := m.RestoreWithPassword(outFile, ""); err != nil {
		t.Errorf("RestoreWithPassword: %v", err)
	}
}

func TestRestoreIncrementalCombinesBaseChangesAndDeletions(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)
	changedPath := filepath.Join(configDir, "changed.yml")
	deletedPath := filepath.Join(configDir, "deleted.yml")
	if err := os.WriteFile(changedPath, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deletedPath, []byte("remove me"), 0600); err != nil {
		t.Fatal(err)
	}
	baseFile := filepath.Join(backupDir, "vidveil_backup_2026-01-01.tar.gz")
	if err := m.BackupWithOptions(BackupOptions{Filename: baseFile}); err != nil {
		t.Fatalf("create full base: %v", err)
	}
	if err := os.WriteFile(changedPath, []byte("incremental"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(deletedPath); err != nil {
		t.Fatal(err)
	}
	incrementalFile := filepath.Join(backupDir, "vidveil-hourly.tar.gz")
	if err := m.BackupIncremental(incrementalFile); err != nil {
		t.Fatalf("create incremental: %v", err)
	}
	if err := os.WriteFile(changedPath, []byte("live state to replace"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.RestoreWithPassword(incrementalFile, ""); err != nil {
		t.Fatalf("restore incremental: %v", err)
	}
	contents, err := os.ReadFile(changedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "incremental" {
		t.Fatalf("restored changed file = %q, want incremental", contents)
	}
	if _, err := os.Stat(deletedPath); !os.IsNotExist(err) {
		t.Fatalf("deleted path still exists after restore: %v", err)
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

// ── SetUpdateBranch + GetUpdateBranch valid branches ─────────────────────────

// TestSetUpdateBranch_ValidBranches verifies all three valid branches can be set.
func TestSetUpdateBranch_ValidBranches(t *testing.T) {
	for _, branch := range []string{"stable", "beta", "devel", "daily", "rolling", "dev", "development"} {
		m, _, _ := newManagerWithDirs(t)
		if err := m.SetUpdateBranch(branch); err != nil {
			t.Errorf("SetUpdateBranch(%q): got %v, want nil", branch, err)
		}
	}
}

// TestGetUpdateBranch_ReturnsSetBranch verifies GetUpdateBranch reads back
// what SetUpdateBranch wrote.
func TestGetUpdateBranch_ReturnsSetBranch(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.SetUpdateBranch("beta"); err != nil {
		t.Fatalf("SetUpdateBranch: %v", err)
	}
	if got := m.GetUpdateBranch(); got != "beta" {
		t.Errorf("GetUpdateBranch() = %q, want %q", got, "beta")
	}
}

// ── SetMaintenanceMode enable + disable ───────────────────────────────────────

// TestSetMaintenanceMode_EnableCreatesFlag verifies the enable path creates the
// flag file and IsMaintenanceMode returns true.
func TestSetMaintenanceMode_EnableCreatesFlag(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)

	if err := m.SetMaintenanceMode(true); err != nil {
		t.Fatalf("SetMaintenanceMode(true): %v", err)
	}
	if !m.IsMaintenanceMode() {
		t.Error("IsMaintenanceMode() = false after enable, want true")
	}
}

// TestSetMaintenanceMode_EnableThenDisable verifies the round-trip leaves
// maintenance mode off.
func TestSetMaintenanceMode_EnableThenDisable(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)

	if err := m.SetMaintenanceMode(true); err != nil {
		t.Fatalf("SetMaintenanceMode(true): %v", err)
	}
	if err := m.SetMaintenanceMode(false); err != nil {
		t.Fatalf("SetMaintenanceMode(false): %v", err)
	}
	if m.IsMaintenanceMode() {
		t.Error("IsMaintenanceMode() = true after disable, want false")
	}
}

// ── BackupWithOptions encrypted path ─────────────────────────────────────────

// TestBackupWithOptions_EncryptedPath verifies BackupWithOptions with a
// password creates a .enc file and that RestoreWithPassword can decrypt it.
func TestBackupWithOptions_EncryptedPath(t *testing.T) {
	backupDir := t.TempDir()
	m, configDir, _ := newManagerWithDirs(t)

	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.BackupWithOptions(BackupOptions{Filename: filepath.Join(backupDir, "vidveil_backup_2026-01-01.tar.gz")}); err != nil {
		t.Fatalf("create full backup base: %v", err)
	}

	outFile := filepath.Join(backupDir, "enc_backup.tar.gz.enc")
	err := m.BackupWithOptions(BackupOptions{
		Filename:   outFile,
		Password:   "testpassword123",
		MaxBackups: 5,
	})
	if err != nil {
		t.Fatalf("BackupWithOptions encrypted: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Error("BackupWithOptions encrypted: output file missing")
	}

	if err := m.RestoreWithPassword(outFile, "testpassword123"); err != nil {
		t.Errorf("RestoreWithPassword correct password: %v", err)
	}
}

// ── BackupWithOptions IncludeSSL path ────────────────────────────────────────

// TestBackupWithOptions_IncludeSSL verifies IncludeSSL=true runs without error
// even when the ssl/ subdirectory does not exist (the stat guard skips it).
func TestBackupWithOptions_IncludeSSL(t *testing.T) {
	backupDir := t.TempDir()
	m, configDir, _ := newManagerWithDirs(t)

	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(backupDir, "ssl_backup.tar.gz")
	err := m.BackupWithOptions(BackupOptions{
		Filename:   outFile,
		IncludeSSL: true,
		MaxBackups: 5,
	})
	if err != nil {
		t.Fatalf("BackupWithOptions IncludeSSL=true: %v", err)
	}
}

// ── BackupIncremental timestamp naming ───────────────────────────────────────

// TestBackupIncremental_AutoNaming verifies BackupIncremental uses auto-naming
// when no filename is given. We pass empty string via Backup(), which delegates
// to BackupIncremental with an empty path.
func TestBackupIncremental_AutoNaming(t *testing.T) {
	m, _, _ := newManagerWithDirs(t)
	if err := m.BackupWithOptions(BackupOptions{Filename: filepath.Join(m.paths.Backup, "vidveil_backup_2026-01-01.tar.gz")}); err != nil {
		t.Fatalf("create full backup base: %v", err)
	}

	if err := m.BackupIncremental(""); err != nil {
		t.Fatalf("BackupIncremental auto-name: %v", err)
	}

	entries, err := os.ReadDir(m.paths.Backup)
	if err != nil {
		t.Fatalf("reading backup dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("BackupIncremental auto-name: no files in backup dir")
	}
}

// ── verifyBackup empty-size path ─────────────────────────────────────────────

// TestVerifyBackup_ZeroSizeFile verifies verifyBackup rejects an empty file.
func TestVerifyBackup_ZeroSizeFile(t *testing.T) {
	backupDir := t.TempDir()
	m, _, _ := newManagerWithDirs(t)

	emptyFile := filepath.Join(backupDir, "empty.tar.gz")
	if err := os.WriteFile(emptyFile, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	if err := m.verifyBackup(emptyFile, "sha256:e3b0", ""); err == nil {
		t.Error("verifyBackup zero-size: expected error, got nil")
	}
}

// ── applyRetentionWithOptions weekly/monthly/yearly paths ────────────────────

// TestApplyRetentionWithOptions_KeepWeekly verifies keepWeekly>0 path does not
// panic or error when there are fewer files than the keep count.
func TestApplyRetentionWithOptions_KeepWeeklyMonthlyYearly(t *testing.T) {
	backupDir := t.TempDir()
	m, configDir, _ := newManagerWithDirs(t)

	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("x: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		fname := filepath.Join(backupDir, time.Now().Add(-time.Duration(i)*24*time.Hour).Format("vidveil_backup_2006-01-02_150405")+".tar.gz")
		if err := m.BackupWithOptions(BackupOptions{Filename: fname, MaxBackups: 10}); err != nil {
			t.Errorf("backup %s: %v", fname, err)
		}
	}

	if err := m.applyRetentionWithOptions(10, 2, 1, 1, ""); err != nil {
		t.Errorf("applyRetentionWithOptions weekly/monthly/yearly: %v", err)
	}
}

// ── BackupDailyFull ────────────────────────────────────────────────────────────

// TestBackupDailyFull_CreatesBothFiles verifies the first run creates only a
// full and the next run creates the cumulative incremental before its new full.
func TestBackupDailyFull_CreatesBothFiles(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)
	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.BackupDailyFull(BackupOptions{IncludeData: true, MaxBackups: 5}); err != nil {
		t.Fatalf("BackupDailyFull: %v", err)
	}

	dateStr := time.Now().UTC().Format("2006-01-02")
	fullFile := filepath.Join(backupDir, "vidveil_backup_"+dateStr+".tar.gz")
	if _, err := os.Stat(fullFile); err != nil {
		t.Errorf("BackupDailyFull: expected full backup file %s: %v", fullFile, err)
	}

	dailyFile := filepath.Join(backupDir, "vidveil-daily.tar.gz")
	if _, err := os.Stat(dailyFile); !os.IsNotExist(err) {
		t.Fatalf("first backup run must skip the incremental, stat error = %v", err)
	}
	if err := m.BackupDailyFull(BackupOptions{IncludeData: true, MaxBackups: 5}); err != nil {
		t.Fatalf("second BackupDailyFull: %v", err)
	}
	if _, err := os.Stat(dailyFile); err != nil {
		t.Errorf("BackupDailyFull: expected daily incremental file %s: %v", dailyFile, err)
	}
}

// TestBackupDailyFull_EncryptedNamesUseEncExtension verifies both files get the
// .enc suffix when a password is supplied.
func TestBackupDailyFull_EncryptedNamesUseEncExtension(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)
	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}

	options := BackupOptions{IncludeData: true, MaxBackups: 5, Password: "s3cret"}
	if err := m.BackupDailyFull(options); err != nil {
		t.Fatalf("BackupDailyFull encrypted: %v", err)
	}

	dateStr := time.Now().UTC().Format("2006-01-02")
	fullFile := filepath.Join(backupDir, "vidveil_backup_"+dateStr+".tar.gz.enc")
	if _, err := os.Stat(fullFile); err != nil {
		t.Errorf("BackupDailyFull encrypted: expected full backup file %s: %v", fullFile, err)
	}

	dailyFile := filepath.Join(backupDir, "vidveil-daily.tar.gz.enc")
	if _, err := os.Stat(dailyFile); !os.IsNotExist(err) {
		t.Fatalf("first encrypted run must skip the incremental, stat error = %v", err)
	}
	if err := m.BackupDailyFull(options); err != nil {
		t.Fatalf("second BackupDailyFull encrypted: %v", err)
	}
	if _, err := os.Stat(dailyFile); err != nil {
		t.Errorf("BackupDailyFull encrypted: expected daily incremental file %s: %v", dailyFile, err)
	}
}

func TestBackupDailyFull_PasswordChangePreservesPreviousIncremental(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)
	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0600); err != nil {
		t.Fatal(err)
	}
	first := BackupOptions{IncludeData: true, MaxBackups: 5, Password: "first-password"}
	if err := m.BackupDailyFull(first); err != nil {
		t.Fatalf("initial full: %v", err)
	}
	if err := m.BackupDailyFull(first); err != nil {
		t.Fatalf("initial incremental: %v", err)
	}
	dailyFile := filepath.Join(backupDir, "vidveil-daily.tar.gz.enc")
	previousIncremental, err := os.ReadFile(dailyFile)
	if err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.Password = "second-password"
	passwordChanged, err := m.latestFullPasswordChanged(changed.Password)
	if err != nil || !passwordChanged {
		t.Fatalf("password change detection = %v, %v; want true, nil", passwordChanged, err)
	}
	if err := m.BackupDailyFull(changed); err != nil {
		t.Fatalf("full after password change: %v", err)
	}
	currentIncremental, err := os.ReadFile(dailyFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(previousIncremental, currentIncremental) {
		t.Fatal("password change replaced the existing incremental before a new base had an incremental")
	}
	if err := m.BackupDailyFull(changed); err != nil {
		t.Fatalf("incremental after password change: %v", err)
	}
	updatedIncremental, err := os.ReadFile(dailyFile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(previousIncremental, updatedIncremental) {
		t.Fatal("next run did not replace the old incremental")
	}
}

func TestBackupDailyFull_RemovingEncryptionReplacesOldIncremental(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, _, _ := newManagerWithDirs(t)
	encrypted := BackupOptions{IncludeData: true, MaxBackups: 5, Password: "old-password"}
	if err := m.BackupDailyFull(encrypted); err != nil {
		t.Fatalf("initial encrypted full: %v", err)
	}
	if err := m.BackupDailyFull(encrypted); err != nil {
		t.Fatalf("initial encrypted incremental: %v", err)
	}
	oldDaily := filepath.Join(backupDir, "vidveil-daily.tar.gz.enc")
	plain := encrypted
	plain.Password = ""
	passwordChanged, err := m.latestFullPasswordChanged("")
	if err != nil || !passwordChanged {
		t.Fatalf("encryption removal detection = %v, %v; want true, nil", passwordChanged, err)
	}
	if err := m.BackupDailyFull(plain); err != nil {
		t.Fatalf("full after removing encryption: %v", err)
	}
	if _, err := os.Stat(oldDaily); err != nil {
		t.Fatalf("old encrypted incremental should remain until the new base has an incremental: %v", err)
	}
	if err := m.BackupDailyFull(plain); err != nil {
		t.Fatalf("plain incremental after encryption removal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "vidveil-daily.tar.gz")); err != nil {
		t.Fatalf("new plain incremental missing: %v", err)
	}
	if _, err := os.Stat(oldDaily); !os.IsNotExist(err) {
		t.Fatalf("stale encrypted incremental remains after replacement: %v", err)
	}
}

// TestBackupDailyFull_DailyIncrementalSurvivesRetention verifies the fixed
// vidveil-daily file is never swept by count-based retention even when
// MaxBackups is small and other backups already exist.
func TestBackupDailyFull_DailyIncrementalSurvivesRetention(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("BACKUP_DIR", backupDir)
	m, configDir, _ := newManagerWithDirs(t)
	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		fname := filepath.Join(backupDir, time.Now().Add(-time.Duration(i+1)*24*time.Hour).Format("vidveil_backup_2006-01-02")+".tar.gz")
		if err := m.BackupWithOptions(BackupOptions{Filename: fname, MaxBackups: 10}); err != nil {
			t.Fatalf("seed backup %d: %v", i, err)
		}
	}

	if err := m.BackupDailyFull(BackupOptions{IncludeData: true, MaxBackups: 1}); err != nil {
		t.Fatalf("BackupDailyFull: %v", err)
	}

	dailyFile := filepath.Join(backupDir, "vidveil-daily.tar.gz")
	if _, err := os.Stat(dailyFile); err != nil {
		t.Errorf("BackupDailyFull: daily incremental should survive retention: %v", err)
	}
}

// TestBackupDailyFull_FullBackupErrorPropagates verifies an error from the
// full-backup step (invalid Filename directory) is returned wrapped.
func TestBackupDailyFull_FullBackupErrorPropagates(t *testing.T) {
	m, configDir, _ := newManagerWithDirs(t)
	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("config: true"), 0644); err != nil {
		t.Fatal(err)
	}

	err := m.BackupDailyFull(BackupOptions{
		Filename:    filepath.Join(string([]byte{0}), "bad.tar.gz"),
		IncludeData: true,
		MaxBackups:  5,
	})
	if err == nil {
		t.Fatal("BackupDailyFull: expected error for invalid full-backup filename, got nil")
	}
}

func TestBackupArchiveMasksConfigSecrets(t *testing.T) {
	m, configDir, _ := newManagerWithDirs(t)
	secretData := []byte("server:\n  token: adm_private-token\n  security:\n    encryption_key: 0123456789abcdef\n  backup:\n    encryption_password: enc:v1:private-ciphertext\n")
	if err := os.WriteFile(filepath.Join(configDir, "secrets.yml"), secretData, 0600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	tarWriter := tar.NewWriter(&archive)
	if _, _, err := m.addDirToTar(tarWriter, configDir, "config", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(&archive)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name != "config/secrets.yml" {
			continue
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "adm_private-token") || strings.Contains(text, "private-ciphertext") {
			t.Fatalf("backup secrets leaked a credential: %s", text)
		}
		if !strings.Contains(text, `token: ""`) || !strings.Contains(text, `encryption_password: "********"`) {
			t.Fatalf("backup secrets do not contain required masked values: %s", text)
		}
		return
	}
	t.Fatal("backup archive omitted config/secrets.yml")
}
