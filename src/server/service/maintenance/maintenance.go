// SPDX-License-Identifier: MIT
// AI.md PART 22: Backup & Restore
package maintenance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"golang.org/x/crypto/argon2"
)

// MaintenanceManager handles maintenance operations
type MaintenanceManager struct {
	paths   *config.AppPaths
	version string
}

// BackupOptions configures backup behavior per AI.md PART 22
type BackupOptions struct {
	Filename    string // Output filename (auto-generated if empty)
	Password    string // Encryption password (empty = no encryption)
	IncludeSSL  bool   // Include SSL certificates
	IncludeData bool   // Include data directory
	MaxBackups  int    // Maximum daily backups to keep (0 = use default 1)
	KeepWeekly  int    // Weekly backups (Sunday) to keep (0 = disabled)
	KeepMonthly int    // Monthly backups (1st) to keep (0 = disabled)
	KeepYearly  int    // Yearly backups (Jan 1st) to keep (0 = disabled)
}

// BackupManifest contains backup metadata per AI.md PART 22
type BackupManifest struct {
	Version          string   `json:"version"`
	CreatedAt        string   `json:"created_at"`
	CreatedBy        string   `json:"created_by"`
	AppVersion       string   `json:"app_version"`
	Contents         []string `json:"contents"`
	Encrypted        bool     `json:"encrypted"`
	EncryptionMethod string   `json:"encryption_method,omitempty"`
	Checksum         string   `json:"checksum"`
}

// NewMaintenanceManager creates a new maintenance manager
func NewMaintenanceManager(configDir, dataDir, version string) *MaintenanceManager {
	return &MaintenanceManager{
		paths:   config.GetAppPaths(configDir, dataDir),
		version: version,
	}
}

// Backup creates a backup of configuration and data (simple version)
func (m *MaintenanceManager) Backup(backupFile string) error {
	return m.BackupWithOptions(BackupOptions{
		Filename:    backupFile,
		IncludeData: true,
		MaxBackups:  1, // Default per AI.md PART 22
	})
}

// BackupWithOptions creates a backup with full options per AI.md PART 22
func (m *MaintenanceManager) BackupWithOptions(opts BackupOptions) error {
	// Generate filename per PART 22: vidveil_backup_YYYY-MM-DD_HHMMSS.tar.gz
	backupFile := opts.Filename
	if backupFile == "" {
		timestamp := time.Now().Format("2006-01-02_150405")
		ext := ".tar.gz"
		if opts.Password != "" {
			ext = ".tar.gz.enc"
		}
		backupFile = filepath.Join(m.paths.Backup, fmt.Sprintf("vidveil_backup_%s%s", timestamp, ext))
	}

	// Ensure backup directory exists
	backupDir := filepath.Dir(backupFile)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create archive in memory for encryption support
	var archiveBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&archiveBuf)
	tarWriter := tar.NewWriter(gzWriter)

	// Track contents for manifest
	var contents []string

	// Always include config directory (server.yml, server.db)
	if err := m.addDirToTar(tarWriter, m.paths.Config, "config"); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}
	contents = append(contents, "config/")

	// Include data directory if requested
	if opts.IncludeData {
		if err := m.addDirToTar(tarWriter, m.paths.Data, "data"); err != nil {
			return fmt.Errorf("failed to backup data: %w", err)
		}
		contents = append(contents, "data/")
	}

	// Include SSL certificates if requested
	if opts.IncludeSSL {
		sslDir := filepath.Join(m.paths.Config, "ssl")
		if _, err := os.Stat(sslDir); err == nil {
			if err := m.addDirToTar(tarWriter, sslDir, "ssl"); err != nil {
				return fmt.Errorf("failed to backup ssl: %w", err)
			}
			contents = append(contents, "ssl/")
		}
	}

	// Create manifest
	manifest := BackupManifest{
		Version:    "1.0.0",
		CreatedAt:  time.Now().Format(time.RFC3339),
		CreatedBy:  "system",
		AppVersion: m.version,
		Contents:   contents,
		Encrypted:  opts.Password != "",
	}
	if opts.Password != "" {
		manifest.EncryptionMethod = "AES-256-GCM"
	}

	// Add manifest to archive
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	manifestHeader := &tar.Header{
		Name:    "manifest.json",
		Mode:    0644,
		Size:    int64(len(manifestData)),
		ModTime: time.Now(),
	}
	if err := tarWriter.WriteHeader(manifestHeader); err != nil {
		return fmt.Errorf("failed to write manifest header: %w", err)
	}
	if _, err := tarWriter.Write(manifestData); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Close writers
	tarWriter.Close()
	gzWriter.Close()

	// Calculate checksum
	archiveData := archiveBuf.Bytes()
	checksum := sha256.Sum256(archiveData)
	checksumStr := "sha256:" + hex.EncodeToString(checksum[:])

	// Write final archive (encrypted or plain)
	var finalData []byte
	if opts.Password != "" {
		// Encrypt with AES-256-GCM using Argon2id key derivation
		encrypted, err := m.encryptBackup(archiveData, opts.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt backup: %w", err)
		}
		finalData = encrypted
	} else {
		finalData = archiveData
	}

	// Write to file
	if err := os.WriteFile(backupFile, finalData, 0600); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Verify backup integrity
	if err := m.verifyBackup(backupFile, checksumStr, opts.Password); err != nil {
		os.Remove(backupFile) // Remove failed backup
		return fmt.Errorf("backup verification failed: %w", err)
	}

	// Apply retention policy (default max 1 per AI.md PART 22)
	maxBackups := opts.MaxBackups
	if maxBackups == 0 {
		maxBackups = 1
	}
	if err := m.applyRetentionWithOptions(maxBackups, opts.KeepWeekly, opts.KeepMonthly, opts.KeepYearly); err != nil {
		fmt.Printf("Warning: failed to apply retention policy: %v\n", err)
	}

	fmt.Printf("Backup created: %s\n", backupFile)
	fmt.Printf("Checksum: %s\n", checksumStr)
	return nil
}

// encryptBackup encrypts data using AES-256-GCM with Argon2id key derivation
func (m *MaintenanceManager) encryptBackup(data []byte, password string) ([]byte, error) {
	// Generate salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, salt); err != nil {
		return nil, err
	}

	// Derive key using Argon2id
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(cryptoRand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Format: salt (16) + nonce (12) + ciphertext
	result := make([]byte, len(salt)+len(nonce)+len(ciphertext))
	copy(result[:16], salt)
	copy(result[16:16+len(nonce)], nonce)
	copy(result[16+len(nonce):], ciphertext)

	return result, nil
}

// decryptBackup decrypts AES-256-GCM encrypted data
func (m *MaintenanceManager) decryptBackup(data []byte, password string) ([]byte, error) {
	if len(data) < 28 { // 16 salt + 12 nonce minimum
		return nil, fmt.Errorf("invalid encrypted data")
	}

	// Extract salt, nonce, ciphertext
	salt := data[:16]
	nonce := data[16:28]
	ciphertext := data[28:]

	// Derive key using Argon2id
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password?)")
	}

	return plaintext, nil
}

// verifyBackup verifies backup integrity
func (m *MaintenanceManager) verifyBackup(backupFile, expectedChecksum, password string) error {
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return err
	}

	// Decrypt if encrypted
	if password != "" {
		data, err = m.decryptBackup(data, password)
		if err != nil {
			return err
		}
	}

	// Verify checksum
	checksum := sha256.Sum256(data)
	actualChecksum := "sha256:" + hex.EncodeToString(checksum[:])
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// Verify tar.gz structure
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	hasManifest := false
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar: %w", err)
		}
		if header.Name == "manifest.json" {
			hasManifest = true
		}
	}

	if !hasManifest {
		return fmt.Errorf("missing manifest.json")
	}

	return nil
}

// applyRetention removes old backups to stay under max limit (legacy wrapper)
func (m *MaintenanceManager) applyRetention(maxBackups int) error {
	return m.applyRetentionWithOptions(maxBackups, 0, 0, 0)
}

// applyRetentionWithOptions removes old backups per AI.md PART 22 retention policy
// Priority order: yearly > monthly > weekly > daily
func (m *MaintenanceManager) applyRetentionWithOptions(maxBackups, keepWeekly, keepMonthly, keepYearly int) error {
	if maxBackups <= 0 {
		maxBackups = 1 // Default per PART 22
	}

	backups, err := m.ListBackups()
	if err != nil {
		return err
	}

	// Sort by modified time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Modified.After(backups[j].Modified)
	})

	// Track which backups to keep (by index)
	keep := make(map[int]string) // index -> reason

	// Count trackers
	yearlyKept := 0
	monthlyKept := 0
	weeklyKept := 0
	dailyKept := 0

	// Pass 1: Mark yearly backups (Jan 1st) - highest priority
	for i, b := range backups {
		if keepYearly > 0 && yearlyKept < keepYearly {
			// Check if this is a Jan 1st backup
			if b.Modified.Month() == time.January && b.Modified.Day() == 1 {
				keep[i] = "yearly"
				yearlyKept++
			}
		}
	}

	// Pass 2: Mark monthly backups (1st of month)
	for i, b := range backups {
		if _, ok := keep[i]; ok {
			continue // Already kept
		}
		if keepMonthly > 0 && monthlyKept < keepMonthly {
			// Check if this is a 1st of month backup
			if b.Modified.Day() == 1 {
				keep[i] = "monthly"
				monthlyKept++
			}
		}
	}

	// Pass 3: Mark weekly backups (Sunday)
	for i, b := range backups {
		if _, ok := keep[i]; ok {
			continue // Already kept
		}
		if keepWeekly > 0 && weeklyKept < keepWeekly {
			// Check if this is a Sunday backup
			if b.Modified.Weekday() == time.Sunday {
				keep[i] = "weekly"
				weeklyKept++
			}
		}
	}

	// Pass 4: Mark daily backups (max_backups) - lowest priority
	for i := range backups {
		if _, ok := keep[i]; ok {
			continue // Already kept
		}
		if dailyKept < maxBackups {
			keep[i] = "daily"
			dailyKept++
		}
	}

	// Delete backups not marked for keeping
	for i, b := range backups {
		if _, ok := keep[i]; !ok {
			// Skip incremental files (vidveil-daily.tar.gz, vidveil-hourly.tar.gz)
			if strings.HasPrefix(b.Filename, "vidveil-daily") || strings.HasPrefix(b.Filename, "vidveil-hourly") {
				continue
			}
			if err := os.Remove(b.Path); err != nil {
				fmt.Printf("Warning: failed to delete old backup %s: %v\n", b.Filename, err)
			} else {
				fmt.Printf("Deleted old backup: %s\n", b.Filename)
			}
		}
	}

	return nil
}

// Restore restores from a backup file (simple version)
func (m *MaintenanceManager) Restore(backupFile string) error {
	return m.RestoreWithPassword(backupFile, "")
}

// RestoreWithPassword restores from a backup file with optional decryption
func (m *MaintenanceManager) RestoreWithPassword(backupFile, password string) error {
	if backupFile == "" {
		// Find most recent backup
		files, err := filepath.Glob(filepath.Join(m.paths.Backup, "vidveil_backup_*.tar.gz*"))
		if err != nil || len(files) == 0 {
			return fmt.Errorf("no backup files found in %s", m.paths.Backup)
		}
		// Most recent by name (sorted alphabetically = chronologically with our naming)
		sort.Strings(files)
		backupFile = files[len(files)-1]
	}

	// Read backup file
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Decrypt if .enc extension or password provided
	if strings.HasSuffix(backupFile, ".enc") || password != "" {
		if password == "" {
			return fmt.Errorf("backup is encrypted, password required")
		}
		data, err = m.decryptBackup(data, password)
		if err != nil {
			return fmt.Errorf("failed to decrypt backup: %w", err)
		}
	}

	// Create gzip reader
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
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

		// Skip manifest (informational only)
		if header.Name == "manifest.json" {
			continue
		}

		// Determine target path
		var targetPath string
		if strings.HasPrefix(header.Name, "config/") {
			targetPath = filepath.Join(m.paths.Config, strings.TrimPrefix(header.Name, "config/"))
		} else if strings.HasPrefix(header.Name, "data/") {
			targetPath = filepath.Join(m.paths.Data, strings.TrimPrefix(header.Name, "data/"))
		} else if strings.HasPrefix(header.Name, "ssl/") {
			targetPath = filepath.Join(m.paths.Config, header.Name)
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

	fmt.Printf("Restored from: %s\n", backupFile)
	return nil
}

// CheckUpdate checks for available updates from GitHub releases
func (m *MaintenanceManager) CheckUpdate() (*UpdateInfo, error) {
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
func (m *MaintenanceManager) ApplyUpdate(downloadURL string) error {
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
func (m *MaintenanceManager) SetMaintenanceMode(enabled bool) error {
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
func (m *MaintenanceManager) IsMaintenanceMode() bool {
	modeFile := filepath.Join(m.paths.Data, "maintenance.flag")
	_, err := os.Stat(modeFile)
	return err == nil
}

// ResetAdminCredentials clears admin password/token and generates new setup token
// per AI.md PART 26
func (m *MaintenanceManager) ResetAdminCredentials() (string, error) {
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
func (m *MaintenanceManager) SetUpdateBranch(branch string) error {
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
func (m *MaintenanceManager) GetUpdateBranch() string {
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
func (m *MaintenanceManager) addDirToTar(tw *tar.Writer, srcDir, prefix string) error {
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
func (m *MaintenanceManager) ListBackups() ([]BackupInfo, error) {
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

// Only include .tar.gz and .tar.gz.enc files per AI.md PART 22
if !strings.HasSuffix(file.Name(), ".tar.gz") && !strings.HasSuffix(file.Name(), ".tar.gz.enc") {
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
