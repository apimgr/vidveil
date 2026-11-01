// SPDX-License-Identifier: MIT
package maintenance

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectpath "github.com/apimgr/vidveil/src/path"
)

func TestEnsureRestorePathHasNoSymlinksRejectsSymlinkParent(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}

	target, err := resolveRestoreTarget(root, "linked/restore.txt")
	if err != nil {
		t.Fatalf("resolveRestoreTarget: %v", err)
	}
	if err := ensureRestorePathHasNoSymlinks(root, target); err == nil {
		t.Fatal("expected symlinked restore parent to be rejected")
	}
}

func TestApplyRestoredPermissions(t *testing.T) {
	for _, test := range []struct {
		name       string
		privileged bool
		wantDir    os.FileMode
		wantFile   os.FileMode
	}{
		{name: "user", wantDir: 0700, wantFile: 0600},
		{name: "system", privileged: true, wantDir: 0755, wantFile: 0644},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			nested := filepath.Join(root, "nested")
			if err := os.Mkdir(nested, 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(nested, "server.yml")
			if err := os.WriteFile(file, []byte("secret"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := applyRestoredPermissions(root, test.privileged); err != nil {
				t.Fatalf("applyRestoredPermissions: %v", err)
			}
			for path, want := range map[string]os.FileMode{root: test.wantDir, nested: test.wantDir, file: test.wantFile} {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("stat %q: %v", path, err)
				}
				if got := info.Mode().Perm(); got != want {
					t.Errorf("permissions for %q = %04o, want %04o", path, got, want)
				}
			}
		})
	}
}

func TestRestoreFilesAtomicallyAppliesExecutionModePermissions(t *testing.T) {
	base := t.TempDir()
	configRoot := filepath.Join(base, "config")
	if err := restoreFilesAtomically([]restoreFileEntry{{
		name: "config/server.yml", mode: 0o4755, content: []byte("secret"),
	}}, map[string]string{
		"config": configRoot,
		"data":   filepath.Join(base, "data"),
		"ssl":    filepath.Join(base, "ssl"),
	}); err != nil {
		t.Fatalf("restoreFilesAtomically: %v", err)
	}
	wantDir, wantFile := os.FileMode(0700), os.FileMode(0600)
	if projectpath.IsPrivilegedMode() {
		wantDir, wantFile = 0755, 0644
	}
	for path, want := range map[string]os.FileMode{
		configRoot:                              wantDir,
		filepath.Join(configRoot, "server.yml"): wantFile,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %q: %v", path, err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("permissions for %q = %04o, want %04o", path, got, want)
		}
	}
}

func TestLoadRestoreArchiveEnforcesConfiguredLimits(t *testing.T) {
	data := buildTestBackupArchive(t, BackupManifest{Version: "1"}, map[string]string{
		"config/large.yml": strings.Repeat("x", 1<<16),
	})
	tests := []struct {
		name   string
		limits RestoreLimits
	}{
		{name: "archive size", limits: RestoreLimits{MaxArchiveBytes: uint64(len(data) - 1), MaxExpandedBytes: 4 << 30, MaxFiles: 10000, MaxFileBytes: 512 << 20, MaxRatio: 100}},
		{name: "expanded size", limits: RestoreLimits{MaxArchiveBytes: 1 << 30, MaxExpandedBytes: 128, MaxFiles: 10000, MaxFileBytes: 512 << 20, MaxRatio: 100}},
		{name: "file count", limits: RestoreLimits{MaxArchiveBytes: 1 << 30, MaxExpandedBytes: 4 << 30, MaxFiles: 2, MaxFileBytes: 512 << 20, MaxRatio: 100}},
		{name: "single file", limits: RestoreLimits{MaxArchiveBytes: 1 << 30, MaxExpandedBytes: 4 << 30, MaxFiles: 10000, MaxFileBytes: 128, MaxRatio: 100}},
		{name: "compression ratio", limits: RestoreLimits{MaxArchiveBytes: 1 << 30, MaxExpandedBytes: 4 << 30, MaxFiles: 10000, MaxFileBytes: 512 << 20, MaxRatio: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := loadRestoreArchiveWithStore(data, func(header *tar.Header, source io.Reader) (restoreFileEntry, error) {
				content, err := io.ReadAll(source)
				return restoreFileEntry{name: header.Name, content: content}, err
			}, test.limits); err == nil {
				t.Fatalf("archive accepted with %s limit exceeded", test.name)
			}
		})
	}
}

func TestLoadRestoreArchiveLimitFailureRemovesPartialEntries(t *testing.T) {
	data := buildTestBackupArchive(t, BackupManifest{Version: "1"}, map[string]string{
		"config/first.yml":  "first",
		"config/second.yml": "second",
	})
	limits := DefaultRestoreLimits()
	limits.MaxFiles = 2
	tempDir := t.TempDir()
	if _, err := loadRestoreArchiveToTemp(data, tempDir, limits); err == nil {
		t.Fatal("archive accepted above max_files")
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial restore left %d staged files", len(entries))
	}
}

func TestLoadRestoreArchiveToTempSpoolsRegularFiles(t *testing.T) {
	tempDir := t.TempDir()
	data := buildTestBackupArchive(t, BackupManifest{Version: "1"}, map[string]string{
		"config/server.yml": "server:\n  port: 64580\n",
	})
	archive, err := loadRestoreArchiveToTemp(data, tempDir)
	if err != nil {
		t.Fatalf("loadRestoreArchiveToTemp: %v", err)
	}
	if len(archive.files) != 1 {
		t.Fatalf("parsed %d files, want 1", len(archive.files))
	}
	file := archive.files[0]
	if len(file.content) != 0 || file.contentPath == "" {
		t.Fatalf("spooled entry = %#v, want a path and no in-memory content", file)
	}
	got, err := os.ReadFile(file.contentPath)
	if err != nil {
		t.Fatalf("read spooled entry: %v", err)
	}
	if string(got) != "server:\n  port: 64580\n" {
		t.Fatalf("spooled content = %q", got)
	}
}

func TestRestoreFilesAtomicallyCopiesSpooledFiles(t *testing.T) {
	base := t.TempDir()
	contentPath := filepath.Join(base, "spooled-entry")
	if err := os.WriteFile(contentPath, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	configRoot := filepath.Join(base, "config")
	if err := os.Mkdir(configRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := restoreFilesAtomically([]restoreFileEntry{{
		name: "config/server.yml", mode: 0600, contentPath: contentPath,
	}}, map[string]string{
		"config": configRoot,
		"data":   filepath.Join(base, "data"),
		"ssl":    filepath.Join(base, "ssl"),
	}); err != nil {
		t.Fatalf("restoreFilesAtomically: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(configRoot, "server.yml"))
	if err != nil || string(got) != "new" {
		t.Fatalf("restored content = %q, err=%v; want new", got, err)
	}
}

func TestRestoreFilesAtomicallyReplacesRoot(t *testing.T) {
	base := t.TempDir()
	configRoot := filepath.Join(base, "config")
	if err := os.Mkdir(configRoot, 0755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(configRoot, "server.yml")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreFilesAtomically([]restoreFileEntry{{name: "config/server.yml", mode: 0600, content: []byte("new")}}, map[string]string{
		"config": configRoot,
		"data":   filepath.Join(base, "data"),
		"ssl":    filepath.Join(base, "ssl"),
	}); err != nil {
		t.Fatalf("restoreFilesAtomically: %v", err)
	}
	got, err := os.ReadFile(oldPath)
	if err != nil || string(got) != "new" {
		t.Fatalf("restored content = %q, err=%v; want new", got, err)
	}
}

func TestRestoreFilesAtomicallyRollsBackEarlierRootOnFailure(t *testing.T) {
	base := t.TempDir()
	configRoot := filepath.Join(base, "config")
	if err := os.Mkdir(configRoot, 0755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(configRoot, "server.yml")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidDataRoot := filepath.Join(base, "data")
	if err := os.WriteFile(invalidDataRoot, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	err := restoreFilesAtomically([]restoreFileEntry{
		{name: "config/server.yml", mode: 0600, content: []byte("new")},
		{name: "data/file", mode: 0600, content: []byte("new data")},
	}, map[string]string{
		"config": configRoot,
		"data":   invalidDataRoot,
		"ssl":    filepath.Join(base, "ssl"),
	})
	if err == nil {
		t.Fatal("expected invalid data destination error")
	}
	got, readErr := os.ReadFile(oldPath)
	if readErr != nil || string(got) != "old" {
		t.Fatalf("previous config after rollback = %q, err=%v; want old", got, readErr)
	}
}
