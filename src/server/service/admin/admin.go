// SPDX-License-Identifier: MIT
// AI.md PART 17: Admin Panel - Admin Credentials Management
// Admin credentials stored in database, NOT config file
// Password hashing uses Argon2id per AI.md PART 2
package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/service/validation"
	"golang.org/x/crypto/argon2"
)

// Service manages admin credentials per AI.md PART 31
type Service struct {
	db           *sql.DB
	mu           sync.RWMutex
	setupToken   string
	isFirstRun   bool
	tokenExpires time.Time
}

// Admin represents an admin user
type Admin struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	TOTPEnabled  bool      `json:"totp_enabled"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	LoginCount   int       `json:"login_count"`
	IsPrimary    bool      `json:"is_primary"`
}

// NewService creates a new admin service
func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}

// GetDB returns the database connection for admin-related queries
func (s *Service) GetDB() *sql.DB {
	return s.db
}

// Initialize checks for first run and generates setup token if needed
// Per AI.md PART 31: App is FULLY FUNCTIONAL before setup
func (s *Service) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if any admins exist
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM admin_credentials").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check admin count: %w", err)
	}

	s.isFirstRun = count == 0

	if s.isFirstRun {
		// Generate setup token
		token, err := generateSecureToken(32)
		if err != nil {
			return fmt.Errorf("failed to generate setup token: %w", err)
		}

		s.setupToken = token
		s.tokenExpires = time.Now().Add(24 * time.Hour)

		// Store setup token in database
		_, err = s.db.Exec(`
			INSERT INTO setup_tokens (token, purpose, expires_at)
			VALUES (?, 'initial_setup', ?)
		`, hashToken(token), s.tokenExpires)
		if err != nil {
			return fmt.Errorf("failed to store setup token: %w", err)
		}

		// Console output is handled in main.go per AI.md PART 31
	}

	return nil
}

// IsFirstRun returns true if no admin accounts exist
func (s *Service) IsFirstRun() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isFirstRun
}

// GetSetupToken returns the setup token (only shown once)
func (s *Service) GetSetupToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.setupToken
}

// ValidateSetupToken checks if a setup token is valid
func (s *Service) ValidateSetupToken(token string) bool {
	var usedAt sql.NullTime
	var expires time.Time

	err := s.db.QueryRow(`
		SELECT expires_at, used_at FROM setup_tokens
		WHERE token = ? AND purpose = 'initial_setup'
	`, hashToken(token)).Scan(&expires, &usedAt)

	if err != nil {
		return false
	}

	// Token already used
	if usedAt.Valid {
		return false
	}

	// Token expired
	if time.Now().After(expires) {
		return false
	}

	return true
}

// CreateAdmin creates a new admin account
// Uses Argon2id for password hashing per AI.md PART 2
// Server admin accounts are exempt from username blocklist per PART 31
func (s *Service) CreateAdmin(username, password string, isPrimary bool) (*Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate username (admin accounts exempt from blocklist)
	if err := validation.ValidateUsername(username, true); err != nil {
		return nil, err
	}

	// Validate password (stricter rules for admin accounts per PART 22)
	if err := validation.ValidateAdminPassword(password); err != nil {
		return nil, err
	}

	// Hash password using Argon2id
	hash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	result, err := s.db.Exec(`
		INSERT INTO admin_credentials (username, password_hash, is_primary)
		VALUES (?, ?, ?)
	`, username, hash, isPrimary)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin: %w", err)
	}

	id, _ := result.LastInsertId()
	s.isFirstRun = false

	return &Admin{
		ID:        id,
		Username:  username,
		IsPrimary: isPrimary,
		CreatedAt: time.Now(),
	}, nil
}

// CreateAdminWithSetupToken creates admin using setup token
func (s *Service) CreateAdminWithSetupToken(token, username, password string) (*Admin, error) {
	if !s.ValidateSetupToken(token) {
		return nil, fmt.Errorf("invalid or expired setup token")
	}

	// Mark token as used
	_, err := s.db.Exec(`
		UPDATE setup_tokens SET used_at = ?, used_by = ?
		WHERE token = ? AND purpose = 'initial_setup'
	`, time.Now(), username, hashToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	return s.CreateAdmin(username, password, true)
}

// Authenticate validates admin credentials
// Uses Argon2id for password verification per AI.md PART 2
func (s *Service) Authenticate(username, password string) (*Admin, error) {
	var admin Admin
	var passwordHash string

	err := s.db.QueryRow(`
		SELECT id, username, password_hash, totp_enabled, created_at, last_login, login_count, is_primary
		FROM admin_credentials WHERE username = ?
	`, username).Scan(&admin.ID, &admin.Username, &passwordHash, &admin.TOTPEnabled,
		&admin.CreatedAt, &admin.LastLogin, &admin.LoginCount, &admin.IsPrimary)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Verify password using Argon2id
	valid, err := verifyPassword(password, passwordHash)
	if err != nil || !valid {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	now := time.Now()
	admin.LastLogin = &now
	admin.LoginCount++

	_, _ = s.db.Exec(`
		UPDATE admin_credentials SET last_login = ?, login_count = login_count + 1
		WHERE id = ?
	`, now, admin.ID)

	return &admin, nil
}

// ChangePassword updates admin password
// Uses Argon2id for password hashing per AI.md PART 2
func (s *Service) ChangePassword(adminID int64, currentPassword, newPassword string) error {
	var passwordHash string
	err := s.db.QueryRow(`
		SELECT password_hash FROM admin_credentials WHERE id = ?
	`, adminID).Scan(&passwordHash)
	if err != nil {
		return fmt.Errorf("admin not found")
	}

	// Verify current password using Argon2id
	valid, err := verifyPassword(currentPassword, passwordHash)
	if err != nil || !valid {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password using Argon2id
	newHash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE admin_credentials SET password_hash = ?, updated_at = ?
		WHERE id = ?
	`, newHash, time.Now(), adminID)

	return err
}

// GenerateInviteToken creates an invite token for a new admin
func (s *Service) GenerateInviteToken(invitedBy int64) (string, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return "", err
	}

	// Token valid for 7 days
	expires := time.Now().Add(7 * 24 * time.Hour)

	_, err = s.db.Exec(`
		INSERT INTO setup_tokens (token, purpose, expires_at)
		VALUES (?, 'admin_invite', ?)
	`, hashToken(token), expires)
	if err != nil {
		return "", fmt.Errorf("failed to store invite token: %w", err)
	}

	return token, nil
}

// CreateAPIToken generates an API token for an admin
func (s *Service) CreateAPIToken(adminID int64, name string, permissions string) (string, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return "", err
	}

	prefix := token[:8]
	hash := hashToken(token)

	_, err = s.db.Exec(`
		INSERT INTO api_tokens (admin_id, name, token_hash, token_prefix, permissions)
		VALUES (?, ?, ?, ?, ?)
	`, adminID, name, hash, prefix, permissions)
	if err != nil {
		return "", fmt.Errorf("failed to create API token: %w", err)
	}

	return token, nil
}

// ValidateAPIToken checks if an API token is valid
func (s *Service) ValidateAPIToken(token string) (int64, error) {
	var adminID int64
	var expires sql.NullTime

	err := s.db.QueryRow(`
		SELECT admin_id, expires_at FROM api_tokens
		WHERE token_hash = ?
	`, hashToken(token)).Scan(&adminID, &expires)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("invalid token")
	}
	if err != nil {
		return 0, err
	}

	if expires.Valid && time.Now().After(expires.Time) {
		return 0, fmt.Errorf("token expired")
	}

	// Update last used
	_, _ = s.db.Exec(`
		UPDATE api_tokens SET last_used = ?, use_count = use_count + 1
		WHERE token_hash = ?
	`, time.Now(), hashToken(token))

	return adminID, nil
}

// GetAdminCount returns the number of admin accounts
func (s *Service) GetAdminCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM admin_credentials").Scan(&count)
	return count, err
}

// AdminInvite represents an admin invite
type AdminInvite struct {
	Username  string
	ExpiresAt time.Time
	CreatedBy int64
}

// CreateAdminInvite creates an invite for a new admin per AI.md PART 31
func (s *Service) CreateAdminInvite(createdBy int64, username string, expiresIn time.Duration) (string, error) {
	// Validate username
	if err := validation.ValidateUsername(username, true); err != nil {
		return "", err
	}

	// Check if username already exists
	var exists int
	s.db.QueryRow("SELECT COUNT(*) FROM admin_credentials WHERE username = ?", username).Scan(&exists)
	if exists > 0 {
		return "", fmt.Errorf("username already exists")
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return "", err
	}

	expires := time.Now().Add(expiresIn)

	_, err = s.db.Exec(`
		INSERT INTO setup_tokens (token, purpose, username, expires_at, created_by)
		VALUES (?, 'admin_invite', ?, ?, ?)
	`, hashToken(token), username, expires, createdBy)
	if err != nil {
		return "", fmt.Errorf("failed to create invite: %w", err)
	}

	return token, nil
}

// ValidateInviteToken validates an admin invite token per AI.md PART 31
func (s *Service) ValidateInviteToken(token string) (*AdminInvite, error) {
	var invite AdminInvite
	var usedAt sql.NullTime
	var createdBy sql.NullInt64

	err := s.db.QueryRow(`
		SELECT username, expires_at, used_at, created_by FROM setup_tokens
		WHERE token = ? AND purpose = 'admin_invite'
	`, hashToken(token)).Scan(&invite.Username, &invite.ExpiresAt, &usedAt, &createdBy)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid token")
	}
	if err != nil {
		return nil, err
	}

	// Token already used
	if usedAt.Valid {
		return nil, fmt.Errorf("token already used")
	}

	// Token expired
	if time.Now().After(invite.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	if createdBy.Valid {
		invite.CreatedBy = createdBy.Int64
	}

	return &invite, nil
}

// CreateAdminWithInvite creates an admin account from an invite token per AI.md PART 31
func (s *Service) CreateAdminWithInvite(token, username, password string) (*Admin, error) {
	// Validate the token
	invite, err := s.ValidateInviteToken(token)
	if err != nil {
		return nil, err
	}

	// Ensure username matches
	if invite.Username != username {
		return nil, fmt.Errorf("username mismatch")
	}

	// Mark token as used
	_, err = s.db.Exec(`
		UPDATE setup_tokens SET used_at = ?, used_by = ?
		WHERE token = ? AND purpose = 'admin_invite'
	`, time.Now(), username, hashToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	// Create the admin (non-primary)
	return s.CreateAdmin(username, password, false)
}

// PendingInvite represents a pending admin invite
type PendingInvite struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// ListPendingInvites returns all pending (unused, unexpired) admin invites
func (s *Service) ListPendingInvites() ([]PendingInvite, error) {
	rows, err := s.db.Query(`
		SELECT st.id, st.username, st.expires_at, st.created_at, COALESCE(ac.username, 'System') as created_by
		FROM setup_tokens st
		LEFT JOIN admin_credentials ac ON st.created_by = ac.id
		WHERE st.purpose = 'admin_invite'
		AND st.used_at IS NULL
		AND st.expires_at > ?
		ORDER BY st.created_at DESC
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []PendingInvite
	for rows.Next() {
		var invite PendingInvite
		if err := rows.Scan(&invite.ID, &invite.Username, &invite.ExpiresAt, &invite.CreatedAt, &invite.CreatedBy); err != nil {
			continue
		}
		invites = append(invites, invite)
	}
	return invites, nil
}

// RevokeInvite revokes a pending admin invite
func (s *Service) RevokeInvite(inviteID int64) error {
	result, err := s.db.Exec(`
		DELETE FROM setup_tokens
		WHERE id = ? AND purpose = 'admin_invite' AND used_at IS NULL
	`, inviteID)
	if err != nil {
		return fmt.Errorf("failed to revoke invite: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("invite not found or already used")
	}
	return nil
}

// CleanupExpiredInvites removes expired invites (called by scheduler)
func (s *Service) CleanupExpiredInvites() (int64, error) {
	result, err := s.db.Exec(`
		DELETE FROM setup_tokens
		WHERE purpose = 'admin_invite' AND expires_at < ?
	`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Helper functions

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Argon2id parameters per AI.md PART 2 (OWASP 2023 recommendations)
const (
	// iterations
	argonTime = 3
	// 64 MB memory
	argonMemory = 64 * 1024
	// parallelism
	argonThreads = 4
	// output length in bytes
	argonKeyLen = 32
	// salt length in bytes
	argonSaltLen = 16
)

// hashPassword hashes a password using Argon2id per AI.md PART 2
// Returns PHC string format: $argon2id$v=19$m=65536,t=3,p=4$<base64-salt>$<base64-hash>
func hashPassword(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Encode as PHC string format
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

// GetAdmin retrieves admin details by ID
func (s *Service) GetAdmin(adminID int64) (*Admin, error) {
	var admin Admin
	var lastLogin sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, username, totp_enabled, created_at, last_login, login_count, is_primary
		FROM admin_credentials WHERE id = ?
	`, adminID).Scan(&admin.ID, &admin.Username, &admin.TOTPEnabled,
		&admin.CreatedAt, &lastLogin, &admin.LoginCount, &admin.IsPrimary)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("admin not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	if lastLogin.Valid {
		admin.LastLogin = &lastLogin.Time
	}

	return &admin, nil
}

// GetAPITokenInfo returns info about admin's API token
func (s *Service) GetAPITokenInfo(adminID int64) (prefix string, lastUsed *time.Time, useCount int, err error) {
	var lu sql.NullTime

	err = s.db.QueryRow(`
		SELECT token_prefix, last_used, use_count FROM api_tokens
		WHERE admin_id = ? ORDER BY created_at DESC LIMIT 1
	`, adminID).Scan(&prefix, &lu, &useCount)

	if err == sql.ErrNoRows {
		return "", nil, 0, nil
	}
	if err != nil {
		return "", nil, 0, err
	}

	if lu.Valid {
		lastUsed = &lu.Time
	}
	return prefix, lastUsed, useCount, nil
}

// RegenerateAPIToken creates a new API token, replacing any existing one
func (s *Service) RegenerateAPIToken(adminID int64) (string, error) {
	// Delete existing tokens for this admin
	_, err := s.db.Exec("DELETE FROM api_tokens WHERE admin_id = ?", adminID)
	if err != nil {
		return "", fmt.Errorf("failed to remove old tokens: %w", err)
	}

	// Create new token
	token, err := generateSecureToken(32)
	if err != nil {
		return "", err
	}

	prefix := token[:8]
	hash := hashToken(token)

	_, err = s.db.Exec(`
		INSERT INTO api_tokens (admin_id, name, token_hash, token_prefix, permissions, created_at)
		VALUES (?, 'Primary API Token', ?, ?, 'admin', ?)
	`, adminID, hash, prefix, time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to create API token: %w", err)
	}

	return token, nil
}

// verifyPassword verifies a password against an Argon2id hash
func verifyPassword(password, encodedHash string) (bool, error) {
	// Parse PHC string format
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	if parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported algorithm: %s", parts[1])
	}

	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return false, fmt.Errorf("invalid version: %w", err)
	}

	var memory, time uint32
	var threads uint8
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
	if err != nil {
		return false, fmt.Errorf("invalid parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid hash: %w", err)
	}

	// Compute hash with same parameters
	computedHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedHash)))

	// Constant-time comparison
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1, nil
}

// RecoveryKeyInfo contains info about recovery keys
type RecoveryKeyInfo struct {
	Total     int `json:"total"`
	Remaining int `json:"remaining"`
	Used      int `json:"used"`
}

// GenerateRecoveryKeys generates 10 recovery keys for an admin
// Returns the plaintext keys (only shown once) and stores hashes in database
func (s *Service) GenerateRecoveryKeys(adminID int64) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete any existing recovery keys for this admin
	_, err := s.db.Exec("DELETE FROM recovery_keys WHERE admin_id = ?", adminID)
	if err != nil {
		return nil, fmt.Errorf("failed to clear old recovery keys: %w", err)
	}

	// Generate 10 new recovery keys
	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		// Generate 8-byte random key (16 hex chars)
		keyBytes := make([]byte, 8)
		if _, err := rand.Read(keyBytes); err != nil {
			return nil, fmt.Errorf("failed to generate recovery key: %w", err)
		}

		// Format as XXXX-XXXX-XXXX-XXXX (16 hex chars with dashes)
		keyHex := hex.EncodeToString(keyBytes)
		keys[i] = fmt.Sprintf("%s-%s-%s-%s", keyHex[0:4], keyHex[4:8], keyHex[8:12], keyHex[12:16])

		// Store hash in database
		keyHash := hashToken(strings.ReplaceAll(keys[i], "-", ""))
		_, err := s.db.Exec(`
			INSERT INTO recovery_keys (admin_id, key_hash)
			VALUES (?, ?)
		`, adminID, keyHash)
		if err != nil {
			return nil, fmt.Errorf("failed to store recovery key: %w", err)
		}
	}

	return keys, nil
}

// ValidateRecoveryKey validates a recovery key and marks it as used if valid
// Returns true if the key was valid and unused
func (s *Service) ValidateRecoveryKey(adminID int64, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Normalize key (remove dashes, lowercase)
	normalizedKey := strings.ToLower(strings.ReplaceAll(key, "-", ""))
	keyHash := hashToken(normalizedKey)

	// Look for unused key matching this hash
	var keyID int64
	err := s.db.QueryRow(`
		SELECT id FROM recovery_keys
		WHERE admin_id = ? AND key_hash = ? AND used_at IS NULL
	`, adminID, keyHash).Scan(&keyID)

	if err == sql.ErrNoRows {
		// Key not found or already used
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to validate recovery key: %w", err)
	}

	// Mark key as used
	_, err = s.db.Exec(`
		UPDATE recovery_keys SET used_at = ? WHERE id = ?
	`, time.Now(), keyID)
	if err != nil {
		return false, fmt.Errorf("failed to mark recovery key as used: %w", err)
	}

	return true, nil
}

// GetRecoveryKeysStatus returns info about an admin's recovery keys
func (s *Service) GetRecoveryKeysStatus(adminID int64) (*RecoveryKeyInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total, used int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM recovery_keys WHERE admin_id = ?
	`, adminID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count recovery keys: %w", err)
	}

	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM recovery_keys WHERE admin_id = ? AND used_at IS NOT NULL
	`, adminID).Scan(&used)
	if err != nil {
		return nil, fmt.Errorf("failed to count used keys: %w", err)
	}

	return &RecoveryKeyInfo{
		Total:     total,
		Remaining: total - used,
		Used:      used,
	}, nil
}

// CleanupExpiredSessions removes expired admin sessions (called by scheduler)
// Per AI.md PART 26: session.cleanup runs hourly
func (s *Service) CleanupExpiredSessions() error {
	_, err := s.db.Exec(`
		DELETE FROM admin_sessions WHERE expires_at < ?
	`, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}

// CleanupExpiredTokens removes expired API tokens and reset tokens (called by scheduler)
// Per AI.md PART 26: token.cleanup runs daily
func (s *Service) CleanupExpiredTokens() error {
	// Clean up expired setup tokens (password reset, invites, etc.)
	_, err := s.db.Exec(`
		DELETE FROM setup_tokens WHERE expires_at < ?
	`, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired setup tokens: %w", err)
	}

	// Clean up expired API tokens
	_, err = s.db.Exec(`
		DELETE FROM api_tokens WHERE expires_at IS NOT NULL AND expires_at < ?
	`, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired API tokens: %w", err)
	}

	return nil
}

// GetTOTPSecret returns the TOTP secret for an admin (for 2FA verification)
// Per AI.md PART 17: TOTP Two-Factor Authentication
func (s *Service) GetTOTPSecret(adminID int64) (string, error) {
	var secret sql.NullString
	err := s.db.QueryRow(`
		SELECT totp_secret FROM admin_credentials WHERE id = ?
	`, adminID).Scan(&secret)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("admin not found")
	}
	if err != nil {
		return "", fmt.Errorf("database error: %w", err)
	}

	if !secret.Valid || secret.String == "" {
		return "", fmt.Errorf("TOTP not configured")
	}

	return secret.String, nil
}

// GetTOTPBackupCodes returns the backup codes for an admin
// Per AI.md PART 17: 10 one-time recovery codes
func (s *Service) GetTOTPBackupCodes(adminID int64) ([]string, error) {
	var codesJSON sql.NullString
	err := s.db.QueryRow(`
		SELECT totp_backup_codes FROM admin_credentials WHERE id = ?
	`, adminID).Scan(&codesJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("admin not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	if !codesJSON.Valid || codesJSON.String == "" {
		return []string{}, nil
	}

	// Parse JSON array of codes
	codes := strings.Split(codesJSON.String, ",")
	return codes, nil
}

// UseBackupCode marks a backup code as used and removes it
// Returns true if code was valid and removed
func (s *Service) UseBackupCode(adminID int64, code string) (bool, error) {
	codes, err := s.GetTOTPBackupCodes(adminID)
	if err != nil {
		return false, err
	}

	code = strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	newCodes := make([]string, 0, len(codes))
	found := false

	for _, c := range codes {
		normalized := strings.ToUpper(strings.ReplaceAll(c, "-", ""))
		if normalized == code {
			found = true
			continue // Skip this code (remove it)
		}
		newCodes = append(newCodes, c)
	}

	if !found {
		return false, nil
	}

	// Update backup codes
	codesStr := strings.Join(newCodes, ",")
	_, err = s.db.Exec(`
		UPDATE admin_credentials SET totp_backup_codes = ? WHERE id = ?
	`, codesStr, adminID)

	return true, err
}

// EnableTOTP enables 2FA for an admin account
// Per AI.md PART 17: QR code + manual entry key at /admin/profile/security
func (s *Service) EnableTOTP(adminID int64, secret string, backupCodes []string) error {
	codesStr := strings.Join(backupCodes, ",")
	_, err := s.db.Exec(`
		UPDATE admin_credentials
		SET totp_enabled = TRUE, totp_secret = ?, totp_backup_codes = ?, updated_at = ?
		WHERE id = ?
	`, secret, codesStr, time.Now(), adminID)
	return err
}

// DisableTOTP disables 2FA for an admin account
// Per AI.md PART 17: Requires current TOTP code or recovery key to disable
func (s *Service) DisableTOTP(adminID int64) error {
	_, err := s.db.Exec(`
		UPDATE admin_credentials
		SET totp_enabled = FALSE, totp_secret = NULL, totp_backup_codes = NULL, updated_at = ?
		WHERE id = ?
	`, time.Now(), adminID)
	return err
}
