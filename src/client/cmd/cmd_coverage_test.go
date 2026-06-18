// SPDX-License-Identifier: MIT
// Additional coverage tests for pure-logic functions in the cmd package.
// Targets: TruncateSearchResultText, compareCLIVersions, cliReleaseBinaryName,
// DetectCurrentShellType, OutputDataAsJSON/YAML, PrintCLIVersionInfo,
// IsServerConfigured, GetCLIHelpServerDefault, PrintConnectionWarning,
// CheckServerConnection (nil guard), and various output helpers.
package cmd

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// --- TruncateSearchResultText ---

func TestTruncateSearchResultText_BelowMax(t *testing.T) {
	got := TruncateSearchResultText("hello", 10)
	if got != "hello" {
		t.Errorf("TruncateSearchResultText short = %q, want %q", got, "hello")
	}
}

func TestTruncateSearchResultText_ExactMax(t *testing.T) {
	got := TruncateSearchResultText("hello", 5)
	if got != "hello" {
		t.Errorf("TruncateSearchResultText exact = %q, want %q", got, "hello")
	}
}

func TestTruncateSearchResultText_AboveMax(t *testing.T) {
	got := TruncateSearchResultText("hello world", 8)
	if got != "hello..." {
		t.Errorf("TruncateSearchResultText truncated = %q, want %q", got, "hello...")
	}
}

func TestTruncateSearchResultText_Empty(t *testing.T) {
	got := TruncateSearchResultText("", 10)
	if got != "" {
		t.Errorf("TruncateSearchResultText empty = %q, want empty", got)
	}
}

func TestTruncateSearchResultText_ExactlyThreeLonger(t *testing.T) {
	// 8-char input, max=5: trim to 2 chars + "..."
	got := TruncateSearchResultText("abcdefgh", 5)
	if got != "ab..." {
		t.Errorf("TruncateSearchResultText = %q, want %q", got, "ab...")
	}
}

// --- compareCLIVersions ---

func TestCompareCLIVersions_Equal(t *testing.T) {
	if compareCLIVersions("1.2.3", "1.2.3") != 0 {
		t.Error("compareCLIVersions equal versions should return 0")
	}
}

func TestCompareCLIVersions_AGreater(t *testing.T) {
	if compareCLIVersions("2.0.0", "1.9.9") != 1 {
		t.Error("compareCLIVersions(2.0.0, 1.9.9) should return 1")
	}
}

func TestCompareCLIVersions_BGreater(t *testing.T) {
	if compareCLIVersions("1.0.0", "1.0.1") != -1 {
		t.Error("compareCLIVersions(1.0.0, 1.0.1) should return -1")
	}
}

func TestCompareCLIVersions_MinorDifference(t *testing.T) {
	if compareCLIVersions("1.3.0", "1.2.9") != 1 {
		t.Error("compareCLIVersions(1.3.0, 1.2.9) should return 1")
	}
}

func TestCompareCLIVersions_PatchDifference(t *testing.T) {
	if compareCLIVersions("1.0.5", "1.0.10") != -1 {
		t.Error("compareCLIVersions(1.0.5, 1.0.10) should return -1")
	}
}

func TestCompareCLIVersions_EmptyInputs(t *testing.T) {
	if compareCLIVersions("", "") != 0 {
		t.Error("compareCLIVersions empty strings should return 0")
	}
}

func TestCompareCLIVersions_PartialVersion(t *testing.T) {
	if compareCLIVersions("2", "1.9.9") != 1 {
		t.Error("compareCLIVersions partial version: 2 > 1.9.9 expected")
	}
}

// --- cliReleaseBinaryName ---

func TestCLIReleaseBinaryName_ContainsGOOS(t *testing.T) {
	got := cliReleaseBinaryName()
	if !strings.Contains(got, runtime.GOOS) {
		t.Errorf("cliReleaseBinaryName() = %q, want GOOS %q", got, runtime.GOOS)
	}
}

func TestCLIReleaseBinaryName_ContainsGOARCH(t *testing.T) {
	got := cliReleaseBinaryName()
	if !strings.Contains(got, runtime.GOARCH) {
		t.Errorf("cliReleaseBinaryName() = %q, want GOARCH %q", got, runtime.GOARCH)
	}
}

func TestCLIReleaseBinaryName_NonEmpty(t *testing.T) {
	if cliReleaseBinaryName() == "" {
		t.Error("cliReleaseBinaryName() returned empty string")
	}
}

func TestCLIReleaseBinaryName_NoExeSuffixOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("exe suffix check only meaningful on Linux")
	}
	got := cliReleaseBinaryName()
	if strings.HasSuffix(got, ".exe") {
		t.Errorf("cliReleaseBinaryName() on Linux has .exe suffix: %q", got)
	}
}

// --- DetectCurrentShellType ---

func TestDetectCurrentShellType_BashWhenEmpty(t *testing.T) {
	t.Setenv("SHELL", "")
	got := DetectCurrentShellType()
	if got != "bash" {
		t.Errorf("DetectCurrentShellType() with SHELL='' = %q, want bash", got)
	}
}

func TestDetectCurrentShellType_ZshFromSHELL(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	got := DetectCurrentShellType()
	if got != "zsh" {
		t.Errorf("DetectCurrentShellType() with SHELL=/bin/zsh = %q, want zsh", got)
	}
}

func TestDetectCurrentShellType_FishFromSHELL(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")
	got := DetectCurrentShellType()
	if got != "fish" {
		t.Errorf("DetectCurrentShellType() with SHELL=/usr/bin/fish = %q, want fish", got)
	}
}

func TestDetectCurrentShellType_BashFromSHELL(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	got := DetectCurrentShellType()
	if got != "bash" {
		t.Errorf("DetectCurrentShellType() with SHELL=/bin/bash = %q, want bash", got)
	}
}

// --- OutputDataAsJSON ---

func TestOutputDataAsJSON_NoPanic(t *testing.T) {
	captureStdoutForTest(t, func() error {
		return OutputDataAsJSON(map[string]string{"key": "value"})
	})
}

func TestOutputDataAsJSON_ContainsKey(t *testing.T) {
	out := captureStdoutForTest(t, func() error {
		return OutputDataAsJSON(map[string]string{"hello": "world"})
	})
	if !strings.Contains(out, "hello") {
		t.Errorf("OutputDataAsJSON output = %q, want to contain 'hello'", out)
	}
}

// --- OutputDataAsYAML ---

func TestOutputDataAsYAML_NoPanic(t *testing.T) {
	captureStdoutForTest(t, func() error {
		return OutputDataAsYAML(map[string]string{"key": "value"})
	})
}

func TestOutputDataAsYAML_ContainsKey(t *testing.T) {
	out := captureStdoutForTest(t, func() error {
		return OutputDataAsYAML(map[string]string{"mykey": "myval"})
	})
	if !strings.Contains(out, "mykey") {
		t.Errorf("OutputDataAsYAML output = %q, want to contain 'mykey'", out)
	}
}

// --- PrintCLIVersionInfo ---

func TestPrintCLIVersionInfo_NoPanic(t *testing.T) {
	captureStdoutForTest(t, func() error {
		PrintCLIVersionInfo()
		return nil
	})
}

func TestPrintCLIVersionInfo_ContainsBinaryName(t *testing.T) {
	out := captureStdoutForTest(t, func() error {
		PrintCLIVersionInfo()
		return nil
	})
	if !strings.Contains(out, BinaryName) {
		t.Errorf("PrintCLIVersionInfo output = %q, want BinaryName %q", out, BinaryName)
	}
}

// --- IsServerConfigured ---

func TestIsServerConfigured_FalseWhenNeitherFlagNorEnv(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })
	cliConfigHasSavedServerAddress = false
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	if IsServerConfigured() {
		t.Error("IsServerConfigured() = true with no config and no env, want false")
	}
}

func TestIsServerConfigured_TrueWhenEnvSet(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })
	cliConfigHasSavedServerAddress = false
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "http://example.com")
	if !IsServerConfigured() {
		t.Error("IsServerConfigured() = false when VIDVEIL_SERVER_PRIMARY set, want true")
	}
}

func TestIsServerConfigured_TrueWhenFlagSet(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })
	cliConfigHasSavedServerAddress = true
	if !IsServerConfigured() {
		t.Error("IsServerConfigured() = false when cliConfigHasSavedServerAddress=true, want true")
	}
}

// --- GetCLIHelpServerDefault ---

func TestGetCLIHelpServerDefault_FallsBackToFromConfig(t *testing.T) {
	origConfig := cliConfig
	t.Cleanup(func() { cliConfig = origConfig })
	cliConfig = nil
	got := GetCLIHelpServerDefault()
	if got == "" {
		t.Error("GetCLIHelpServerDefault() returned empty string")
	}
}

// --- CheckServerConnection ---

func TestCheckServerConnection_NilClientReturnsFalse(t *testing.T) {
	origClient := apiClient
	t.Cleanup(func() { apiClient = origClient })
	apiClient = nil
	ok, err := CheckServerConnection()
	if ok {
		t.Error("CheckServerConnection() with nil client = true, want false")
	}
	if err != nil {
		t.Errorf("CheckServerConnection() with nil client error = %v, want nil", err)
	}
}

// --- PrintConnectionWarning ---

// PrintConnectionWarning dereferences cliConfig, so we must provide a non-nil
// config. We capture stderr output via the function's fmt.Fprintf(os.Stderr)
// by redirecting — but since captureStdoutForTest only redirects stdout, we
// just verify no panic occurs.
func TestPrintConnectionWarning_NoPanic(t *testing.T) {
	origConfig := cliConfig
	t.Cleanup(func() { cliConfig = origConfig })
	cliConfig = &CLIConfig{}
	cliConfig.Server.Address = "http://example.com"
	PrintConnectionWarning(fmt.Errorf("connection refused"))
}

func TestPrintConnectionWarning_NilError_NoPanic(t *testing.T) {
	origConfig := cliConfig
	t.Cleanup(func() { cliConfig = origConfig })
	cliConfig = &CLIConfig{}
	cliConfig.Server.Address = "http://example.com"
	PrintConnectionWarning(nil)
}

func TestPrintConnectionWarning_WithDebug_CoversDebugLine(t *testing.T) {
	origConfig := cliConfig
	origDebug := debugModeEnabled
	t.Cleanup(func() {
		cliConfig = origConfig
		debugModeEnabled = origDebug
	})
	cliConfig = &CLIConfig{}
	cliConfig.Server.Address = "http://example.com"
	debugModeEnabled = true
	// With debugModeEnabled=true and non-nil error → covers line 1053
	PrintConnectionWarning(fmt.Errorf("connection refused"))
}
