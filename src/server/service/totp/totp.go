// SPDX-License-Identifier: MIT
// AI.md PART 11: Security - Two-Factor Authentication (2FA) Support
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultDigits is the standard TOTP code length
	DefaultDigits = 6
	// DefaultPeriod is the standard TOTP time step in seconds
	DefaultPeriod = 30
	// BackupCodeCount is the number of backup codes to generate
	BackupCodeCount = 10
	// BackupCodeLength is the length of each backup code
	BackupCodeLength = 8
)

// TOTPService handles TOTP operations for 2FA
type TOTPService struct {
	issuer string
}

// NewTOTPService creates a new TOTP service
func NewTOTPService(issuer string) *TOTPService {
	return &TOTPService{
		issuer: issuer,
	}
}

// GenerateSecret generates a new TOTP secret
func (s *TOTPService) GenerateSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateBackupCodes generates one-time use backup codes
func (s *TOTPService) GenerateBackupCodes() ([]string, error) {
	codes := make([]string, BackupCodeCount)
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	for i := 0; i < BackupCodeCount; i++ {
		code := make([]byte, BackupCodeLength)
		randomBytes := make([]byte, BackupCodeLength)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		for j := 0; j < BackupCodeLength; j++ {
			code[j] = charset[int(randomBytes[j])%len(charset)]
		}
		// Format as XXXX-XXXX for readability
		codes[i] = string(code[:4]) + "-" + string(code[4:])
	}
	return codes, nil
}

// GetProvisioningURI generates the otpauth:// URI for authenticator apps
func (s *TOTPService) GetProvisioningURI(accountName, secret string) string {
	// otpauth://totp/ISSUER:ACCOUNT?secret=SECRET&issuer=ISSUER&algorithm=SHA1&digits=6&period=30
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		s.issuer,
		accountName,
		secret,
		s.issuer,
		DefaultDigits,
		DefaultPeriod,
	)
}

// ValidateCode validates a TOTP code
func (s *TOTPService) ValidateCode(secret, code string) bool {
	// Allow 1 time step before and after for clock drift
	now := time.Now().Unix()
	for i := int64(-1); i <= 1; i++ {
		counter := (now / DefaultPeriod) + i
		expectedCode := s.generateCode(secret, counter)
		if expectedCode == code {
			return true
		}
	}
	return false
}

// ValidateBackupCode checks if a backup code is valid (caller must track used codes)
func (s *TOTPService) ValidateBackupCode(code string, validCodes []string) bool {
	code = strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	for _, valid := range validCodes {
		normalizedValid := strings.ToUpper(strings.ReplaceAll(valid, "-", ""))
		if code == normalizedValid {
			return true
		}
	}
	return false
}

// generateCode generates a TOTP code for a given counter
func (s *TOTPService) generateCode(secret string, counter int64) string {
	// Decode the base32 secret
	secret = strings.ToUpper(strings.TrimSpace(secret))
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return ""
	}

	// Convert counter to bytes (big-endian)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, uint64(counter))

	// Generate HMAC-SHA1
	mac := hmac.New(sha1.New, secretBytes)
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	// Generate 6-digit code
	return fmt.Sprintf("%06d", code%1000000)
}

// TOTPConfig holds 2FA configuration per AI.md PART 31
type TOTPConfig struct {
	// Enabled allows 2FA to be enabled by users
	Enabled bool `yaml:"enabled"`
	// Required forces all users to enable 2FA
	Required bool `yaml:"required"`
	// RememberDevice allows "trust this device" for N days
	RememberDeviceDays int `yaml:"remember_device_days"`
}

// DefaultTOTPConfig returns the default 2FA configuration
func DefaultTOTPConfig() TOTPConfig {
	return TOTPConfig{
		Enabled:            true,
		Required:           false,
		RememberDeviceDays: 30,
	}
}

// SetupData contains data needed to set up 2FA for a user
type SetupData struct {
	Secret        string   `json:"secret"`
	ProvisionURI  string   `json:"provision_uri"`
	QRCodeDataURL string   `json:"qr_code_data_url,omitempty"`
	BackupCodes   []string `json:"backup_codes"`
}

// Setup generates all data needed to enable 2FA for an account
func (s *TOTPService) Setup(accountName string) (*SetupData, error) {
	secret, err := s.GenerateSecret()
	if err != nil {
		return nil, err
	}

	backupCodes, err := s.GenerateBackupCodes()
	if err != nil {
		return nil, err
	}

	return &SetupData{
		Secret:       secret,
		ProvisionURI: s.GetProvisioningURI(accountName, secret),
		BackupCodes:  backupCodes,
	}, nil
}
