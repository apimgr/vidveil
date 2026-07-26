// SPDX-License-Identifier: MIT
// AI.md PART 5: deterministic coverage for the env-var-with-default helper.
package config

import "testing"

func TestEnvDefault(t *testing.T) {
	const key = "VIDVEIL_ENVDEFAULT_TEST"

	t.Run("set non-empty returns value", func(t *testing.T) {
		t.Setenv(key, "actual")
		if got := envDefault(key, "fallback"); got != "actual" {
			t.Errorf("envDefault = %q, want actual", got)
		}
	})

	t.Run("set empty returns default", func(t *testing.T) {
		t.Setenv(key, "")
		if got := envDefault(key, "fallback"); got != "fallback" {
			t.Errorf("envDefault = %q, want fallback", got)
		}
	})
}
