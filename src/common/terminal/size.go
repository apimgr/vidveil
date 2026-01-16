// SPDX-License-Identifier: MIT
// Package terminal provides terminal utilities
// See AI.md PART 7 for specification
package terminal

import (
	"os"

	"github.com/charmbracelet/x/term"
)

// SizeMode represents terminal size categories
type SizeMode int

const (
	SizeModeMicro     SizeMode = iota // <40 cols or <10 rows
	SizeModeMinimal                   // 40-59 cols or 10-15 rows
	SizeModeCompact                   // 60-79 cols or 16-23 rows
	SizeModeStandard                  // 80-119 cols and 24-39 rows
	SizeModeWide                      // 120-199 cols and 40-59 rows
	SizeModeUltrawide                 // 200-399 cols and 60-79 rows
	SizeModeMassive                   // 400+ cols and 80+ rows
)

// TerminalSize holds terminal dimensions and size mode
type TerminalSize struct {
	Cols int
	Rows int
	Mode SizeMode
}

// GetTerminalSize returns the current terminal size
func GetTerminalSize() TerminalSize {
	cols, rows, _ := term.GetSize(os.Stdout.Fd())
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	return TerminalSize{
		Cols: cols,
		Rows: rows,
		Mode: calculateMode(cols, rows),
	}
}

// calculateMode determines the size mode based on dimensions
func calculateMode(cols, rows int) SizeMode {
	switch {
	case cols < 40 || rows < 10:
		return SizeModeMicro
	case cols < 60 || rows < 16:
		return SizeModeMinimal
	case cols < 80 || rows < 24:
		return SizeModeCompact
	case cols < 120 || rows < 40:
		return SizeModeStandard
	case cols < 200 || rows < 60:
		return SizeModeWide
	case cols < 400 || rows < 80:
		return SizeModeUltrawide
	default:
		return SizeModeMassive
	}
}

// ShowASCIIArt returns true if size is large enough for ASCII art
func (s SizeMode) ShowASCIIArt() bool { return s >= SizeModeStandard }

// ShowBorders returns true if size is large enough for borders
func (s SizeMode) ShowBorders() bool { return s >= SizeModeCompact }

// ShowSidebar returns true if size is large enough for sidebar
func (s SizeMode) ShowSidebar() bool { return s >= SizeModeWide }

// ShowIcons returns true if size is large enough for icons
func (s SizeMode) ShowIcons() bool { return s >= SizeModeMinimal }

// String returns the string representation of the size mode
func (s SizeMode) String() string {
	return [...]string{
		"micro", "minimal", "compact", "standard", "wide", "ultrawide", "massive",
	}[s]
}
