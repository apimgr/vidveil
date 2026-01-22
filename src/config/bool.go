// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"strings"
)

// Truthy values (case-insensitive)
// Per AI.md PART 5 - NON-NEGOTIABLE
var truthyValues = map[string]bool{
	"1": true, "y": true, "t": true,
	"yes": true, "true": true, "on": true, "ok": true,
	"enable": true, "enabled": true,
	"yep": true, "yup": true, "yeah": true,
	"aye": true, "si": true, "oui": true, "da": true, "hai": true,
	"affirmative": true, "accept": true, "allow": true, "grant": true,
	"sure": true, "totally": true,
}

// Falsy values (case-insensitive)
// Per AI.md PART 5 - NON-NEGOTIABLE
var falsyValues = map[string]bool{
	"0": true, "n": true, "f": true,
	"no": true, "false": true, "off": true,
	"disable": true, "disabled": true,
	"nope": true, "nah": true, "nay": true,
	"nein": true, "non": true, "niet": true, "iie": true, "lie": true,
	"negative": true, "reject": true, "block": true, "revoke": true,
	"deny": true, "never": true, "noway": true,
}

// ParseBoolWithDefault parses a string into a boolean using truthy/falsy values.
// Returns the parsed value and nil on success.
// Returns false and an error for invalid values.
// Empty string returns the provided default value.
func ParseBoolWithDefault(s string, defaultVal bool) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if s == "" {
		return defaultVal, nil
	}

	if truthyValues[s] {
		return true, nil
	}

	if falsyValues[s] {
		return false, nil
	}

	return false, fmt.Errorf("invalid boolean value: %q", s)
}

// MustParseBool parses a string into a boolean, panics on invalid value.
// Use only during initialization where invalid config should halt startup.
func MustParseBool(s string, defaultVal bool) bool {
	val, err := ParseBoolWithDefault(s, defaultVal)
	if err != nil {
		panic(err)
	}
	return val
}

// IsTruthy returns true if the string is a truthy value.
// Returns false for empty, invalid, or falsy values (no error).
func IsTruthy(s string) bool {
	return truthyValues[strings.TrimSpace(strings.ToLower(s))]
}

// IsFalsy returns true if the string is a falsy value.
// Returns false for empty, invalid, or truthy values (no error).
func IsFalsy(s string) bool {
	return falsyValues[strings.TrimSpace(strings.ToLower(s))]
}

// IsValidBool returns true if the string is a valid boolean value (truthy or falsy)
func IsValidBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return truthyValues[s] || falsyValues[s]
}

// ParseBool parses a string into a boolean using truthy/falsy values.
// Returns true for truthy values, false for everything else (empty, falsy, invalid).
// This is the primary function to use - NEVER use strconv.ParseBool().
func ParseBool(s string) bool {
	return IsTruthy(s)
}
