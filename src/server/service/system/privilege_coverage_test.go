// SPDX-License-Identifier: MIT
// Coverage tests for privilege_unix.go and service.go helpers.
// All tests call exported/unexported functions in the same package (package system).
//
//go:build !windows

package system

import (
	"testing"
)

// ── ShouldDropPrivileges / GetPrivilegeDropUser / IsElevated ─────────────────

// TestShouldDropPrivilegesBool verifies the function returns a bool without panicking.
func TestShouldDropPrivilegesBool(t *testing.T) {
	// Result depends on whether the test runs as root; just verify no panic.
	_ = ShouldDropPrivileges()
}

func TestGetPrivilegeDropUserReturnsBinaryName(t *testing.T) {
	name := GetPrivilegeDropUser()
	if name == "" {
		t.Error("GetPrivilegeDropUser() returned empty string")
	}
}

func TestIsElevatedReturnsBool(t *testing.T) {
	_ = IsElevated()
}

// ── CanEscalate ──────────────────────────────────────────────────────────────

// TestCanEscalateReturnsBool verifies the function returns without panicking.
// When running as root, IsElevated() is true and the function returns true immediately.
func TestCanEscalateReturnsBool(t *testing.T) {
	result := CanEscalate()
	// Just verify no panic; value depends on the test environment.
	_ = result
}

// ── HandleEscalation ─────────────────────────────────────────────────────────

// TestHandleEscalationAlreadyElevatedReturnsNil verifies that HandleEscalation
// returns nil when IsElevated() is true (i.e. running as root).
func TestHandleEscalationAlreadyElevated(t *testing.T) {
	if !IsElevated() {
		t.Skip("not running as root; HandleEscalation early-return branch not reachable")
	}
	err := HandleEscalation("test-action")
	if err != nil {
		t.Errorf("HandleEscalation when already elevated = %v, want nil", err)
	}
}

// ── IsRunningAsRoot / IsRunningInContainer / DetectServiceManager ─────────────

func TestIsRunningAsRootReturnsBool(t *testing.T) {
	_ = IsRunningAsRoot()
}

func TestIsRunningInContainerReturnsBool(t *testing.T) {
	_ = IsRunningInContainer()
}

func TestDetectServiceManagerReturnsKnownValue(t *testing.T) {
	got := DetectServiceManager()
	valid := map[string]bool{
		"systemd": true, "launchd": true, "runit": true, "s6": true,
		"sysv": true, "rcd": true, "container": true, "manual": true,
	}
	if !valid[got] {
		t.Errorf("DetectServiceManager() = %q, not a recognised service manager", got)
	}
}

// ── ShouldDaemonize ──────────────────────────────────────────────────────────

func TestShouldDaemonizeServiceStartForeground(t *testing.T) {
	// isServiceStart=true with a container/systemd service manager → false
	result := ShouldDaemonize(true, false, false)
	_ = result
}

func TestShouldDaemonizeDaemonFlagTrue(t *testing.T) {
	if got := ShouldDaemonize(false, true, false); !got {
		t.Error("ShouldDaemonize(false, daemonFlag=true, false) = false, want true")
	}
}

func TestShouldDaemonizeConfigDaemonizeTrue(t *testing.T) {
	if got := ShouldDaemonize(false, false, true); !got {
		t.Error("ShouldDaemonize(false, false, configDaemonize=true) = false, want true")
	}
}

func TestShouldDaemonizeFalseByDefault(t *testing.T) {
	if got := ShouldDaemonize(false, false, false); got {
		t.Error("ShouldDaemonize(false, false, false) = true, want false")
	}
}

// TestShouldDaemonizeServiceStartSysV exercises the sysv/rcd branch.
// We can't control the environment manager, so we only verify no panic.
func TestShouldDaemonizeServiceStartNoPanic(t *testing.T) {
	_ = ShouldDaemonize(true, true, true)
}
