// SPDX-License-Identifier: MIT
// AI.md PART 11: API Token Security
// VidVeil is stateless - no PART 34 (users) or PART 35 (orgs), only admin tokens

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// TokenPrefix types per PART 11
// VidVeil only uses admin tokens (no users or orgs)
const (
	// PrefixAdmin is admin primary token (all projects)
	PrefixAdmin = "adm_"
	// PrefixAdminAgt is admin agent token
	PrefixAdminAgt = "adm_agt_"
)

// TokenScope defines access level per PART 11
type TokenScope string

const (
	// ScopeGlobal grants all permissions owner has access to
	ScopeGlobal TokenScope = "global"
	// ScopeReadWrite grants read and write (no delete, no admin)
	ScopeReadWrite TokenScope = "read-write"
	// ScopeRead grants read-only operations
	ScopeRead TokenScope = "read"
)

// ExpirationOptions per AI.md PART 11
var ExpirationOptions = map[string]time.Duration{
	"never":   0,
	"7days":   7 * 24 * time.Hour,
	"1month":  30 * 24 * time.Hour,
	"6months": 180 * 24 * time.Hour,
	"1year":   365 * 24 * time.Hour,
}

// TokenInfo holds validated token information
type TokenInfo struct {
	// OwnerType is 'admin' (VidVeil only has admins)
	OwnerType string
	// OwnerID is admin.id
	OwnerID int64
	// Name is user-provided label
	Name string
	// Scope is 'global', 'read-write', or 'read'
	Scope TokenScope
	// IsAgent indicates whether this is an agent token
	IsAgent bool
}

// GenerateToken creates a secure token with prefix per PART 11
// Format: {prefix}_{32_alphanumeric_chars}
// Example: adm_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
func GenerateToken(prefix string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Use base64 URL encoding for alphanumeric output
	encoded := base64.RawURLEncoding.EncodeToString(bytes)
	if len(encoded) > 32 {
		encoded = encoded[:32]
	}

	return prefix + encoded, nil
}

// HashToken returns SHA-256 hash of token per PART 11
// Tokens are never stored in plaintext
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GetTokenPrefix extracts first 8 chars for display per PART 11
// Example: "adm_a1b2..." for display purposes
func GetTokenPrefix(token string) string {
	if len(token) >= 8 {
		return token[:8] + "..."
	}
	return token
}

// ValidateTokenFormat checks if token follows format rules per PART 11
// VidVeil only supports admin tokens (adm_ and adm_agt_)
func ValidateTokenFormat(token string) bool {
	// Check for agent prefix first (adm_agt_)
	if strings.HasPrefix(token, PrefixAdminAgt) {
		return len(strings.TrimPrefix(token, PrefixAdminAgt)) == 32
	}

	// Standard admin token: adm_{32_chars}
	if strings.HasPrefix(token, PrefixAdmin) {
		return len(strings.TrimPrefix(token, PrefixAdmin)) == 32
	}

	return false
}

// GetTokenType returns the type of token based on prefix
// VidVeil only supports admin tokens
func GetTokenType(token string) string {
	if strings.HasPrefix(token, PrefixAdminAgt) {
		return "admin_agent"
	}
	if strings.HasPrefix(token, PrefixAdmin) {
		return "admin"
	}
	return "unknown"
}

// IsAgentToken checks if token is an agent token
func IsAgentToken(token string) bool {
	return strings.HasPrefix(token, PrefixAdminAgt)
}
