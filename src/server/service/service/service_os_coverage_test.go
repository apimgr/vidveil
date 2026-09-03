// SPDX-License-Identifier: MIT
// AI.md PART 28: Direct coverage for OS-specific dispatch functions.
// darwin*, windows*, and bsd* methods have no build constraints, so they can
// be called directly from tests running on Linux to exercise their code paths
// (all will fail with "command not found" or "permission denied", which is expected).
package service

import (
	"testing"
)

// newOSTestManager returns a SystemServiceManager for OS-function testing.
func newOSTestManager(t *testing.T) *SystemServiceManager {
	t.Helper()
	m, err := NewSystemServiceManager("vidveil-os-test", "VidVeil OS Test", "OS-specific coverage")
	if err != nil {
		t.Fatalf("NewSystemServiceManager: %v", err)
	}
	return m
}

// ── launchdLabel / launchdPlistPath ──────────────────────────────────────────

func TestLaunchdLabel_ReturnsExpectedFormat(t *testing.T) {
	m := newOSTestManager(t)
	got := m.launchdLabel()
	if got == "" {
		t.Error("launchdLabel: returned empty string")
	}
	if got != "io.github.apimgr.vidveil-os-test" {
		t.Errorf("launchdLabel = %q, want io.github.apimgr.vidveil-os-test", got)
	}
}

func TestLaunchdPlistPath_ReturnsNonEmpty(t *testing.T) {
	m := newOSTestManager(t)
	got := m.launchdPlistPath()
	if got == "" {
		t.Error("launchdPlistPath: returned empty string")
	}
}

// ── Darwin methods — fail with command not found on Linux ────────────────────

func TestDarwinStatus_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	status, _ := m.darwinStatus()
	if status == "" {
		t.Error("darwinStatus: returned empty status")
	}
}

func TestDarwinStart_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	// Returns error on non-darwin (launchctl not found) — ignore error.
	_ = m.darwinStart()
}

func TestDarwinStop_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.darwinStop()
}

func TestDarwinRestart_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.darwinRestart()
}

func TestDarwinReload_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.darwinReload()
}

func TestDarwinInstall_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	// Will fail at os.WriteFile (no /Library/LaunchDaemons on Linux) — ignore error.
	_ = m.darwinInstall()
}

func TestDarwinUninstall_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	// launchctl unload fails (command not found); os.Remove returns IsNotExist — ignore.
	_ = m.darwinUninstall()
}

func TestDarwinDisable_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.darwinDisable()
}

// ── Windows methods — fail with command not found on Linux ───────────────────

func TestWindowsStatus_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	status, _ := m.windowsStatus()
	if status == "" {
		t.Error("windowsStatus: returned empty status")
	}
}

func TestWindowsStart_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.windowsStart()
}

func TestWindowsStop_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.windowsStop()
}

func TestWindowsRestart_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.windowsRestart()
}

func TestWindowsInstall_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.windowsInstall()
}

func TestWindowsUninstall_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.windowsUninstall()
}

func TestWindowsDisable_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.windowsDisable()
}

// ── BSD methods — fail with command not found on Linux ───────────────────────

func TestBSDStatus_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	status, _ := m.bsdStatus()
	if status == "" {
		t.Error("bsdStatus: returned empty status")
	}
}

func TestBSDStart_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.bsdStart()
}

func TestBSDStop_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.bsdStop()
}

func TestBSDRestart_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.bsdRestart()
}

func TestBSDReload_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	_ = m.bsdReload()
}

func TestBSDInstall_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	// Fails at os.WriteFile (/usr/local/etc/rc.d/ may not be writable) — ignore.
	_ = m.bsdInstall()
}

func TestBSDUninstall_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	// bsdStop fails (service not found); os.Remove returns IsNotExist — ignore.
	_ = m.bsdUninstall()
}

func TestBSDDisable_NoPanic(t *testing.T) {
	m := newOSTestManager(t)
	// Reads /etc/rc.conf.local which doesn't exist on Linux — returns error, ignore.
	_ = m.bsdDisable()
}
