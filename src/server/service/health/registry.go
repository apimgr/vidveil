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

// StatFunc returns one public-safe, non-negative aggregate counter.
type StatFunc func() uint64

// StatResult is a public aggregate counter in registration order.
type StatResult struct {
	Key   string
	Value uint64
}

type statEntry struct {
	key   string
	value StatFunc
	last  uint64
}

type entry struct {
	key      string
	critical bool
	check    CheckFunc
	status   string
}

type Registry struct {
	mu       sync.RWMutex
	entries  []entry
	byKey    map[string]int
	stats    []statEntry
	statKeys map[string]struct{}
}

func NewRegistry() *Registry {
	return &Registry{byKey: make(map[string]int), statKeys: make(map[string]struct{})}
}

// RegisterStat registers a public-safe aggregate stat callback once. Values
// are sampled when the cached health snapshot is refreshed, never per request.
func (r *Registry) RegisterStat(key string, value StatFunc) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid health stat key %q", key)
	}
	switch key {
	case "requests_total", "requests_24h", "active_connections":
		return fmt.Errorf("health stat key %q is reserved", key)
	}
	if value == nil {
		return fmt.Errorf("health stat %q has no callback", key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.statKeys[key]; exists {
		return fmt.Errorf("health stat %q already registered", key)
	}
	r.statKeys[key] = struct{}{}
	r.stats = append(r.stats, statEntry{key: key, value: value})
	return nil
}

// Stats samples registered callbacks in their stable registration order.
func (r *Registry) Stats() []StatResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make([]StatResult, 0, len(r.stats))
	for _, item := range r.stats {
		results = append(results, StatResult{Key: item.key, Value: item.last})
	}
	return results
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
	r.mu.RLock()
	stats := append([]statEntry(nil), r.stats...)
	r.mu.RUnlock()
	for i := range stats {
		stats[i].last = stats[i].value()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, updated := range entries {
		if index, exists := r.byKey[updated.key]; exists {
			r.entries[index].status = updated.status
		}
	}
	for _, updated := range stats {
		for index := range r.stats {
			if r.stats[index].key == updated.key {
				r.stats[index].last = updated.last
				break
			}
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
