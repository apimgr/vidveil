// SPDX-License-Identifier: MIT
// AI.md PART 11: App Secrets Management
// Manages installation_secret, cookie_signing_key, and csrf_token_secret

package secrets

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"
)

// SecretKey identifies a secret in the app_secrets table
type SecretKey string

const (
	// InstallationSecret is the 32-byte root secret for HMAC/PGP derivation
	// Per PART 11: Used for security.txt ID generation, PGP key encryption
	InstallationSecret SecretKey = "installation_secret"

	// CookieSigningKey is used to sign session cookies (HMAC-SHA256)
	// Per PART 11: Auto-rotated every 90 days; previous key valid for 7 days
	CookieSigningKey SecretKey = "cookie_signing_key"

	// CSRFTokenSecret is used to generate CSRF tokens
	CSRFTokenSecret SecretKey = "csrf_token_secret"
)

// Manager handles app secrets lifecycle
type Manager struct {
	db *sql.DB
}

// NewManager creates a new secrets manager
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// EnsureSecrets initializes all required secrets on first run
// Per PART 11: Secrets are generated on first startup if missing
func (m *Manager) EnsureSecrets(ctx context.Context) error {
	for _, key := range []SecretKey{InstallationSecret, CookieSigningKey, CSRFTokenSecret} {
		exists, err := m.exists(ctx, key)
		if err != nil {
			return fmt.Errorf("check secret %s: %w", key, err)
		}
		if !exists {
			if err := m.generate(ctx, key); err != nil {
				return fmt.Errorf("generate secret %s: %w", key, err)
			}
		}
	}
	return nil
}

// Get retrieves a secret value (base64-encoded)
func (m *Manager) Get(ctx context.Context, key SecretKey) ([]byte, error) {
	var value string
	err := m.db.QueryRowContext(ctx,
		"SELECT value FROM app_secrets WHERE key = ?", string(key)).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("secret %s not found", key)
		}
		return nil, fmt.Errorf("get secret %s: %w", key, err)
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode secret %s: %w", key, err)
	}
	return decoded, nil
}

// GetInstallationSecret returns the installation secret bytes
// Convenience method for the most commonly used secret
func (m *Manager) GetInstallationSecret(ctx context.Context) ([]byte, error) {
	return m.Get(ctx, InstallationSecret)
}

// Rotate rotates a secret, keeping the previous value for 7 days
// Per PART 11: Previous secret kept for 7 days to validate in-flight operations
func (m *Manager) Rotate(ctx context.Context, key SecretKey) error {
	// Get current value
	var currentValue string
	err := m.db.QueryRowContext(ctx,
		"SELECT value FROM app_secrets WHERE key = ?", string(key)).Scan(&currentValue)
	if err != nil {
		return fmt.Errorf("get current secret: %w", err)
	}

	// Generate new value
	newValue, err := generateSecretBytes()
	if err != nil {
		return fmt.Errorf("generate new secret: %w", err)
	}

	// Update with new value, store previous
	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour)

	_, err = m.db.ExecContext(ctx, `
		UPDATE app_secrets
		SET value = ?,
		    rotated_at = ?,
		    previous_value = ?,
		    expires_at = ?
		WHERE key = ?`,
		base64.StdEncoding.EncodeToString(newValue),
		now,
		currentValue,
		expiresAt,
		string(key))
	if err != nil {
		return fmt.Errorf("rotate secret: %w", err)
	}

	return nil
}

// ValidateWithPrevious checks if a value matches current or previous (unexpired) secret
// Per PART 11: Previous key valid for 7 days after rotation
func (m *Manager) ValidateWithPrevious(ctx context.Context, key SecretKey, value []byte) (bool, error) {
	var current, previous sql.NullString
	var expiresAt sql.NullTime

	err := m.db.QueryRowContext(ctx,
		"SELECT value, previous_value, expires_at FROM app_secrets WHERE key = ?",
		string(key)).Scan(&current, &previous, &expiresAt)
	if err != nil {
		return false, fmt.Errorf("get secret for validation: %w", err)
	}

	// Check current value
	if current.Valid {
		currentBytes, err := base64.StdEncoding.DecodeString(current.String)
		if err == nil && constantTimeEqual(currentBytes, value) {
			return true, nil
		}
	}

	// Check previous value if not expired
	if previous.Valid && (!expiresAt.Valid || time.Now().Before(expiresAt.Time)) {
		prevBytes, err := base64.StdEncoding.DecodeString(previous.String)
		if err == nil && constantTimeEqual(prevBytes, value) {
			return true, nil
		}
	}

	return false, nil
}

// exists checks if a secret key exists in the database
func (m *Manager) exists(ctx context.Context, key SecretKey) (bool, error) {
	var count int
	err := m.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM app_secrets WHERE key = ?", string(key)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// generate creates a new secret and stores it
func (m *Manager) generate(ctx context.Context, key SecretKey) error {
	secretBytes, err := generateSecretBytes()
	if err != nil {
		return err
	}

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO app_secrets (key, value, created_at)
		VALUES (?, ?, ?)`,
		string(key),
		base64.StdEncoding.EncodeToString(secretBytes),
		time.Now())
	return err
}

// generateSecretBytes generates 32 random bytes
func generateSecretBytes() ([]byte, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	return bytes, nil
}

// constantTimeEqual compares two byte slices in constant time
// Per PART 11: Use constant-time comparison for all secret comparisons
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
