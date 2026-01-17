// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - TUI Responsive Layout
package tui

// LayoutMode represents the terminal size category
type LayoutMode int

const (
	LayoutMicro     LayoutMode = iota // <40 cols
	LayoutMinimal                     // 40-59 cols
	LayoutCompact                     // 60-79 cols
	LayoutStandard                    // 80-119 cols
	LayoutWide                        // 120-199 cols
	LayoutUltrawide                   // 200-399 cols
	LayoutMassive                     // 400+ cols
)

// String returns the string representation of the layout mode
func (m LayoutMode) String() string {
	return [...]string{
		"micro", "minimal", "compact", "standard",
		"wide", "ultrawide", "massive",
	}[m]
}

// GetLayoutMode determines layout mode from terminal dimensions
func GetLayoutMode(cols, rows int) LayoutMode {
	// Use the more constraining dimension
	switch {
	case cols < 40 || rows < 10:
		return LayoutMicro
	case cols < 60 || rows < 16:
		return LayoutMinimal
	case cols < 80 || rows < 24:
		return LayoutCompact
	case cols < 120 || rows < 40:
		return LayoutStandard
	case cols < 200 || rows < 60:
		return LayoutWide
	case cols < 400 || rows < 80:
		return LayoutUltrawide
	default:
		return LayoutMassive
	}
}

// LayoutConfig holds layout configuration for a mode
type LayoutConfig struct {
	ShowBorders    bool
	ShowHeader     bool
	ShowFooter     bool
	ShowSidebar    bool
	SidebarWidth   int
	MaxColumns     int
	TruncateAt     int
	UseAbbrev      bool
	VerticalScroll bool
	MultiPane      bool
	TileLayout     bool
}

// Config returns the layout configuration for a mode
func (m LayoutMode) Config() LayoutConfig {
	configs := map[LayoutMode]LayoutConfig{
		LayoutMicro: {
			ShowBorders:    false,
			ShowHeader:     false,
			ShowFooter:     false,
			ShowSidebar:    false,
			MaxColumns:     2,
			TruncateAt:     30,
			UseAbbrev:      true,
			VerticalScroll: true,
		},
		LayoutMinimal: {
			ShowBorders:    false,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    false,
			MaxColumns:     3,
			TruncateAt:     40,
			UseAbbrev:      true,
			VerticalScroll: true,
		},
		LayoutCompact: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    false,
			MaxColumns:     4,
			TruncateAt:     60,
			UseAbbrev:      false,
			VerticalScroll: true,
		},
		LayoutStandard: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    false,
			MaxColumns:     6,
			TruncateAt:     80,
			UseAbbrev:      false,
			VerticalScroll: true,
		},
		LayoutWide: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    true,
			SidebarWidth:   30,
			MaxColumns:     8,
			TruncateAt:     120,
			UseAbbrev:      false,
			VerticalScroll: true,
		},
		LayoutUltrawide: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    true,
			SidebarWidth:   40,
			MaxColumns:     12,
			TruncateAt:     200,
			UseAbbrev:      false,
			VerticalScroll: false,
			MultiPane:      true,
		},
		LayoutMassive: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    true,
			SidebarWidth:   50,
			MaxColumns:     20,
			TruncateAt:     0, // No truncation
			UseAbbrev:      false,
			VerticalScroll: false,
			MultiPane:      true,
			TileLayout:     true,
		},
	}
	return configs[m]
}

// Spacing constants per AI.md PART 33
const (
	SpaceXS = 1 // Micro spacing
	SpaceS  = 2 // Small spacing
	SpaceM  = 4 // Medium spacing
	SpaceL  = 6 // Large spacing
	SpaceXL = 8 // Extra large spacing
)

// Spacing returns appropriate spacing for layout mode
func (m LayoutMode) Spacing() int {
	switch m {
	case LayoutMicro, LayoutMinimal:
		return SpaceXS
	case LayoutCompact:
		return SpaceS
	case LayoutStandard:
		return SpaceM
	case LayoutWide:
		return SpaceL
	default:
		return SpaceXL
	}
}
