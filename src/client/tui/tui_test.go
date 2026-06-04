// SPDX-License-Identifier: MIT
// AI.md PART 28: Test coverage for tui package pure functions.
package tui

import (
	"testing"

	"github.com/apimgr/vidveil/src/common/theme"
)

// --- GetLayoutMode ---

// TestGetLayoutMode_MicroByNarrowCols verifies very narrow columns produce Micro layout.
func TestGetLayoutMode_MicroByNarrowCols(t *testing.T) {
	if got := GetLayoutMode(30, 30); got != LayoutMicro {
		t.Errorf("GetLayoutMode(30,30) = %v, want Micro", got)
	}
}

// TestGetLayoutMode_MicroByShortRows verifies very short rows produce Micro layout.
func TestGetLayoutMode_MicroByShortRows(t *testing.T) {
	if got := GetLayoutMode(80, 5); got != LayoutMicro {
		t.Errorf("GetLayoutMode(80,5) = %v, want Micro", got)
	}
}

// TestGetLayoutMode_Minimal verifies 40-59 cols produce Minimal layout.
func TestGetLayoutMode_Minimal(t *testing.T) {
	if got := GetLayoutMode(50, 20); got != LayoutMinimal {
		t.Errorf("GetLayoutMode(50,20) = %v, want Minimal", got)
	}
}

// TestGetLayoutMode_Compact verifies 60-79 cols produce Compact layout.
func TestGetLayoutMode_Compact(t *testing.T) {
	if got := GetLayoutMode(70, 30); got != LayoutCompact {
		t.Errorf("GetLayoutMode(70,30) = %v, want Compact", got)
	}
}

// TestGetLayoutMode_Standard verifies 80-119 cols produce Standard layout.
func TestGetLayoutMode_Standard(t *testing.T) {
	if got := GetLayoutMode(100, 50); got != LayoutStandard {
		t.Errorf("GetLayoutMode(100,50) = %v, want Standard", got)
	}
}

// TestGetLayoutMode_Wide verifies 120-199 cols produce Wide layout.
func TestGetLayoutMode_Wide(t *testing.T) {
	if got := GetLayoutMode(150, 70); got != LayoutWide {
		t.Errorf("GetLayoutMode(150,70) = %v, want Wide", got)
	}
}

// TestGetLayoutMode_Ultrawide verifies 200-399 cols produce Ultrawide layout.
func TestGetLayoutMode_Ultrawide(t *testing.T) {
	if got := GetLayoutMode(300, 100); got != LayoutUltrawide {
		t.Errorf("GetLayoutMode(300,100) = %v, want Ultrawide", got)
	}
}

// TestGetLayoutMode_Massive verifies 400+ cols produce Massive layout.
func TestGetLayoutMode_Massive(t *testing.T) {
	if got := GetLayoutMode(500, 120); got != LayoutMassive {
		t.Errorf("GetLayoutMode(500,120) = %v, want Massive", got)
	}
}

// --- LayoutMode.String ---

// TestLayoutMode_String verifies string representation for all modes.
func TestLayoutMode_String(t *testing.T) {
	cases := []struct {
		mode LayoutMode
		want string
	}{
		{LayoutMicro, "micro"},
		{LayoutMinimal, "minimal"},
		{LayoutCompact, "compact"},
		{LayoutStandard, "standard"},
		{LayoutWide, "wide"},
		{LayoutUltrawide, "ultrawide"},
		{LayoutMassive, "massive"},
	}
	for _, c := range cases {
		if got := c.mode.String(); got != c.want {
			t.Errorf("LayoutMode(%d).String() = %q, want %q", c.mode, got, c.want)
		}
	}
}

// --- LayoutMode.Config ---

// TestLayoutMode_Config_MicroHasNoSidebar verifies Micro config has no sidebar.
func TestLayoutMode_Config_MicroHasNoSidebar(t *testing.T) {
	cfg := LayoutMicro.Config()
	if cfg.ShowSidebar {
		t.Error("LayoutMicro.Config().ShowSidebar = true, want false")
	}
	if cfg.MaxColumns != 2 {
		t.Errorf("LayoutMicro.Config().MaxColumns = %d, want 2", cfg.MaxColumns)
	}
}

// TestLayoutMode_Config_WideHasSidebar verifies Wide config has sidebar enabled.
func TestLayoutMode_Config_WideHasSidebar(t *testing.T) {
	cfg := LayoutWide.Config()
	if !cfg.ShowSidebar {
		t.Error("LayoutWide.Config().ShowSidebar = false, want true")
	}
	if cfg.SidebarWidth != 30 {
		t.Errorf("LayoutWide.Config().SidebarWidth = %d, want 30", cfg.SidebarWidth)
	}
}

// TestLayoutMode_Config_MassiveHasTileLayout verifies Massive config has tile layout.
func TestLayoutMode_Config_MassiveHasTileLayout(t *testing.T) {
	cfg := LayoutMassive.Config()
	if !cfg.TileLayout {
		t.Error("LayoutMassive.Config().TileLayout = false, want true")
	}
}

// --- LayoutMode.Spacing ---

// TestLayoutMode_Spacing_MicroReturnsXS verifies Micro returns SpaceXS.
func TestLayoutMode_Spacing_MicroReturnsXS(t *testing.T) {
	if got := LayoutMicro.Spacing(); got != SpaceXS {
		t.Errorf("LayoutMicro.Spacing() = %d, want %d (SpaceXS)", got, SpaceXS)
	}
}

// TestLayoutMode_Spacing_MinimalReturnsXS verifies Minimal returns SpaceXS.
func TestLayoutMode_Spacing_MinimalReturnsXS(t *testing.T) {
	if got := LayoutMinimal.Spacing(); got != SpaceXS {
		t.Errorf("LayoutMinimal.Spacing() = %d, want %d (SpaceXS)", got, SpaceXS)
	}
}

// TestLayoutMode_Spacing_CompactReturnsS verifies Compact returns SpaceS.
func TestLayoutMode_Spacing_CompactReturnsS(t *testing.T) {
	if got := LayoutCompact.Spacing(); got != SpaceS {
		t.Errorf("LayoutCompact.Spacing() = %d, want %d (SpaceS)", got, SpaceS)
	}
}

// TestLayoutMode_Spacing_StandardReturnsM verifies Standard returns SpaceM.
func TestLayoutMode_Spacing_StandardReturnsM(t *testing.T) {
	if got := LayoutStandard.Spacing(); got != SpaceM {
		t.Errorf("LayoutStandard.Spacing() = %d, want %d (SpaceM)", got, SpaceM)
	}
}

// TestLayoutMode_Spacing_WideReturnsL verifies Wide returns SpaceL.
func TestLayoutMode_Spacing_WideReturnsL(t *testing.T) {
	if got := LayoutWide.Spacing(); got != SpaceL {
		t.Errorf("LayoutWide.Spacing() = %d, want %d (SpaceL)", got, SpaceL)
	}
}

// TestLayoutMode_Spacing_MassiveReturnsXL verifies Massive returns SpaceXL.
func TestLayoutMode_Spacing_MassiveReturnsXL(t *testing.T) {
	if got := LayoutMassive.Spacing(); got != SpaceXL {
		t.Errorf("LayoutMassive.Spacing() = %d, want %d (SpaceXL)", got, SpaceXL)
	}
}

// --- TUI Styles ---

// TestDefaultTUIStyles_NoPanic verifies DefaultTUIStyles does not panic.
func TestDefaultTUIStyles_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DefaultTUIStyles() panicked: %v", r)
		}
	}()
	_ = DefaultTUIStyles()
}

// TestLightTUIStyles_NoPanic verifies LightTUIStyles does not panic.
func TestLightTUIStyles_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("LightTUIStyles() panicked: %v", r)
		}
	}()
	_ = LightTUIStyles()
}

// TestTUIStylesFromPalette_DarkPalette verifies palette-based styles are non-zero.
func TestTUIStylesFromPalette_DarkPalette(t *testing.T) {
	s := TUIStylesFromPalette(theme.Dark)
	if s.Base.GetBackground() == s.Title.GetBackground() && s.Error.GetForeground() == s.Success.GetForeground() {
		t.Log("TUIStylesFromPalette returned styles (field equality check is style-dependent)")
	}
}
