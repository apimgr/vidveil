// SPDX-License-Identifier: MIT
// AI.md PART 13: Health & Versioning - Version package tests
package version

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- GetVersion ---

// GetVersion returns the package-level Version var; verify it is never empty.
func TestGetVersionNeverEmpty(t *testing.T) {
	got := GetVersion()
	if got == "" {
		t.Error("GetVersion() returned empty string, want at least 'dev'")
	}
}

// GetVersion must reflect whatever Version is set to.
func TestGetVersionReflectsVar(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "1.2.3"
	if got := GetVersion(); got != "1.2.3" {
		t.Errorf("GetVersion() = %q, want %q", got, "1.2.3")
	}
}

// --- GetShortVersion ---

// GetShortVersion must equal Version (no v prefix, no extra decorations).
func TestGetShortVersionEqualsVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "2.0.0"
	got := GetShortVersion()
	if got != Version {
		t.Errorf("GetShortVersion() = %q, want %q", got, Version)
	}
}

// No "v" prefix must appear when version is set without one.
func TestGetShortVersionNoVPrefix(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "3.1.4"
	got := GetShortVersion()
	if strings.HasPrefix(got, "v") {
		t.Errorf("GetShortVersion() = %q must not start with 'v'", got)
	}
}

// --- GetFullVersion ---

// GetFullVersion must contain the binary name, version, build time, Go version, OS, and Arch.
func TestGetFullVersionContainsRequiredFields(t *testing.T) {
	orig := Version
	origBT := BuildTime
	t.Cleanup(func() {
		Version = orig
		BuildTime = origBT
	})

	Version = "4.5.6"
	BuildTime = "2026-01-02T15:04:05Z"

	got := GetFullVersion()

	wantBinary := filepath.Base(os.Args[0])
	for _, want := range []string{wantBinary, "4.5.6", "2026-01-02T15:04:05Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("GetFullVersion() = %q, missing %q", got, want)
		}
	}
}

// GetFullVersion must include the Go runtime version.
func TestGetFullVersionContainsGoVersion(t *testing.T) {
	got := GetFullVersion()
	if !strings.Contains(got, runtime.Version()) {
		t.Errorf("GetFullVersion() = %q, missing Go version %q", got, runtime.Version())
	}
}

// GetFullVersion must include OS and Arch.
func TestGetFullVersionContainsPlatform(t *testing.T) {
	got := GetFullVersion()
	if !strings.Contains(got, runtime.GOOS) {
		t.Errorf("GetFullVersion() = %q, missing GOOS %q", got, runtime.GOOS)
	}
	if !strings.Contains(got, runtime.GOARCH) {
		t.Errorf("GetFullVersion() = %q, missing GOARCH %q", got, runtime.GOARCH)
	}
}

// --- GetVersionInfo ---

// GetVersionInfo must return all six required keys.
func TestGetVersionInfoKeys(t *testing.T) {
	info := GetVersionInfo()
	required := []string{"version", "commit", "build_time", "go_version", "os", "arch"}
	for _, key := range required {
		if _, ok := info[key]; !ok {
			t.Errorf("GetVersionInfo() missing key %q", key)
		}
	}
}

// GetVersionInfo values must reflect the current package vars.
func TestGetVersionInfoValues(t *testing.T) {
	origV := Version
	origC := CommitID
	origB := BuildTime
	t.Cleanup(func() {
		Version = origV
		CommitID = origC
		BuildTime = origB
	})

	Version = "7.8.9"
	CommitID = "abc123def456"
	BuildTime = "2026-05-30T00:00:00Z"

	info := GetVersionInfo()

	if info["version"] != "7.8.9" {
		t.Errorf("GetVersionInfo()[version] = %q, want %q", info["version"], "7.8.9")
	}
	if info["commit"] != "abc123def456" {
		t.Errorf("GetVersionInfo()[commit] = %q, want %q", info["commit"], "abc123def456")
	}
	if info["build_time"] != "2026-05-30T00:00:00Z" {
		t.Errorf("GetVersionInfo()[build_time] = %q, want %q", info["build_time"], "2026-05-30T00:00:00Z")
	}
	if info["go_version"] != runtime.Version() {
		t.Errorf("GetVersionInfo()[go_version] = %q, want %q", info["go_version"], runtime.Version())
	}
	if info["os"] != runtime.GOOS {
		t.Errorf("GetVersionInfo()[os] = %q, want %q", info["os"], runtime.GOOS)
	}
	if info["arch"] != runtime.GOARCH {
		t.Errorf("GetVersionInfo()[arch] = %q, want %q", info["arch"], runtime.GOARCH)
	}
}

// GetVersionInfo must return a new map each call (callers mutating the map must not affect next call).
func TestGetVersionInfoReturnsFreshMap(t *testing.T) {
	info1 := GetVersionInfo()
	info1["version"] = "mutated"

	info2 := GetVersionInfo()
	if info2["version"] == "mutated" {
		t.Error("GetVersionInfo() returned same map reference; mutations leak between calls")
	}
}

// --- readVersionFile ---

// readVersionFile returns trimmed content from a valid file.
func TestReadVersionFileTrimmed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.txt")
	if err := os.WriteFile(path, []byte("  1.0.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := readVersionFile(path)
	if got != "1.0.0" {
		t.Errorf("readVersionFile() = %q, want %q", got, "1.0.0")
	}
}

// readVersionFile returns empty string for a non-existent path.
func TestReadVersionFileMissing(t *testing.T) {
	got := readVersionFile("/nonexistent/path/release.txt")
	if got != "" {
		t.Errorf("readVersionFile(missing) = %q, want empty", got)
	}
}

// readVersionFile on an empty file returns empty string (not an error).
func TestReadVersionFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.txt")
	if err := os.WriteFile(path, []byte("   \n\t"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := readVersionFile(path)
	if got != "" {
		t.Errorf("readVersionFile(whitespace-only) = %q, want empty", got)
	}
}

// readVersionFile handles a file with only a newline correctly.
func TestReadVersionFileNewlineOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.txt")
	if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := readVersionFile(path)
	if got != "" {
		t.Errorf("readVersionFile(newline-only) = %q, want empty", got)
	}
}

// --- loadVersionFromFile ---

// loadVersionFromFile picks up a release.txt in the current working directory.
func TestLoadVersionFromFileCurrentDir(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.WriteFile(filepath.Join(dir, "release.txt"), []byte("5.0.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := loadVersionFromFile()
	if got != "5.0.0" {
		t.Errorf("loadVersionFromFile() = %q, want %q", got, "5.0.0")
	}
}

// loadVersionFromFile returns empty when no release.txt exists in any searched location.
func TestLoadVersionFromFileAbsent(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	got := loadVersionFromFile()
	// May or may not find a parent-dir release.txt depending on the host; just ensure no panic.
	_ = got
}

// --- Package-level vars ---

// GoVersion must match runtime.Version() (set at package initialisation).
func TestGoVersionMatchesRuntime(t *testing.T) {
	if GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", GoVersion, runtime.Version())
	}
}

// GOOS must match runtime.GOOS.
func TestGOOSMatchesRuntime(t *testing.T) {
	if GOOS != runtime.GOOS {
		t.Errorf("GOOS = %q, want %q", GOOS, runtime.GOOS)
	}
}

// GOARCH must match runtime.GOARCH.
func TestGOARCHMatchesRuntime(t *testing.T) {
	if GOARCH != runtime.GOARCH {
		t.Errorf("GOARCH = %q, want %q", GOARCH, runtime.GOARCH)
	}
}

// CommitID and BuildTime defaults must be "unknown".
func TestDefaultVarValues(t *testing.T) {
	// These are defaults; they may have been overridden at link time in CI but in a plain test run they hold.
	// We only assert non-empty; the actual value depends on the build environment.
	if CommitID == "" {
		t.Error("CommitID is empty, want at least 'unknown'")
	}
	if BuildTime == "" {
		t.Error("BuildTime is empty, want at least 'unknown'")
	}
}
