// SPDX-License-Identifier: MIT
package validation

import (
	"strings"
	"testing"
)

// TestValidateUsername tests admin username validation
// VidVeil only has server admins, no regular users
func TestValidateUsername(t *testing.T) {
	tests := []struct {
		username    string
		shouldError bool
		errorMsg    string
	}{
		// Valid usernames (admin)
		{"validuser", false, ""},
		{"valid-user", false, ""},
		{"user123", false, ""},
		{"abc", false, ""},
		{"admin", false, ""}, // admins can use reserved names
		{"root", false, ""},
		// max length
		{strings.Repeat("a", 32), false, ""},

		// Invalid - too short
		{"ab", true, "at least 3 characters"},
		{"", true, "at least 3 characters"},

		// Invalid - too long
		{strings.Repeat("a", 33), true, "cannot exceed 32 characters"},

		// Invalid - starts with number
		{"123user", true, "must start with a letter"},

		// Invalid - special characters
		{"user@name", true, "only contain"},
		{"user.name", true, "only contain"},
		{"user name", true, "only contain"},
	}

	for _, tt := range tests {
		name := tt.username
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			// VidVeil only uses admin validation (isAdmin=true)
			err := ValidateUsername(tt.username, true)
			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateUsername(%q, true) expected error containing %q, got nil",
						tt.username, tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateUsername(%q, true) error = %q, want containing %q",
						tt.username, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateUsername(%q, true) unexpected error: %v",
						tt.username, err)
				}
			}
		})
	}
}

// TestValidateAdminPassword tests admin password validation
// VidVeil only has server admins - 12 char minimum
func TestValidateAdminPassword(t *testing.T) {
	tests := []struct {
		password    string
		shouldError bool
		errorMsg    string
	}{
		// Valid passwords (must have upper, lower, number, special, 12+ chars)
		{"StrongP@ss12", false, ""},
		{"Valid12345!A", false, ""},
		{"MyP@ssw0rd12", false, ""},

		// Invalid - too short (admin requires 12 chars)
		{"StrongP@s1", true, "at least 12 characters"},
		{"short", true, "at least 12 characters"},
		{"", true, "at least 12 characters"},

		// Invalid - leading/trailing whitespace per AI.md PART 1
		{" StrongP@ss12", true, "leading or trailing whitespace"},
		{"StrongP@ss12 ", true, "leading or trailing whitespace"},
		{" StrongP@ss12 ", true, "leading or trailing whitespace"},

		// Invalid - missing requirements
		{"password12345", true, "uppercase letter"},
		{"123456789012", true, "uppercase letter"},
		{"PASSWORD12345", true, "lowercase letter"},
		{"PasswordABCDE", true, "number"},
		{"Password12345", true, "special character"},
	}

	for _, tt := range tests {
		name := tt.password
		if len(name) > 20 {
			name = name[:20] + "..."
		}
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			err := ValidateAdminPassword(tt.password)
			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateAdminPassword(%q) expected error, got nil", tt.password)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateAdminPassword error = %q, want containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateAdminPassword(%q) unexpected error: %v", tt.password, err)
				}
			}
		})
	}
}

func TestUsernameFormat(t *testing.T) {
	tests := []struct {
		name   string
		valid  bool
		reason string
	}{
		{"validuser", true, ""},
		{"valid-user", true, ""},
		{"valid123", true, ""},
		{"a-b-c", true, ""},

		// Invalid cases
		{"_invalid", false, "must start with letter"},
		{"-invalid", false, "must start with letter"},
		{"1invalid", false, "must start with letter"},
		{"invalid_", false, "cannot end with underscore"},
		{"invalid-", false, "cannot end with hyphen"},
		{"inv--alid", false, "consecutive special chars"},
		{"inv__alid", false, "consecutive special chars"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.name, true)
			if tt.valid && err != nil {
				t.Errorf("ValidateUsername(%q) should be valid, got error: %v", tt.name, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("ValidateUsername(%q) should be invalid: %s", tt.name, tt.reason)
			}
		})
	}
}

func TestUsernameError(t *testing.T) {
	err := &UsernameError{
		Field:   "username",
		Message: "test error message",
	}

	if err.Error() != "test error message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error message")
	}
}
