// SPDX-License-Identifier: MIT
package maintenance

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMaintenanceStatusReadWriteAndClear(t *testing.T) {
	dataDir := t.TempDir()
	started := time.Now().UTC().Truncate(time.Second)
	lastAttempt := started.Add(time.Minute)
	nextAttempt := lastAttempt.Add(MaintenanceRetryInterval)
	want := MaintenanceStatus{
		Reason: "database_connection", Message: "Database connection recovery is in progress.", Since: started,
		SelfHealing: SelfHealingStatus{Enabled: true, Attempts: 3, LastAttempt: &lastAttempt, NextAttempt: &nextAttempt},
	}
	if err := os.WriteFile(filepath.Join(dataDir, "maintenance.flag"), []byte(started.Format(time.RFC3339)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteMaintenanceStatus(dataDir, want); err != nil {
		t.Fatal(err)
	}
	got, active := ReadMaintenanceStatus(dataDir)
	if !active || got == nil || got.Reason != want.Reason || got.SelfHealing.Attempts != want.SelfHealing.Attempts {
		t.Fatalf("ReadMaintenanceStatus = %#v, %t", got, active)
	}
	info, err := os.Stat(maintenanceStatusPath(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("status file mode = %o, want 600", info.Mode().Perm())
	}
	if err := ClearMaintenanceStatus(dataDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(maintenanceStatusPath(dataDir)); !os.IsNotExist(err) {
		t.Errorf("status file remains after clear: %v", err)
	}
}

func TestSelfHealingAttemptStatusHonorsLimit(t *testing.T) {
	last := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	next := last.Add(MaintenanceRetryInterval)
	tests := []struct {
		name        string
		attempts    uint64
		maxAttempts uint64
		enabled     bool
		exhausted   bool
		hasNext     bool
	}{
		{name: "unlimited", attempts: 100, enabled: true, hasNext: true},
		{name: "under limit", attempts: 2, maxAttempts: 3, enabled: true, hasNext: true},
		{name: "at limit", attempts: 3, maxAttempts: 3, exhausted: true},
		{name: "over limit", attempts: 4, maxAttempts: 3, exhausted: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := SelfHealingAttemptStatus(test.attempts, test.maxAttempts, last, next)
			if got.Enabled != test.enabled || got.Exhausted != test.exhausted || (got.NextAttempt != nil) != test.hasNext {
				t.Errorf("SelfHealingAttemptStatus = %#v", got)
			}
			if got.Attempts != test.attempts || got.LastAttempt == nil || !got.LastAttempt.Equal(last) {
				t.Errorf("attempt metadata = %#v", got)
			}
		})
	}
}

func TestReadMaintenanceStatusManualFlagFallback(t *testing.T) {
	dataDir := t.TempDir()
	started := time.Now().UTC().Truncate(time.Second)
	if err := os.WriteFile(filepath.Join(dataDir, "maintenance.flag"), []byte(started.Format(time.RFC3339)), 0600); err != nil {
		t.Fatal(err)
	}
	got, active := ReadMaintenanceStatus(dataDir)
	if !active || got == nil || got.Reason != "manual" || got.SelfHealing.Enabled {
		t.Fatalf("ReadMaintenanceStatus manual fallback = %#v, %t", got, active)
	}
	if !got.Since.Equal(started) {
		t.Errorf("maintenance since = %s, want %s", got.Since, started)
	}
}

func TestReadMaintenanceStatusPreservesExhaustedSelfHealing(t *testing.T) {
	dataDir := t.TempDir()
	started := time.Now().UTC().Truncate(time.Second)
	if err := os.WriteFile(filepath.Join(dataDir, "maintenance.flag"), []byte(started.Format(time.RFC3339)), 0600); err != nil {
		t.Fatal(err)
	}
	want := MaintenanceStatus{
		Reason: "database_connection", Since: started,
		SelfHealing: SelfHealingStatus{Exhausted: true, Attempts: 4},
	}
	if err := WriteMaintenanceStatus(dataDir, want); err != nil {
		t.Fatal(err)
	}
	got, active := ReadMaintenanceStatus(dataDir)
	if !active || got == nil {
		t.Fatalf("ReadMaintenanceStatus = %#v, %t", got, active)
	}
	if got.SelfHealing.Enabled || !got.SelfHealing.Exhausted || got.SelfHealing.NextAttempt != nil {
		t.Errorf("exhausted self-healing state = %#v, want disabled, exhausted, and no next attempt", got.SelfHealing)
	}
	if got.Message != "Self-healing retry limit reached; correct the cause and restart the service." {
		t.Errorf("message = %q", got.Message)
	}
}
