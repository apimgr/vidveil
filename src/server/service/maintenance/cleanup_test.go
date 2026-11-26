// SPDX-License-Identifier: MIT
// AI.md PART 13: Disk-pressure cleanup tests.
package maintenance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

func TestCleanupDiskPressureUsesConfiguredRetentionAndAgeLimits(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	manager, base := newMaintMgrWithTempDirs(t)
	logDir := filepath.Join(base, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	oldLog := filepath.Join(logDir, "old.log")
	if err := os.WriteFile(oldLog, []byte("expired log"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldLog, old, old); err != nil {
		t.Fatal(err)
	}
	recentLog := filepath.Join(logDir, "current.log")
	if err := os.WriteFile(recentLog, []byte("current log"), 0600); err != nil {
		t.Fatal(err)
	}

	tempRoot := filepath.Join(os.TempDir(), "apimgr")
	projectTemp := filepath.Join(tempRoot, "vidveil-stale")
	unrelatedTemp := filepath.Join(tempRoot, "other-stale")
	for _, dir := range []string{projectTemp, unrelatedTemp} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	staleTempFile := filepath.Join(projectTemp, "stale.tmp")
	if err := os.WriteFile(staleTempFile, []byte("expired temporary data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(staleTempFile, old, old); err != nil {
		t.Fatal(err)
	}
	unrelatedFile := filepath.Join(unrelatedTemp, "keep.tmp")
	if err := os.WriteFile(unrelatedFile, []byte("another project"), 0600); err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 3; index++ {
		backup := filepath.Join(manager.paths.Backup, "backup-"+string(rune('a'+index))+".tar.gz")
		if err := os.WriteFile(backup, []byte("backup"), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Now().Add(-time.Duration(index) * time.Hour)
		if err := os.Chtimes(backup, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}

	freed, err := manager.CleanupDiskPressure(config.BackupRetentionConfig{MaxBackups: 1}, logDir, 7)
	if err != nil {
		t.Fatalf("CleanupDiskPressure: %v", err)
	}
	if freed <= 0 {
		t.Fatalf("CleanupDiskPressure freed %d bytes, want a positive amount", freed)
	}
	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Errorf("expired log still exists: %v", err)
	}
	if _, err := os.Stat(recentLog); err != nil {
		t.Errorf("recent log was removed: %v", err)
	}
	if _, err := os.Stat(staleTempFile); !os.IsNotExist(err) {
		t.Errorf("expired project temporary file still exists: %v", err)
	}
	if _, err := os.Stat(unrelatedFile); err != nil {
		t.Errorf("unrelated project temporary file was removed: %v", err)
	}
	backups, err := manager.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("backup count after cleanup = %d, want 1 per configured retention", len(backups))
	}
}
