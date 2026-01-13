// SPDX-License-Identifier: MIT
// AI.md PART 13: Health & Versioning
package version

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// Version is the application version
	// Loaded from release.txt, git tag, or defaults to "dev"
	Version = "dev"

	// CommitID is the git commit hash
	CommitID = "unknown"

	// BuildTime is the build timestamp
	BuildTime = "unknown"

	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()

	// OS/Arch platform
	GOOS   = runtime.GOOS
	GOARCH = runtime.GOARCH
)

func init() {
	// Try to load version from release.txt in project root
	// Per AI.md PART 13 lines 11419-11423
	if v := loadVersionFromFile(); v != "" {
		Version = v
	}
}

// loadVersionFromFile reads version from release.txt
// Searches: current dir, parent dirs, executable dir
func loadVersionFromFile() string {
	// Try current directory first
	if v := readVersionFile("release.txt"); v != "" {
		return v
	}

	// Try executable directory
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if v := readVersionFile(filepath.Join(exeDir, "release.txt")); v != "" {
			return v
		}
	}

	// Try parent directories (up to 3 levels)
	dir, _ := os.Getwd()
	for i := 0; i < 3; i++ {
		if v := readVersionFile(filepath.Join(dir, "release.txt")); v != "" {
			return v
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// readVersionFile reads and trims version from file
func readVersionFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// GetVersion returns the current version string
// Per AI.md PART 1: "Get()" alone is ambiguous - get what?
func GetVersion() string {
	return Version
}

// GetFull returns the full version string for display
// Per AI.md PART 13: --version Output (no v prefix)
func GetFull() string {
	return fmt.Sprintf("vidveil %s\nBuilt: %s\nGo: %s\nOS/Arch: %s/%s",
		Version, BuildTime, GoVersion, GOOS, GOARCH)
}

// GetShort returns version string (no v prefix per AI.md version rules)
func GetShort() string {
	return Version
}

// Info returns version info as a map for JSON responses
func Info() map[string]string {
	return map[string]string{
		"version":    Version,
		"commit":     CommitID,
		"build_time": BuildTime,
		"go_version": GoVersion,
		"os":         GOOS,
		"arch":       GOARCH,
	}
}
