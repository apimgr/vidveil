// SPDX-License-Identifier: MIT
// Coverage tests for pure helper functions in main.go.
// Functions that call os.Exit are NOT tested here; only pure
// or stdout-writing functions are exercised.
package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// captureStdout redirects os.Stdout to a buffer for the duration of f,
// returning the captured output.
func captureStdout(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	orig := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

// ── printBashCompletions ──────────────────────────────────────────────────────

func TestPrintBashCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printBashCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printBashCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintBashCompletions_ContainsCompleteCommand(t *testing.T) {
	out := captureStdout(func() { printBashCompletions("vidveil") })
	if !strings.Contains(out, "complete") {
		t.Error("printBashCompletions: output does not contain 'complete' directive")
	}
}

func TestPrintBashCompletions_ContainsHelpFlag(t *testing.T) {
	out := captureStdout(func() { printBashCompletions("vidveil") })
	if !strings.Contains(out, "--help") {
		t.Error("printBashCompletions: output does not contain '--help' flag")
	}
}

// ── printZshCompletions ───────────────────────────────────────────────────────

func TestPrintZshCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printZshCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printZshCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintZshCompletions_ContainsCompdef(t *testing.T) {
	out := captureStdout(func() { printZshCompletions("vidveil") })
	if !strings.Contains(out, "#compdef") {
		t.Error("printZshCompletions: output does not contain '#compdef'")
	}
}

func TestPrintZshCompletions_ContainsArguments(t *testing.T) {
	out := captureStdout(func() { printZshCompletions("vidveil") })
	if !strings.Contains(out, "_arguments") {
		t.Error("printZshCompletions: output does not contain '_arguments'")
	}
}

// ── printFishCompletions ──────────────────────────────────────────────────────

func TestPrintFishCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printFishCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printFishCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintFishCompletions_ContainsCompleteDirective(t *testing.T) {
	out := captureStdout(func() { printFishCompletions("vidveil") })
	if !strings.Contains(out, "complete -c vidveil") {
		t.Error("printFishCompletions: output does not start with 'complete -c vidveil'")
	}
}

func TestPrintFishCompletions_ContainsVersionFlag(t *testing.T) {
	out := captureStdout(func() { printFishCompletions("vidveil") })
	if !strings.Contains(out, "version") {
		t.Error("printFishCompletions: output does not contain 'version' flag")
	}
}

// ── printPowerShellCompletions ────────────────────────────────────────────────

func TestPrintPowerShellCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printPowerShellCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printPowerShellCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintPowerShellCompletions_ContainsRegisterArgCompleter(t *testing.T) {
	out := captureStdout(func() { printPowerShellCompletions("vidveil") })
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Error("printPowerShellCompletions: output does not contain 'Register-ArgumentCompleter'")
	}
}

// ── printCompletions ──────────────────────────────────────────────────────────

func TestPrintCompletions_Bash(t *testing.T) {
	out := captureStdout(func() { printCompletions("bash", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions bash: output does not contain bash-specific 'compgen'")
	}
}

func TestPrintCompletions_Zsh(t *testing.T) {
	out := captureStdout(func() { printCompletions("zsh", "vidveil") })
	if !strings.Contains(out, "#compdef") {
		t.Error("printCompletions zsh: output does not contain '#compdef'")
	}
}

func TestPrintCompletions_Fish(t *testing.T) {
	out := captureStdout(func() { printCompletions("fish", "vidveil") })
	if !strings.Contains(out, "complete -c") {
		t.Error("printCompletions fish: output does not contain fish 'complete -c'")
	}
}

func TestPrintCompletions_Powershell(t *testing.T) {
	out := captureStdout(func() { printCompletions("powershell", "vidveil") })
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Error("printCompletions powershell: output does not contain 'Register-ArgumentCompleter'")
	}
}

func TestPrintCompletions_Pwsh(t *testing.T) {
	out := captureStdout(func() { printCompletions("pwsh", "vidveil") })
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Error("printCompletions pwsh: output does not contain 'Register-ArgumentCompleter'")
	}
}

func TestPrintCompletions_Sh(t *testing.T) {
	out := captureStdout(func() { printCompletions("sh", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions sh: output should use bash completions (contains 'compgen')")
	}
}

func TestPrintCompletions_Dash(t *testing.T) {
	out := captureStdout(func() { printCompletions("dash", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions dash: output should use bash completions (contains 'compgen')")
	}
}

func TestPrintCompletions_Ksh(t *testing.T) {
	out := captureStdout(func() { printCompletions("ksh", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions ksh: output should use bash completions (contains 'compgen')")
	}
}

// ── printInit ─────────────────────────────────────────────────────────────────

func TestPrintInit_Bash(t *testing.T) {
	out := captureStdout(func() { printInit("bash", "vidveil") })
	if !strings.Contains(out, "source") {
		t.Errorf("printInit bash: expected 'source' in output, got %q", out)
	}
}

func TestPrintInit_Zsh(t *testing.T) {
	out := captureStdout(func() { printInit("zsh", "vidveil") })
	if !strings.Contains(out, "source") {
		t.Errorf("printInit zsh: expected 'source' in output, got %q", out)
	}
}

func TestPrintInit_Fish(t *testing.T) {
	out := captureStdout(func() { printInit("fish", "vidveil") })
	if !strings.Contains(out, "source") {
		t.Errorf("printInit fish: expected 'source' in output, got %q", out)
	}
}

func TestPrintInit_Sh(t *testing.T) {
	out := captureStdout(func() { printInit("sh", "vidveil") })
	if !strings.Contains(out, "eval") {
		t.Errorf("printInit sh: expected 'eval' in output, got %q", out)
	}
}

func TestPrintInit_Dash(t *testing.T) {
	out := captureStdout(func() { printInit("dash", "vidveil") })
	if !strings.Contains(out, "eval") {
		t.Errorf("printInit dash: expected 'eval' in output, got %q", out)
	}
}

func TestPrintInit_Ksh(t *testing.T) {
	out := captureStdout(func() { printInit("ksh", "vidveil") })
	if !strings.Contains(out, "eval") {
		t.Errorf("printInit ksh: expected 'eval' in output, got %q", out)
	}
}

func TestPrintInit_Powershell(t *testing.T) {
	out := captureStdout(func() { printInit("powershell", "vidveil") })
	if !strings.Contains(out, "Invoke-Expression") {
		t.Errorf("printInit powershell: expected 'Invoke-Expression' in output, got %q", out)
	}
}

func TestPrintInit_Pwsh(t *testing.T) {
	out := captureStdout(func() { printInit("pwsh", "vidveil") })
	if !strings.Contains(out, "Invoke-Expression") {
		t.Errorf("printInit pwsh: expected 'Invoke-Expression' in output, got %q", out)
	}
}

// ── getDisplayAddress ─────────────────────────────────────────────────────────

func TestGetDisplayAddress_ContainsPort(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Port = "8080"
	got := getDisplayAddress(cfg)
	if !strings.Contains(got, "8080") {
		t.Errorf("getDisplayAddress: %q does not contain port '8080'", got)
	}
}

func TestGetDisplayAddress_NonEmpty(t *testing.T) {
	cfg := config.DefaultAppConfig()
	got := getDisplayAddress(cfg)
	if got == "" {
		t.Error("getDisplayAddress: returned empty string")
	}
}
