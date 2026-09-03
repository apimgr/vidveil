// SPDX-License-Identifier: MIT
package path

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- Constants ---

func TestProjectConstants(t *testing.T) {
	if ProjectOrg != "apimgr" {
		t.Errorf("ProjectOrg = %q, want %q", ProjectOrg, "apimgr")
	}
	if ProjectName != "vidveil" {
		t.Errorf("ProjectName = %q, want %q", ProjectName, "vidveil")
	}
}

// --- pathOverride (internal) ---
// Tested indirectly via GetAppPaths but also directly through the exported wrappers.

// --- GetDefaultConfigDir ---
// Covers: root vs user, current GOOS branch is always exercised.

func TestGetDefaultConfigDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultConfigDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultConfigDir(isRoot=%v) returned empty string", isRoot)
		}
		if !strings.Contains(got, ProjectOrg) {
			t.Errorf("GetDefaultConfigDir(isRoot=%v) = %q, want path containing %q", isRoot, got, ProjectOrg)
		}
		if !strings.Contains(got, ProjectName) {
			t.Errorf("GetDefaultConfigDir(isRoot=%v) = %q, want path containing %q", isRoot, got, ProjectName)
		}
	}
}

func TestGetDefaultConfigDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultConfigDir(true)
	want := "/etc/apimgr/vidveil"
	if got != want {
		t.Errorf("GetDefaultConfigDir(root) = %q, want %q", got, want)
	}
}

func TestGetDefaultConfigDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultConfigDir(false)
	want := filepath.Join(home, ".config", "apimgr", "vidveil")
	if got != want {
		t.Errorf("GetDefaultConfigDir(user) = %q, want %q", got, want)
	}
}

// --- GetDefaultDataDir ---

func TestGetDefaultDataDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultDataDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultDataDir(isRoot=%v) returned empty string", isRoot)
		}
		if !strings.Contains(got, ProjectOrg) {
			t.Errorf("GetDefaultDataDir(isRoot=%v) = %q, want path containing %q", isRoot, got, ProjectOrg)
		}
	}
}

func TestGetDefaultDataDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultDataDir(true)
	want := "/var/lib/apimgr/vidveil"
	if got != want {
		t.Errorf("GetDefaultDataDir(root) = %q, want %q", got, want)
	}
}

func TestGetDefaultDataDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultDataDir(false)
	want := filepath.Join(home, ".local", "share", "apimgr", "vidveil")
	if got != want {
		t.Errorf("GetDefaultDataDir(user) = %q, want %q", got, want)
	}
}

// --- GetDefaultCacheDir ---

func TestGetDefaultCacheDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultCacheDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultCacheDir(isRoot=%v) returned empty string", isRoot)
		}
	}
}

func TestGetDefaultCacheDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultCacheDir(true)
	want := "/var/cache/apimgr/vidveil"
	if got != want {
		t.Errorf("GetDefaultCacheDir(root) = %q, want %q", got, want)
	}
}

func TestGetDefaultCacheDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultCacheDir(false)
	want := filepath.Join(home, ".cache", "apimgr", "vidveil")
	if got != want {
		t.Errorf("GetDefaultCacheDir(user) = %q, want %q", got, want)
	}
}

// --- GetDefaultLogDir ---

func TestGetDefaultLogDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultLogDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultLogDir(isRoot=%v) returned empty string", isRoot)
		}
	}
}

func TestGetDefaultLogDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultLogDir(true)
	want := "/var/log/apimgr/vidveil"
	if got != want {
		t.Errorf("GetDefaultLogDir(root) = %q, want %q", got, want)
	}
}

func TestGetDefaultLogDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultLogDir(false)
	want := filepath.Join(home, ".local", "log", "apimgr", "vidveil")
	if got != want {
		t.Errorf("GetDefaultLogDir(user) = %q, want %q", got, want)
	}
}

// --- GetDefaultPIDFile ---

func TestGetDefaultPIDFileNonEmptyOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows returns empty PID path by design")
	}
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultPIDFile(isRoot)
		if got == "" {
			t.Errorf("GetDefaultPIDFile(isRoot=%v) returned empty string on %s", isRoot, runtime.GOOS)
		}
		if !strings.HasSuffix(got, ".pid") {
			t.Errorf("GetDefaultPIDFile(isRoot=%v) = %q, want suffix .pid", isRoot, got)
		}
		if !strings.Contains(got, ProjectName) {
			t.Errorf("GetDefaultPIDFile(isRoot=%v) = %q, want path containing %q", isRoot, got, ProjectName)
		}
	}
}

func TestGetDefaultPIDFileLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultPIDFile(true)
	want := "/var/run/apimgr/vidveil.pid"
	if got != want {
		t.Errorf("GetDefaultPIDFile(root) = %q, want %q", got, want)
	}
}

// --- GetDefaultSSLDir ---

func TestGetDefaultSSLDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultSSLDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultSSLDir(isRoot=%v) returned empty string", isRoot)
		}
		if !strings.Contains(got, "ssl") {
			t.Errorf("GetDefaultSSLDir(isRoot=%v) = %q, want path containing 'ssl'", isRoot, got)
		}
	}
}

func TestGetDefaultSSLDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultSSLDir(true)
	want := "/etc/apimgr/vidveil/ssl"
	if got != want {
		t.Errorf("GetDefaultSSLDir(root) = %q, want %q", got, want)
	}
}

// --- GetDefaultSecurityDir ---

func TestGetDefaultSecurityDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultSecurityDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultSecurityDir(isRoot=%v) returned empty string", isRoot)
		}
		if !strings.Contains(got, "security") {
			t.Errorf("GetDefaultSecurityDir(isRoot=%v) = %q, want path containing 'security'", isRoot, got)
		}
	}
}

func TestGetDefaultSecurityDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultSecurityDir(true)
	want := "/var/lib/apimgr/vidveil/security"
	if got != want {
		t.Errorf("GetDefaultSecurityDir(root) = %q, want %q", got, want)
	}
}

// --- GetDefaultBackupDir ---

func TestGetDefaultBackupDirNonEmpty(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		got := GetDefaultBackupDir(isRoot)
		if got == "" {
			t.Errorf("GetDefaultBackupDir(isRoot=%v) returned empty string", isRoot)
		}
	}
}

func TestGetDefaultBackupDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultBackupDir(false)
	want := filepath.Join(home, ".local", "share", "Backups", "apimgr", "vidveil")
	if got != want {
		t.Errorf("GetDefaultBackupDir(user) = %q, want %q", got, want)
	}
}

// --- GetDatabaseDir ---

func TestGetDatabaseDirDefault(t *testing.T) {
	t.Setenv("DATABASE_DIR", "")
	got := GetDatabaseDir("/data")
	want := "/data/db"
	if got != want {
		t.Errorf("GetDatabaseDir('/data') = %q, want %q", got, want)
	}
}

func TestGetDatabaseDirEnvOverride(t *testing.T) {
	t.Setenv("DATABASE_DIR", "/custom/db")
	got := GetDatabaseDir("/data")
	want := "/custom/db"
	if got != want {
		t.Errorf("GetDatabaseDir with DATABASE_DIR set = %q, want %q", got, want)
	}
}

func TestGetDatabaseDirEmptyDataDir(t *testing.T) {
	t.Setenv("DATABASE_DIR", "")
	got := GetDatabaseDir("")
	want := "db"
	if got != want {
		t.Errorf("GetDatabaseDir('') = %q, want %q", got, want)
	}
}

// --- GetAppPaths ---
// Tests that all fields are populated and env vars are respected.

func TestGetAppPathsAllFieldsNonEmpty(t *testing.T) {
	// Clear all override env vars so defaults are used.
	for _, env := range []string{
		"CONFIG_DIR", "DATA_DIR", "CACHE_DIR", "LOG_DIR",
		"BACKUP_DIR", "PID_FILE", "SSL_DIR", "SECURITY_DIR",
	} {
		t.Setenv(env, "")
	}

	p := GetAppPaths("", "")

	if p.Config == "" {
		t.Error("GetAppPaths: Config is empty")
	}
	if p.Data == "" {
		t.Error("GetAppPaths: Data is empty")
	}
	if p.Cache == "" {
		t.Error("GetAppPaths: Cache is empty")
	}
	if p.Log == "" {
		t.Error("GetAppPaths: Log is empty")
	}
	if p.Backup == "" {
		t.Error("GetAppPaths: Backup is empty")
	}
	if p.SSL == "" {
		t.Error("GetAppPaths: SSL is empty")
	}
	if p.Security == "" {
		t.Error("GetAppPaths: Security is empty")
	}
	// PIDFile is empty on Windows by design; skip that assertion on Windows.
	if runtime.GOOS != "windows" && p.PIDFile == "" {
		t.Error("GetAppPaths: PIDFile is empty on non-windows platform")
	}
}

func TestGetAppPathsExplicitConfigDirOverridesDefault(t *testing.T) {
	t.Setenv("CONFIG_DIR", "")
	p := GetAppPaths("/explicit/config", "")
	if p.Config != "/explicit/config" {
		t.Errorf("GetAppPaths: Config = %q, want /explicit/config", p.Config)
	}
}

func TestGetAppPathsExplicitDataDirOverridesDefault(t *testing.T) {
	t.Setenv("DATA_DIR", "")
	p := GetAppPaths("", "/explicit/data")
	if p.Data != "/explicit/data" {
		t.Errorf("GetAppPaths: Data = %q, want /explicit/data", p.Data)
	}
}

func TestGetAppPathsEnvVarOverridesDefault(t *testing.T) {
	t.Setenv("CONFIG_DIR", "/env/config")
	p := GetAppPaths("", "")
	if p.Config != "/env/config" {
		t.Errorf("GetAppPaths: Config = %q, want /env/config from CONFIG_DIR", p.Config)
	}
}

func TestGetAppPathsExplicitArgBeatsEnvVar(t *testing.T) {
	// Explicit arg takes priority over env var (pathOverride order).
	t.Setenv("CONFIG_DIR", "/env/config")
	p := GetAppPaths("/explicit/config", "")
	if p.Config != "/explicit/config" {
		t.Errorf("GetAppPaths: Config = %q, want /explicit/config (explicit arg beats env)", p.Config)
	}
}

func TestGetAppPathsAllEnvVarOverrides(t *testing.T) {
	t.Setenv("CACHE_DIR", "/env/cache")
	t.Setenv("LOG_DIR", "/env/log")
	t.Setenv("BACKUP_DIR", "/env/backup")
	t.Setenv("PID_FILE", "/env/pid.pid")
	t.Setenv("SSL_DIR", "/env/ssl")
	t.Setenv("SECURITY_DIR", "/env/security")

	p := GetAppPaths("", "")

	if p.Cache != "/env/cache" {
		t.Errorf("Cache = %q, want /env/cache", p.Cache)
	}
	if p.Log != "/env/log" {
		t.Errorf("Log = %q, want /env/log", p.Log)
	}
	if p.Backup != "/env/backup" {
		t.Errorf("Backup = %q, want /env/backup", p.Backup)
	}
	if p.PIDFile != "/env/pid.pid" {
		t.Errorf("PIDFile = %q, want /env/pid.pid", p.PIDFile)
	}
	if p.SSL != "/env/ssl" {
		t.Errorf("SSL = %q, want /env/ssl", p.SSL)
	}
	if p.Security != "/env/security" {
		t.Errorf("Security = %q, want /env/security", p.Security)
	}
}

// --- EnsureDir ---

func TestEnsureDirCreatesNewDirectory(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "newdir")

	if err := EnsureDir(target, false); err != nil {
		t.Fatalf("EnsureDir: unexpected error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q exists but is not a directory", target)
	}
}

func TestEnsureDirIdempotentOnExistingDirectory(t *testing.T) {
	tmp := t.TempDir()

	// First call
	if err := EnsureDir(tmp, false); err != nil {
		t.Fatalf("EnsureDir (first call): %v", err)
	}
	// Second call must not fail
	if err := EnsureDir(tmp, false); err != nil {
		t.Errorf("EnsureDir (second call, dir already exists): %v", err)
	}
}

func TestEnsureDirCreatesNestedDirectories(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c")

	if err := EnsureDir(nested, false); err != nil {
		t.Fatalf("EnsureDir nested: %v", err)
	}
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("nested directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", nested)
	}
}

func TestEnsureDirUserPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not enforced on Windows")
	}
	tmp := t.TempDir()
	target := filepath.Join(tmp, "userdir")

	if err := EnsureDir(target, false); err != nil {
		t.Fatalf("EnsureDir(user): %v", err)
	}

	info, _ := os.Stat(target)
	// User-mode: expect 0700
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("EnsureDir(user) permission = %04o, want 0700", perm)
	}
}

func TestEnsureDirRootPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not enforced on Windows")
	}
	tmp := t.TempDir()
	target := filepath.Join(tmp, "rootdir")

	if err := EnsureDir(target, true); err != nil {
		t.Fatalf("EnsureDir(root): %v", err)
	}

	info, _ := os.Stat(target)
	// Root-mode: expect 0755
	if perm := info.Mode().Perm(); perm != 0755 {
		t.Errorf("EnsureDir(root) permission = %04o, want 0755", perm)
	}
}

func TestEnsureDirWriteTestFileIsRemoved(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "writecheckdir")

	if err := EnsureDir(target, false); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == ".write-test" {
			t.Error("EnsureDir left .write-test file behind")
		}
	}
}

// --- EnsurePIDFile ---

func TestEnsurePIDFileCreatesParentDirectory(t *testing.T) {
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "run", "vidveil.pid")

	if err := EnsurePIDFile(pidPath, false); err != nil {
		t.Fatalf("EnsurePIDFile: %v", err)
	}

	parent := filepath.Dir(pidPath)
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", parent)
	}
}

func TestEnsurePIDFileIdempotent(t *testing.T) {
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "run", "vidveil.pid")

	if err := EnsurePIDFile(pidPath, false); err != nil {
		t.Fatalf("EnsurePIDFile (first): %v", err)
	}
	if err := EnsurePIDFile(pidPath, false); err != nil {
		t.Errorf("EnsurePIDFile (second): %v", err)
	}
}

func TestEnsurePIDFileDoesNotCreateThePIDFileItself(t *testing.T) {
	// EnsurePIDFile only creates the directory; the file is written by the caller.
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "run", "vidveil.pid")

	if err := EnsurePIDFile(pidPath, false); err != nil {
		t.Fatalf("EnsurePIDFile: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("EnsurePIDFile created the PID file itself; expected it to remain absent")
	}
}

// --- EnsureDir: write-check failure ---

func TestEnsureDirFailsWhenDirectoryNotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not enforced on Windows")
	}
	tmp := t.TempDir()
	target := filepath.Join(tmp, "readonly")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("setup: MkdirAll: %v", err)
	}
	// Make directory read-only so the write-test inside EnsureDir fails.
	if err := os.Chmod(target, 0555); err != nil {
		t.Fatalf("setup: Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(target, 0755) })

	err := EnsureDir(target, false)
	if err == nil {
		t.Error("EnsureDir on read-only directory = nil error, want error")
	}
}

// --- normalizePath (internal, tested via SafePath) ---
// These cases exercise the normalizePath logic through the exported API.

func TestSafePathValidInputs(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo", "foo"},
		{"foo/bar", "foo/bar"},
		{"foo-bar", "foo-bar"},
		{"foo_bar", "foo_bar"},
		{"foo123", "foo123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := SafePath(tt.input)
			if err != nil {
				t.Fatalf("SafePath(%q) = error %v, want nil", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("SafePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafePathBlocksTraversal(t *testing.T) {
	inputs := []string{
		"../etc/passwd",
		"foo/../bar",
		"foo/../../etc",
		"a/b/../../../c",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := SafePath(input)
			if err != ErrPathTraversal {
				t.Errorf("SafePath(%q) = %v, want ErrPathTraversal", input, err)
			}
		})
	}
}

func TestSafePathBlocksInvalidChars(t *testing.T) {
	inputs := []string{
		"UPPERCASE",
		"has space",
		"exclamation!",
		"semi;colon",
		"foo@bar",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := SafePath(input)
			if err == nil {
				t.Errorf("SafePath(%q) = nil error, want an error", input)
			}
		})
	}
}

func TestSafePathBlocksTooLong(t *testing.T) {
	// Path longer than 2048 bytes triggers ErrPathTooLong.
	long := strings.Repeat("a", 2049)
	_, err := SafePath(long)
	if err != ErrPathTooLong {
		t.Errorf("SafePath(2049-byte path) = %v, want ErrPathTooLong", err)
	}
}

func TestSafePathEmptyInput(t *testing.T) {
	// Empty input: validatePath splits on "/" giving one empty segment which is skipped,
	// then normalizePath("") returns "". No error expected.
	got, err := SafePath("")
	if err != nil {
		t.Errorf("SafePath('') = error %v, want nil", err)
	}
	if got != "" {
		t.Errorf("SafePath('') = %q, want empty string", got)
	}
}

// --- validatePathSegment (tested via SafePath) ---

func TestSafePathSegmentTooLong(t *testing.T) {
	// A single segment > 64 chars triggers ErrPathTooLong via validatePathSegment.
	longSeg := strings.Repeat("a", 65)
	_, err := SafePath(longSeg)
	if err != ErrPathTooLong {
		t.Errorf("SafePath(%d-char segment) = %v, want ErrPathTooLong", len(longSeg), err)
	}
}

func TestSafePathSegmentExactlyMaxLength(t *testing.T) {
	// A segment of exactly 64 lowercase chars is valid.
	seg := strings.Repeat("a", 64)
	got, err := SafePath(seg)
	if err != nil {
		t.Fatalf("SafePath(64-char segment) = error %v, want nil", err)
	}
	if got != seg {
		t.Errorf("SafePath(64-char segment) = %q, want %q", got, seg)
	}
}

// --- SafeFilePath ---

func TestSafeFilePathHappyPath(t *testing.T) {
	tmp := t.TempDir()
	got, err := SafeFilePath(tmp, "subdir/file")
	if err != nil {
		t.Fatalf("SafeFilePath: unexpected error: %v", err)
	}
	want := filepath.Join(tmp, "subdir", "file")
	if got != want {
		t.Errorf("SafeFilePath = %q, want %q", got, want)
	}
}

func TestSafeFilePathBlocksTraversal(t *testing.T) {
	tmp := t.TempDir()
	_, err := SafeFilePath(tmp, "../outside")
	if err == nil {
		t.Error("SafeFilePath('..') = nil error, want ErrPathTraversal")
	}
}

func TestSafeFilePathBlocksInvalidSegment(t *testing.T) {
	tmp := t.TempDir()
	_, err := SafeFilePath(tmp, "INVALID/path")
	if err == nil {
		t.Error("SafeFilePath(INVALID) = nil error, want an error")
	}
}

func TestSafeFilePathEmptyUserPath(t *testing.T) {
	// Empty user path: SafePath("") = ("", nil), filepath.Join(base, "") = base.
	// The result should equal the base dir (absPath == absBase branch).
	tmp := t.TempDir()
	got, err := SafeFilePath(tmp, "")
	if err != nil {
		t.Fatalf("SafeFilePath(base, '') = error %v, want nil", err)
	}
	abs, _ := filepath.Abs(tmp)
	if got != abs {
		t.Errorf("SafeFilePath(base, '') = %q, want %q", got, abs)
	}
}

// --- PathSecurityMiddleware ---
// Covers: traversal blocking, path normalization, trailing slash preservation.

func newPassthroughHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
}

func TestPathSecurityMiddlewarePassesCleanPath(t *testing.T) {
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/foo/bar", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Path"); got != "/foo/bar" {
		t.Errorf("X-Path = %q, want /foo/bar", got)
	}
}

func TestPathSecurityMiddlewareBlocksDoubleDot(t *testing.T) {
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/foo/../bar", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("traversal in path: status = %d, want 400", rec.Code)
	}
}

func TestPathSecurityMiddlewareBlocksEncodedDot(t *testing.T) {
	// %2e%2e is URL-encoded ".." — must be blocked even though the decoded path looks safe.
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/foo/%2e%2e/bar", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("encoded traversal: status = %d, want 400", rec.Code)
	}
}

func TestPathSecurityMiddlewareNormalizesDoubleSlash(t *testing.T) {
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/foo//bar", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("double-slash path: status = %d, want 200", rec.Code)
	}
	// path.Clean collapses // → /
	if got := rec.Header().Get("X-Path"); got != "/foo/bar" {
		t.Errorf("X-Path = %q, want /foo/bar", got)
	}
}

func TestPathSecurityMiddlewarePreservesTrailingSlash(t *testing.T) {
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/foo/bar/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("trailing-slash path: status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Path"); got != "/foo/bar/" {
		t.Errorf("X-Path = %q, want /foo/bar/", got)
	}
}

func TestPathSecurityMiddlewareRootPath(t *testing.T) {
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("root path: status = %d, want 200", rec.Code)
	}
	// Root trailing slash must NOT be doubled.
	if got := rec.Header().Get("X-Path"); got != "/" {
		t.Errorf("X-Path = %q, want /", got)
	}
}

func TestPathSecurityMiddlewareEnsuresLeadingSlash(t *testing.T) {
	// Manually craft a request with no leading slash (unusual but possible via RawPath).
	handler := PathSecurityMiddleware(newPassthroughHandler())
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	// Directly mutate path to simulate missing leading slash.
	req.URL.Path = "no-leading-slash"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("no-leading-slash: status = %d, want 200", rec.Code)
	}
	got := rec.Header().Get("X-Path")
	if !strings.HasPrefix(got, "/") {
		t.Errorf("X-Path = %q, want leading /", got)
	}
}
