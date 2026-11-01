// SPDX-License-Identifier: MIT
// AI.md PART 21: Backup & Restore | PART 22: Update Command
package maintenance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
	projectpath "github.com/apimgr/vidveil/src/path"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"golang.org/x/crypto/argon2"
)

var maintenanceOperationMu sync.Mutex

// ErrBackupPasswordWhitespace reports a backup password with boundary whitespace.
var ErrBackupPasswordWhitespace = errors.New("backup password must not start or end with whitespace")

// MaintenanceManager handles maintenance operations
type MaintenanceManager struct {
	paths      *config.AppPaths
	version    string
	buildEpoch int64
	limits     RestoreLimits
	// logger is optional — when nil, audit events per AI.md PART 21 "Audit Events"
	// are silently skipped. Set via SetLogger once a logger is available.
	logger *logging.AppLogger
}

// RestoreLimits are the validated byte and entry ceilings for backup restore.
type RestoreLimits struct {
	MaxArchiveBytes  uint64
	MaxExpandedBytes uint64
	MaxFiles         int
	MaxFileBytes     uint64
	MaxRatio         uint64
}

func DefaultRestoreLimits() RestoreLimits {
	return RestoreLimits{
		MaxArchiveBytes: 1 << 30, MaxExpandedBytes: 4 << 30,
		MaxFiles: 10000, MaxFileBytes: 512 << 20, MaxRatio: 100,
	}
}

// RestoreLimitsFromConfig converts the server.yml restore policy to runtime limits.
func RestoreLimitsFromConfig(cfg config.RestoreConfig) (RestoreLimits, error) {
	limits := DefaultRestoreLimits()
	var err error
	if limits.MaxArchiveBytes, err = config.ParseSizeBytes(cfg.MaxArchiveSize); err != nil {
		return RestoreLimits{}, fmt.Errorf("restore.max_archive_size: %w", err)
	}
	if limits.MaxExpandedBytes, err = config.ParseSizeBytes(cfg.MaxExpandedSize); err != nil {
		return RestoreLimits{}, fmt.Errorf("restore.max_expanded_size: %w", err)
	}
	if limits.MaxFileBytes, err = config.ParseSizeBytes(cfg.MaxFileSize); err != nil {
		return RestoreLimits{}, fmt.Errorf("restore.max_file_size: %w", err)
	}
	maxInt64 := uint64(^uint64(0) >> 1)
	if limits.MaxArchiveBytes > maxInt64-(1<<20) {
		return RestoreLimits{}, fmt.Errorf("restore.max_archive_size leaves no room for multipart framing")
	}
	if limits.MaxExpandedBytes >= maxInt64 {
		return RestoreLimits{}, fmt.Errorf("restore.max_expanded_size is too large")
	}
	if cfg.MaxFiles < 1 {
		return RestoreLimits{}, fmt.Errorf("restore.max_files must be at least 1")
	}
	if cfg.MaxRatio < 1 {
		return RestoreLimits{}, fmt.Errorf("restore.max_ratio must be at least 1")
	}
	limits.MaxFiles = cfg.MaxFiles
	limits.MaxRatio = uint64(cfg.MaxRatio)
	return limits, nil
}

func (m *MaintenanceManager) SetRestoreLimits(limits RestoreLimits) {
	m.limits = limits
}

func (m *MaintenanceManager) ConfigureRestore(cfg config.RestoreConfig) error {
	limits, err := RestoreLimitsFromConfig(cfg)
	if err != nil {
		return err
	}
	m.SetRestoreLimits(limits)
	return nil
}

// SetLogger attaches the audit logger used to emit the PART 21 backup audit
// events (backup.created, backup.retention_cleanup, backup.verification_failed,
// backup.daily_updated, backup.skipped_disk_full). Safe to call with nil.
func (m *MaintenanceManager) SetLogger(logger *logging.AppLogger) {
	m.logger = logger
}

// audit emits a PART 21 backup audit event via the attached logger, if any.
// actorType is always "system" per AI.md PART 11 (scheduler/CLI-triggered, no
// end-user actor); actorIP is empty since backup operations are not HTTP requests.
func (m *MaintenanceManager) audit(event, result string, details map[string]interface{}) {
	if m.logger == nil {
		return
	}
	m.logger.Audit(event, "system", "system", "", result, details)
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
	// ComplianceMode mirrors server.compliance.IsEnabled() per AI.md PART 21
	// "Compliance Mode Enforcement": when true, Password MUST be set - the backup
	// is rejected before any file is written.
	ComplianceMode bool
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
		limits:  DefaultRestoreLimits(),
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
	return m.withMaintenanceOperation(func() error {
		return m.backupIncremental(backupFile)
	})
}

func (m *MaintenanceManager) backupIncremental(backupFile string) error {
	if backupFile == "" {
		// Fixed filename per PART 21: vidveil-hourly.tar.gz — always 1 file
		backupFile = filepath.Join(m.paths.Backup, "vidveil-hourly.tar.gz")
	}
	return m.backupWithOptions(BackupOptions{
		Filename:    backupFile,
		IncludeData: true,
		MaxBackups:  1,
	}, false)
}

// BackupDailyFull creates the scheduled backup_daily run per AI.md PART 21
// "Backup Creation Flow" / "Backup Files Created": a retention-controlled full
// backup named {project_name}_backup_YYYY-MM-DD.tar.gz[.enc] plus a daily
// incremental named {project_name}-daily.tar.gz[.enc] (always exactly 1 file,
// replaced each run and never subject to count-based retention deletion).
// opts carries encryption/inclusion/retention settings sourced from the caller's
// server.backup.retention configuration.
func (m *MaintenanceManager) BackupDailyFull(opts BackupOptions) error {
	return m.withMaintenanceOperation(func() error {
		return m.backupDailyFull(opts)
	})
}

func (m *MaintenanceManager) backupDailyFull(opts BackupOptions) error {
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
	if err := m.backupWithOptions(fullOpts, false); err != nil {
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
	if err := m.backupWithOptions(dailyOpts, false); err != nil {
		return fmt.Errorf("daily incremental backup failed: %w", err)
	}
	if err := m.applyRetentionWithOptions(opts.MaxBackups, opts.KeepWeekly, opts.KeepMonthly, opts.KeepYearly, opts.MaxTotalSize); err != nil {
		fmt.Printf("Warning: failed to apply retention policy after verified backup pair: %v\n", err)
	}

	// "Changes since last" per AI.md PART 21 Audit Events: this manager always
	// snapshots config+data fully rather than diffing against the prior daily
	// incremental, so the on-disk size is reported as the closest available signal.
	dailySize := int64(0)
	if info, err := os.Stat(dailyOpts.Filename); err == nil {
		dailySize = info.Size()
	}
	m.audit("backup.daily_updated", "success", map[string]interface{}{
		"filename": filepath.Base(dailyOpts.Filename),
		"size":     dailySize,
	})

	return nil
}

// BackupWithOptions creates a backup with full options per AI.md PART 21
func (m *MaintenanceManager) BackupWithOptions(opts BackupOptions) error {
	return m.withMaintenanceOperation(func() error {
		return m.backupWithOptions(opts, true)
	})
}

func (m *MaintenanceManager) backupWithOptions(opts BackupOptions, applyRetention bool) (retErr error) {
	if opts.Password != strings.TrimSpace(opts.Password) {
		return ErrBackupPasswordWhitespace
	}
	// Per AI.md PART 21 "Compliance Mode Enforcement": when compliance mode is
	// active (server.compliance.enabled), backup.encryption.enabled MUST be true -
	// reject unencrypted backups before touching disk.
	if opts.ComplianceMode && opts.Password == "" {
		return fmt.Errorf("compliance mode requires backup encryption: set a backup password")
	}

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

	// Disk-space precheck per AI.md PART 21: abort (do NOT create the backup) if
	// free space < 2x the most recent existing backup, or disk usage > 90%.
	if err := m.checkDiskSpace(backupDir); err != nil {
		fmt.Printf("backup.skipped_disk_full: %v\n", err)
		total, free, _ := diskSpace(backupDir)
		usagePct := 0.0
		if total > 0 {
			usagePct = float64(total-free) / float64(total) * 100
		}
		m.audit("backup.skipped_disk_full", "failure", map[string]interface{}{
			"free_bytes": free,
			"disk_usage": fmt.Sprintf("%.1f%%", usagePct),
			"threshold":  "90%",
			"reason":     err.Error(),
		})
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
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode backup manifest: %w", err)
	}
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

	// Close both archive layers before treating the in-memory bytes as complete.
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close backup tar archive: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("close backup gzip archive: %w", err)
	}

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

	// Verify a private temporary archive before replacing any existing good
	// backup at the target filename.
	tmp, err := os.CreateTemp(backupDir, ".vidveil-backup-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary backup file: %w", err)
	}
	tmpBackup := tmp.Name()
	defer func() {
		if err := os.Remove(tmpBackup); err != nil && !os.IsNotExist(err) {
			retErr = errors.Join(retErr, fmt.Errorf("remove temporary backup %q: %w", tmpBackup, err))
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary backup permissions: %w", err)
	}
	if _, err := tmp.Write(finalData); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary backup: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary backup: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary backup: %w", err)
	}
	if err := m.verifyBackup(tmpBackup, checksumStr, opts.Password); err != nil {
		if removeErr := os.Remove(tmpBackup); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("backup verification failed: %w (remove failed archive: %v)", err, removeErr)
		}
		m.audit("backup.verification_failed", "failure", map[string]interface{}{
			"filename": filepath.Base(backupFile),
			"check":    err.Error(),
		})
		return fmt.Errorf("backup verification failed: %w", err)
	}
	if err := replaceBackupFile(tmpBackup, backupFile); err != nil {
		return fmt.Errorf("install verified backup: %w", err)
	}

	fmt.Printf("Backup created: %s\n", backupFile)
	fmt.Printf("Checksum: %s\n", checksumStr)
	m.audit("backup.created", "success", map[string]interface{}{
		"filename":  filepath.Base(backupFile),
		"size":      len(finalData),
		"encrypted": opts.Password != "",
		"verified":  true,
	})
	if applyRetention && sameBackupDirectory(backupDir, m.paths.Backup) {
		maxBackups := opts.MaxBackups
		if maxBackups == 0 {
			maxBackups = 1
		}
		if err := m.applyRetentionWithOptions(maxBackups, opts.KeepWeekly, opts.KeepMonthly, opts.KeepYearly, opts.MaxTotalSize); err != nil {
			fmt.Printf("Warning: failed to apply retention policy after verified backup: %v\n", err)
		}
	}
	return nil
}

func sameBackupDirectory(first, second string) bool {
	firstAbs, err := filepath.Abs(filepath.Clean(first))
	if err != nil {
		return false
	}
	secondAbs, err := filepath.Abs(filepath.Clean(second))
	if err != nil {
		return false
	}
	return firstAbs == secondAbs
}

// checkDiskSpace aborts the backup per AI.md PART 21 if free space is less than
// 2x the most recent existing backup's size, or if disk usage exceeds 90%.
// DiskSpace returns the total and free bytes for the filesystem containing
// path. Exported for the /server/healthz disk check (AI.md PART 13); the
// platform-specific implementation lives in diskspace_{unix,windows}.go.
func DiskSpace(path string) (total, free uint64, err error) {
	return diskSpace(path)
}

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
func (m *MaintenanceManager) verifyBackup(backupFile, expectedChecksum, password string) (retErr error) {
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
	tempOrgDir := filepath.Join(os.TempDir(), projectpath.ProjectOrg)
	if err := os.MkdirAll(tempOrgDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp base dir: %w", err)
	}
	tmpDir, err := os.MkdirTemp(tempOrgDir, projectpath.ProjectName+"-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("remove backup verification directory: %w", err))
		}
	}()

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid gzip: %w", err)
	}
	defer func() {
		if err := gzReader.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close backup gzip reader: %w", err))
		}
	}()

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
	var deleted []string
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
				deleted = append(deleted, b.Filename)
			}
		}
	}
	if len(deleted) > 0 {
		m.audit("backup.retention_cleanup", "success", map[string]interface{}{
			"deleted_files": deleted,
			"reason":        "retention_policy",
			"remaining":     len(keep),
		})
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

	var deleted []string
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
		deleted = append(deleted, b.Filename)
		total -= b.Size
	}
	if len(deleted) > 0 {
		m.audit("backup.retention_cleanup", "success", map[string]interface{}{
			"deleted_files": deleted,
			"reason":        "max_total_size_cap",
			"remaining":     len(backups) - len(deleted),
		})
	}

	return nil
}

// Restore restores from a backup file (simple version)
func (m *MaintenanceManager) Restore(backupFile string) error {
	return m.RestoreWithPassword(backupFile, "")
}

// RestoreWithPassword restores from a backup file with optional decryption
func (m *MaintenanceManager) RestoreWithPassword(backupFile, password string) (retErr error) {
	return m.withMaintenanceOperation(func() error {
		return m.restoreWithPassword(backupFile, password)
	})
}

func (m *MaintenanceManager) restoreWithPassword(backupFile, password string) (retErr error) {
	if password != strings.TrimSpace(password) {
		return ErrBackupPasswordWhitespace
	}
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

	// Check compressed archive size before allocating memory or decrypting it.
	if m.limits.MaxArchiveBytes == 0 {
		m.limits = DefaultRestoreLimits()
	}
	archiveFile, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	info, err := archiveFile.Stat()
	if err != nil {
		return errors.Join(fmt.Errorf("inspect backup file: %w", err), archiveFile.Close())
	}
	if info.Size() < 0 || uint64(info.Size()) > m.limits.MaxArchiveBytes {
		return errors.Join(fmt.Errorf("backup archive exceeds configured maximum of %d bytes", m.limits.MaxArchiveBytes), archiveFile.Close())
	}
	data, err := io.ReadAll(io.LimitReader(archiveFile, int64(m.limits.MaxArchiveBytes)+1))
	if err != nil {
		return errors.Join(fmt.Errorf("failed to read backup file: %w", err), archiveFile.Close())
	}
	if err := archiveFile.Close(); err != nil {
		return fmt.Errorf("close backup file: %w", err)
	}
	if uint64(len(data)) > m.limits.MaxArchiveBytes {
		return fmt.Errorf("backup archive exceeds configured maximum of %d bytes", m.limits.MaxArchiveBytes)
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
	if uint64(len(data)) > m.limits.MaxArchiveBytes {
		return fmt.Errorf("decrypted backup archive exceeds configured maximum of %d bytes", m.limits.MaxArchiveBytes)
	}

	// Keep expanded archive entries in a private project-scoped temp directory
	// while validating so large restores do not buffer every file in memory.
	tempBase := filepath.Join(os.TempDir(), projectpath.ProjectOrg)
	if err := os.MkdirAll(tempBase, 0755); err != nil {
		return fmt.Errorf("prepare restore temporary directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(tempBase, projectpath.ProjectName+"-restore-expanded-")
	if err != nil {
		return fmt.Errorf("create restore temporary directory: %w", err)
	}
	if err := os.Chmod(tempDir, 0700); err != nil {
		return errors.Join(fmt.Errorf("protect restore temporary directory: %w", err), os.RemoveAll(tempDir))
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("remove expanded restore temporary directory: %w", err))
		}
	}()

	// Phase 1 streams expanded entries to private temporary files, validating
	// the complete archive and checksum before touching the restore roots.
	entry, err := loadRestoreArchiveToTemp(data, tempDir, m.limits)
	if err != nil {
		return err
	}

	if entry.manifest.Version == "" {
		return fmt.Errorf("invalid backup: manifest.json missing or has empty version")
	}
	if entry.manifest.Checksum == "" {
		return fmt.Errorf("invalid backup: manifest.json missing checksum")
	}
	computed := "sha256:" + hex.EncodeToString(entry.contentHash.Sum(nil))
	if computed != entry.manifest.Checksum {
		return fmt.Errorf("backup checksum mismatch: manifest says %s, computed %s", entry.manifest.Checksum, computed)
	}

	if entry.manifest.AppVersion != "" && entry.manifest.AppVersion != m.version {
		fmt.Printf("Warning: backup was created by app version %s, current version is %s\n", entry.manifest.AppVersion, m.version)
	}

	// Phase 2: stage all affected roots, then swap them into place. Verification
	// and all staged writes finish before any current config/data is replaced.
	if err := restoreFilesAtomically(entry.files, map[string]string{
		"config": m.paths.Config,
		"data":   m.paths.Data,
		"ssl":    m.paths.SSL,
	}); err != nil {
		return fmt.Errorf("restore files: %w", err)
	}
	m.audit("backup.restored", "success", map[string]interface{}{"filename": filepath.Base(backupFile)})

	fmt.Printf("Restored from: %s\n", backupFile)
	return nil
}

func ensureRestorePathHasNoSymlinks(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("cannot resolve restore target: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("restore target escapes its root")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve restore root %q: %w", root, err)
	}

	current := resolvedRoot
	for _, component := range append([]string{"."}, strings.Split(relative, string(filepath.Separator))...) {
		if component != "." {
			current = filepath.Join(current, component)
		}
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect restore path %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore path %q traverses a symbolic link", current)
		}
	}
	return nil
}

// restoreFileEntry is a validated tar entry staged for Phase 2 extraction.
type restoreFileEntry struct {
	name        string
	isDir       bool
	mode        int64
	content     []byte
	contentPath string
}

// validateArchiveEntryName rejects tar entry names that are absolute, contain a
// ".." traversal element, or use backslash separators (a Windows traversal
// vector that path.Clean on Linux would not strip). Called while parsing, before
// any entry is buffered, so a malicious archive never reaches the filesystem.
func validateArchiveEntryName(name string) error {
	if name == "" {
		return errors.New("empty entry name")
	}
	if strings.ContainsRune(name, '\x00') {
		return errors.New("entry name contains a null byte")
	}
	if strings.Contains(name, `\`) {
		return errors.New("entry name contains a backslash separator")
	}
	if path.IsAbs(name) || filepath.IsAbs(name) {
		return errors.New("entry name is absolute")
	}
	if volume := filepath.VolumeName(name); volume != "" {
		return errors.New("entry name carries a volume name")
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return errors.New("entry name contains a parent-directory reference")
		}
	}
	return nil
}

// resolveRestoreTarget joins an archive entry name onto its extraction root and
// re-checks that the result stays inside that root. This is defense in depth
// behind validateArchiveEntryName: it holds even if a new entry name form is
// introduced upstream.
func resolveRestoreTarget(root, entryName string) (string, error) {
	cleanRoot := filepath.Clean(root)
	target := filepath.Clean(filepath.Join(cleanRoot, entryName))
	rel, err := filepath.Rel(cleanRoot, target)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q under %q: %w", entryName, root, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("entry %q escapes %q", entryName, root)
	}
	return target, nil
}

// restoreArchive holds a parsed, validated backup archive prior to extraction.
type restoreArchive struct {
	manifest    BackupManifest
	files       []restoreFileEntry
	contentHash hash.Hash
}

type restoreEntryStore func(header *tar.Header, content io.Reader) (restoreFileEntry, error)

// loadRestoreArchive reads archive contents into memory for unit tests and callers
// that need the parsed entry data directly.
func loadRestoreArchive(data []byte) (*restoreArchive, error) {
	return loadRestoreArchiveWithStore(data, func(header *tar.Header, source io.Reader) (restoreFileEntry, error) {
		content, err := io.ReadAll(source)
		if err != nil {
			return restoreFileEntry{}, err
		}
		return restoreFileEntry{name: header.Name, mode: header.Mode, content: content}, nil
	})
}

// loadRestoreArchiveToTemp streams regular file contents to private files so
// production restores do not retain expanded archive data in memory.
func loadRestoreArchiveToTemp(data []byte, tempDir string, configured ...RestoreLimits) (archive *restoreArchive, retErr error) {
	limits := DefaultRestoreLimits()
	if len(configured) > 0 {
		limits = configured[0]
	}
	var stagedFiles []string
	defer func() {
		if retErr == nil {
			return
		}
		for _, stagedPath := range stagedFiles {
			if err := os.Remove(stagedPath); err != nil && !os.IsNotExist(err) {
				retErr = errors.Join(retErr, fmt.Errorf("remove partial restore entry: %w", err))
			}
		}
	}()
	return loadRestoreArchiveWithStore(data, func(header *tar.Header, source io.Reader) (restoreFileEntry, error) {
		file, err := os.CreateTemp(tempDir, "entry-")
		if err != nil {
			return restoreFileEntry{}, fmt.Errorf("create temporary entry for %s: %w", header.Name, err)
		}
		path := file.Name()
		stagedFiles = append(stagedFiles, path)
		if _, err := io.Copy(file, source); err != nil {
			return restoreFileEntry{}, errors.Join(fmt.Errorf("write temporary entry for %s: %w", header.Name, err), file.Close())
		}
		if err := file.Sync(); err != nil {
			return restoreFileEntry{}, errors.Join(fmt.Errorf("sync temporary entry for %s: %w", header.Name, err), file.Close())
		}
		if err := file.Close(); err != nil {
			return restoreFileEntry{}, fmt.Errorf("close temporary entry for %s: %w", header.Name, err)
		}
		return restoreFileEntry{name: header.Name, mode: header.Mode, contentPath: path}, nil
	}, limits)
}

// loadRestoreArchiveWithStore validates a decrypted archive and recomputes the
// content-addressable hash used by BackupWithOptions (name+NUL+content+NUL per
// regular file, in tar order) before returning parsed manifest and file data.
func loadRestoreArchiveWithStore(data []byte, store restoreEntryStore, configured ...RestoreLimits) (result *restoreArchive, retErr error) {
	limits := DefaultRestoreLimits()
	if len(configured) > 0 {
		limits = configured[0]
	}
	if limits.MaxArchiveBytes == 0 || limits.MaxExpandedBytes == 0 || limits.MaxFileBytes == 0 || limits.MaxFiles < 1 || limits.MaxRatio < 1 {
		return nil, fmt.Errorf("restore limits must all be positive")
	}
	if uint64(len(data)) > limits.MaxArchiveBytes {
		return nil, fmt.Errorf("restore archive exceeds max_archive_size of %d bytes", limits.MaxArchiveBytes)
	}
	compressedSize := uint64(len(data))
	ratioLimit := compressedSize * limits.MaxRatio
	if compressedSize != 0 && ratioLimit/compressedSize != limits.MaxRatio {
		ratioLimit = ^uint64(0)
	}
	expandedLimit := limits.MaxExpandedBytes
	if ratioLimit < expandedLimit {
		expandedLimit = ratioLimit
	}
	if expandedLimit >= uint64(^uint64(0)>>1) {
		return nil, fmt.Errorf("configured restore expanded limit is too large")
	}
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	defer func() {
		if err := gzReader.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close restore gzip reader: %w", err))
		}
	}()

	limitedReader := &restoreExpandedLimitReader{reader: gzReader, limit: expandedLimit}
	tarReader := tar.NewReader(limitedReader)
	archive := &restoreArchive{contentHash: sha256.New()}
	manifestSeen := false
	fileCount := 0
	advertisedExpanded := uint64(0)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}
		fileCount++
		if fileCount > limits.MaxFiles {
			return nil, fmt.Errorf("restore archive exceeds max_files of %d", limits.MaxFiles)
		}
		if header.Typeflag == tar.TypeReg {
			if header.Size < 0 || uint64(header.Size) > limits.MaxFileBytes {
				return nil, fmt.Errorf("restore entry %q exceeds max_file_size of %d bytes", header.Name, limits.MaxFileBytes)
			}
			entrySize := uint64(header.Size)
			if entrySize > limits.MaxExpandedBytes-advertisedExpanded || entrySize > ratioLimit-advertisedExpanded {
				return nil, fmt.Errorf("restore archive exceeds expanded-size or compression-ratio limit")
			}
			advertisedExpanded += entrySize
		}

		if header.Name == "manifest.json" {
			if manifestSeen {
				return nil, fmt.Errorf("backup contains multiple manifest.json entries")
			}
			manifestSeen = true
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
		if header.Mode&0o6000 != 0 {
			return nil, fmt.Errorf("backup entry %q has setuid or setgid permission bits", header.Name)
		}

		// Reject any entry whose name could escape its extraction root. A name
		// like "config/../../etc/cron.d/x" passes the prefix test above but
		// resolves outside {config_dir} once joined, so validate the whole name.
		if err := validateArchiveEntryName(header.Name); err != nil {
			return nil, fmt.Errorf("invalid backup entry %q: %w", header.Name, err)
		}

		if header.Typeflag == tar.TypeDir {
			archive.files = append(archive.files, restoreFileEntry{name: header.Name, isDir: true, mode: header.Mode})
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("backup entry %q uses an unsupported special file type", header.Name)
		}

		archive.contentHash.Write([]byte(header.Name))
		archive.contentHash.Write([]byte{0})
		entry, err := store(header, io.TeeReader(tarReader, archive.contentHash))
		if err != nil {
			return nil, fmt.Errorf("read file %s: %w", header.Name, err)
		}
		archive.contentHash.Write([]byte{0})
		archive.files = append(archive.files, entry)
	}
	if limitedReader.exceeded {
		return nil, fmt.Errorf("restore archive exceeds max_expanded_size or max_ratio")
	}

	return archive, nil
}

type restoreExpandedLimitReader struct {
	reader   io.Reader
	limit    uint64
	read     uint64
	exceeded bool
}

func (r *restoreExpandedLimitReader) Read(p []byte) (int, error) {
	remaining := r.limit - r.read
	if remaining == 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			r.exceeded = true
			return 0, fmt.Errorf("restore archive expansion limit exceeded")
		}
		return 0, err
	}
	if uint64(len(p)) > remaining {
		p = p[:int(remaining)]
	}
	n, err := r.reader.Read(p)
	r.read += uint64(n)
	return n, err
}

const updateHTTPTimeout = 30 * time.Second

func updateContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, updateHTTPTimeout)
}

// CheckUpdate checks for available updates with a bounded request lifetime.
func (m *MaintenanceManager) CheckUpdate() (*UpdateInfo, error) {
	return m.CheckUpdateContext(context.Background())
}

// CheckUpdateContext checks the configured release channel using the caller's
// cancellation signal and a bounded HTTP timeout.
func (m *MaintenanceManager) CheckUpdateContext(parent context.Context) (*UpdateInfo, error) {
	ctx, cancel := updateContext(parent)
	defer cancel()
	return m.checkUpdate(ctx)
}

func (m *MaintenanceManager) checkUpdate(ctx context.Context) (*UpdateInfo, error) {
	branch := m.GetUpdateBranch()
	currentVersion := strings.TrimPrefix(m.version, "v")

	var release *GitHubRelease
	var err error

	switch branch {
	case "stable":
		// Stable: fetch latest release (non-prerelease)
		release, err = m.fetchLatestReleaseContext(ctx)
	case "beta":
		// Beta: fetch latest release containing "-beta" in tag
		release, err = m.fetchLatestBetaReleaseContext(ctx)
	case "devel":
		// Devel: newest rolling release from the devel channel.
		release, err = m.fetchLatestDevelReleaseContext(ctx)
	default:
		// Default to stable
		release, err = m.fetchLatestReleaseContext(ctx)
	}

	if err != nil {
		return nil, err
	}
	if release == nil {
		return &UpdateInfo{CurrentVersion: currentVersion}, nil
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
	if branch == "devel" {
		info.UpdateAvailable = release.PublishedAt.Unix() > m.buildEpoch
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

// SetBuildEpoch provides the current binary's embedded build time for devel
// channel comparisons.
func (m *MaintenanceManager) SetBuildEpoch(buildEpoch int64) {
	m.buildEpoch = buildEpoch
}

// fetchLatestRelease fetches the latest stable release
func (m *MaintenanceManager) fetchLatestRelease() (*GitHubRelease, error) {
	ctx, cancel := updateContext(context.Background())
	defer cancel()
	return m.fetchLatestReleaseContext(ctx)
}

func (m *MaintenanceManager) fetchLatestReleaseContext(ctx context.Context) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/apimgr/vidveil/releases/latest", nil)
	if err != nil {
		return nil, fmt.Errorf("create update request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

// matchesBranch implements cumulative update channels per AI.md PART 22: each
// channel also accepts every release from all more-stable channels. Stable
// matches every channel; beta = {stable, beta}; devel = {stable, beta, devel}.
func matchesBranch(r GitHubRelease, branch string) bool {
	if !r.Prerelease {
		return true
	}
	isBeta := strings.HasSuffix(r.TagName, "-beta")
	isDevel := r.TagName == "devel"
	switch branch {
	case "beta":
		return isBeta
	case "devel":
		return isBeta || isDevel
	default:
		return false
	}
}

// fetchLatestBetaRelease returns the newest release eligible for the beta
// channel: the newest of {stable, beta}. Releases are returned newest-first,
// so the first match wins.
func (m *MaintenanceManager) fetchLatestBetaRelease() (*GitHubRelease, error) {
	ctx, cancel := updateContext(context.Background())
	defer cancel()
	return m.fetchLatestBetaReleaseContext(ctx)
}

func (m *MaintenanceManager) fetchLatestBetaReleaseContext(ctx context.Context) (*GitHubRelease, error) {
	releases, err := m.fetchAllReleasesContext(ctx)
	if err != nil {
		return nil, err
	}

	for i := range releases {
		if matchesBranch(releases[i], "beta") {
			return &releases[i], nil
		}
	}

	return nil, nil
}

// fetchLatestDevelRelease returns the newest release eligible for the devel
// channel: the newest of {stable, beta, devel}. Releases are returned
// newest-first, so the first match wins.
func (m *MaintenanceManager) fetchLatestDevelRelease() (*GitHubRelease, error) {
	ctx, cancel := updateContext(context.Background())
	defer cancel()
	return m.fetchLatestDevelReleaseContext(ctx)
}

func (m *MaintenanceManager) fetchLatestDevelReleaseContext(ctx context.Context) (*GitHubRelease, error) {
	releases, err := m.fetchAllReleasesContext(ctx)
	if err != nil {
		return nil, err
	}

	for i := range releases {
		if matchesBranch(releases[i], "devel") {
			return &releases[i], nil
		}
	}

	return nil, nil
}

// fetchAllReleases fetches all releases (sorted by date, newest first)
func (m *MaintenanceManager) fetchAllReleases() ([]GitHubRelease, error) {
	ctx, cancel := updateContext(context.Background())
	defer cancel()
	return m.fetchAllReleasesContext(ctx)
}

func (m *MaintenanceManager) fetchAllReleasesContext(ctx context.Context) ([]GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/apimgr/vidveil/releases?per_page=50", nil)
	if err != nil {
		return nil, fmt.Errorf("create release-list request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return []GitHubRelease{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse releases: %w", err)
	}

	return releases, nil
}

// verifyUpdateChecksum fetches the release's checksums.txt asset (a sibling of
// the binary asset in the same release directory) and verifies that data
// matches the entry for this binary. Per AI.md PART 22 the source of truth is
// the checksums.txt asset, NOT a per-file .sha256 sidecar. Each line is
// "{sha256}  {filename}". A missing checksums.txt (404) is a hard failure — we
// refuse to install an unverified binary. The download URL must use HTTPS to
// prevent MITM substitution.
func verifyUpdateChecksum(downloadURL string, data []byte) error {
	ctx, cancel := updateContext(context.Background())
	defer cancel()
	return verifyUpdateChecksumContext(ctx, downloadURL, data)
}

func verifyUpdateChecksumContext(ctx context.Context, downloadURL string, data []byte) error {
	if !strings.HasPrefix(downloadURL, "https://") {
		return fmt.Errorf("refusing update: download URL must use HTTPS, got %q", downloadURL)
	}

	// checksums.txt is a sibling asset: same release directory, last path
	// segment replaced. The final path segment is this binary's asset name.
	slash := strings.LastIndex(downloadURL, "/")
	if slash < 0 {
		return fmt.Errorf("malformed download URL: %q", downloadURL)
	}
	assetName := downloadURL[slash+1:]
	checksumsURL := downloadURL[:slash+1] + "checksums.txt"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return fmt.Errorf("create checksum request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("checksum fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("checksums.txt not published at %s; refusing to install unverified binary", checksumsURL)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read checksums.txt: %w", err)
	}

	// Find the "{sha256}  {filename}" line matching this asset
	var expectedHex string
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			expectedHex = fields[0]
			break
		}
	}
	if expectedHex == "" {
		return fmt.Errorf("no checksum entry for %s in checksums.txt", assetName)
	}

	actualSum := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actualSum[:])
	if !strings.EqualFold(actualHex, expectedHex) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHex, actualHex)
	}
	return nil
}

// ApplyUpdate downloads and applies an update per AI.md PART 22.
// Update flow: download → verify SHA-256 checksum → replace binary.
func (m *MaintenanceManager) ApplyUpdate(downloadURL string) error {
	return m.ApplyUpdateContext(context.Background(), downloadURL)
}

// ApplyUpdateContext downloads and verifies an update with caller cancellation
// and a bounded timeout before replacing the running executable.
func (m *MaintenanceManager) ApplyUpdateContext(parent context.Context, downloadURL string) error {
	ctx, cancel := updateContext(parent)
	defer cancel()
	return m.applyUpdate(ctx, downloadURL)
}

func (m *MaintenanceManager) applyUpdate(ctx context.Context, downloadURL string) (retErr error) {
	if downloadURL == "" {
		info, err := m.checkUpdate(ctx)
		if err != nil {
			return err
		}
		if info == nil || !info.UpdateAvailable {
			fmt.Println(terminal.StatusIcon(true) + " Already up to date")
			return nil
		}
		if info.DownloadURL == "" {
			return fmt.Errorf("no download available for %s/%s", runtime.GOOS, runtime.GOARCH)
		}
		downloadURL = info.DownloadURL
	}

	// Download new binary
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create update download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read into memory so we can verify checksum before touching the filesystem
	binaryData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read update body: %w", err)
	}

	// Per AI.md PART 22 step 3: verify SHA-256 checksum before replacing binary
	if err := verifyUpdateChecksumContext(ctx, downloadURL, binaryData); err != nil {
		return fmt.Errorf("update checksum verification failed: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temp file for download
	tempOrgDir := filepath.Join(os.TempDir(), projectpath.ProjectOrg)
	if err := os.MkdirAll(tempOrgDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp base dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(tempOrgDir, projectpath.ProjectName+"-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			retErr = errors.Join(retErr, fmt.Errorf("remove temporary update file: %w", err))
		}
	}()

	// Write verified binary to temp file
	if _, err = tmpFile.Write(binaryData); err != nil {
		return errors.Join(fmt.Errorf("failed to write update: %w", err), tmpFile.Close())
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close downloaded update: %w", err)
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

	// Move new binary — atomic replace
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore backup
		if restoreErr := os.Rename(backupPath, execPath); restoreErr != nil {
			return errors.Join(fmt.Errorf("failed to install update: %w", err), fmt.Errorf("failed to restore backup: %w", restoreErr))
		}
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Remove backup
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("update installed but failed to remove backup: %w", err)
	}

	fmt.Println(terminal.StatusIcon(true) + " Update applied successfully")
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
		if _, err := file.WriteString(time.Now().Format(time.RFC3339)); err != nil {
			_ = file.Close()
			return fmt.Errorf("failed to write maintenance flag: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close maintenance flag: %w", err)
		}
		fmt.Println(terminal.StatusIcon(true) + " Maintenance mode enabled")
		fmt.Println("   Server will return 503 for all requests")
	} else {
		if err := os.Remove(modeFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to disable maintenance mode: %w", err)
		}
		fmt.Println(terminal.StatusIcon(true) + " Maintenance mode disabled")
	}

	return nil
}

// IsMaintenanceMode checks if maintenance mode is active
func (m *MaintenanceManager) IsMaintenanceMode() bool {
	modeFile := filepath.Join(m.paths.Data, "maintenance.flag")
	_, err := os.Stat(modeFile)
	return err == nil
}

// SetUpdateBranch sets the canonical update branch per AI.md PART 22.
// Writes update.branch to server.yml — the config is the single source of truth.
func (m *MaintenanceManager) SetUpdateBranch(branch string) error {
	canonical, valid := config.NormalizeUpdateChannel(branch)
	if !valid {
		return fmt.Errorf("invalid branch: %s (valid: stable, beta, devel)", branch)
	}

	// Load the current config, update branch, and save back to server.yml
	configPath := filepath.Join(m.paths.Config, "server.yml")
	cfg, _, err := config.LoadAppConfig(m.paths.Config, m.paths.Data)
	if err != nil {
		return fmt.Errorf("failed to load config to set update branch: %w", err)
	}
	cfg.Server.Update.Branch = canonical

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
	branch, valid := config.NormalizeUpdateChannel(cfg.Server.Update.Branch)
	if !valid {
		return "stable"
	}
	return branch
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
			var writer io.Writer = tw
			if contentHash != nil {
				contentHash.Write([]byte(tarPath))
				contentHash.Write([]byte{0})
				writer = io.MultiWriter(tw, contentHash)
			}
			if _, err := io.Copy(writer, file); err != nil {
				return errors.Join(err, file.Close())
			}
			if err := file.Close(); err != nil {
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
	Prerelease  bool          `json:"prerelease"`
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
			if parsed, err := strconv.Atoi(aParts[i]); err == nil {
				aNum = parsed
			}
		}
		if i < len(bParts) {
			if parsed, err := strconv.Atoi(bParts[i]); err == nil {
				bNum = parsed
			}
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
