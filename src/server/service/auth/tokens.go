// SPDX-License-Identifier: MIT
// AI.md PART 11: API Token Security

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
const (
	// PrefixAdmin is admin primary token (all projects)
	PrefixAdmin = "adm_"
	// PrefixUser is user primary token (multi-user)
	PrefixUser = "usr_"
	// PrefixOrg is organization token (orgs)
	PrefixOrg = "org_"
	// PrefixAdminAgt is admin agent token
	PrefixAdminAgt = "adm_agt_"
	// PrefixUserAgt is user agent token
	PrefixUserAgt = "usr_agt_"
	// PrefixOrgAgt is org agent token
	PrefixOrgAgt = "org_agt_"
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
	"never":    0,
	"7days":    7 * 24 * time.Hour,
	"1month":   30 * 24 * time.Hour,
	"6months":  180 * 24 * time.Hour,
	"1year":    365 * 24 * time.Hour,
}

// TokenInfo holds validated token information
type TokenInfo struct {
	// OwnerType is 'admin', 'user', or 'org'
	OwnerType string
	// OwnerID is admin.id, user.id, or org.id
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
func ValidateTokenFormat(token string) bool {
	// Check for compound agent prefixes first (adm_agt_, usr_agt_, org_agt_)
	if strings.HasPrefix(token, PrefixAdminAgt) {
		return len(strings.TrimPrefix(token, PrefixAdminAgt)) == 32
	}
	if strings.HasPrefix(token, PrefixUserAgt) {
		return len(strings.TrimPrefix(token, PrefixUserAgt)) == 32
	}
	if strings.HasPrefix(token, PrefixOrgAgt) {
		return len(strings.TrimPrefix(token, PrefixOrgAgt)) == 32
	}

	// Standard single-prefix tokens: {prefix}_{32_chars}
	parts := strings.SplitN(token, "_", 2)
	if len(parts) != 2 {
		return false
	}

	prefix := parts[0] + "_"
	body := parts[1]

	switch prefix {
	case PrefixAdmin, PrefixUser, PrefixOrg:
		// Valid prefixes
	default:
		return false
	}

	return len(body) == 32
}

// GetTokenType returns the type of token based on prefix
func GetTokenType(token string) string {
	if strings.HasPrefix(token, PrefixAdminAgt) {
		return "admin_agent"
	}
	if strings.HasPrefix(token, PrefixUserAgt) {
		return "user_agent"
	}
	if strings.HasPrefix(token, PrefixOrgAgt) {
		return "org_agent"
	}
	if strings.HasPrefix(token, PrefixAdmin) {
		return "admin"
	}
	if strings.HasPrefix(token, PrefixUser) {
		return "user"
	}
	if strings.HasPrefix(token, PrefixOrg) {
		return "org"
	}
	return "unknown"
}

// IsAgentToken checks if token is an agent token
func IsAgentToken(token string) bool {
	return strings.HasPrefix(token, PrefixAdminAgt) ||
		strings.HasPrefix(token, PrefixUserAgt) ||
		strings.HasPrefix(token, PrefixOrgAgt)
}
