// SPDX-License-Identifier: MIT
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// ProjectOrg is the organization name
	ProjectOrg = "apimgr"
	// ProjectName is the project name
	ProjectName = "vidveil"
)

// Paths holds OS-appropriate directory paths
type Paths struct {
	Config string
	Data   string
	Cache  string
	Log    string
	Backup string
}

// Get returns OS-appropriate paths per AI.md PART 3
func Get(configDir, dataDir string) *Paths {
	isRoot := os.Geteuid() == 0

	paths := &Paths{}

	if configDir != "" {
		paths.Config = configDir
	} else {
		paths.Config = GetDefaultConfigDir(isRoot)
	}

	if dataDir != "" {
		paths.Data = dataDir
	} else {
		paths.Data = GetDefaultDataDir(isRoot)
	}

	paths.Cache = GetDefaultCacheDir(isRoot)
	paths.Log = GetDefaultLogDir(isRoot)
	paths.Backup = GetDefaultBackupDir(isRoot)

	return paths
}

// GetDefaultConfigDir returns OS-appropriate config directory
func GetDefaultConfigDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/etc/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Application Support/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName)
		}
		return filepath.Join(os.Getenv("APPDATA"), ProjectOrg, ProjectName)
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/usr/local/etc/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", ProjectOrg, ProjectName)
	}
}

// GetDefaultDataDir returns OS-appropriate data directory
func GetDefaultDataDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/lib/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Application Support/%s/%s/data", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "data")
		}
		return filepath.Join(os.Getenv("LocalAppData"), ProjectOrg, ProjectName)
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/db/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName)
	}
}

// GetDefaultCacheDir returns OS-appropriate cache directory per AI.md PART 8
func GetDefaultCacheDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/cache/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Caches/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Caches", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "cache")
		}
		return filepath.Join(os.Getenv("LocalAppData"), ProjectOrg, ProjectName, "cache")
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/cache/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", ProjectOrg, ProjectName)
	}
}

// GetDefaultLogDir returns OS-appropriate log directory per AI.md PART 8
func GetDefaultLogDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/log/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		// User log path: ~/.local/log/apimgr/vidveil/ per spec
		return filepath.Join(home, ".local", "log", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Logs/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "logs")
		}
		return filepath.Join(os.Getenv("LocalAppData"), ProjectOrg, ProjectName, "logs")
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/log/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "log", ProjectOrg, ProjectName)
	}
}

// GetDefaultBackupDir returns OS-appropriate backup directory per AI.md PART 8
func GetDefaultBackupDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/mnt/Backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		// User backup path: ~/.local/share/Backups/apimgr/vidveil/ per spec
		return filepath.Join(home, ".local", "share", "Backups", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Backups", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), "Backups", ProjectOrg, ProjectName)
		}
		return filepath.Join(os.Getenv("LocalAppData"), "Backups", ProjectOrg, ProjectName)
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "Backups", ProjectOrg, ProjectName)
	}
}
