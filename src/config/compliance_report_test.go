// SPDX-License-Identifier: MIT
// AI.md PART 5: deterministic coverage for the compliance-report data helpers
// backing "--maintenance compliance report".
package config

import "testing"

func TestComplianceStandardsStableOrder(t *testing.T) {
	c := ComplianceConfig{}
	got := c.Standards()
	if len(got) != 11 {
		t.Fatalf("Standards() returned %d entries, want 11", len(got))
	}
	wantFirst := "gdpr"
	wantLast := "pdpa"
	if got[0].Key != wantFirst {
		t.Errorf("first standard key = %q, want %q", got[0].Key, wantFirst)
	}
	if got[len(got)-1].Key != wantLast {
		t.Errorf("last standard key = %q, want %q", got[len(got)-1].Key, wantLast)
	}
	// A zero-value config has every standard disabled.
	for _, s := range got {
		if s.Enabled {
			t.Errorf("standard %q unexpectedly enabled on zero-value config", s.Key)
		}
	}
}

func TestComplianceEnabledStandards(t *testing.T) {
	t.Run("none enabled", func(t *testing.T) {
		c := ComplianceConfig{}
		if got := c.EnabledStandards(); len(got) != 0 {
			t.Errorf("EnabledStandards() = %v, want empty", got)
		}
		if c.IsEnabled() {
			t.Error("IsEnabled() = true on zero-value config, want false")
		}
	})

	t.Run("subset enabled preserves order", func(t *testing.T) {
		c := ComplianceConfig{GDPR: true, HIPAA: true, PDPA: true}
		got := c.EnabledStandards()
		if len(got) != 3 {
			t.Fatalf("EnabledStandards() = %v, want 3 entries", got)
		}
		// Order must follow Standards() order: gdpr, hipaa, pdpa.
		if got[0] != "GDPR (EU General Data Protection Regulation)" {
			t.Errorf("first enabled = %q, want GDPR", got[0])
		}
		if got[2] != "PDPA (Singapore Personal Data Protection Act)" {
			t.Errorf("last enabled = %q, want PDPA", got[2])
		}
		if !c.IsEnabled() {
			t.Error("IsEnabled() = false, want true when standards enabled")
		}
	})
}
