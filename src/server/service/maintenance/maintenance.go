// SPDX-License-Identifier: MIT
package maintenance

import (
	"archive/tar"
	"compress/gzip"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// Manager handles maintenance operations
type Manager struct {
	paths   *config.Paths
	version string
}

// New creates a new maintenance manager
func New(configDir, dataDir, version string) *Manager {
	return &Manager{
		paths:   config.GetPaths(configDir, dataDir),
		version: version,
	}
}

// Backup creates a backup of configuration and data
func (m *Manager) Backup(backupFile string) error {
	if backupFile == "" {
		timestamp := time.Now().Format("20060102150405")
		backupFile = filepath.Join(m.paths.Backup, fmt.Sprintf("vidveil-%s.tar.gz", timestamp))
	}

	// Ensure backup directory exists
	backupDir := filepath.Dir(backupFile)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create the backup file
	file, err := os.Create(backupFile)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add config directory
	if err := m.addDirToTar(tarWriter, m.paths.Config, "config"); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}

	// Add data directory
	if err := m.addDirToTar(tarWriter, m.paths.Data, "data"); err != nil {
		return fmt.Errorf("failed to backup data: %w", err)
	}

	fmt.Printf("✅ Backup created: %s\n", backupFile)
	return nil
}

// Restore restores from a backup file
func (m *Manager) Restore(backupFile string) error {
	if backupFile == "" {
		// Find most recent backup
		files, err := filepath.Glob(filepath.Join(m.paths.Backup, "vidveil-*.tar.gz"))
		if err != nil || len(files) == 0 {
			return fmt.Errorf("no backup files found in %s", m.paths.Backup)
		}
		// Most recent by name
		backupFile = files[len(files)-1]
	}

	// Open backup file
	file, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Determine target path
		var targetPath string
		if strings.HasPrefix(header.Name, "config/") {
			targetPath = filepath.Join(m.paths.Config, strings.TrimPrefix(header.Name, "config/"))
		} else if strings.HasPrefix(header.Name, "data/") {
			targetPath = filepath.Join(m.paths.Data, strings.TrimPrefix(header.Name, "data/"))
		} else {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to extract file: %w", err)
			}
			outFile.Close()
			os.Chmod(targetPath, os.FileMode(header.Mode))
		}
	}

	fmt.Printf("✅ Restored from: %s\n", backupFile)
	return nil
}

// CheckUpdate checks for available updates from GitHub releases
func (m *Manager) CheckUpdate() (*UpdateInfo, error) {
	// Fetch latest release from GitHub API
	resp, err := http.Get("https://api.github.com/repos/apimgr/vidveil/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("failed to check updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(m.version, "v")

	info := &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		UpdateAvailable: latestVersion != currentVersion && compareVersions(latestVersion, currentVersion) > 0,
		ReleaseURL:      release.HTMLURL,
		ReleaseNotes:    release.Body,
		PublishedAt:     release.PublishedAt,
	}

	// Find download URL for current platform
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(platform)) {
			info.DownloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return info, nil
}

// ApplyUpdate downloads and applies an update
func (m *Manager) ApplyUpdate(downloadURL string) error {
	if downloadURL == "" {
		info, err := m.CheckUpdate()
		if err != nil {
			return err
		}
		if !info.UpdateAvailable {
			fmt.Println("✅ Already up to date")
			return nil
		}
		if info.DownloadURL == "" {
			return fmt.Errorf("no download available for %s/%s", runtime.GOOS, runtime.GOARCH)
		}
		downloadURL = info.DownloadURL
	}

	// Download new binary
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "vidveil-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Download to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to save update: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Backup current binary
	backupPath := execPath + ".bak"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	fmt.Println("✅ Update applied successfully")
	fmt.Println("   Please restart the service to use the new version")
	return nil
}

// SetMaintenanceMode enables or disables maintenance mode
func (m *Manager) SetMaintenanceMode(enabled bool) error {
	modeFile := filepath.Join(m.paths.Data, "maintenance.flag")

	if enabled {
		file, err := os.Create(modeFile)
		if err != nil {
			return fmt.Errorf("failed to enable maintenance mode: %w", err)
		}
		file.WriteString(time.Now().Format(time.RFC3339))
		file.Close()
		fmt.Println("✅ Maintenance mode enabled")
		fmt.Println("   Server will return 503 for all requests")
	} else {
		if err := os.Remove(modeFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to disable maintenance mode: %w", err)
		}
		fmt.Println("✅ Maintenance mode disabled")
	}

	return nil
}

// IsMaintenanceMode checks if maintenance mode is active
func (m *Manager) IsMaintenanceMode() bool {
	modeFile := filepath.Join(m.paths.Data, "maintenance.flag")
	_, err := os.Stat(modeFile)
	return err == nil
}

// ResetAdminCredentials clears admin password/token and generates new setup token
// per AI.md PART 26
func (m *Manager) ResetAdminCredentials() (string, error) {
	// Generate new setup token
	tokenBytes := make([]byte, 32)
	if _, err := io.ReadFull(cryptoRand.Reader, tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate setup token: %w", err)
	}
	setupToken := fmt.Sprintf("setup_%x", tokenBytes)

	// Write setup token to file (will be read by admin service on startup)
	setupFile := filepath.Join(m.paths.Data, "setup_token")
	if err := os.MkdirAll(filepath.Dir(setupFile), 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.WriteFile(setupFile, []byte(setupToken), 0600); err != nil {
		return "", fmt.Errorf("failed to save setup token: %w", err)
	}

	// Create reset flag file to signal admin service to clear credentials on startup
	resetFile := filepath.Join(m.paths.Data, "admin_reset.flag")
	if err := os.WriteFile(resetFile, []byte(time.Now().Format(time.RFC3339)), 0600); err != nil {
		return "", fmt.Errorf("failed to create reset flag: %w", err)
	}

	return setupToken, nil
}

// SetUpdateBranch sets the update branch (stable, beta, daily) per AI.md PART 14
func (m *Manager) SetUpdateBranch(branch string) error {
	branchFile := filepath.Join(m.paths.Config, "update-branch")

	// Validate branch
	validBranches := map[string]bool{"stable": true, "beta": true, "daily": true}
	if !validBranches[branch] {
		return fmt.Errorf("invalid branch: %s (valid: stable, beta, daily)", branch)
	}

	// Ensure config directory exists
	if err := os.MkdirAll(m.paths.Config, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write branch file
	if err := os.WriteFile(branchFile, []byte(branch), 0644); err != nil {
		return fmt.Errorf("failed to set update branch: %w", err)
	}

	return nil
}

// GetUpdateBranch gets the current update branch (defaults to stable)
func (m *Manager) GetUpdateBranch() string {
	branchFile := filepath.Join(m.paths.Config, "update-branch")
	data, err := os.ReadFile(branchFile)
	// Default per AI.md
	if err != nil {
		return "stable"
	}
	branch := strings.TrimSpace(string(data))
	if branch == "" {
		return "stable"
	}
	return branch
}

// Helper to add directory to tar
func (m *Manager) addDirToTar(tw *tar.Writer, srcDir, prefix string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		tarPath := filepath.Join(prefix, relPath)

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = tarPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Write file content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateInfo contains update information
type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
	ReleaseNotes    string
	DownloadURL     string
	PublishedAt     time.Time
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string          `json:"tag_name"`
	HTMLURL     string          `json:"html_url"`
	Body        string          `json:"body"`
	PublishedAt time.Time       `json:"published_at"`
	Assets      []GitHubAsset   `json:"assets"`
}

// GitHubAsset represents a GitHub release asset
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// compareVersions compares two semantic version strings
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum > bNum {
			return 1
		}
		if aNum < bNum {
			return -1
		}
	}
	return 0
}

// BackupInfo contains information about a backup file
type BackupInfo struct {
Filename string
Path     string
Size     int64
Modified time.Time
SizeHuman string
}

// ListBackups lists all available backups in the backup directory
func (m *Manager) ListBackups() ([]BackupInfo, error) {
backupDir := m.paths.Backup

// Ensure backup directory exists
if err := os.MkdirAll(backupDir, 0755); err != nil {
return nil, fmt.Errorf("failed to create backup directory: %w", err)
}

files, err := os.ReadDir(backupDir)
if err != nil {
return nil, fmt.Errorf("failed to read backup directory: %w", err)
}

var backups []BackupInfo
for _, file := range files {
if file.IsDir() {
continue
}

// Only include .tar.gz files
if !strings.HasSuffix(file.Name(),".tar.gz"){
continue
}

info, err := file.Info()
if err != nil {
continue
}

// Format size as human-readable
sizeHuman := formatBytes(info.Size())

backups = append(backups, BackupInfo{
Filename:  file.Name(),
Path:      filepath.Join(backupDir, file.Name()),
Size:      info.Size(),
Modified:  info.ModTime(),
SizeHuman: sizeHuman,
})
}

// Sort by modification time, newest first
for i := 0; i < len(backups); i++ {
for j := i + 1; j < len(backups); j++ {
if backups[j].Modified.After(backups[i].Modified) {
backups[i], backups[j] = backups[j], backups[i]
}
}
}

return backups, nil
}

// formatBytes formats bytes as human-readable size
func formatBytes(bytes int64) string {
const unit = 1024
if bytes < unit {
return fmt.Sprintf("%d B", bytes)
}
div, exp := int64(unit), 0
for n := bytes / unit; n >= unit; n /= unit {
div *= unit
exp++
}
return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
