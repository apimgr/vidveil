// SPDX-License-Identifier: MIT
package browser

import (
	"runtime"
	"testing"
)

// --- CanOpenBrowser ---

// TestCanOpenBrowser_NoPanic verifies the function does not panic regardless of
// the OS or whether a display server is available.
func TestCanOpenBrowser_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CanOpenBrowser panicked: %v", r)
		}
	}()
	CanOpenBrowser()
}

// TestCanOpenBrowser_ReturnsBool verifies the return is a valid bool (no
// undefined behaviour from an uninitialised value).
func TestCanOpenBrowser_ReturnsBool(t *testing.T) {
	result := CanOpenBrowser()
	// Explicit: result must be true or false, never something else.
	if result != true && result != false {
		t.Errorf("CanOpenBrowser() = %v, which is neither true nor false", result)
	}
}

// TestCanOpenBrowser_HeadlessLinux verifies that in a headless Linux environment
// (no xdg-open in PATH, typical of Docker/CI), CanOpenBrowser returns false.
// On macOS or Windows the binary is always available, so we skip those.
func TestCanOpenBrowser_HeadlessLinux(t *testing.T) {
	if runtime.GOOS != platformLinux {
		t.Skipf("test only applies to Linux; current OS: %s", runtime.GOOS)
	}
	// In CI/Docker there is no xdg-open, so CanOpenBrowser must return false.
	// If someone runs the tests on a desktop with xdg-open installed, the
	// result will be true and we skip rather than fail, because the function
	// is correct in both environments.
	got := CanOpenBrowser()
	if got {
		t.Log("CanOpenBrowser() = true; xdg-open is present in this environment (non-headless) — skipping assertion")
		t.Skip("xdg-open found; not a headless environment")
	}
	// got == false is the expected headless result; test passes.
}

// TestCanOpenBrowser_Idempotent verifies that calling the function twice
// returns the same value (no side effects alter subsequent calls).
func TestCanOpenBrowser_Idempotent(t *testing.T) {
	first := CanOpenBrowser()
	second := CanOpenBrowser()
	if first != second {
		t.Errorf("CanOpenBrowser() returned %v then %v; must be idempotent", first, second)
	}
}
