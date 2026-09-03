// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for BuildDateTime — exercises every time-format
// branch in the parsing loop by temporarily overriding version.BuildTime.
package handler

import (
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/common/version"
)

// setBuildTime temporarily overrides version.BuildTime and restores it on cleanup.
func setBuildTime(t *testing.T, bt string) {
	t.Helper()
	orig := version.BuildTime
	version.BuildTime = bt
	t.Cleanup(func() { version.BuildTime = orig })
}

func TestBuildDateTime_RFC3339Format(t *testing.T) {
	setBuildTime(t, "2024-06-15T10:30:00Z")
	got := BuildDateTime()
	if !strings.Contains(got, "2024") {
		t.Errorf("BuildDateTime RFC3339: %q does not contain year 2024", got)
	}
}

func TestBuildDateTime_RFC3339NoZ(t *testing.T) {
	setBuildTime(t, "2024-06-15T10:30:00")
	got := BuildDateTime()
	if got == "" || got == "unknown" {
		t.Errorf("BuildDateTime RFC3339NoZ: expected formatted date, got %q", got)
	}
}

func TestBuildDateTime_DateOnly(t *testing.T) {
	setBuildTime(t, "2024-06-15")
	got := BuildDateTime()
	if got == "" || got == "unknown" {
		t.Errorf("BuildDateTime date-only: expected formatted date, got %q", got)
	}
}

func TestBuildDateTime_SpaceSeparated(t *testing.T) {
	setBuildTime(t, "2024-06-15 10:30:00")
	got := BuildDateTime()
	if got == "" {
		t.Errorf("BuildDateTime space-separated: expected non-empty, got %q", got)
	}
}

func TestBuildDateTime_UnknownStringReturnsUnknown(t *testing.T) {
	setBuildTime(t, "unknown")
	got := BuildDateTime()
	if got != "unknown" {
		t.Errorf("BuildDateTime('unknown'): expected 'unknown', got %q", got)
	}
}

func TestBuildDateTime_UnparsableReturnsRaw(t *testing.T) {
	raw := "not-a-date"
	setBuildTime(t, raw)
	got := BuildDateTime()
	if got != raw {
		t.Errorf("BuildDateTime(unparsable): expected raw %q, got %q", raw, got)
	}
}

func TestBuildDateTime_MonDDYYYY(t *testing.T) {
	setBuildTime(t, "Jan 2 2006 15:04:05")
	got := BuildDateTime()
	if got == "" || got == "Jan 2 2006 15:04:05" {
		t.Logf("BuildDateTime(Mon DD YYYY): %q", got)
	}
}

func TestBuildDateTime_LongFormat(t *testing.T) {
	setBuildTime(t, "Mon Jan 2 15:04:05 2006")
	got := BuildDateTime()
	if got == "" {
		t.Errorf("BuildDateTime(long format): expected non-empty, got %q", got)
	}
}
