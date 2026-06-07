// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for pure-output cmd functions.
// Covers PrintSearchCommandHelp, OutputSearchResultsAs*, PrintProbeCommandHelp,
// OutputProbeResultsAs*, RunShellCommand, PrintShellCommandHelp,
// OutputShellCompletionScript, OutputShellInitSnippet, and all shell-specific
// completion script generators. No network calls are made.
package cmd

import (
	"testing"

	"github.com/apimgr/vidveil/src/client/api"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func makeSearchResponse() *api.SearchResponse {
	return &api.SearchResponse{
		Ok:    true,
		Query: "test",
		Count: 2,
		Results: []api.SearchResult{
			{
				Title:    "Test Video One",
				URL:      "https://example.com/1",
				Duration: "10:00",
				Views:    "1000",
				Engine:   "ph",
			},
			{
				Title:       "Test Video Two With A Very Long Title That Goes Beyond Fifty Characters",
				URL:         "https://example.com/2",
				Duration:    "5:30",
				Views:       "500",
				Engine:      "xv",
				Description: "A test video",
				Tags:        []string{"test", "video"},
			},
		},
	}
}

func makeProbeResults() []EngineProbeResult {
	return []EngineProbeResult{
		{
			Name:           "ph",
			DisplayName:    "PornHub",
			Tier:           1,
			Available:      true,
			Enabled:        true,
			ResponseTimeMS: 200,
			ResultCount:    20,
		},
		{
			Name:           "xv",
			DisplayName:    "XVideos",
			Tier:           1,
			Available:      false,
			Enabled:        true,
			ResponseTimeMS: 0,
			Error:          "timeout",
		},
	}
}

// ── search.go output functions ───────────────────────────────────────────────

func TestPrintSearchCommandHelp_NoPanic(t *testing.T) {
	PrintSearchCommandHelp()
}

func TestOutputSearchResultsAsJSON_NoPanic(t *testing.T) {
	resp := makeSearchResponse()
	if err := OutputSearchResultsAsJSON(resp); err != nil {
		t.Errorf("OutputSearchResultsAsJSON: unexpected error: %v", err)
	}
}

func TestOutputSearchResultsAsPlain_WithResults_NoPanic(t *testing.T) {
	resp := makeSearchResponse()
	if err := OutputSearchResultsAsPlain(resp); err != nil {
		t.Errorf("OutputSearchResultsAsPlain: unexpected error: %v", err)
	}
}

func TestOutputSearchResultsAsPlain_WithDurationAndViews_NoPanic(t *testing.T) {
	resp := &api.SearchResponse{
		Ok:    true,
		Query: "test",
		Count: 1,
		Results: []api.SearchResult{
			{Title: "T", URL: "u", Duration: "3:00", Views: "100"},
		},
	}
	if err := OutputSearchResultsAsPlain(resp); err != nil {
		t.Errorf("OutputSearchResultsAsPlain: %v", err)
	}
}

func TestOutputSearchResultsAsTable_NoPanic(t *testing.T) {
	resp := makeSearchResponse()
	if err := OutputSearchResultsAsTable(resp); err != nil {
		t.Errorf("OutputSearchResultsAsTable: unexpected error: %v", err)
	}
}

func TestOutputSearchResultsAsTable_Empty_NoPanic(t *testing.T) {
	resp := &api.SearchResponse{Ok: true, Query: "none", Count: 0}
	if err := OutputSearchResultsAsTable(resp); err != nil {
		t.Errorf("OutputSearchResultsAsTable empty: %v", err)
	}
}

// ── probe.go output functions ─────────────────────────────────────────────────

func TestPrintProbeCommandHelp_NoPanic(t *testing.T) {
	PrintProbeCommandHelp()
}

func TestOutputProbeResultsAsJSON_NoPanic(t *testing.T) {
	results := makeProbeResults()
	if err := OutputProbeResultsAsJSON(results); err != nil {
		t.Errorf("OutputProbeResultsAsJSON: %v", err)
	}
}

func TestOutputProbeResultsAsYAML_NoPanic(t *testing.T) {
	results := makeProbeResults()
	if err := OutputProbeResultsAsYAML(results); err != nil {
		t.Errorf("OutputProbeResultsAsYAML: %v", err)
	}
}

func TestOutputProbeResultsAsTable_NoPanic(t *testing.T) {
	results := makeProbeResults()
	probeVerboseMode = false
	if err := OutputProbeResultsAsTable(results); err != nil {
		t.Errorf("OutputProbeResultsAsTable: %v", err)
	}
}

func TestOutputProbeResultsAsTable_Verbose_NoPanic(t *testing.T) {
	results := makeProbeResults()
	probeVerboseMode = true
	defer func() { probeVerboseMode = false }()
	if err := OutputProbeResultsAsTable(results); err != nil {
		t.Errorf("OutputProbeResultsAsTable verbose: %v", err)
	}
}

func TestOutputProbeResultsAsTable_WithCapabilities_NoPanic(t *testing.T) {
	results := []EngineProbeResult{
		{
			Name:           "ph",
			DisplayName:    "PornHub",
			Available:      true,
			ResultCount:    5,
			ResponseTimeMS: 100,
			Capabilities: map[string]interface{}{
				"has_preview":  true,
				"has_download": true,
			},
			FieldStats: map[string]int{
				ProbeFieldStatHasThumbnail: 5,
				ProbeFieldStatHasDuration:  5,
			},
		},
	}
	probeVerboseMode = true
	defer func() { probeVerboseMode = false }()
	if err := OutputProbeResultsAsTable(results); err != nil {
		t.Errorf("OutputProbeResultsAsTable with caps: %v", err)
	}
}

func TestOutputProbeResultsAsCSV_NoPanic(t *testing.T) {
	results := makeProbeResults()
	if err := OutputProbeResultsAsCSV(results); err != nil {
		t.Errorf("OutputProbeResultsAsCSV: %v", err)
	}
}

func TestOutputProbeResultsAsCSV_WithCapabilitiesAndStats_NoPanic(t *testing.T) {
	results := []EngineProbeResult{
		{
			Name:        "ph",
			DisplayName: "PornHub",
			Capabilities: map[string]interface{}{
				"has_preview": true,
			},
			FieldStats: map[string]int{
				ProbeFieldStatHasThumbnail: 3,
			},
		},
	}
	if err := OutputProbeResultsAsCSV(results); err != nil {
		t.Errorf("OutputProbeResultsAsCSV with data: %v", err)
	}
}

// ── shell.go functions ────────────────────────────────────────────────────────

func TestRunShellCommand_NoArgs_ReturnsError(t *testing.T) {
	err := RunShellCommand([]string{})
	if err == nil {
		t.Error("RunShellCommand no args: expected error, got nil")
	}
}

func TestRunShellCommand_Help_NoPanic(t *testing.T) {
	err := RunShellCommand([]string{"--help"})
	if err != nil {
		t.Errorf("RunShellCommand --help: %v", err)
	}
}

func TestRunShellCommand_ShortHelp_NoPanic(t *testing.T) {
	err := RunShellCommand([]string{"-h"})
	if err != nil {
		t.Errorf("RunShellCommand -h: %v", err)
	}
}

func TestRunShellCommand_Completions_Bash_NoPanic(t *testing.T) {
	err := RunShellCommand([]string{"completions", "bash"})
	if err != nil {
		t.Errorf("RunShellCommand completions bash: %v", err)
	}
}

func TestRunShellCommand_Completions_Zsh_NoPanic(t *testing.T) {
	err := RunShellCommand([]string{"completions", "zsh"})
	if err != nil {
		t.Errorf("RunShellCommand completions zsh: %v", err)
	}
}

func TestRunShellCommand_Init_Bash_NoPanic(t *testing.T) {
	err := RunShellCommand([]string{"init", "bash"})
	if err != nil {
		t.Errorf("RunShellCommand init bash: %v", err)
	}
}

func TestRunShellCommand_Unknown_ReturnsError(t *testing.T) {
	err := RunShellCommand([]string{"bogus"})
	if err == nil {
		t.Error("RunShellCommand bogus: expected error")
	}
}

func TestPrintShellCommandHelp_NoPanic(t *testing.T) {
	PrintShellCommandHelp()
}

func TestOutputShellCompletionScript_Bash_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("bash"); err != nil {
		t.Errorf("OutputShellCompletionScript bash: %v", err)
	}
}

func TestOutputShellCompletionScript_Zsh_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("zsh"); err != nil {
		t.Errorf("OutputShellCompletionScript zsh: %v", err)
	}
}

func TestOutputShellCompletionScript_Fish_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("fish"); err != nil {
		t.Errorf("OutputShellCompletionScript fish: %v", err)
	}
}

func TestOutputShellCompletionScript_Sh_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("sh"); err != nil {
		t.Errorf("OutputShellCompletionScript sh: %v", err)
	}
}

func TestOutputShellCompletionScript_Dash_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("dash"); err != nil {
		t.Errorf("OutputShellCompletionScript dash: %v", err)
	}
}

func TestOutputShellCompletionScript_Ksh_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("ksh"); err != nil {
		t.Errorf("OutputShellCompletionScript ksh: %v", err)
	}
}

func TestOutputShellCompletionScript_Powershell_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("powershell"); err != nil {
		t.Errorf("OutputShellCompletionScript powershell: %v", err)
	}
}

func TestOutputShellCompletionScript_Pwsh_NoPanic(t *testing.T) {
	if err := OutputShellCompletionScript("pwsh"); err != nil {
		t.Errorf("OutputShellCompletionScript pwsh: %v", err)
	}
}

func TestOutputShellCompletionScript_Unknown_ReturnsError(t *testing.T) {
	if err := OutputShellCompletionScript("csh"); err == nil {
		t.Error("OutputShellCompletionScript csh: expected error")
	}
}

func TestOutputShellCompletionScript_Empty_UsesDetected(t *testing.T) {
	// With empty string, it detects from env; result depends on $SHELL
	// Just ensure it doesn't panic — error is acceptable for unknown shells
	_ = OutputShellCompletionScript("")
}

func TestOutputShellInitSnippet_Bash_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("bash"); err != nil {
		t.Errorf("OutputShellInitSnippet bash: %v", err)
	}
}

func TestOutputShellInitSnippet_Zsh_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("zsh"); err != nil {
		t.Errorf("OutputShellInitSnippet zsh: %v", err)
	}
}

func TestOutputShellInitSnippet_Fish_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("fish"); err != nil {
		t.Errorf("OutputShellInitSnippet fish: %v", err)
	}
}

func TestOutputShellInitSnippet_Sh_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("sh"); err != nil {
		t.Errorf("OutputShellInitSnippet sh: %v", err)
	}
}

func TestOutputShellInitSnippet_Dash_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("dash"); err != nil {
		t.Errorf("OutputShellInitSnippet dash: %v", err)
	}
}

func TestOutputShellInitSnippet_Ksh_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("ksh"); err != nil {
		t.Errorf("OutputShellInitSnippet ksh: %v", err)
	}
}

func TestOutputShellInitSnippet_Powershell_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("powershell"); err != nil {
		t.Errorf("OutputShellInitSnippet powershell: %v", err)
	}
}

func TestOutputShellInitSnippet_Pwsh_NoPanic(t *testing.T) {
	if err := OutputShellInitSnippet("pwsh"); err != nil {
		t.Errorf("OutputShellInitSnippet pwsh: %v", err)
	}
}

func TestOutputShellInitSnippet_Unknown_ReturnsError(t *testing.T) {
	if err := OutputShellInitSnippet("csh"); err == nil {
		t.Error("OutputShellInitSnippet csh: expected error")
	}
}

func TestOutputZshCompletionScript_NoPanic(t *testing.T) {
	if err := OutputZshCompletionScript(); err != nil {
		t.Errorf("OutputZshCompletionScript: %v", err)
	}
}

func TestOutputFishCompletionScript_NoPanic(t *testing.T) {
	if err := OutputFishCompletionScript(); err != nil {
		t.Errorf("OutputFishCompletionScript: %v", err)
	}
}

func TestOutputPowershellCompletionScript_NoPanic(t *testing.T) {
	if err := OutputPowershellCompletionScript(); err != nil {
		t.Errorf("OutputPowershellCompletionScript: %v", err)
	}
}
