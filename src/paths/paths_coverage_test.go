// SPDX-License-Identifier: MIT
package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- pathOverride direct unit tests ---
// The existing test file exercises pathOverride only through GetAppPaths.
// These tests hit all three branches of the function directly.

func TestPathOverrideExplicitWins(t *testing.T) {
	t.Setenv("TEST_PATHOVERRIDE_ENV", "from-env")
	got := pathOverride("explicit", "TEST_PATHOVERRIDE_ENV", "fallback")
	if got != "explicit" {
		t.Errorf("pathOverride(explicit, env, fallback) = %q, want %q", got, "explicit")
	}
}

func TestPathOverrideEnvWinsWhenNoExplicit(t *testing.T) {
	t.Setenv("TEST_PATHOVERRIDE_ENV", "from-env")
	got := pathOverride("", "TEST_PATHOVERRIDE_ENV", "fallback")
	if got != "from-env" {
		t.Errorf("pathOverride('', env-set, fallback) = %q, want %q", got, "from-env")
	}
}

func TestPathOverrideFallbackWhenNeitherSet(t *testing.T) {
	t.Setenv("TEST_PATHOVERRIDE_ENV", "")
	got := pathOverride("", "TEST_PATHOVERRIDE_ENV", "fallback")
	if got != "fallback" {
		t.Errorf("pathOverride('', env-empty, fallback) = %q, want %q", got, "fallback")
	}
}

// Explicit takes priority over env even when env is also set.
func TestPathOverrideExplicitBeatsEnv(t *testing.T) {
	t.Setenv("TEST_PATHOVERRIDE_ENV", "from-env")
	got := pathOverride("explicit", "TEST_PATHOVERRIDE_ENV", "fallback")
	if got != "explicit" {
		t.Errorf("pathOverride: explicit should beat env, got %q", got)
	}
}

// --- Linux exact-path coverage for branches not tested in paths_test.go ---

func TestGetDefaultBackupDirLinuxRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	got := GetDefaultBackupDir(true)
	want := "/mnt/Backups/apimgr/vidveil"
	if got != want {
		t.Errorf("GetDefaultBackupDir(root) = %q, want %q", got, want)
	}
}

func TestGetDefaultPIDFileLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultPIDFile(false)
	want := filepath.Join(home, ".local", "share", "apimgr", "vidveil", "vidveil.pid")
	if got != want {
		t.Errorf("GetDefaultPIDFile(user) = %q, want %q", got, want)
	}
}

func TestGetDefaultSSLDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultSSLDir(false)
	want := filepath.Join(home, ".config", "apimgr", "vidveil", "ssl")
	if got != want {
		t.Errorf("GetDefaultSSLDir(user) = %q, want %q", got, want)
	}
}

func TestGetDefaultSecurityDirLinuxUser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only assertion")
	}
	home, _ := os.UserHomeDir()
	got := GetDefaultSecurityDir(false)
	want := filepath.Join(home, ".local", "share", "apimgr", "vidveil", "security")
	if got != want {
		t.Errorf("GetDefaultSecurityDir(user) = %q, want %q", got, want)
	}
}

// --- validatePath boundary: exactly 2048 chars must pass, 2049 must fail ---

func TestValidatePathExactly2048Chars(t *testing.T) {
	// Build a path of exactly 2048 chars using 32 segments of 63 lowercase chars + separator.
	// Each segment is valid: 63 'a' chars (≤64 limit).
	seg := strings.Repeat("a", 63)
	// 32 segments joined by "/" = 32*63 + 31 separators = 2016 + 31 = 2047 chars.
	// Add one more char to the last segment to reach exactly 2048.
	segments := make([]string, 32)
	for i := range segments {
		segments[i] = seg
	}
	// Replace last segment: 63+1 = 64 chars (still valid per per-segment rule).
	segments[31] = strings.Repeat("a", 64)
	p := strings.Join(segments, "/")
	if len(p) != 2048 {
		t.Fatalf("test setup: path length = %d, want 2048", len(p))
	}
	if err := validatePath(p); err != nil {
		t.Errorf("validatePath(2048-char path) = %v, want nil", err)
	}
}

func TestValidatePathExceeds2048Chars(t *testing.T) {
	long := strings.Repeat("a", 2049)
	if err := validatePath(long); err != ErrPathTooLong {
		t.Errorf("validatePath(2049-char path) = %v, want ErrPathTooLong", err)
	}
}

// --- validatePathSegment direct boundary tests ---

func TestValidatePathSegmentEmpty(t *testing.T) {
	if err := validatePathSegment(""); err != ErrInvalidPath {
		t.Errorf("validatePathSegment('') = %v, want ErrInvalidPath", err)
	}
}

func TestValidatePathSegmentExactly64(t *testing.T) {
	seg := strings.Repeat("a", 64)
	if err := validatePathSegment(seg); err != nil {
		t.Errorf("validatePathSegment(64-char) = %v, want nil", err)
	}
}

func TestValidatePathSegmentExactly65(t *testing.T) {
	seg := strings.Repeat("a", 65)
	if err := validatePathSegment(seg); err != ErrPathTooLong {
		t.Errorf("validatePathSegment(65-char) = %v, want ErrPathTooLong", err)
	}
}

func TestValidatePathSegmentDotIsTraversal(t *testing.T) {
	// The "." case is covered by the explicit check in validatePathSegment.
	// However, validatePath's pre-check fires for ".." first; "." bypasses it.
	// validatePathSegment rejects "." because it doesn't match the regex (no dots).
	if err := validatePathSegment("."); err == nil {
		t.Error("validatePathSegment('.') = nil, want an error")
	}
}

// Regression: the regex `^[a-z0-9_-]+$` rejects ".." before the explicit
// segment == ".." check fires, so the function returns ErrInvalidPath.
// The ErrPathTraversal branch inside validatePathSegment is unreachable dead code.
// This test documents the actual behavior; if the implementation is fixed to
// return ErrPathTraversal the test will flag it and should be updated.
func TestValidatePathSegmentDoubleDotReturnsInvalidPath(t *testing.T) {
	err := validatePathSegment("..")
	if err == nil {
		t.Fatal("validatePathSegment('..') = nil, want an error")
	}
	// The regex rejects ".." before the traversal guard runs — ErrInvalidPath is returned.
	// ErrPathTraversal would be more semantically precise but the current code is ErrInvalidPath.
	if err != ErrInvalidPath {
		t.Errorf("validatePathSegment('..') = %v, want ErrInvalidPath (regex fires before traversal check)", err)
	}
}

// --- normalizePath direct tests (all unexported; exercised here within the package) ---

func TestNormalizePathEmpty(t *testing.T) {
	if got := normalizePath(""); got != "" {
		t.Errorf("normalizePath('') = %q, want ''", got)
	}
}

func TestNormalizePathStripsLeadingSlash(t *testing.T) {
	got := normalizePath("/foo/bar")
	if strings.HasPrefix(got, "/") {
		t.Errorf("normalizePath('/foo/bar') = %q, still has leading slash", got)
	}
}

func TestNormalizePathStripsTrailingSlash(t *testing.T) {
	got := normalizePath("foo/bar/")
	if strings.HasSuffix(got, "/") {
		t.Errorf("normalizePath('foo/bar/') = %q, still has trailing slash", got)
	}
}

func TestNormalizePathCollapsesDoubleSlash(t *testing.T) {
	got := normalizePath("foo//bar")
	if strings.Contains(got, "//") {
		t.Errorf("normalizePath('foo//bar') = %q, still contains //", got)
	}
	if got != "foo/bar" {
		t.Errorf("normalizePath('foo//bar') = %q, want 'foo/bar'", got)
	}
}

func TestNormalizePathRejectsTraversalAfterClean(t *testing.T) {
	// path.Clean of "a/../../b" yields "../b" which still contains ".."
	got := normalizePath("a/../../b")
	if got != "" {
		t.Errorf("normalizePath('a/../../b') = %q, want '' (traversal rejected)", got)
	}
}

func TestNormalizePathSimple(t *testing.T) {
	got := normalizePath("foo/bar/baz")
	if got != "foo/bar/baz" {
		t.Errorf("normalizePath('foo/bar/baz') = %q, want 'foo/bar/baz'", got)
	}
}

// --- GetAppPaths with CONFIG_DIR and DATA_DIR both set via env (no explicit args) ---

func TestGetAppPathsEnvConfigAndData(t *testing.T) {
	t.Setenv("CONFIG_DIR", "/env/cfg")
	t.Setenv("DATA_DIR", "/env/dat")
	p := GetAppPaths("", "")
	if p.Config != "/env/cfg" {
		t.Errorf("Config = %q, want /env/cfg", p.Config)
	}
	if p.Data != "/env/dat" {
		t.Errorf("Data = %q, want /env/dat", p.Data)
	}
}

// Explicit args always beat env regardless of which field.
func TestGetAppPathsExplicitArgsBeatEnvForBothFields(t *testing.T) {
	t.Setenv("CONFIG_DIR", "/env/cfg")
	t.Setenv("DATA_DIR", "/env/dat")
	p := GetAppPaths("/arg/cfg", "/arg/dat")
	if p.Config != "/arg/cfg" {
		t.Errorf("Config = %q, want /arg/cfg (explicit arg beats env)", p.Config)
	}
	if p.Data != "/arg/dat" {
		t.Errorf("Data = %q, want /arg/dat (explicit arg beats env)", p.Data)
	}
}

// When env vars are cleared and no explicit args, GetAppPaths must return non-empty defaults.
func TestGetAppPathsFallsBackToDefaults(t *testing.T) {
	for _, env := range []string{
		"CONFIG_DIR", "DATA_DIR", "CACHE_DIR", "LOG_DIR",
		"BACKUP_DIR", "PID_FILE", "SSL_DIR", "SECURITY_DIR",
	} {
		t.Setenv(env, "")
	}
	p := GetAppPaths("", "")
	if p.Config == "" {
		t.Error("GetAppPaths fallback: Config is empty")
	}
	if p.Data == "" {
		t.Error("GetAppPaths fallback: Data is empty")
	}
	if p.Cache == "" {
		t.Error("GetAppPaths fallback: Cache is empty")
	}
	if p.Log == "" {
		t.Error("GetAppPaths fallback: Log is empty")
	}
}

// --- EnsureDir: write-test cleanup on error path (directory exists but not writable) ---
// Regression: EnsureDir must not leave .write-test behind when the directory is writable.

func TestEnsureDirWriteTestNotLeftAfterSuccess(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "testdir")
	if err := EnsureDir(target, false); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == ".write-test" {
			t.Error("EnsureDir left .write-test behind after successful call")
		}
	}
}

// EnsurePIDFile with a root-level pidPath must create the correct parent.
func TestEnsurePIDFileRootPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not enforced on Windows")
	}
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "pid", "vidveil.pid")
	if err := EnsurePIDFile(pidPath, true); err != nil {
		t.Fatalf("EnsurePIDFile(root): %v", err)
	}
	info, err := os.Stat(filepath.Dir(pidPath))
	if err != nil {
		t.Fatalf("parent dir stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0755 {
		t.Errorf("EnsurePIDFile(root) parent perm = %04o, want 0755", perm)
	}
}
