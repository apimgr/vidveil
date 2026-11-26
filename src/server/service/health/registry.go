// SPDX-License-Identifier: MIT
// Package health caches public-safe checks for runtime datasets and dependencies.
package health

import (
	"fmt"
	"regexp"
	"sync"
)

const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
	StatusError    = "error"
)

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type CheckFunc func() string

type CheckResult struct {
	Key      string
	Status   string
	Critical bool
}

type entry struct {
	key      string
	critical bool
	check    CheckFunc
	status   string
}

type Registry struct {
	mu      sync.RWMutex
	entries []entry
	byKey   map[string]int
}

func NewRegistry() *Registry {
	return &Registry{byKey: make(map[string]int)}
}

func (r *Registry) Register(key string, critical bool, check CheckFunc) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid health check key %q", key)
	}
	if check == nil {
		return fmt.Errorf("health check %q has no callback", key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if index, exists := r.byKey[key]; exists {
		r.entries[index].critical = critical
		r.entries[index].check = check
		return nil
	}
	r.byKey[key] = len(r.entries)
	r.entries = append(r.entries, entry{key: key, critical: critical, check: check})
	return nil
}

// Refresh runs registered callbacks outside the lock and stores only their
// contract status. Call this from startup and the health refresh scheduler.
func (r *Registry) Refresh() {
	r.mu.RLock()
	entries := append([]entry(nil), r.entries...)
	r.mu.RUnlock()
	for i := range entries {
		status := entries[i].check()
		if status != StatusOK && status != StatusDegraded && status != StatusError {
			status = StatusError
		}
		entries[i].status = status
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, updated := range entries {
		if index, exists := r.byKey[updated.key]; exists {
			r.entries[index].status = updated.status
		}
	}
}

// Snapshot returns cached checks in registration order and never runs a probe.
func (r *Registry) Snapshot() []CheckResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make([]CheckResult, 0, len(r.entries))
	for _, item := range r.entries {
		results = append(results, CheckResult{Key: item.key, Status: item.status, Critical: item.critical})
	}
	return results
}
