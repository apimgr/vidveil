// SPDX-License-Identifier: MIT
// Package terminal provides terminal utilities
// See AI.md PART 7 for specification
package terminal

import (
	"os"
	"strings"
)

// TerminalSymbols provides Unicode and ASCII symbol sets
type TerminalSymbols struct {
	Checkmark    string
	Cross        string
	Arrow        string
	Bullet       string
	Ellipsis     string
	Spinner      []string
	BoxTopLeft   string
	BoxTopRight  string
	BoxBotLeft   string
	BoxBotRight  string
	BoxHoriz     string
	BoxVert      string
	BoxTeeLeft   string
	BoxTeeRight  string
	BoxTeeTop    string
	BoxTeeBot    string
	BoxCross     string
}

// Unicode symbols (for terminals that support UTF-8)
var UnicodeSymbols = TerminalSymbols{
	Checkmark:   "✓",
	Cross:       "✗",
	Arrow:       "→",
	Bullet:      "•",
	Ellipsis:    "…",
	Spinner:     []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	BoxTopLeft:  "┌",
	BoxTopRight: "┐",
	BoxBotLeft:  "└",
	BoxBotRight: "┘",
	BoxHoriz:    "─",
	BoxVert:     "│",
	BoxTeeLeft:  "├",
	BoxTeeRight: "┤",
	BoxTeeTop:   "┬",
	BoxTeeBot:   "┴",
	BoxCross:    "┼",
}

// ASCII symbols (for terminals without UTF-8 support)
var ASCIISymbols = TerminalSymbols{
	Checkmark:   "+",
	Cross:       "x",
	Arrow:       "->",
	Bullet:      "*",
	Ellipsis:    "...",
	Spinner:     []string{"|", "/", "-", "\\"},
	BoxTopLeft:  "+",
	BoxTopRight: "+",
	BoxBotLeft:  "+",
	BoxBotRight: "+",
	BoxHoriz:    "-",
	BoxVert:     "|",
	BoxTeeLeft:  "+",
	BoxTeeRight: "+",
	BoxTeeTop:   "+",
	BoxTeeBot:   "+",
	BoxCross:    "+",
}

// GetTerminalSymbols returns the appropriate symbol set based on terminal capabilities
func GetTerminalSymbols() TerminalSymbols {
	if SupportsUnicode() {
		return UnicodeSymbols
	}
	return ASCIISymbols
}

// SupportsUnicode checks if the terminal supports Unicode
func SupportsUnicode() bool {
	lang := os.Getenv("LANG")
	lcAll := os.Getenv("LC_ALL")
	return strings.Contains(strings.ToUpper(lang), "UTF") ||
		strings.Contains(strings.ToUpper(lcAll), "UTF")
}
