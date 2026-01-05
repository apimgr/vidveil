package auth

import (
"crypto/rand"
"encoding/base64"
"fmt"
"strings"
)

// TokenPrefix types per PART 11
const (
PrefixAdmin = "adm_" // Admin primary token (all projects)
PrefixUser  = "usr_" // User primary token (multi-user)
PrefixOrg   = "org_" // Organization token (orgs)
PrefixKey   = "key_" // Scoped API key
)

// GenerateToken creates a secure token with prefix per PART 11
// Format: {prefix}_{32_alphanumeric_chars}
// Example: adm_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
func GenerateToken(prefix string) (string, error) {
bytes := make([]byte, 32)
if _, err := rand.Read(bytes); err != nil {
return "", fmt.Errorf("failed to generate random bytes: %w", err)
}

encoded := base64.RawURLEncoding.EncodeToString(bytes)
if len(encoded) > 32 {
encoded = encoded[:32]
}

return prefix + encoded, nil
}

// GetTokenPrefix extracts first 8 chars for display
func GetTokenPrefix(token string) string {
if len(token) >= 8 {
return token[:8]
}
return token
}

// ValidateTokenFormat checks if token follows format rules
func ValidateTokenFormat(token string) bool {
parts := strings.SplitN(token, "_", 2)
if len(parts) != 2 {
return false
}

prefix := parts[0] + "_"
body := parts[1]

switch prefix {
case PrefixAdmin, PrefixUser, PrefixOrg, PrefixKey:
default:
return false
}

if len(body) != 32 {
return false
}

return true
}
