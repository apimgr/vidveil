// SPDX-License-Identifier: MIT
// AI.md PART 8/21: deterministic coverage for backup-retention validation,
// FQDN detection, and container detection.
package config

import (
	"testing"
)

func TestValidateBackupRetention_NegativesClampToDefaults(t *testing.T) {
	cfg := DefaultAppConfig()
	r := &cfg.Server.Backup.Retention
	r.MaxBackups = -5
	r.KeepWeekly = -1
	r.KeepMonthly = -1
	r.KeepYearly = -1
	validateBackupRetention(cfg)
	if r.MaxBackups != 1 {
		t.Errorf("MaxBackups = %d, want clamped to 1", r.MaxBackups)
	}
	if r.KeepWeekly != 0 || r.KeepMonthly != 0 || r.KeepYearly != 0 {
		t.Errorf("negative keep_* must clamp to 0, got weekly=%d monthly=%d yearly=%d", r.KeepWeekly, r.KeepMonthly, r.KeepYearly)
	}
}

func TestValidateBackupRetention_ZeroMaxBackupsClamps(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Backup.Retention.MaxBackups = 0
	validateBackupRetention(cfg)
	if cfg.Server.Backup.Retention.MaxBackups != 1 {
		t.Errorf("MaxBackups = %d, want 1 for zero input", cfg.Server.Backup.Retention.MaxBackups)
	}
}

func TestValidateBackupRetention_AboveRecommendedWarnsButKeeps(t *testing.T) {
	// Values above recommended thresholds warn but are left unchanged.
	cfg := DefaultAppConfig()
	r := &cfg.Server.Backup.Retention
	r.MaxBackups = 10
	r.KeepWeekly = 9
	r.KeepMonthly = 13
	r.KeepYearly = 3
	validateBackupRetention(cfg)
	if r.MaxBackups != 10 || r.KeepWeekly != 9 || r.KeepMonthly != 13 || r.KeepYearly != 3 {
		t.Errorf("above-recommended values must be preserved, got %+v", *r)
	}
}

func TestGetFQDN_DomainEnvOverride(t *testing.T) {
	t.Setenv("DOMAIN", "override.example.com")
	if got := GetFQDN(); got != "override.example.com" {
		t.Errorf("GetFQDN = %q, want override.example.com", got)
	}
}

func TestGetFQDN_NoDomainReturnsNonEmpty(t *testing.T) {
	// With DOMAIN cleared, resolution falls through hostname/IP detection and
	// must always return a non-empty value (worst case "localhost").
	t.Setenv("DOMAIN", "")
	if got := GetFQDN(); got == "" {
		t.Error("GetFQDN returned empty string with no DOMAIN override")
	}
}

func TestIsRunningInContainer_DoesNotPanic(t *testing.T) {
	// Exercises the /proc/1/comm read path; result is environment-dependent but
	// the call must be side-effect free and return a bool.
	_ = IsRunningInContainer()
}
