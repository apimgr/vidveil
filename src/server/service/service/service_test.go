// SPDX-License-Identifier: MIT
// AI.md PART 28: Test coverage for service package.
package service

import (
	"testing"
)

// --- NewSystemServiceManager ---

// TestNewSystemServiceManager_ReturnsManager verifies a manager is created without error.
func TestNewSystemServiceManager_ReturnsManager(t *testing.T) {
	m, err := NewSystemServiceManager("vidveil", "VidVeil", "VidVeil proxy service")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	if m == nil {
		t.Fatal("NewSystemServiceManager() returned nil manager")
	}
}

// TestNewSystemServiceManager_SetsName verifies the name field is set.
func TestNewSystemServiceManager_SetsName(t *testing.T) {
	m, err := NewSystemServiceManager("myservice", "My Service", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	if m.name != "myservice" {
		t.Errorf("name = %q, want %q", m.name, "myservice")
	}
}

// TestNewSystemServiceManager_SetsDisplayName verifies displayName is set.
func TestNewSystemServiceManager_SetsDisplayName(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "Display Name", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	if m.displayName != "Display Name" {
		t.Errorf("displayName = %q, want %q", m.displayName, "Display Name")
	}
}

// TestNewSystemServiceManager_SetsDescription verifies description is set.
func TestNewSystemServiceManager_SetsDescription(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "disp", "my description")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	if m.description != "my description" {
		t.Errorf("description = %q, want %q", m.description, "my description")
	}
}

// TestNewSystemServiceManager_ExecPathNonEmpty verifies the executable path is set.
func TestNewSystemServiceManager_ExecPathNonEmpty(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "disp", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	if m.execPath == "" {
		t.Error("execPath is empty, want non-empty executable path")
	}
}

// --- hasSystemd / hasOpenRC / hasSysVinit / hasRunit ---

// TestHasSystemd_NoPanic verifies hasSystemd does not panic.
func TestHasSystemd_NoPanic(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "disp", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasSystemd() panicked: %v", r)
		}
	}()
	_ = m.hasSystemd()
}

// TestHasOpenRC_NoPanic verifies hasOpenRC does not panic.
func TestHasOpenRC_NoPanic(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "disp", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasOpenRC() panicked: %v", r)
		}
	}()
	_ = m.hasOpenRC()
}

// TestHasSysVinit_NoPanic verifies hasSysVinit does not panic.
func TestHasSysVinit_NoPanic(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "disp", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasSysVinit() panicked: %v", r)
		}
	}()
	_ = m.hasSysVinit()
}

// TestHasRunit_NoPanic verifies hasRunit does not panic.
func TestHasRunit_NoPanic(t *testing.T) {
	m, err := NewSystemServiceManager("svc", "disp", "desc")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasRunit() panicked: %v", r)
		}
	}()
	_ = m.hasRunit()
}

// --- GetServiceStatus ---

// TestGetServiceStatus_NoPanic verifies GetServiceStatus does not panic (may return error).
func TestGetServiceStatus_NoPanic(t *testing.T) {
	m, err := NewSystemServiceManager("nonexistent-svc-xyz", "Non-existent", "test")
	if err != nil {
		t.Fatalf("NewSystemServiceManager() error: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetServiceStatus() panicked: %v", r)
		}
	}()
	_, _ = m.GetServiceStatus()
}
