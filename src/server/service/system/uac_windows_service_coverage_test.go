// SPDX-License-Identifier: MIT
// AI.md PART 23/24: Coverage for non-Windows UAC and Windows-service stubs.
//go:build !windows

package system

import "testing"

func TestRunAsWindowsService_ReturnsUnsupportedError(t *testing.T) {
	err := RunAsWindowsService(func() error { return nil })
	if err == nil {
		t.Error("RunAsWindowsService() = nil, want error on non-Windows platform")
	}
}

func TestIsWindowsService_ReturnsFalse(t *testing.T) {
	if IsWindowsService() {
		t.Error("IsWindowsService() = true, want false on non-Windows platform")
	}
}

func TestRunAsAdmin_ElevatedRunsCommand(t *testing.T) {
	if !IsRunningElevated() {
		t.Skip("not running as root; RunAsAdmin elevated branch not reachable")
	}
	if err := RunAsAdmin("true"); err != nil {
		t.Errorf("RunAsAdmin(true) = %v, want nil when elevated", err)
	}
}

func TestRunAsAdmin_ElevatedCommandFails(t *testing.T) {
	if !IsRunningElevated() {
		t.Skip("not running as root; RunAsAdmin elevated branch not reachable")
	}
	if err := RunAsAdmin("false"); err == nil {
		t.Error("RunAsAdmin(false) = nil, want error from failing command")
	}
}
