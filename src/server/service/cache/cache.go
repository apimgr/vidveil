// SPDX-License-Identifier: MIT
// AI.md PART 9: Caching (see also PART 12 for full configuration)
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/model"
	"github.com/redis/go-redis/v9"
)

// CacheType represents the type of cache backend
type CacheType string

const (
	CacheTypeMemory CacheType = "memory"
	CacheTypeValkey CacheType = "valkey"
	CacheTypeRedis  CacheType = "redis"
)

// SearchResultCache defines the interface for a distributed search result cache
type SearchResultCache interface {
	Get(key string) (*model.SearchResponse, bool)
	Set(key string, response *model.SearchResponse)
	Delete(key string)
	Clear()
	Size() int
	Stats() map[string]interface{}
	Close() error
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Type CacheType `yaml:"type"`
	// TTL in seconds
	TTL     int `yaml:"ttl"`
	MaxSize int `yaml:"max_size"`
	// Valkey/Redis settings
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
}

// NewSearchResultCache creates a new cache based on configuration
func NewSearchResultCache(cfg CacheConfig) (SearchResultCache, error) {
	ttl := time.Duration(cfg.TTL) * time.Second
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	switch cfg.Type {
	case CacheTypeValkey, CacheTypeRedis:
		return NewValkeyCache(cfg.Addr, cfg.Password, cfg.DB, cfg.Prefix, ttl)
	default:
		return NewSearchCache(ttl, cfg.MaxSize), nil
	}
}

// SearchCache provides in-memory caching for search results
type SearchCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	maxSize int
	ctx     context.Context
	cancel  context.CancelFunc
}

type cacheEntry struct {
	response  *model.SearchResponse
	createdAt time.Time
}

// NewSearchCache creates a new search cache
func NewSearchCache(ttl time.Duration, maxSize int) *SearchCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	if maxSize == 0 {
		maxSize = 1000
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &SearchCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Close stops the cache cleanup goroutine
func (c *SearchCache) Close() error {
	c.cancel()
	return nil
}

// Get retrieves a cached search response
func (c *SearchCache) Get(key string) (*model.SearchResponse, bool) {
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
func (c *SearchCache) Set(key string, response *model.SearchResponse) {
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

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
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

// ValkeyCache provides distributed caching using Valkey/Redis
type ValkeyCache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
	mu     sync.RWMutex
	closed bool
}

// NewValkeyCache creates a new Valkey/Redis cache using go-redis
func NewValkeyCache(addr, password string, db int, prefix string, ttl time.Duration) (*ValkeyCache, error) {
	if addr == "" {
		addr = "localhost:6379"
	}
	if prefix == "" {
		prefix = "vidveil:"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("valkey/redis connection failed: %w", err)
	}

	return &ValkeyCache{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}, nil
}

// Get retrieves a cached search response from Valkey/Redis
func (v *ValkeyCache) Get(key string) (*model.SearchResponse, bool) {
	v.mu.RLock()
	closed := v.closed
	v.mu.RUnlock()
	if closed {
		return nil, false
	}

	ctx := context.Background()
	data, err := v.client.Get(ctx, v.prefix+key).Bytes()
	if err != nil {
		return nil, false
	}

	var response model.SearchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, false
	}
	return &response, true
}

// Set stores a search response in Valkey/Redis
func (v *ValkeyCache) Set(key string, response *model.SearchResponse) {
	v.mu.RLock()
	closed := v.closed
	v.mu.RUnlock()
	if closed {
		return
	}

	data, err := json.Marshal(response)
	if err != nil {
		return
	}

	ctx := context.Background()
	v.client.Set(ctx, v.prefix+key, data, v.ttl)
}

// Delete removes a specific key from Valkey/Redis
func (v *ValkeyCache) Delete(key string) {
	v.mu.RLock()
	closed := v.closed
	v.mu.RUnlock()
	if closed {
		return
	}

	ctx := context.Background()
	v.client.Del(ctx, v.prefix+key)
}

// Clear removes all entries with our prefix from Valkey/Redis
func (v *ValkeyCache) Clear() {
	v.mu.RLock()
	closed := v.closed
	v.mu.RUnlock()
	if closed {
		return
	}

	ctx := context.Background()
	var cursor uint64
	for {
		keys, nextCursor, err := v.client.Scan(ctx, cursor, v.prefix+"*", 100).Result()
		if err != nil {
			break
		}
		if len(keys) > 0 {
			v.client.Del(ctx, keys...)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

// Size returns the approximate number of cached entries
func (v *ValkeyCache) Size() int {
	v.mu.RLock()
	closed := v.closed
	v.mu.RUnlock()
	if closed {
		return 0
	}

	ctx := context.Background()
	var count int
	var cursor uint64
	for {
		keys, nextCursor, err := v.client.Scan(ctx, cursor, v.prefix+"*", 100).Result()
		if err != nil {
			break
		}
		count += len(keys)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return count
}

// Stats returns cache statistics
func (v *ValkeyCache) Stats() map[string]interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()

	stats := map[string]interface{}{
		"type":    "valkey",
		"prefix":  v.prefix,
		"ttl_sec": v.ttl.Seconds(),
		"closed":  v.closed,
	}

	if !v.closed {
		ctx := context.Background()
		if info, err := v.client.Info(ctx, "memory", "stats").Result(); err == nil {
			stats["info"] = info
		}
		if opts := v.client.Options(); opts != nil {
			stats["addr"] = opts.Addr
			stats["db"] = opts.DB
		}
	}

	return stats
}

// Close closes the Valkey/Redis connection
func (v *ValkeyCache) Close() error {
	v.mu.Lock()
	v.closed = true
	v.mu.Unlock()
	return v.client.Close()
}

// Compile-time interface check
var _ SearchResultCache = (*SearchCache)(nil)
var _ SearchResultCache = (*ValkeyCache)(nil)
