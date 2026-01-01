// SPDX-License-Identifier: MIT
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

// Get returns the current version string
func Get() string {
return Version
}

// GetFull returns the full version string with v prefix for display
// Per AI.md PART 13 lines 11427-11432
func GetFull() string {
return fmt.Sprintf("vidveil v%s\nBuilt: %s\nGo: %s\nOS/Arch: %s/%s",
Version, BuildTime, GoVersion, GOOS, GOARCH)
}

// GetShort returns version with v prefix
func GetShort() string {
return "v" + Version
}
