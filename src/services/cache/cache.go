// SPDX-License-Identifier: MIT
package cache

import (
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/models"
)

// SearchCache provides in-memory caching for search results
type SearchCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	maxSize int
}

type cacheEntry struct {
	response  *models.SearchResponse
	createdAt time.Time
}

// New creates a new search cache
func New(ttl time.Duration, maxSize int) *SearchCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	if maxSize == 0 {
		maxSize = 1000
	}

	c := &SearchCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a cached search response
func (c *SearchCache) Get(key string) (*models.SearchResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.createdAt) > c.ttl {
		return nil, false
	}

	return entry.response, true
}

// Set stores a search response in cache
func (c *SearchCache) Set(key string, response *models.SearchResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		response:  response,
		createdAt: time.Now(),
	}
}

// Delete removes a specific key from cache
func (c *SearchCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all entries from cache
func (c *SearchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Size returns the current number of cached entries
func (c *SearchCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats returns cache statistics
func (c *SearchCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"size":     len(c.entries),
		"max_size": c.maxSize,
		"ttl_sec":  c.ttl.Seconds(),
	}
}

// evictOldest removes the oldest 10% of entries
func (c *SearchCache) evictOldest() {
	// Find oldest entries
	type keyTime struct {
		key string
		t   time.Time
	}

	var items []keyTime
	for k, v := range c.entries {
		items = append(items, keyTime{k, v.createdAt})
	}

	// Sort by time (oldest first)
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].t.Before(items[i].t) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Remove oldest 10%
	toRemove := len(items) / 10
	if toRemove < 1 {
		toRemove = 1
	}

	for i := 0; i < toRemove && i < len(items); i++ {
		delete(c.entries, items[i].key)
	}
}

// cleanup periodically removes expired entries
func (c *SearchCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.Sub(entry.createdAt) > c.ttl {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// CacheKey generates a cache key for a search query
func CacheKey(query string, page int, engines []string) string {
	key := query + "|" + string(rune(page))
	if len(engines) > 0 {
		for _, e := range engines {
			key += "|" + e
		}
	}
	return key
}
