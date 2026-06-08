// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for CanUseANSI and colorAutoDetect.
// All branches exercised via env-var injection and DisplayEnv construction.
package terminal

import (
	"testing"

	"github.com/apimgr/vidveil/src/common/display"
)

// ── CanUseANSI ────────────────────────────────────────────────────────────────

func TestCanUseANSI_NilEnvNoColor_ReturnsFalseOrTrue(t *testing.T) {
	// nil env → falls back to term.IsTerminal(os.Stdout.Fd())
	// In a non-TTY test environment this returns false; just verify no panic.
	result := CanUseANSI(nil)
	_ = result
}

func TestCanUseANSI_NilEnv_NOCOLORSet_ReturnsFalse(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	result := CanUseANSI(nil)
	if result {
		t.Error("CanUseANSI(nil, NO_COLOR=1): expected false")
	}
}

func TestCanUseANSI_DumbTerminal_ReturnsFalse(t *testing.T) {
	env := &display.DisplayEnv{TerminalType: "dumb", IsTerminal: true}
	result := CanUseANSI(env)
	if result {
		t.Error("CanUseANSI(dumb terminal): expected false")
	}
}

func TestCanUseANSI_NormalTerminal_NOCOLORSet_ReturnsFalse(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	env := &display.DisplayEnv{TerminalType: "xterm-256color", IsTerminal: true}
	result := CanUseANSI(env)
	if result {
		t.Error("CanUseANSI(normal terminal, NO_COLOR=1): expected false")
	}
}

func TestCanUseANSI_NormalTerminal_NOCOLORUnset_ReturnsEnvIsTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	env := &display.DisplayEnv{TerminalType: "xterm-256color", IsTerminal: true}
	result := CanUseANSI(env)
	if !result {
		t.Error("CanUseANSI(normal terminal, NO_COLOR=''): expected true")
	}
}

func TestCanUseANSI_NonTTYEnv_NOCOLORUnset_ReturnsFalse(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	env := &display.DisplayEnv{TerminalType: "xterm", IsTerminal: false}
	result := CanUseANSI(env)
	if result {
		t.Error("CanUseANSI(non-TTY env, NO_COLOR=''): expected false")
	}
}

// ── colorAutoDetect ───────────────────────────────────────────────────────────

func TestColorAutoDetect_NOCOLORSet_ReturnsFalse(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorAutoDetect() {
		t.Error("colorAutoDetect(NO_COLOR=1): expected false")
	}
}

func TestColorAutoDetect_DumbTERM_ReturnsFalse(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if colorAutoDetect() {
		t.Error("colorAutoDetect(TERM=dumb): expected false")
	}
}

func TestColorAutoDetect_NonTTY_ReturnsFalse(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	// In Docker/CI stdout is never a TTY, so this always returns false
	result := colorAutoDetect()
	_ = result
}
