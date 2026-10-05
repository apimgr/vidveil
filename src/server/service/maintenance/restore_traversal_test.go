// SPDX-License-Identifier: MIT
// AI.md PART 21: Backup & Restore — archive traversal hardening
package maintenance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// buildArchive produces a gzipped tar from name/content pairs, where an entry
// with an empty content value and a trailing slash is written as a directory.
func buildArchive(t *testing.T, entries [][2]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for _, entry := range entries {
		name := entry[0]
		isDir := strings.HasSuffix(name, "/")
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(entry[1]))}
		if isDir {
			header.Typeflag = tar.TypeDir
			header.Mode = 0o755
			header.Size = 0
		} else {
			header.Typeflag = tar.TypeReg
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write header %q: %v", name, err)
		}
		if !isDir {
			if _, err := tarWriter.Write([]byte(entry[1])); err != nil {
				t.Fatalf("write body %q: %v", name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func TestValidateArchiveEntryName(t *testing.T) {
	tests := []struct {
		name    string
		entry   string
		wantErr bool
	}{
		{"plain file", "config/server.yml", false},
		{"nested file", "data/db/sqlite/server.db", false},
		{"directory", "config/template/", false},
		{"dot segment", "config/./server.yml", false},
		{"parent traversal", "config/../../etc/cron.d/backdoor", true},
		{"traversal via data", "data/../../root/.ssh/authorized_keys", true},
		{"trailing parent", "config/subdir/..", true},
		{"absolute path", "/etc/shadow", true},
		{"windows absolute", `C:\Windows\System32\evil.dll`, true},
		{"backslash separator", `config\..\..\evil`, true},
		{"empty name", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateArchiveEntryName(tc.entry)
			if tc.wantErr && err == nil {
				t.Fatalf("validateArchiveEntryName(%q) = nil, want an error", tc.entry)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateArchiveEntryName(%q) = %v, want nil", tc.entry, err)
			}
		})
	}
}

func TestResolveRestoreTarget(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "etc", "apimgr", "vidveil")

	tests := []struct {
		name    string
		entry   string
		want    string
		wantErr bool
	}{
		{"plain", "server.yml", filepath.Join(root, "server.yml"), false},
		{"nested", "db/sqlite/server.db", filepath.Join(root, "db", "sqlite", "server.db"), false},
		{"traversal", "../../../etc/shadow", "", true},
		// Absolute names are rejected at parse time by validateArchiveEntryName;
		// Join neutralizes them regardless, so containment still holds here.
		{"absolute is contained", "/etc/shadow", filepath.Join(root, "etc", "shadow"), false},
		{"deep traversal", "a/b/../../../../outside", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveRestoreTarget(root, tc.entry)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveRestoreTarget(%q) = %q, want an error", tc.entry, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveRestoreTarget(%q) = %v, want nil", tc.entry, err)
			}
			if got != tc.want {
				t.Fatalf("resolveRestoreTarget(%q) = %q, want %q", tc.entry, got, tc.want)
			}
		})
	}
}

// TestLoadRestoreArchiveRejectsTraversal proves the parser refuses a malicious
// entry before anything is buffered, rather than deferring to Phase 2.
func TestLoadRestoreArchiveRejectsTraversal(t *testing.T) {
	tests := []struct {
		name    string
		entries [][2]string
	}{
		{
			name:    "config prefix with parent traversal",
			entries: [][2]string{{"config/../../evil.txt", "pwned"}},
		},
		{
			name:    "data prefix with parent traversal",
			entries: [][2]string{{"data/../../evil.txt", "pwned"}},
		},
		{
			name:    "directory with parent traversal",
			entries: [][2]string{{"config/../../evil/", ""}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadRestoreArchive(buildArchive(t, tc.entries))
			if err == nil {
				t.Fatal("loadRestoreArchive accepted a traversing entry, want an error")
			}
			if !strings.Contains(err.Error(), "invalid backup entry") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestLoadRestoreArchiveAcceptsLegitimateEntries guards against over-blocking:
// normal backup contents must still parse.
func TestLoadRestoreArchiveAcceptsLegitimateEntries(t *testing.T) {
	entries := [][2]string{
		{"config/server.yml", "server:\n  port: 64580\n"},
		{"config/template/", ""},
		{"data/db/sqlite/server.db", "sqlite-bytes"},
		{"ssl/local/example.com/cert.pem", "cert"},
	}

	archive, err := loadRestoreArchive(buildArchive(t, entries))
	if err != nil {
		t.Fatalf("loadRestoreArchive = %v, want nil", err)
	}
	if len(archive.files) != len(entries) {
		t.Fatalf("got %d files, want %d", len(archive.files), len(entries))
	}
}

// TestRestoreWithPasswordRejectsTraversal is the end-to-end guard: a malicious
// archive must fail the restore and must not write outside the config root.
func TestRestoreWithPasswordRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	sslDir := filepath.Join(base, "ssl")
	backupDir := filepath.Join(base, "backup")

	for _, dir := range []string{configDir, dataDir, sslDir, backupDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	// The traversal target sits beside the config root, not inside it.
	canary := filepath.Join(base, "canary.txt")

	backupFile := filepath.Join(backupDir, "vidveil_backup_2026-01-01_000000.tar.gz")
	payload := buildArchive(t, [][2]string{{"config/../canary.txt", "overwritten"}})
	if err := os.WriteFile(backupFile, payload, 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	m := &MaintenanceManager{
		paths: &config.AppPaths{
			Config: configDir,
			Data:   dataDir,
			SSL:    sslDir,
			Backup: backupDir,
		},
		version: "test",
	}

	if err := m.Restore(backupFile); err == nil {
		t.Fatal("Restore accepted a traversing archive, want an error")
	}

	if _, err := os.Stat(canary); !os.IsNotExist(err) {
		t.Fatalf("canary at %s exists (err=%v); restore escaped the config root", canary, err)
	}
}
