// SPDX-License-Identifier: MIT
// AI.md PART 9: Caching (see also PART 12 for full configuration)
package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/metric"
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
	// Check reports whether the configured cache backend is reachable.
	Check(context.Context) error
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Type CacheType `yaml:"type"`
	// TTL in seconds
	TTL     int `yaml:"ttl"`
	MaxSize int `yaml:"max_size"`
	// URL takes precedence over Addr when set (redis:// or valkey://)
	URL string `yaml:"url"`
	// Valkey/Redis connection settings
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
	// TLS settings per AI.md PART 12
	TLS           bool `yaml:"tls"`
	TLSSkipVerify bool `yaml:"tls_skip_verify"`
	// Connection pool per AI.md PART 12
	PoolSize int `yaml:"pool_size"`
	MinIdle  int `yaml:"min_idle"`
}

// NewSearchResultCache creates a new cache based on configuration
func NewSearchResultCache(cfg CacheConfig) (SearchResultCache, error) {
	ttl := time.Duration(cfg.TTL) * time.Second
	if ttl == 0 {
		// API response cache TTL per AI.md PART 12 "Cache TTL Defaults" (30s,
		// frequently-changing data). Search results are an API response cache.
		ttl = 30 * time.Second
	}

	switch cfg.Type {
	case CacheTypeValkey, CacheTypeRedis:
		return NewValkeyCache(cfg, ttl)
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
	bytes   int64
	ctx     context.Context
	cancel  context.CancelFunc
}

type cacheEntry struct {
	response  *model.SearchResponse
	createdAt time.Time
	bytes     int64
}

// NewSearchCache creates a new search cache
func NewSearchCache(ttl time.Duration, maxSize int) *SearchCache {
	if ttl == 0 {
		ttl = 30 * time.Second
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

// Check reports an in-process cache as healthy.
func (c *SearchCache) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	want := &model.SearchResponse{Ok: true, Message: "healthz:probe"}
	c.Set("healthz:probe", want)
	got, ok := c.Get("healthz:probe")
	c.Delete("healthz:probe")
	if !ok || got == nil || got.Ok != want.Ok || got.Message != want.Message {
		return fmt.Errorf("in-process cache probe value mismatch")
	}
	return ctx.Err()
}

// Get retrieves a cached search response
func (c *SearchCache) Get(key string) (*model.SearchResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		metric.CacheMissesTotal.WithLabelValues("search").Inc()
		return nil, false
	}

	// Check if expired
	if time.Since(entry.createdAt) > c.ttl {
		metric.CacheMissesTotal.WithLabelValues("search").Inc()
		return nil, false
	}

	metric.CacheHitsTotal.WithLabelValues("search").Inc()
	return entry.response, true
}

// Set stores a search response in cache
func (c *SearchCache) Set(key string, response *model.SearchResponse) {
	encoded, err := json.Marshal(response)
	var entryBytes int64
	if err == nil {
		entryBytes = int64(len(encoded))
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	if prior, exists := c.entries[key]; exists {
		c.bytes -= prior.bytes
	}
	c.entries[key] = &cacheEntry{
		response:  response,
		createdAt: time.Now(),
		bytes:     entryBytes,
	}
	c.bytes += entryBytes
	metric.CacheSize.WithLabelValues("search").Set(float64(len(c.entries)))
	metric.CacheBytes.WithLabelValues("search").Set(float64(c.bytes))
}

// Delete removes a specific key from cache
func (c *SearchCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, exists := c.entries[key]; exists {
		c.bytes -= entry.bytes
	}
	delete(c.entries, key)
	metric.CacheSize.WithLabelValues("search").Set(float64(len(c.entries)))
	metric.CacheBytes.WithLabelValues("search").Set(float64(c.bytes))
}

// Clear removes all entries from cache
func (c *SearchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.bytes = 0
	metric.CacheSize.WithLabelValues("search").Set(0)
	metric.CacheBytes.WithLabelValues("search").Set(0)
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
		if entry := c.entries[items[i].key]; entry != nil {
			c.bytes -= entry.bytes
			delete(c.entries, items[i].key)
		}
		metric.CacheEvictions.WithLabelValues("search").Inc()
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
					c.bytes -= entry.bytes
					delete(c.entries, key)
				}
			}
			metric.CacheSize.WithLabelValues("search").Set(float64(len(c.entries)))
			metric.CacheBytes.WithLabelValues("search").Set(float64(c.bytes))
			c.mu.Unlock()
		}
	}
}

// CacheKey generates a cache key for a search query.
// Per AI.md PART 9 key naming: hierarchical colon-separated segments,
// lowercase (e.g. "search:{query}:{page}:{engines}").
func CacheKey(query string, page int, engines []string) string {
	key := "search:" + strings.ToLower(query) + ":" + strconv.Itoa(page)
	if len(engines) > 0 {
		for _, e := range engines {
			key += ":" + strings.ToLower(e)
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

// NewValkeyCache creates a new Valkey/Redis cache using go-redis, honoring the
// AI.md PART 12 connection settings (auth, TLS, pool sizing).
func NewValkeyCache(cfg CacheConfig, ttl time.Duration) (*ValkeyCache, error) {
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "vidveil:"
	}

	var opts *redis.Options
	// Per AI.md PART 12: url takes precedence over host/port when set.
	if cfg.URL != "" {
		// go-redis only understands redis://[s]; normalize the valkey:// scheme.
		normalized := cfg.URL
		if strings.HasPrefix(normalized, "valkeys://") {
			normalized = "rediss://" + strings.TrimPrefix(normalized, "valkeys://")
		} else if strings.HasPrefix(normalized, "valkey://") {
			normalized = "redis://" + strings.TrimPrefix(normalized, "valkey://")
		}
		parsed, err := redis.ParseURL(normalized)
		if err != nil {
			return nil, fmt.Errorf("valkey/redis url parse failed: %w", err)
		}
		opts = parsed
	} else {
		addr := cfg.Addr
		if addr == "" {
			addr = "localhost:6379"
		}
		opts = &redis.Options{
			Addr:     addr,
			Username: cfg.Username,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}

	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdle > 0 {
		opts.MinIdleConns = cfg.MinIdle
	}
	if cfg.TLS && opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{InsecureSkipVerify: cfg.TLSSkipVerify}
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
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
		metric.CacheMissesTotal.WithLabelValues("search").Inc()
		return nil, false
	}

	ctx := context.Background()
	data, err := v.client.Get(ctx, v.prefix+key).Bytes()
	if err != nil {
		metric.CacheMissesTotal.WithLabelValues("search").Inc()
		return nil, false
	}

	var response model.SearchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		metric.CacheMissesTotal.WithLabelValues("search").Inc()
		return nil, false
	}
	metric.CacheHitsTotal.WithLabelValues("search").Inc()
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

// Check verifies external cache reachability for health reporting.
func (v *ValkeyCache) Check(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	key := v.prefix + "healthz:probe"
	value := `{"ok":true,"message":"healthz:probe"}`
	if err := v.client.Set(ctx, key, value, time.Second).Err(); err != nil {
		return err
	}
	defer v.client.Del(context.Background(), key)
	got, err := v.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	if got != value {
		return fmt.Errorf("configured cache probe value mismatch")
	}
	return v.client.Del(ctx, key).Err()
}

// Compile-time interface check
var _ SearchResultCache = (*SearchCache)(nil)
var _ SearchResultCache = (*ValkeyCache)(nil)

// WarmCacheData holds pre-computed data for cache warming per AI.md PART 9
type WarmCacheData struct {
	// BlocklistHashes contains blocklist IP/domain hashes for fast lookup
	BlocklistHashes []string
	// GeoIPInfo contains GeoIP database metadata (version, update date)
	GeoIPInfo map[string]string
	// EngineStatus contains engine availability status
	EngineStatus map[string]bool
}

// WarmableCache extends SearchResultCache with warming capability
type WarmableCache interface {
	SearchResultCache
	// Warm pre-populates cache with frequently accessed data
	Warm(ctx context.Context, data WarmCacheData) error
}

// Warm pre-populates the in-memory cache with startup data per AI.md PART 9
// This reduces cold-start latency for frequently accessed data
func (c *SearchCache) Warm(ctx context.Context, data WarmCacheData) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Cache is for search results only — warming is a no-op for SearchCache
	// GeoIP, blocklist, and config are handled by their respective services
	// This method exists for interface compliance and future extensibility
	return nil
}

// Warm pre-populates the Valkey/Redis cache with startup data per AI.md PART 9
func (v *ValkeyCache) Warm(ctx context.Context, data WarmCacheData) error {
	v.mu.RLock()
	if v.closed {
		v.mu.RUnlock()
		return nil
	}
	v.mu.RUnlock()

	pipe := v.client.Pipeline()

	// Store GeoIP metadata with 7-day TTL
	if len(data.GeoIPInfo) > 0 {
		key := v.prefix + "geoip:info"
		jsonData, _ := json.Marshal(data.GeoIPInfo)
		pipe.Set(ctx, key, jsonData, 7*24*time.Hour)
	}

	// Store engine status with 5-minute TTL
	if len(data.EngineStatus) > 0 {
		key := v.prefix + "engines:status"
		jsonData, _ := json.Marshal(data.EngineStatus)
		pipe.Set(ctx, key, jsonData, 5*time.Minute)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	return err
}
