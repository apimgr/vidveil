// SPDX-License-Identifier: MIT

package main

import (
	"testing"
	"time"
)

// TestPrivateExportRateLimit verifies the per-operator 1-per-hour rate limit for
// private-key exports (AI.md PART 12): a fresh operator is allowed, a second
// export within the window is rejected, a different operator is unaffected, and
// the limit clears once the window has elapsed.
func TestPrivateExportRateLimit(t *testing.T) {
	configDir := t.TempDir()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

	// No prior export: allowed.
	if err := checkPrivateExportRateLimit(configDir, "alice", now); err != nil {
		t.Fatalf("first export must be allowed, got %v", err)
	}

	if err := recordPrivateExport(configDir, "alice", now); err != nil {
		t.Fatalf("recordPrivateExport: %v", err)
	}

	// Same operator, 30 minutes later: rate limited.
	if err := checkPrivateExportRateLimit(configDir, "alice", now.Add(30*time.Minute)); err == nil {
		t.Error("export within the window must be rate limited")
	}

	// Different operator: unaffected.
	if err := checkPrivateExportRateLimit(configDir, "bob", now.Add(30*time.Minute)); err != nil {
		t.Errorf("a different operator must not be rate limited, got %v", err)
	}

	// Same operator, just past the window: allowed again.
	if err := checkPrivateExportRateLimit(configDir, "alice", now.Add(time.Hour+time.Minute)); err != nil {
		t.Errorf("export after the window must be allowed, got %v", err)
	}
}

// TestPrivateExportStateRoundTrip verifies the export-state file survives a
// second operator being recorded without clobbering the first.
func TestPrivateExportStateRoundTrip(t *testing.T) {
	configDir := t.TempDir()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

	if err := recordPrivateExport(configDir, "alice", now); err != nil {
		t.Fatalf("record alice: %v", err)
	}
	if err := recordPrivateExport(configDir, "bob", now.Add(2*time.Hour)); err != nil {
		t.Fatalf("record bob: %v", err)
	}

	state, err := loadPrivateExportState(configDir)
	if err != nil {
		t.Fatalf("loadPrivateExportState: %v", err)
	}
	if _, ok := state["alice"]; !ok {
		t.Error("alice's export timestamp was lost when bob's was recorded")
	}
	if _, ok := state["bob"]; !ok {
		t.Error("bob's export timestamp was not recorded")
	}
}

// TestLoadPrivateExportStateMissing verifies a missing state file yields an empty
// map rather than an error (first-ever export path).
func TestLoadPrivateExportStateMissing(t *testing.T) {
	state, err := loadPrivateExportState(t.TempDir())
	if err != nil {
		t.Fatalf("missing state file must not error, got %v", err)
	}
	if len(state) != 0 {
		t.Errorf("missing state file must yield empty map, got %d entries", len(state))
	}
}
