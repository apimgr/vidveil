// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - OS-specific paths
package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	projectOrg  = "apimgr"
	projectName = "vidveil"
)

// ConfigDir returns the CLI config directory
// Linux/macOS: ~/.config/apimgr/vidveil/
// Windows: %APPDATA%\apimgr\vidveil\
func ConfigDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), projectOrg, projectName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", projectOrg, projectName)
}

// DataDir returns the CLI data directory
// Linux/macOS: ~/.local/share/apimgr/vidveil/
// Windows: %LOCALAPPDATA%\apimgr\vidveil\data\
func DataDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), projectOrg, projectName, "data")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", projectOrg, projectName)
}

// CacheDir returns the CLI cache directory
// Linux/macOS: ~/.cache/apimgr/vidveil/
// Windows: %LOCALAPPDATA%\apimgr\vidveil\cache\
func CacheDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), projectOrg, projectName, "cache")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", projectOrg, projectName)
}

// LogDir returns the CLI log directory
// Linux/macOS: ~/.local/log/apimgr/vidveil/
// Windows: %LOCALAPPDATA%\apimgr\vidveil\log\
func LogDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), projectOrg, projectName, "log")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "log", projectOrg, projectName)
}

// ConfigFile returns the CLI config file path
func ConfigFile() string {
	return filepath.Join(ConfigDir(), "cli.yml")
}

// ResolveConfigFilePath resolves the CLI --config flag to a config file path.
// Bare names resolve inside ConfigDir(), absolute paths are used as-is,
// and missing yaml extensions default to .yml after checking existing files.
func ResolveConfigFilePath(configFlag string) string {
	if configFlag == "" {
		return ConfigFile()
	}

	if strings.HasPrefix(configFlag, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			configFlag = filepath.Join(home, configFlag[2:])
		}
	}

	if filepath.IsAbs(configFlag) {
		return resolveConfigYAMLExtension(configFlag)
	}

	return resolveConfigYAMLExtension(filepath.Join(ConfigDir(), configFlag))
}

// TokenFile returns the CLI token file path
// Per AI.md PART 33: Token stored separately from config for security
func TokenFile() string {
	return filepath.Join(ConfigDir(), "token")
}

// LogFile returns the CLI log file path
func LogFile() string {
	return filepath.Join(LogDir(), "cli.log")
}

func resolveConfigYAMLExtension(path string) string {
	extension := filepath.Ext(path)
	if extension == ".yml" || extension == ".yaml" {
		return path
	}

	if extension == "" {
		if fileExists(path + ".yml") {
			return path + ".yml"
		}
		if fileExists(path + ".yaml") {
			return path + ".yaml"
		}
		return path + ".yml"
	}

	return path
}

func fileExists(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !fileInfo.IsDir()
}

// EnsureClientDirs creates all CLI directories with correct permissions.
// Called on every startup before any file operations.
func EnsureClientDirs() error {
	dirs := []string{
		ConfigDir(),
		DataDir(),
		CacheDir(),
		LogDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		// Ensure permissions even if dir existed
		if err := os.Chmod(dir, 0700); err != nil {
			return err
		}
		if err := EnsurePathOwnership(dir); err != nil {
			return err
		}
	}
	return nil
}
