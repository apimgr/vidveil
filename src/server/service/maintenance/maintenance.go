// SPDX-License-Identifier: MIT
// AI.md PART 21: Backup & Restore | PART 22: Update Command
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
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
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

// BackupOptions configures backup behavior per AI.md PART 21
type BackupOptions struct {
	// Filename is output filename (auto-generated if empty)
	Filename string
	// Password is encryption password (empty = no encryption)
	Password string
	// IncludeSSL determines if SSL certificates are included
	IncludeSSL bool
	// IncludeData determines if data directory is included
	IncludeData bool
	// MaxBackups is maximum daily backups to keep (0 = use default 1)
	MaxBackups int
	// KeepWeekly is weekly backups (Sunday) to keep (0 = disabled)
	KeepWeekly int
	// KeepMonthly is monthly backups (1st) to keep (0 = disabled)
	KeepMonthly int
	// KeepYearly is yearly backups (Jan 1st) to keep (0 = disabled)
	KeepYearly int
	// MaxTotalSize is a hard cap on total backup directory size per AI.md PART 21.
	// Accepts a percentage ("10%") or absolute size ("50G", "50GB"); "" or "0" disables the cap.
	MaxTotalSize string
}

// BackupManifest contains backup metadata per AI.md PART 21
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
	// Default per AI.md PART 21: MaxBackups=1
	return m.BackupWithOptions(BackupOptions{
		Filename:    backupFile,
		IncludeData: true,
		MaxBackups:  1,
	})
}

// BackupIncremental creates an hourly incremental backup per AI.md PART 21.
// Always writes to a fixed filename so only one file is kept (replaced each hour).
func (m *MaintenanceManager) BackupIncremental(backupFile string) error {
	if backupFile == "" {
		// Fixed filename per PART 21: vidveil-hourly.tar.gz — always 1 file
		backupFile = filepath.Join(m.paths.Backup, "vidveil-hourly.tar.gz")
	}
	return m.BackupWithOptions(BackupOptions{
		Filename:    backupFile,
		IncludeData: true,
		MaxBackups:  1,
	})
}

// BackupDailyFull creates the scheduled backup_daily run per AI.md PART 21
// "Backup Creation Flow" / "Backup Files Created": a retention-controlled full
// backup named {project_name}_backup_YYYY-MM-DD.tar.gz[.enc] plus a daily
// incremental named {project_name}-daily.tar.gz[.enc] (always exactly 1 file,
// replaced each run and never subject to count-based retention deletion).
// opts carries encryption/inclusion/retention settings sourced from the caller's
// server.backup.retention configuration.
func (m *MaintenanceManager) BackupDailyFull(opts BackupOptions) error {
	ext := ".tar.gz"
	if opts.Password != "" {
		ext = ".tar.gz.enc"
	}

	// Step 3-4: full backup, retention-controlled, date-only filename per PART 21.
	fullOpts := opts
	if fullOpts.Filename == "" {
		dateStr := time.Now().Format("2006-01-02")
		fullOpts.Filename = filepath.Join(m.paths.Backup, fmt.Sprintf("vidveil_backup_%s%s", dateStr, ext))
	}
	if err := m.BackupWithOptions(fullOpts); err != nil {
		return fmt.Errorf("daily full backup failed: %w", err)
	}

	// Step 5-6: daily incremental, fixed filename, always exactly 1 file.
	dailyOpts := opts
	dailyOpts.Filename = filepath.Join(m.paths.Backup, fmt.Sprintf("vidveil-daily%s", ext))
	dailyOpts.MaxBackups = 1
	dailyOpts.KeepWeekly = 0
	dailyOpts.KeepMonthly = 0
	dailyOpts.KeepYearly = 0
	dailyOpts.MaxTotalSize = ""
	if err := m.BackupWithOptions(dailyOpts); err != nil {
		return fmt.Errorf("daily incremental backup failed: %w", err)
	}

	return nil
}

// BackupWithOptions creates a backup with full options per AI.md PART 21
func (m *MaintenanceManager) BackupWithOptions(opts BackupOptions) error {
	// Generate filename per PART 21: vidveil_backup_YYYY-MM-DD_HHMMSS.tar.gz
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

	// Run retention sweep BEFORE creating the new archive per AI.md PART 21
	// Backup Creation Flow step 1 (prevents transient over-capacity of the backup dir).
	maxBackups := opts.MaxBackups
	if maxBackups == 0 {
		maxBackups = 1
	}
	if err := m.applyRetentionWithOptions(maxBackups, opts.KeepWeekly, opts.KeepMonthly, opts.KeepYearly, opts.MaxTotalSize); err != nil {
		fmt.Printf("Warning: failed to apply retention policy: %v\n", err)
	}

	// Disk-space precheck per AI.md PART 21: abort (do NOT create the backup) if
	// free space < 2x the most recent existing backup, or disk usage > 90%.
	if err := m.checkDiskSpace(backupDir); err != nil {
		fmt.Printf("backup.skipped_disk_full: %v\n", err)
		return err
	}

	// Create archive in memory for encryption support
	var archiveBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&archiveBuf)
	tarWriter := tar.NewWriter(gzWriter)

	// Track contents for manifest, and a content-addressable hash (name+content per
	// regular file) so the manifest checksum is independent of tar/gzip metadata.
	var contents []string
	contentHash := sha256.New()

	// Always include config directory (server.yml, server.db)
	if err := m.addDirToTar(tarWriter, m.paths.Config, "config", contentHash); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}
	contents = append(contents, "config/")

	// Include data directory if requested
	if opts.IncludeData {
		if err := m.addDirToTar(tarWriter, m.paths.Data, "data", contentHash); err != nil {
			return fmt.Errorf("failed to backup data: %w", err)
		}
		contents = append(contents, "data/")
	}

	// Include SSL certificates if requested
	if opts.IncludeSSL {
		sslDir := m.paths.SSL
		if _, err := os.Stat(sslDir); err == nil {
			if err := m.addDirToTar(tarWriter, sslDir, "ssl", contentHash); err != nil {
				return fmt.Errorf("failed to backup ssl: %w", err)
			}
			contents = append(contents, "ssl/")
		}
	}

	// Create manifest
	manifestChecksum := "sha256:" + hex.EncodeToString(contentHash.Sum(nil))
	manifest := BackupManifest{
		Version:    "1.0.0",
		CreatedAt:  time.Now().Format(time.RFC3339),
		CreatedBy:  "system",
		AppVersion: m.version,
		Contents:   contents,
		Encrypted:  opts.Password != "",
		Checksum:   manifestChecksum,
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
		// Remove failed backup
		os.Remove(backupFile)
		return fmt.Errorf("backup verification failed: %w", err)
	}

	fmt.Printf("Backup created: %s\n", backupFile)
	fmt.Printf("Checksum: %s\n", checksumStr)
	return nil
}

// checkDiskSpace aborts the backup per AI.md PART 21 if free space is less than
// 2x the most recent existing backup's size, or if disk usage exceeds 90%.
func (m *MaintenanceManager) checkDiskSpace(backupDir string) error {
	total, free, err := diskSpace(backupDir)
	if err != nil {
		// Can't determine disk space (e.g. unsupported platform) - don't block the backup.
		return nil
	}

	if total > 0 {
		used := total - free
		if float64(used)/float64(total) > 0.90 {
			return fmt.Errorf("disk usage exceeds 90%% threshold, aborting backup")
		}
	}

	backups, err := m.ListBackups()
	if err == nil && len(backups) > 0 {
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].Modified.After(backups[j].Modified)
		})
		mostRecentSize := uint64(backups[0].Size)
		if mostRecentSize > 0 && free < 2*mostRecentSize {
			return fmt.Errorf("free space (%s) is less than 2x most recent backup size (%s), aborting backup",
				formatBytes(int64(free)), formatBytes(int64(mostRecentSize)))
		}
	}

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
	// 16 salt + 12 nonce minimum
	if len(data) < 28 {
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

// verifyBackup verifies backup integrity per AI.md PART 21 (7 checks):
// 1. File exists  2. Size > 0  3. Checksum valid  4. Decrypt test
// 5. Manifest readable  6. Content extraction  7. Database integrity
func (m *MaintenanceManager) verifyBackup(backupFile, expectedChecksum, password string) error {
	// Check 1: File exists
	info, err := os.Stat(backupFile)
	if err != nil {
		return fmt.Errorf("backup file missing: %w", err)
	}

	// Check 2: Size > 0
	if info.Size() == 0 {
		return fmt.Errorf("backup file is empty")
	}

	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Check 4: Decrypt test (if encrypted)
	if password != "" {
		data, err = m.decryptBackup(data, password)
		if err != nil {
			return fmt.Errorf("decrypt test failed: %w", err)
		}
	}

	// Check 3: Checksum valid — SHA-256 of (decrypted) archive must match manifest
	checksum := sha256.Sum256(data)
	actualChecksum := "sha256:" + hex.EncodeToString(checksum[:])
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// Checks 5, 6, 7: extract to temp dir, parse manifest, verify database
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		return fmt.Errorf("failed to create temp base dir: %w", err)
	}
	tmpDir, err := os.MkdirTemp("/tmp/apimgr", "vidveil-XXXXXX")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var manifestData []byte
	var dbData []byte

	// Check 6: Content extraction — test-extract every entry to temp dir
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar: %w", err)
		}

		// Sanitize path to prevent path traversal
		cleanName := filepath.Clean(header.Name)
		destPath := filepath.Join(tmpDir, cleanName)

		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(destPath, 0700); err != nil {
				return fmt.Errorf("failed to extract directory %s: %w", header.Name, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
			return fmt.Errorf("failed to create parent for %s: %w", header.Name, err)
		}

		contents, err := io.ReadAll(tarReader)
		if err != nil {
			return fmt.Errorf("failed to extract %s: %w", header.Name, err)
		}

		if err := os.WriteFile(destPath, contents, 0600); err != nil {
			return fmt.Errorf("failed to write extracted %s: %w", header.Name, err)
		}

		if cleanName == "manifest.json" {
			manifestData = contents
		}
		if cleanName == "server.db" {
			dbData = contents
		}
	}

	// Check 5: Manifest readable — must be present and JSON-parseable
	if manifestData == nil {
		return fmt.Errorf("missing manifest.json")
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("manifest.json not parseable: %w", err)
	}
	if manifest.Version == "" {
		return fmt.Errorf("manifest.json missing version field")
	}

	// Check 7: Database integrity — verify SQLite magic header
	if dbData != nil {
		if err := verifySQLiteIntegrity(dbData); err != nil {
			return fmt.Errorf("database integrity check failed: %w", err)
		}
	}

	return nil
}

// verifySQLiteIntegrity checks the SQLite magic header per AI.md PART 21
// The first 16 bytes of every SQLite file must be "SQLite format 3\x00"
func verifySQLiteIntegrity(data []byte) error {
	const sqliteMagic = "SQLite format 3\x00"
	if len(data) < len(sqliteMagic) {
		return fmt.Errorf("file too small to be a SQLite database (%d bytes)", len(data))
	}
	if string(data[:len(sqliteMagic)]) != sqliteMagic {
		return fmt.Errorf("invalid SQLite header")
	}
	return nil
}

// applyRetention removes old backups to stay under max limit (legacy wrapper)
func (m *MaintenanceManager) applyRetention(maxBackups int) error {
	return m.applyRetentionWithOptions(maxBackups, 0, 0, 0, "")
}

// applyRetentionWithOptions removes old backups per AI.md PART 21 retention policy
// Priority order: yearly > monthly > weekly > daily, followed by a max_total_size hard cap.
func (m *MaintenanceManager) applyRetentionWithOptions(maxBackups, keepWeekly, keepMonthly, keepYearly int, maxTotalSize string) error {
	if maxBackups <= 0 {
		// Default per PART 21
		maxBackups = 1
	}

	backups, err := m.ListBackups()
	if err != nil {
		return err
	}

	// Sort by modified time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Modified.After(backups[j].Modified)
	})

	// Track which backups to keep (index -> reason)
	keep := make(map[int]string)

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
		// Already kept
		if _, ok := keep[i]; ok {
			continue
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
		// Already kept
		if _, ok := keep[i]; ok {
			continue
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
		// Already kept
		if _, ok := keep[i]; ok {
			continue
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

	// Enforce max_total_size hard cap per AI.md PART 21: delete oldest-first (never the
	// vidveil-daily/vidveil-hourly incrementals) until the backup dir is back under the cap.
	if err := m.enforceMaxTotalSize(maxTotalSize); err != nil {
		fmt.Printf("Warning: failed to enforce max_total_size: %v\n", err)
	}

	return nil
}

// parseSizeString parses a max_total_size value per AI.md PART 21. Accepted forms:
//   - percentage: "10%" (percentage of total disk size for the backup dir's filesystem)
//   - suffixed size: "50G", "50GB", "500M", "500MB", "1T", "1TB" (case-insensitive)
//   - bare bytes: "1048576"
//   - "" or "0" disables the cap (returns 0, false, nil)
func (m *MaintenanceManager) parseSizeString(s, path string) (bytesLimit uint64, enabled bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, false, nil
	}

	if strings.HasSuffix(s, "%") {
		pctStr := strings.TrimSuffix(s, "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return 0, false, fmt.Errorf("invalid percentage %q: %w", s, err)
		}
		total, _, err := diskSpace(path)
		if err != nil {
			return 0, false, fmt.Errorf("failed to determine disk size: %w", err)
		}
		return uint64(pct / 100 * float64(total)), true, nil
	}

	upper := strings.ToUpper(s)
	multipliers := []struct {
		suffixes []string
		mult     uint64
	}{
		{[]string{"TB", "T"}, 1024 * 1024 * 1024 * 1024},
		{[]string{"GB", "G"}, 1024 * 1024 * 1024},
		{[]string{"MB", "M"}, 1024 * 1024},
		{[]string{"KB", "K"}, 1024},
	}
	for _, m := range multipliers {
		for _, suf := range m.suffixes {
			if strings.HasSuffix(upper, suf) {
				numStr := strings.TrimSpace(upper[:len(upper)-len(suf)])
				n, err := strconv.ParseFloat(numStr, 64)
				if err != nil {
					return 0, false, fmt.Errorf("invalid size %q: %w", s, err)
				}
				return uint64(n * float64(m.mult)), true, nil
			}
		}
	}

	// Bare bytes
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid size %q: %w", s, err)
	}
	return n, true, nil
}

// enforceMaxTotalSize deletes oldest backups (never vidveil-daily/vidveil-hourly incrementals)
// until the backup directory's total size is under the max_total_size cap.
func (m *MaintenanceManager) enforceMaxTotalSize(maxTotalSize string) error {
	limit, enabled, err := m.parseSizeString(maxTotalSize, m.paths.Backup)
	if err != nil {
		return err
	}
	if !enabled || limit == 0 {
		return nil
	}

	backups, err := m.ListBackups()
	if err != nil {
		return err
	}

	var total int64
	for _, b := range backups {
		total += b.Size
	}
	if uint64(total) <= limit {
		return nil
	}

	// Oldest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Modified.Before(backups[j].Modified)
	})

	for _, b := range backups {
		if uint64(total) <= limit {
			break
		}
		if strings.HasPrefix(b.Filename, "vidveil-daily") || strings.HasPrefix(b.Filename, "vidveil-hourly") {
			continue
		}
		if err := os.Remove(b.Path); err != nil {
			fmt.Printf("Warning: failed to delete old backup %s: %v\n", b.Filename, err)
			continue
		}
		fmt.Printf("Deleted old backup (max_total_size cap): %s\n", b.Filename)
		total -= b.Size
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

	// Phase 1: decrypt + parse fully into memory, validating before touching disk.
	entry, err := loadRestoreArchive(data)
	if err != nil {
		return err
	}

	if entry.manifest.Version == "" {
		return fmt.Errorf("invalid backup: manifest.json missing or has empty version")
	}

	if entry.manifest.Checksum != "" {
		computed := "sha256:" + hex.EncodeToString(entry.contentHash.Sum(nil))
		if computed != entry.manifest.Checksum {
			return fmt.Errorf("backup checksum mismatch: manifest says %s, computed %s", entry.manifest.Checksum, computed)
		}
	}

	if entry.manifest.AppVersion != "" && entry.manifest.AppVersion != m.version {
		fmt.Printf("Warning: backup was created by app version %s, current version is %s\n", entry.manifest.AppVersion, m.version)
	}

	// Phase 2: all checks passed - write buffered contents to real paths.
	for _, f := range entry.files {
		var targetPath string
		switch {
		case strings.HasPrefix(f.name, "config/"):
			targetPath = filepath.Join(m.paths.Config, strings.TrimPrefix(f.name, "config/"))
		case strings.HasPrefix(f.name, "data/"):
			targetPath = filepath.Join(m.paths.Data, strings.TrimPrefix(f.name, "data/"))
		case strings.HasPrefix(f.name, "ssl/"):
			targetPath = filepath.Join(m.paths.SSL, strings.TrimPrefix(f.name, "ssl/"))
		default:
			continue
		}

		if f.isDir {
			if err := os.MkdirAll(targetPath, os.FileMode(f.mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
		if err := os.WriteFile(targetPath, f.content, os.FileMode(f.mode)); err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	fmt.Printf("Restored from: %s\n", backupFile)
	return nil
}

// restoreFileEntry is a fully-buffered tar entry staged for Phase 2 extraction.
type restoreFileEntry struct {
	name    string
	isDir   bool
	mode    int64
	content []byte
}

// restoreArchive holds a fully-parsed, validated backup archive prior to extraction.
type restoreArchive struct {
	manifest    BackupManifest
	files       []restoreFileEntry
	contentHash hash.Hash
}

// loadRestoreArchive decompresses and parses a decrypted backup archive fully into
// memory, per AI.md PART 21's Phase 1 (validate before writing anything to disk).
// It recomputes the same content-addressable hash used by BackupWithOptions
// (name+NUL+content+NUL per regular file, in tar order) for checksum verification.
func loadRestoreArchive(data []byte) (*restoreArchive, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	archive := &restoreArchive{contentHash: sha256.New()}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Name == "manifest.json" {
			manifestData, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest: %w", err)
			}
			if err := json.Unmarshal(manifestData, &archive.manifest); err != nil {
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}
			continue
		}

		// Only these prefixes are ever extracted; others are ignored per Phase 2.
		relevant := strings.HasPrefix(header.Name, "config/") ||
			strings.HasPrefix(header.Name, "data/") ||
			strings.HasPrefix(header.Name, "ssl/")
		if !relevant {
			continue
		}

		if header.Typeflag == tar.TypeDir {
			archive.files = append(archive.files, restoreFileEntry{name: header.Name, isDir: true, mode: header.Mode})
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", header.Name, err)
		}
		archive.contentHash.Write([]byte(header.Name))
		archive.contentHash.Write([]byte{0})
		archive.contentHash.Write(content)
		archive.contentHash.Write([]byte{0})

		archive.files = append(archive.files, restoreFileEntry{name: header.Name, mode: header.Mode, content: content})
	}

	return archive, nil
}

// CheckUpdate checks for available updates from GitHub releases
// Per AI.md PART 22: Respects update branch setting (stable, beta, daily)
func (m *MaintenanceManager) CheckUpdate() (*UpdateInfo, error) {
	branch := m.GetUpdateBranch()
	currentVersion := strings.TrimPrefix(m.version, "v")

	var release *GitHubRelease
	var err error

	switch branch {
	case "stable":
		// Stable: fetch latest release (non-prerelease)
		release, err = m.fetchLatestRelease()
	case "beta":
		// Beta: fetch latest release containing "-beta" in tag
		release, err = m.fetchLatestBetaRelease()
	case "daily":
		// Daily: fetch latest release matching YYYYMMDDHHMM format
		release, err = m.fetchLatestDailyRelease()
	default:
		// Default to stable
		release, err = m.fetchLatestRelease()
	}

	if err != nil {
		return nil, err
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")

	info := &UpdateInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
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

// fetchLatestRelease fetches the latest stable release
func (m *MaintenanceManager) fetchLatestRelease() (*GitHubRelease, error) {
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

	return &release, nil
}

// fetchLatestBetaRelease fetches the latest beta release (tag contains "-beta")
func (m *MaintenanceManager) fetchLatestBetaRelease() (*GitHubRelease, error) {
	releases, err := m.fetchAllReleases()
	if err != nil {
		return nil, err
	}

	for _, release := range releases {
		if strings.Contains(strings.ToLower(release.TagName), "-beta") {
			return &release, nil
		}
	}

	return nil, fmt.Errorf("no beta releases found")
}

// fetchLatestDailyRelease fetches the latest daily release (tag matches YYYYMMDDHHMM)
func (m *MaintenanceManager) fetchLatestDailyRelease() (*GitHubRelease, error) {
	releases, err := m.fetchAllReleases()
	if err != nil {
		return nil, err
	}

	// Daily builds have tags like "202602011200" (12 digits)
	dailyPattern := regexp.MustCompile(`^\d{12}$`)

	for _, release := range releases {
		if dailyPattern.MatchString(release.TagName) {
			return &release, nil
		}
	}

	return nil, fmt.Errorf("no daily releases found")
}

// fetchAllReleases fetches all releases (sorted by date, newest first)
func (m *MaintenanceManager) fetchAllReleases() ([]GitHubRelease, error) {
	resp, err := http.Get("https://api.github.com/repos/apimgr/vidveil/releases?per_page=50")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse releases: %w", err)
	}

	return releases, nil
}

// verifyUpdateChecksum fetches the companion .sha256 sidecar for downloadURL and
// verifies that data matches. A missing checksum file (404) is a hard failure —
// we refuse to install an unverified binary. Both the download URL and checksum
// URL must use HTTPS to prevent MITM substitution.
func verifyUpdateChecksum(downloadURL string, data []byte) error {
	if !strings.HasPrefix(downloadURL, "https://") {
		return fmt.Errorf("refusing update: download URL must use HTTPS, got %q", downloadURL)
	}

	checksumURL := downloadURL + ".sha256"
	resp, err := http.Get(checksumURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("checksum fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("checksum sidecar not published at %s; refusing to install unverified binary", checksumURL)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read checksum file: %w", err)
	}

	// Expected format: "<hex-sha256>  <filename>" (sha256sum output) or just "<hex-sha256>"
	line := strings.TrimSpace(strings.SplitN(string(body), "\n", 2)[0])
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return fmt.Errorf("checksum file is empty")
	}
	expectedHex := fields[0]

	actualSum := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actualSum[:])
	if actualHex != expectedHex {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHex, actualHex)
	}
	return nil
}

// ApplyUpdate downloads and applies an update per AI.md PART 22.
// Update flow: download → verify SHA-256 checksum → replace binary.
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

	// Read into memory so we can verify checksum before touching the filesystem
	binaryData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read update body: %w", err)
	}

	// Per AI.md PART 22 step 3: verify SHA-256 checksum before replacing binary
	if err := verifyUpdateChecksum(downloadURL, binaryData); err != nil {
		return fmt.Errorf("update checksum verification failed: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temp file for download
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		return fmt.Errorf("failed to create temp base dir: %w", err)
	}
	tmpFile, err := os.CreateTemp("/tmp/apimgr", "vidveil-XXXXXX")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write verified binary to temp file
	if _, err = tmpFile.Write(binaryData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write update: %w", err)
	}
	tmpFile.Close()

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Backup current binary
	backupPath := execPath + ".bak"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary — atomic replace
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
// per AI.md PART 8 (--maintenance setup command)
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

// SetUpdateBranch sets the update branch (stable, beta, daily) per AI.md PART 22.
// Writes update.branch to server.yml — the config is the single source of truth.
func (m *MaintenanceManager) SetUpdateBranch(branch string) error {
	// Validate branch
	validBranches := map[string]bool{"stable": true, "beta": true, "daily": true}
	if !validBranches[branch] {
		return fmt.Errorf("invalid branch: %s (valid: stable, beta, daily)", branch)
	}

	// Load the current config, update branch, and save back to server.yml
	configPath := filepath.Join(m.paths.Config, "server.yml")
	cfg, _, err := config.LoadAppConfig(m.paths.Config, m.paths.Data)
	if err != nil {
		return fmt.Errorf("failed to load config to set update branch: %w", err)
	}
	cfg.Server.Update.Branch = branch

	if err := config.SaveAppConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config with new update branch: %w", err)
	}
	return nil
}

// GetUpdateBranch gets the current update branch from server.yml (defaults to stable)
func (m *MaintenanceManager) GetUpdateBranch() string {
	cfg, _, err := config.LoadAppConfig(m.paths.Config, m.paths.Data)
	if err != nil || cfg.Server.Update.Branch == "" {
		return "stable"
	}
	return cfg.Server.Update.Branch
}

// Helper to add directory to tar. contentHash, if non-nil, is fed name+NUL+content+NUL
// for every regular file (in walk/tar order) to build a content-addressable checksum
// that is independent of tar/gzip metadata (mtimes, permissions, etc).
func (m *MaintenanceManager) addDirToTar(tw *tar.Writer, srcDir, prefix string, contentHash hash.Hash) error {
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

			var writer io.Writer = tw
			if contentHash != nil {
				contentHash.Write([]byte(tarPath))
				contentHash.Write([]byte{0})
				writer = io.MultiWriter(tw, contentHash)
			}
			if _, err := io.Copy(writer, file); err != nil {
				return err
			}
			if contentHash != nil {
				contentHash.Write([]byte{0})
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
	TagName     string        `json:"tag_name"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
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
	Filename  string
	Path      string
	Size      int64
	Modified  time.Time
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

		// Only include .tar.gz and .tar.gz.enc files per AI.md PART 21
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
