// SPDX-License-Identifier: MIT
// AI.md PART 8: NO_COLOR Support
// All binaries (server, client, agent) MUST respect the NO_COLOR standard.
package terminal

import (
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ColorMode represents the color output mode
type ColorMode int

const (
	// ColorModeAuto auto-detects color support (default)
	ColorModeAuto ColorMode = iota
	// ColorModeAlways forces color output
	ColorModeAlways
	// ColorModeNever disables color output
	ColorModeNever
)

// colorConfig holds the global color configuration
// Per AI.md PART 8: Thread-safe access required
var (
	colorConfig struct {
		mode      ColorMode
		emojiMode ColorMode
		mu        sync.RWMutex
	}
)

// SetColorMode sets the color output mode
// Per AI.md PART 8: CLI flag > config > NO_COLOR env > auto-detect
func SetColorMode(mode ColorMode) {
	colorConfig.mu.Lock()
	defer colorConfig.mu.Unlock()
	colorConfig.mode = mode
}

// SetEmojiMode sets the emoji output mode
// Per AI.md PART 8: Config can force emojis on even when NO_COLOR is set
func SetEmojiMode(mode ColorMode) {
	colorConfig.mu.Lock()
	defer colorConfig.mu.Unlock()
	colorConfig.emojiMode = mode
}

// ParseColorFlag parses the --color flag value
// Per AI.md PART 8: Accepts always, never, auto
func ParseColorFlag(value string) ColorMode {
	switch strings.ToLower(value) {
	case "always", "yes", "true", "on", "1":
		return ColorModeAlways
	case "never", "no", "false", "off", "0":
		return ColorModeNever
	default:
		return ColorModeAuto
	}
}

// ColorEnabled checks if color output should be used
// Per AI.md PART 8: Priority order (highest to lowest):
// 1. CLI flag (--color=always/never)
// 2. Config file (output.color)
// 3. NO_COLOR env var (non-empty = disable)
// 4. Auto-detect (TTY check, TERM variable)
func ColorEnabled() bool {
	colorConfig.mu.RLock()
	mode := colorConfig.mode
	colorConfig.mu.RUnlock()

	switch mode {
	case ColorModeAlways:
		return true
	case ColorModeNever:
		return false
	default:
		// Auto-detect
		return colorAutoDetect()
	}
}

// colorAutoDetect performs automatic color detection
// Per AI.md PART 8
func colorAutoDetect() bool {
	// Per AI.md PART 8: NO_COLOR env var (non-empty = disable)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if stdout is a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}

	// TERM=dumb disables colors
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return true
}

// EmojiEnabled checks if emoji output should be used
// Per AI.md PART 8:
// - Config can force emojis on (overrides NO_COLOR for emojis only)
// - Otherwise, NO_COLOR disables emojis
// - TERM=dumb disables emojis
func EmojiEnabled() bool {
	colorConfig.mu.RLock()
	mode := colorConfig.emojiMode
	colorConfig.mu.RUnlock()

	// Config override (force emojis on)
	if mode == ColorModeAlways {
		return true
	}

	// Config disable (force emojis off)
	if mode == ColorModeNever {
		return false
	}

	// Auto-detect: NO_COLOR disables emojis
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// TERM=dumb disables emojis
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return true
}

// ANSI color codes
// Per AI.md PART 8: Standard ANSI escape sequences
const (
	ANSIReset     = "\033[0m"
	ANSIBold      = "\033[1m"
	ANSIDim       = "\033[2m"
	ANSIItalic    = "\033[3m"
	ANSIUnderline = "\033[4m"

	// Foreground colors
	ANSIBlack   = "\033[30m"
	ANSIRed     = "\033[31m"
	ANSIGreen   = "\033[32m"
	ANSIYellow  = "\033[33m"
	ANSIBlue    = "\033[34m"
	ANSIMagenta = "\033[35m"
	ANSICyan    = "\033[36m"
	ANSIWhite   = "\033[37m"

	// Bright foreground colors
	ANSIBrightBlack   = "\033[90m"
	ANSIBrightRed     = "\033[91m"
	ANSIBrightGreen   = "\033[92m"
	ANSIBrightYellow  = "\033[93m"
	ANSIBrightBlue    = "\033[94m"
	ANSIBrightMagenta = "\033[95m"
	ANSIBrightCyan    = "\033[96m"
	ANSIBrightWhite   = "\033[97m"

	// Background colors
	ANSIBgBlack   = "\033[40m"
	ANSIBgRed     = "\033[41m"
	ANSIBgGreen   = "\033[42m"
	ANSIBgYellow  = "\033[43m"
	ANSIBgBlue    = "\033[44m"
	ANSIBgMagenta = "\033[45m"
	ANSIBgCyan    = "\033[46m"
	ANSIBgWhite   = "\033[47m"
)

// Color applies color code if colors are enabled
// Returns the string wrapped with ANSI codes, or plain string if colors disabled
func Color(text, colorCode string) string {
	if !ColorEnabled() {
		return text
	}
	return colorCode + text + ANSIReset
}

// Bold returns bold text if colors enabled
func Bold(text string) string {
	return Color(text, ANSIBold)
}

// Red returns red text if colors enabled
func Red(text string) string {
	return Color(text, ANSIRed)
}

// Green returns green text if colors enabled
func Green(text string) string {
	return Color(text, ANSIGreen)
}

// Yellow returns yellow text if colors enabled
func Yellow(text string) string {
	return Color(text, ANSIYellow)
}

// Blue returns blue text if colors enabled
func Blue(text string) string {
	return Color(text, ANSIBlue)
}

// Cyan returns cyan text if colors enabled
func Cyan(text string) string {
	return Color(text, ANSICyan)
}

// Dim returns dim text if colors enabled
func Dim(text string) string {
	return Color(text, ANSIDim)
}

// Emoji returns the emoji if enabled, otherwise returns the fallback text
// Per AI.md PART 8: Emojis disabled when NO_COLOR is set
func Emoji(emoji, fallback string) string {
	if EmojiEnabled() {
		return emoji
	}
	return fallback
}

// StatusIcon returns appropriate status icon based on emoji support
// Per AI.md PART 8
func StatusIcon(success bool) string {
	if success {
		return Emoji("✅", "[OK]")
	}
	return Emoji("❌", "[FAIL]")
}

// WarningIcon returns warning icon based on emoji support
func WarningIcon() string {
	return Emoji("⚠️", "[WARN]")
}

// InfoIcon returns info icon based on emoji support
func InfoIcon() string {
	return Emoji("ℹ️", "[INFO]")
}

// RocketIcon returns rocket icon based on emoji support
func RocketIcon() string {
	return Emoji("🚀", ">>")
}

// GlobeIcon returns globe icon based on emoji support
func GlobeIcon() string {
	return Emoji("🌐", "*")
}

// KeyIcon returns key icon based on emoji support
func KeyIcon() string {
	return Emoji("🔑", "[KEY]")
}

// LockIcon returns lock icon based on emoji support
func LockIcon() string {
	return Emoji("🔒", "[LOCK]")
}

// WrenchIcon returns wrench icon based on emoji support
func WrenchIcon() string {
	return Emoji("🔧", "[DEV]")
}

// BugIcon returns bug icon based on emoji support
func BugIcon() string {
	return Emoji("🐛", "[DEBUG]")
}

// StopIcon returns stop icon based on emoji support
func StopIcon() string {
	return Emoji("🛑", "[STOP]")
}

// UserIcon returns user icon based on emoji support
func UserIcon() string {
	return Emoji("👤", "[USER]")
}
