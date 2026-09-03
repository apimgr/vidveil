// SPDX-License-Identifier: MIT
// AI.md PART 10: deterministic coverage for the connection-pool getters,
// exercising the configured-value branches (defaults are covered elsewhere).
package database

import (
	"testing"
	"time"
)

func TestPoolMaxOpen_ConfiguredAndDefault(t *testing.T) {
	if got := (PoolConfig{MaxOpen: 42}).maxOpen(); got != 42 {
		t.Errorf("maxOpen configured = %d, want 42", got)
	}
	if got := (PoolConfig{MaxOpen: 0}).maxOpen(); got != 25 {
		t.Errorf("maxOpen default = %d, want 25", got)
	}
}

func TestPoolMaxIdle_ConfiguredAndDefault(t *testing.T) {
	if got := (PoolConfig{MaxIdle: 7}).maxIdle(); got != 7 {
		t.Errorf("maxIdle configured = %d, want 7", got)
	}
	if got := (PoolConfig{MaxIdle: 0}).maxIdle(); got != 5 {
		t.Errorf("maxIdle default = %d, want 5", got)
	}
}

func TestPoolMaxLifetime_ConfiguredAndDefault(t *testing.T) {
	if got := (PoolConfig{MaxLifetime: "2h"}).maxLifetime(); got != 2*time.Hour {
		t.Errorf("maxLifetime configured = %v, want 2h", got)
	}
	// Empty and invalid strings both fall back to the 5m default.
	if got := (PoolConfig{MaxLifetime: ""}).maxLifetime(); got != 5*time.Minute {
		t.Errorf("maxLifetime empty = %v, want 5m", got)
	}
	if got := (PoolConfig{MaxLifetime: "not-a-duration"}).maxLifetime(); got != 5*time.Minute {
		t.Errorf("maxLifetime invalid = %v, want 5m", got)
	}
}

func TestPoolMaxIdleTime_ConfiguredAndDefault(t *testing.T) {
	if got := (PoolConfig{MaxIdleTime: "30s"}).maxIdleTime(); got != 30*time.Second {
		t.Errorf("maxIdleTime configured = %v, want 30s", got)
	}
	if got := (PoolConfig{MaxIdleTime: ""}).maxIdleTime(); got != time.Minute {
		t.Errorf("maxIdleTime default = %v, want 1m", got)
	}
	if got := (PoolConfig{MaxIdleTime: "bogus"}).maxIdleTime(); got != time.Minute {
		t.Errorf("maxIdleTime invalid = %v, want 1m", got)
	}
}
