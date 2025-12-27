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

// GetDefaultLogDir returns OS-appropriate log directory
func GetDefaultLogDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/log/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, "logs")
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
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, "logs")
	}
}

// GetDefaultBackupDir returns OS-appropriate backup directory
func GetDefaultBackupDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/mnt/Backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "backups", ProjectOrg, ProjectName)
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
		return filepath.Join(home, ".local", "backups", ProjectOrg, ProjectName)
	}
}
