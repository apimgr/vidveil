// SPDX-License-Identifier: MIT
// AI.md PART 9: Distributed Locks

package cache

import (
	"context"
	"os"
	"sync"
	"time"
)

// nodeID identifies this node for lock ownership
var nodeID = os.Getenv("HOSTNAME")

func init() {
	if nodeID == "" {
		nodeID = "single-node"
	}
}

// LockStore manages distributed locks per AI.md PART 9
type LockStore interface {
	// AcquireLock tries to acquire a lock, returns true if successful
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// ReleaseLock releases a lock if we own it
	ReleaseLock(ctx context.Context, key string) error
	// IsLocked checks if a key is locked
	IsLocked(ctx context.Context, key string) (bool, error)
}

// MemoryLockStore provides in-memory distributed locks for single-node
type MemoryLockStore struct {
	locks map[string]*lockEntry
	mu    sync.Mutex
}

type lockEntry struct {
	owner    string
	expires  time.Time
}

// NewMemoryLockStore creates a new in-memory lock store
func NewMemoryLockStore() *MemoryLockStore {
	ls := &MemoryLockStore{
		locks: make(map[string]*lockEntry),
	}
	// Start cleanup goroutine
	go ls.cleanup()
	return ls
}

// AcquireLock tries to acquire a lock per AI.md PART 9
// Returns true if lock acquired, false if already held
func (ls *MemoryLockStore) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	fullKey := "lock:" + key
	now := time.Now()

	// Check if lock exists and is still valid
	if existing, ok := ls.locks[fullKey]; ok {
		if now.Before(existing.expires) {
			// Lock is held by someone else
			return false, nil
		}
		// Lock expired, we can take it
	}

	// Acquire the lock
	ls.locks[fullKey] = &lockEntry{
		owner:   nodeID,
		expires: now.Add(ttl),
	}
	return true, nil
}

// ReleaseLock releases a lock if we own it per AI.md PART 9
func (ls *MemoryLockStore) ReleaseLock(ctx context.Context, key string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	fullKey := "lock:" + key
	if existing, ok := ls.locks[fullKey]; ok {
		// Only release if we own it
		if existing.owner == nodeID {
			delete(ls.locks, fullKey)
		}
	}
	return nil
}

// IsLocked checks if a key is locked
func (ls *MemoryLockStore) IsLocked(ctx context.Context, key string) (bool, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	fullKey := "lock:" + key
	if existing, ok := ls.locks[fullKey]; ok {
		if time.Now().Before(existing.expires) {
			return true, nil
		}
		// Expired, clean up
		delete(ls.locks, fullKey)
	}
	return false, nil
}

// cleanup periodically removes expired locks
func (ls *MemoryLockStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ls.mu.Lock()
		now := time.Now()
		for key, entry := range ls.locks {
			if now.After(entry.expires) {
				delete(ls.locks, key)
			}
		}
		ls.mu.Unlock()
	}
}

// WithLock executes a function while holding a lock per AI.md PART 9
// This is the recommended way to use distributed locks
func WithLock(ctx context.Context, store LockStore, key string, ttl time.Duration, fn func() error) error {
	acquired, err := store.AcquireLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		// Another node is handling this
		return nil
	}
	defer store.ReleaseLock(ctx, key)
	return fn()
}

// Compile-time interface check
var _ LockStore = (*MemoryLockStore)(nil)
