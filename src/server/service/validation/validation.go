// SPDX-License-Identifier: MIT
// AI.md PART 17: Server Admin username and password validation
// VidVeil is stateless - no PART 34 (Multi-User), only Server Admin validation
package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// Username validation regex per AI.md PART 31
// Allowed: a-z, 0-9, _, - (lowercase only)
// Must start with letter
// Cannot end with _ or -
// No consecutive special chars
var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[_-][a-z0-9]+)*$`)

// UsernameError represents a validation error
type UsernameError struct {
	Field   string
	Message string
}

func (e *UsernameError) Error() string {
	return e.Message
}

// ValidateUsername validates a Server Admin username per AI.md PART 17
// VidVeil only has Server Admins, no regular users - blocklist not applicable
func ValidateUsername(username string, isAdmin bool) error {
	// Convert to lowercase for validation
	username = strings.ToLower(username)

	// Check length
	if len(username) < 3 {
		return &UsernameError{
			Field:   "username",
			Message: "Username must be at least 3 characters",
		}
	}

	if len(username) > 32 {
		return &UsernameError{
			Field:   "username",
			Message: "Username cannot exceed 32 characters",
		}
	}

	// Check format
	if !usernameRegex.MatchString(username) {
		// Determine specific error
		if username[0] < 'a' || username[0] > 'z' {
			return &UsernameError{
				Field:   "username",
				Message: "Username must start with a letter",
			}
		}
		lastChar := username[len(username)-1]
		if lastChar == '_' || lastChar == '-' {
			return &UsernameError{
				Field:   "username",
				Message: "Username cannot end with underscore or hyphen",
			}
		}
		if strings.Contains(username, "__") || strings.Contains(username, "--") ||
			strings.Contains(username, "_-") || strings.Contains(username, "-_") {
			return &UsernameError{
				Field:   "username",
				Message: "Username cannot contain consecutive special characters",
			}
		}
		return &UsernameError{
			Field:   "username",
			Message: "Username can only contain lowercase letters, numbers, underscore, and hyphen",
		}
	}

	return nil
}

// ValidateAdminPassword validates Server Admin password with stricter requirements
// Minimum 12 characters for admin accounts per AI.md PART 17
func ValidateAdminPassword(password string) error {
	// Per AI.md PART 1: Reject passwords with leading/trailing whitespace
	if password != strings.TrimSpace(password) {
		return &UsernameError{
			Field:   "password",
			Message: "Password cannot have leading or trailing whitespace",
		}
	}

	minLen := 12

	if len(password) < minLen {
		return &UsernameError{
			Field:   "password",
			Message: fmt.Sprintf("Password must be at least %d characters", minLen),
		}
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, c := range password {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?`~", c):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one uppercase letter",
		}
	}
	if !hasLower {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one lowercase letter",
		}
	}
	if !hasNumber {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one number",
		}
	}
	if !hasSpecial {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one special character",
		}
	}

	return nil
}
