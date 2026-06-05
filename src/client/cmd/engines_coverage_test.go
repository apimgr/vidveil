// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for engines and bangs output functions.
// Tests OutputEnginesAsJSON, OutputEnginesAsYAML, OutputEnginesAsPlain,
// OutputEnginesAsTable, PrintEnginesCommandHelp, OutputBangsAsJSON,
// OutputBangsAsYAML, OutputBangsAsCSV, OutputBangsAsPlain, OutputBangsAsTable,
// PrintBangsCommandHelp.
package cmd

import (
	"testing"
)

// ── sample data helpers ────────────────────────────────────────────────────────

func sampleEngineInfos() []EngineInfo {
	return []EngineInfo{
		{
			Name:        "pornhub",
			DisplayName: "PornHub",
			Bang:        "ph",
			Tier:        1,
			Enabled:     true,
			Method:      "GET",
			HasPreview:  true,
			HasDownload: false,
		},
		{
			Name:        "xvideos",
			DisplayName: "XVideos",
			Bang:        "xv",
			Tier:        2,
			Enabled:     false,
			Method:      "GET",
			HasPreview:  false,
			HasDownload: true,
		},
	}
}

func sampleBangInfos() []BangInfo {
	return []BangInfo{
		{Bang: "ph", EngineName: "pornhub", DisplayName: "PornHub"},
		{Bang: "xv", EngineName: "xvideos", DisplayName: "XVideos"},
	}
}

// ── OutputEnginesAsJSON ────────────────────────────────────────────────────────

func TestOutputEnginesAsJSON_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputEnginesAsJSON panicked: %v", r)
		}
	}()
	if err := OutputEnginesAsJSON(sampleEngineInfos()); err != nil {
		t.Errorf("OutputEnginesAsJSON: unexpected error: %v", err)
	}
}

func TestOutputEnginesAsJSON_Empty(t *testing.T) {
	if err := OutputEnginesAsJSON([]EngineInfo{}); err != nil {
		t.Errorf("OutputEnginesAsJSON empty: unexpected error: %v", err)
	}
}

// ── OutputEnginesAsYAML ────────────────────────────────────────────────────────

func TestOutputEnginesAsYAML_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputEnginesAsYAML panicked: %v", r)
		}
	}()
	if err := OutputEnginesAsYAML(sampleEngineInfos()); err != nil {
		t.Errorf("OutputEnginesAsYAML: unexpected error: %v", err)
	}
}

// ── OutputEnginesAsPlain ───────────────────────────────────────────────────────

func TestOutputEnginesAsPlain_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputEnginesAsPlain panicked: %v", r)
		}
	}()
	if err := OutputEnginesAsPlain(sampleEngineInfos()); err != nil {
		t.Errorf("OutputEnginesAsPlain: unexpected error: %v", err)
	}
}

func TestOutputEnginesAsPlain_Empty(t *testing.T) {
	if err := OutputEnginesAsPlain([]EngineInfo{}); err != nil {
		t.Errorf("OutputEnginesAsPlain empty: unexpected error: %v", err)
	}
}

// ── OutputEnginesAsTable ───────────────────────────────────────────────────────

func TestOutputEnginesAsTable_NoDetails(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputEnginesAsTable(showDetails=false) panicked: %v", r)
		}
	}()
	if err := OutputEnginesAsTable(sampleEngineInfos(), false); err != nil {
		t.Errorf("OutputEnginesAsTable no-details: unexpected error: %v", err)
	}
}

func TestOutputEnginesAsTable_WithDetails(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputEnginesAsTable(showDetails=true) panicked: %v", r)
		}
	}()
	if err := OutputEnginesAsTable(sampleEngineInfos(), true); err != nil {
		t.Errorf("OutputEnginesAsTable with-details: unexpected error: %v", err)
	}
}

func TestOutputEnginesAsTable_Empty(t *testing.T) {
	if err := OutputEnginesAsTable([]EngineInfo{}, false); err != nil {
		t.Errorf("OutputEnginesAsTable empty: unexpected error: %v", err)
	}
}

// ── PrintEnginesCommandHelp ────────────────────────────────────────────────────

func TestPrintEnginesCommandHelp_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintEnginesCommandHelp panicked: %v", r)
		}
	}()
	PrintEnginesCommandHelp()
}

// ── OutputBangsAsJSON ─────────────────────────────────────────────────────────

func TestOutputBangsAsJSON_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputBangsAsJSON panicked: %v", r)
		}
	}()
	if err := OutputBangsAsJSON(sampleBangInfos()); err != nil {
		t.Errorf("OutputBangsAsJSON: unexpected error: %v", err)
	}
}

func TestOutputBangsAsJSON_Empty(t *testing.T) {
	if err := OutputBangsAsJSON([]BangInfo{}); err != nil {
		t.Errorf("OutputBangsAsJSON empty: unexpected error: %v", err)
	}
}

// ── OutputBangsAsYAML ─────────────────────────────────────────────────────────

func TestOutputBangsAsYAML_NoPanic(t *testing.T) {
	if err := OutputBangsAsYAML(sampleBangInfos()); err != nil {
		t.Errorf("OutputBangsAsYAML: unexpected error: %v", err)
	}
}

// ── OutputBangsAsCSV ──────────────────────────────────────────────────────────

func TestOutputBangsAsCSV_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputBangsAsCSV panicked: %v", r)
		}
	}()
	if err := OutputBangsAsCSV(sampleBangInfos()); err != nil {
		t.Errorf("OutputBangsAsCSV: unexpected error: %v", err)
	}
}

func TestOutputBangsAsCSV_Empty(t *testing.T) {
	if err := OutputBangsAsCSV([]BangInfo{}); err != nil {
		t.Errorf("OutputBangsAsCSV empty: unexpected error: %v", err)
	}
}

// ── OutputBangsAsPlain ────────────────────────────────────────────────────────

func TestOutputBangsAsPlain_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputBangsAsPlain panicked: %v", r)
		}
	}()
	if err := OutputBangsAsPlain(sampleBangInfos()); err != nil {
		t.Errorf("OutputBangsAsPlain: unexpected error: %v", err)
	}
}

func TestOutputBangsAsPlain_Empty(t *testing.T) {
	if err := OutputBangsAsPlain([]BangInfo{}); err != nil {
		t.Errorf("OutputBangsAsPlain empty: unexpected error: %v", err)
	}
}

// ── OutputBangsAsTable ────────────────────────────────────────────────────────

func TestOutputBangsAsTable_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OutputBangsAsTable panicked: %v", r)
		}
	}()
	if err := OutputBangsAsTable(sampleBangInfos()); err != nil {
		t.Errorf("OutputBangsAsTable: unexpected error: %v", err)
	}
}

func TestOutputBangsAsTable_Empty(t *testing.T) {
	if err := OutputBangsAsTable([]BangInfo{}); err != nil {
		t.Errorf("OutputBangsAsTable empty: unexpected error: %v", err)
	}
}

// ── PrintBangsCommandHelp ─────────────────────────────────────────────────────

func TestPrintBangsCommandHelp_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintBangsCommandHelp panicked: %v", r)
		}
	}()
	PrintBangsCommandHelp()
}
