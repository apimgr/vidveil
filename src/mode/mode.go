// SPDX-License-Identifier: MIT
// Package mode handles application mode (production/development) and debug settings
// per AI.md PART 5 - NON-NEGOTIABLE
package mode

import (
	"os"
	"runtime"
	"strings"

	"github.com/apimgr/vidveil/src/config"
)

var (
	currentMode  = Production
	debugEnabled = false
)

// Mode represents the application mode
type Mode int

const (
	// Production mode (default) - secure defaults, minimal logging
	Production Mode = iota
	// Development mode - relaxed security, verbose logging
	Development
)

// String returns the mode as a string
func (m Mode) String() string {
	switch m {
	case Development:
		return "development"
	default:
		return "production"
	}
}

// Set sets the application mode from a string
// Accepts: dev, development, prod, production
func Set(m string) {
	switch strings.ToLower(m) {
	case "dev", "development":
		currentMode = Development
	default:
		currentMode = Production
	}
	updateProfilingSettings()
}

// SetDebug enables or disables debug mode
// Debug mode enables pprof endpoints, verbose logging, etc.
func SetDebug(enabled bool) {
	debugEnabled = enabled
	updateProfilingSettings()
}

// updateProfilingSettings enables/disables profiling based on debug flag
func updateProfilingSettings() {
	if debugEnabled {
		// Enable profiling when debug is on
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
	} else {
		// Disable profiling when debug is off
		runtime.SetBlockProfileRate(0)
		runtime.SetMutexProfileFraction(0)
	}
}

// Current returns the current mode
func Current() Mode {
	return currentMode
}

// IsDevelopment returns true if in development mode
func IsDevelopment() bool {
	return currentMode == Development
}

// IsProduction returns true if in production mode
func IsProduction() bool {
	return currentMode == Production
}

// IsDebug returns true if debug mode is enabled (--debug or DEBUG=true)
// Debug mode is separate from development mode
func IsDebug() bool {
	return debugEnabled
}

// ModeString returns mode string with debug suffix if enabled
// Example: "production", "production [debugging]", "development [debugging]"
func ModeString() string {
	s := currentMode.String()
	if debugEnabled {
		s += " [debugging]"
	}
	return s
}

// FromEnv sets mode and debug from environment variables
// MODE env var sets mode, DEBUG env var sets debug
func FromEnv() {
	if m := os.Getenv("MODE"); m != "" {
		Set(m)
	}
	if config.IsTruthy(os.Getenv("DEBUG")) {
		SetDebug(true)
	}
}

// Initialize initializes mode from CLI flags and environment
// CLI flags take priority over environment variables
func Initialize(modeFlag string, debugFlag bool) {
	// Start with environment
	FromEnv()

	// CLI flags override environment
	if modeFlag != "" {
		Set(modeFlag)
	}
	if debugFlag {
		SetDebug(true)
	}
}

// ConsoleIcon returns an emoji icon for the current mode
// Used for console output per AI.md
func ConsoleIcon() string {
	if IsProduction() {
		return "🔒"
	}
	return "🔧"
}

// ConsoleModeMessage returns a formatted mode message for console output
// Example: "🔒 Running in mode: production [debugging]"
func ConsoleModeMessage() string {
	return ConsoleIcon() + " Running in mode: " + ModeString()
}

// ShouldLogVerbose returns true if verbose logging should be enabled
// Verbose in development mode OR when debug is enabled
func ShouldLogVerbose() bool {
	return IsDevelopment() || IsDebug()
}

// ShouldCacheTemplates returns true if templates should be cached
// Cached in production mode, not cached in development (hot reload)
func ShouldCacheTemplates() bool {
	return IsProduction() && !IsDebug()
}

// ShouldCacheStatic returns true if static files should be cached
// Cached in production mode, not cached in development (hot reload)
func ShouldCacheStatic() bool {
	return IsProduction() && !IsDebug()
}

// ShouldShowStackTraces returns true if stack traces should be shown in errors
// Shown in development mode or when debug is enabled
func ShouldShowStackTraces() bool {
	return IsDevelopment() || IsDebug()
}

// ShouldEnableDebugEndpoints returns true if debug endpoints should be registered
// Only enabled when debug flag is set, regardless of mode
func ShouldEnableDebugEndpoints() bool {
	return IsDebug()
}
