// SPDX-License-Identifier: MIT
package path

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

// AppPaths holds OS-appropriate directory paths per AI.md PART 4.
type AppPaths struct {
	Config   string
	Data     string
	Cache    string
	Log      string
	Backup   string
	PIDFile  string
	SSL      string
	Security string
}

func pathOverride(explicit, envName, fallback string) string {
	if explicit != "" {
		return explicit
	}

	if envValue := os.Getenv(envName); envValue != "" {
		return envValue
	}

	return fallback
}

// GetAppPaths returns OS-appropriate paths per AI.md PART 4.
func GetAppPaths(configDir, dataDir string) *AppPaths {
	isRoot := os.Geteuid() == 0

	return &AppPaths{
		Config:   pathOverride(configDir, "CONFIG_DIR", GetDefaultConfigDir(isRoot)),
		Data:     pathOverride(dataDir, "DATA_DIR", GetDefaultDataDir(isRoot)),
		Cache:    pathOverride("", "CACHE_DIR", GetDefaultCacheDir(isRoot)),
		Log:      pathOverride("", "LOG_DIR", GetDefaultLogDir(isRoot)),
		Backup:   pathOverride("", "BACKUP_DIR", GetDefaultBackupDir(isRoot)),
		PIDFile:  pathOverride("", "PID_FILE", GetDefaultPIDFile(isRoot)),
		SSL:      pathOverride("", "SSL_DIR", GetDefaultSSLDir(isRoot)),
		Security: pathOverride("", "SECURITY_DIR", GetDefaultSecurityDir(isRoot)),
	}
}

// GetDatabaseDir returns the SQLite database directory.
func GetDatabaseDir(dataDir string) string {
	if envValue := os.Getenv("DATABASE_DIR"); envValue != "" {
		return envValue
	}

	return filepath.Join(dataDir, "db")
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

// GetDefaultCacheDir returns OS-appropriate cache directory per AI.md PART 4
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

// GetDefaultLogDir returns OS-appropriate log directory per AI.md PART 4
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

// GetDefaultPIDFile returns the OS-appropriate PID file path per AI.md PART 4.
func GetDefaultPIDFile(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/run/%s/%s.pid", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, ProjectName+".pid")
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/var/run/%s/%s.pid", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName, ProjectName+".pid")
	case "windows":
		// Windows uses the Service Manager; no PID file
		return ""
	default:
		// BSD
		if isRoot {
			return fmt.Sprintf("/var/run/%s/%s.pid", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, ProjectName+".pid")
	}
}

// GetDefaultSSLDir returns the OS-appropriate SSL directory per AI.md PART 4.
// Sub-directories letsencrypt/ and local/ live inside this path.
func GetDefaultSSLDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/etc/%s/%s/ssl", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", ProjectOrg, ProjectName, "ssl")
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Application Support/%s/%s/ssl", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName, "ssl")
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "ssl")
		}
		return filepath.Join(os.Getenv("APPDATA"), ProjectOrg, ProjectName, "ssl")
	default:
		// BSD
		if isRoot {
			return fmt.Sprintf("/usr/local/etc/%s/%s/ssl", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", ProjectOrg, ProjectName, "ssl")
	}
}

// GetDefaultSecurityDir returns the OS-appropriate security directory per AI.md PART 4.
// Sub-directories geoip/, blocklists/, cve/, trivy/ live inside this path.
func GetDefaultSecurityDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/lib/%s/%s/security", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, "security")
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Application Support/%s/%s/data/security", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName, "data", "security")
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "data", "security")
		}
		return filepath.Join(os.Getenv("LocalAppData"), ProjectOrg, ProjectName, "security")
	default:
		// BSD
		if isRoot {
			return fmt.Sprintf("/var/db/%s/%s/security", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, "security")
	}
}

// GetDefaultBackupDir returns OS-appropriate backup directory per AI.md PART 4
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
