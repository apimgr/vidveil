// SPDX-License-Identifier: MIT
package validation

import (
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		username    string
		isAdmin     bool
		shouldError bool
		errorMsg    string
	}{
		// Valid usernames
		{"validuser", false, false, ""},
		{"valid-user", false, false, ""},
		{"user123", false, false, ""},
		{"abc", false, false, ""},
		// max length
		{strings.Repeat("a", 32), false, false, ""},

		// Invalid - too short
		{"ab", false, true, "at least 3 characters"},
		{"", false, true, "at least 3 characters"},

		// Invalid - too long
		{strings.Repeat("a", 33), false, true, "cannot exceed 32 characters"},

		// Invalid - starts with number
		{"123user", false, true, "must start with a letter"},

		// Invalid - special characters
		{"user@name", false, true, "only contain"},
		{"user.name", false, true, "only contain"},
		{"user name", false, true, "only contain"},

		// Admin exemption from blocklist
		{"root", true, false, ""},
	}

	for _, tt := range tests {
		name := tt.username
		if tt.isAdmin {
			name += "_admin"
		}
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			err := ValidateUsername(tt.username, tt.isAdmin)
			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateUsername(%q, %v) expected error containing %q, got nil",
						tt.username, tt.isAdmin, tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateUsername(%q, %v) error = %q, want containing %q",
						tt.username, tt.isAdmin, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateUsername(%q, %v) unexpected error: %v",
						tt.username, tt.isAdmin, err)
				}
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password    string
		shouldError bool
		errorMsg    string
	}{
		// Valid passwords (must have upper, lower, number, special)
		{"StrongP@ss1", false, ""},
		{"Valid123!", false, ""},
		{"MyP@ssw0rd", false, ""},

		// Invalid - too short
		{"short", true, "at least 8 characters"},
		{"1234567", true, "at least 8 characters"},
		{"", true, "at least 8 characters"},

		// Invalid - leading/trailing whitespace per AI.md PART 1
		{" StrongP@ss1", true, "leading or trailing whitespace"},
		{"StrongP@ss1 ", true, "leading or trailing whitespace"},
		{" StrongP@ss1 ", true, "leading or trailing whitespace"},
		{"\tStrongP@ss1", true, "leading or trailing whitespace"},
		{"StrongP@ss1\n", true, "leading or trailing whitespace"},

		// Invalid - missing requirements
		{"password123", true, "uppercase letter"},
		{"12345678", true, "uppercase letter"},
		{strings.Repeat("a", 8), true, "uppercase letter"},
		{"PASSWORD123", true, "lowercase letter"},
		{"PasswordABC", true, "number"},
		{"Password123", true, "special character"},
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
			err := ValidatePassword(tt.password)
			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidatePassword(%q) expected error, got nil", tt.password)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidatePassword error = %q, want containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePassword(%q) unexpected error: %v", tt.password, err)
				}
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email       string
		shouldError bool
	}{
		// Valid emails
		{"user@example.com", false},
		{"user.name@example.com", false},
		{"user+tag@example.com", false},
		{"user@sub.example.com", false},
		{"user@example.co.uk", false},

		// Invalid emails
		{"", true},
		{"invalid", true},
		{"@example.com", true},
		{"user@", true},
		{"user@.com", true},
		{"user@example", true},
		{"user example.com", true},
	}

	for _, tt := range tests {
		name := tt.email
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.shouldError && err == nil {
				t.Errorf("ValidateEmail(%q) expected error, got nil", tt.email)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ValidateEmail(%q) unexpected error: %v", tt.email, err)
			}
		})
	}
}

func TestUsernameBlocklist(t *testing.T) {
	// Test that blocklisted names are rejected for non-admins
	blockedNames := []string{
		"admin", "administrator", "root", "system", "moderator",
		"support", "help", "info", "contact", "webmaster",
	}

	for _, name := range blockedNames {
		t.Run(name+"_blocked", func(t *testing.T) {
			err := ValidateUsername(name, false)
			if err == nil {
				t.Errorf("ValidateUsername(%q, false) should be blocked", name)
			}
		})
	}

	// Test non-blocked names
	allowedNames := []string{"john", "jane", "testuser", "myname"}
	for _, name := range allowedNames {
		t.Run(name+"_allowed", func(t *testing.T) {
			err := ValidateUsername(name, false)
			if err != nil {
				t.Errorf("ValidateUsername(%q, false) unexpected error: %v", name, err)
			}
		})
	}
}

func TestUsernameFormat(t *testing.T) {
	tests := []struct {
		name    string
		valid   bool
		reason  string
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
			// Use admin to skip blocklist
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
