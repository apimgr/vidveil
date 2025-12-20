// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 31: Admin Credentials Management
// Admin credentials stored in database, NOT config file
// Password hashing uses Argon2id per TEMPLATE.md PART 2
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

	"github.com/apimgr/vidveil/src/services/validation"
	"golang.org/x/crypto/argon2"
)

// Service manages admin credentials per TEMPLATE.md PART 31
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

// Initialize checks for first run and generates setup token if needed
// Per TEMPLATE.md PART 31: App is FULLY FUNCTIONAL before setup
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

		// Console output is handled in main.go per TEMPLATE.md PART 31
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
// Uses Argon2id for password hashing per TEMPLATE.md PART 2
// Server admin accounts are exempt from username blocklist per PART 31
func (s *Service) CreateAdmin(username, password string, isPrimary bool) (*Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate username (admin accounts exempt from blocklist)
	if err := validation.ValidateUsername(username, true); err != nil {
		return nil, err
	}

	// Validate password
	if err := validation.ValidatePassword(password); err != nil {
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
// Uses Argon2id for password verification per TEMPLATE.md PART 2
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
// Uses Argon2id for password hashing per TEMPLATE.md PART 2
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

	expires := time.Now().Add(7 * 24 * time.Hour) // 7 days

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

// Argon2id parameters per TEMPLATE.md PART 2 (OWASP 2023 recommendations)
const (
	argonTime    = 3         // iterations
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4         // parallelism
	argonKeyLen  = 32        // output length in bytes
	argonSaltLen = 16        // salt length in bytes
)

// hashPassword hashes a password using Argon2id per TEMPLATE.md PART 2
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
