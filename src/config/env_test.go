// SPDX-License-Identifier: MIT
package config

import (
	"testing"
)

// VIDVEIL_SERVER_PORT must override server.port (full path form).
func TestApplyEnvOverridesFullPath(t *testing.T) {
	t.Setenv("VIDVEIL_SERVER_PORT", "8080")
	cfg := DefaultAppConfig()
	ApplyEnvOverrides(cfg)
	if cfg.Server.Port != "8080" {
		t.Errorf("VIDVEIL_SERVER_PORT: got %q, want %q", cfg.Server.Port, "8080")
	}
}

// VIDVEIL_DATABASE_DRIVER must reach server.database.driver via the SERVER_-less alias.
func TestApplyEnvOverridesServerAlias(t *testing.T) {
	t.Setenv("VIDVEIL_DATABASE_DRIVER", "sqlite")
	cfg := DefaultAppConfig()
	cfg.Server.Database.Driver = "other"
	ApplyEnvOverrides(cfg)
	if cfg.Server.Database.Driver != "sqlite" {
		t.Errorf("VIDVEIL_DATABASE_DRIVER alias: got %q, want %q", cfg.Server.Database.Driver, "sqlite")
	}
}

// The full-path name must win over the SERVER_-less alias when both are set.
func TestApplyEnvOverridesFullPathWinsOverAlias(t *testing.T) {
	t.Setenv("VIDVEIL_SERVER_PORT", "9001")
	t.Setenv("VIDVEIL_PORT", "9002")
	cfg := DefaultAppConfig()
	ApplyEnvOverrides(cfg)
	if cfg.Server.Port != "9001" {
		t.Errorf("full path must win: got %q, want %q", cfg.Server.Port, "9001")
	}
}

// Boolean fields must accept true/false, yes/no, 1/0, on/off per AI.md.
func TestApplyEnvOverridesBool(t *testing.T) {
	cases := map[string]bool{"true": true, "yes": true, "1": true, "on": true, "false": false, "no": false, "0": false, "off": false}
	for val, want := range cases {
		t.Setenv("VIDVEIL_SERVER_PIDFILE", val)
		cfg := DefaultAppConfig()
		cfg.Server.PIDFile = !want
		ApplyEnvOverrides(cfg)
		if cfg.Server.PIDFile != want {
			t.Errorf("VIDVEIL_SERVER_PIDFILE=%q: got %v, want %v", val, cfg.Server.PIDFile, want)
		}
	}
}

// An invalid boolean value must warn and leave the config value unchanged.
func TestApplyEnvOverridesInvalidBoolIgnored(t *testing.T) {
	t.Setenv("VIDVEIL_SERVER_PIDFILE", "banana")
	cfg := DefaultAppConfig()
	cfg.Server.PIDFile = true
	ApplyEnvOverrides(cfg)
	if cfg.Server.PIDFile != true {
		t.Error("invalid boolean must be ignored, config value changed")
	}
}

// An invalid integer value must warn and leave the config value unchanged.
func TestApplyEnvOverridesInvalidIntIgnored(t *testing.T) {
	t.Setenv("VIDVEIL_SERVER_RATE_LIMIT_REQUESTS", "notanumber")
	cfg := DefaultAppConfig()
	want := cfg.Server.RateLimit.Requests
	ApplyEnvOverrides(cfg)
	if cfg.Server.RateLimit.Requests != want {
		t.Errorf("invalid integer must be ignored: got %d, want %d", cfg.Server.RateLimit.Requests, want)
	}
}

// Integer fields must be parsed from env strings.
func TestApplyEnvOverridesInt(t *testing.T) {
	t.Setenv("VIDVEIL_SERVER_RATE_LIMIT_REQUESTS", "250")
	cfg := DefaultAppConfig()
	ApplyEnvOverrides(cfg)
	if cfg.Server.RateLimit.Requests != 250 {
		t.Errorf("VIDVEIL_SERVER_RATE_LIMIT_REQUESTS: got %d, want 250", cfg.Server.RateLimit.Requests)
	}
}

// Nested non-server sections use the full path only (no alias).
func TestApplyEnvOverridesNonServerSection(t *testing.T) {
	t.Setenv("VIDVEIL_SEARCH_FILTER_PREMIUM", "false")
	cfg := DefaultAppConfig()
	cfg.Search.FilterPremium = true
	ApplyEnvOverrides(cfg)
	if cfg.Search.FilterPremium != false {
		t.Error("VIDVEIL_SEARCH_FILTER_PREMIUM=false must set search.filter_premium to false")
	}
}

// String list fields must split on commas.
func TestApplyEnvOverridesStringSlice(t *testing.T) {
	t.Setenv("VIDVEIL_SEARCH_DEFAULT_ENGINES", "xvideos, xnxx")
	cfg := DefaultAppConfig()
	ApplyEnvOverrides(cfg)
	if len(cfg.Search.DefaultEngines) != 2 || cfg.Search.DefaultEngines[0] != "xvideos" || cfg.Search.DefaultEngines[1] != "xnxx" {
		t.Errorf("VIDVEIL_SEARCH_DEFAULT_ENGINES: got %v, want [xvideos xnxx]", cfg.Search.DefaultEngines)
	}
}
