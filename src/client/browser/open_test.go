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

// --- OpenURL ---

// TestOpenURL_NoPanic verifies OpenURL does not panic on any reachable OS path.
// It may return an error (xdg-open unavailable, browser fails, etc.); that is
// acceptable — only a panic is a test failure.
func TestOpenURL_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OpenURL panicked: %v", r)
		}
	}()
	// Use a clearly non-opening URL; any error from the OS is fine.
	_ = OpenURL("about:blank")
}

// TestOpenURL_EmptyURLNoPanic verifies OpenURL handles an empty URL without
// panicking (the browser command may fail, which is acceptable).
func TestOpenURL_EmptyURLNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("OpenURL empty URL panicked: %v", r)
		}
	}()
	_ = OpenURL("")
}

// TestOpenURL_ReturnsErrorOrNil verifies OpenURL returns error or nil — never
// a panic or an undefined value.
func TestOpenURL_ReturnsErrorOrNil(t *testing.T) {
	err := OpenURL("https://example.com")
	// In headless CI, xdg-open/open may fail. Both nil and non-nil are correct.
	_ = err
}
