// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/engine"
	// Register promauto metrics with the default Prometheus registry.
	prommetric "github.com/apimgr/vidveil/src/server/service/metric"
)

// slidingWindowCounter tracks counts in a 24-hour sliding window using hourly buckets.
// Each bucket represents one hour of counts. The ring rotates hourly.
type slidingWindowCounter struct {
	mu sync.RWMutex
	// 24 hourly buckets
	buckets     [24]uint64
	bucketHours [24]int64
	// Current bucket index (0-23)
	currentHour int
	// Last rotation timestamp
	lastRotate time.Time
}

// newSlidingWindowCounter creates a new 24h sliding window counter
func newSlidingWindowCounter() *slidingWindowCounter {
	now := time.Now().UTC().Truncate(time.Hour)
	hour := now.Unix() / 3600
	return &slidingWindowCounter{
		lastRotate:  now,
		currentHour: int(hour % 24),
		bucketHours: [24]int64{hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour, hour},
	}
}

// increment adds one to the current bucket, rotating if needed
func (s *slidingWindowCounter) increment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotateLocked()
	hour := s.lastRotate.Unix() / 3600
	if s.bucketHours[s.currentHour] != hour {
		s.buckets[s.currentHour] = 0
		s.bucketHours[s.currentHour] = hour
	}
	s.buckets[s.currentHour]++
}

// rotateLocked rotates buckets if an hour has passed. Must be called with lock held.
func (s *slidingWindowCounter) rotateLocked() {
	now := time.Now().UTC().Truncate(time.Hour)
	hoursPassed := int(now.Sub(s.lastRotate).Hours())

	if hoursPassed <= 0 {
		return
	}

	// Clear buckets that have expired (up to 24 hours worth)
	if hoursPassed >= 24 {
		// All buckets are stale, clear everything
		for i := range s.buckets {
			s.buckets[i] = 0
		}
		s.currentHour = int(now.Hour()) % 24
		hour := now.Unix() / 3600
		for i := range s.bucketHours {
			s.bucketHours[i] = hour
		}
	} else {
		// Rotate through the buckets, clearing expired ones
		previousHour := s.lastRotate.Unix() / 3600
		for i := 0; i < hoursPassed; i++ {
			s.currentHour = (s.currentHour + 1) % 24
			s.buckets[s.currentHour] = 0
			s.bucketHours[s.currentHour] = previousHour + int64(i+1)
		}
	}
	s.lastRotate = now
}

func (s *slidingWindowCounter) snapshot() map[int64]uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotateLocked()
	current := s.lastRotate.Unix() / 3600
	result := make(map[int64]uint64, 24)
	for index, count := range s.buckets {
		hour := s.bucketHours[index]
		if count > 0 && hour <= current && current-hour < 24 {
			result[hour] = count
		}
	}
	return result
}

func (s *slidingWindowCounter) restore(hourly map[int64]uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Truncate(time.Hour)
	current := now.Unix() / 3600
	s.lastRotate = now
	s.currentHour = int(current % 24)
	for i := range s.buckets {
		s.buckets[i], s.bucketHours[i] = 0, current-int64((s.currentHour-i+24)%24)
	}
	for hour, count := range hourly {
		if hour <= current && current-hour < 24 {
			index := int(hour % 24)
			s.bucketHours[index], s.buckets[index] = hour, count
		}
	}
}

// count returns the total count across all 24 buckets
func (s *slidingWindowCounter) count() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Rotate to ensure stale buckets are cleared before counting
	s.rotateLocked()

	var total uint64
	current := s.lastRotate.Unix() / 3600
	for index, b := range s.buckets {
		hour := s.bucketHours[index]
		if hour <= current && current-hour < 24 {
			total += b
		}
	}
	return total
}

// ServerMetrics holds application metrics per AI.md PART 13
type ServerMetrics struct {
	appConfig *config.AppConfig
	engineMgr *engine.EngineManager
	startTime time.Time

	// Counters per AI.md PART 13
	requestsTotal    uint64
	searchesTotal    uint64
	searchErrors     uint64
	apiRequestsTotal uint64
	// cacheHitsTotal tracks how many searches were served from the cache
	cacheHitsTotal uint64
	// activeConnections tracks current active connections
	activeConnections int64

	// 24h sliding window counter per AI.md PART 13 (stats.requests_24h)
	requests24h *slidingWindowCounter
	// searches24h tracks searches in the last 24 hours
	searches24h *slidingWindowCounter
	persistDB   *database.AppDatabase
	persistStop chan struct{}
	persistDone chan struct{}
	persistOnce sync.Once
}

// SetPersistence restores durable request aggregates and flushes them every
// 30 seconds while the server is running.
func (m *ServerMetrics) SetPersistence(db *database.AppDatabase) error {
	if db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	snapshot, err := db.LoadRequestStats(ctx)
	cancel()
	if err != nil {
		return err
	}
	m.persistDB = db
	atomic.StoreUint64(&m.requestsTotal, snapshot.Total)
	if m.requests24h != nil {
		m.requests24h.restore(snapshot.Hourly)
	}
	m.persistStop = make(chan struct{})
	m.persistDone = make(chan struct{})
	go m.persistLoop()
	return nil
}

func (m *ServerMetrics) persistLoop() {
	defer close(m.persistDone)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.persistStop:
			return
		case <-ticker.C:
			if err := m.flushRequestStats(); err != nil {
				log.Printf("health: failed to persist request statistics: %v", err)
			}
		}
	}
}

func (m *ServerMetrics) flushRequestStats() error {
	if m == nil || m.persistDB == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hourly := make(map[int64]uint64)
	if m.requests24h != nil {
		hourly = m.requests24h.snapshot()
	}
	return m.persistDB.SaveRequestStats(ctx, database.RequestStatsSnapshot{Total: atomic.LoadUint64(&m.requestsTotal), Hourly: hourly})
}

// Close stops periodic persistence and performs a final bounded flush.
func (m *ServerMetrics) Close() error {
	if m == nil || m.persistDB == nil {
		return nil
	}
	m.persistOnce.Do(func() { close(m.persistStop) })
	<-m.persistDone
	return m.flushRequestStats()
}

// NewMetrics creates a new metrics collector
func NewMetrics(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *ServerMetrics {
	return &ServerMetrics{
		appConfig:   appConfig,
		engineMgr:   engineMgr,
		startTime:   time.Now(),
		requests24h: newSlidingWindowCounter(),
		searches24h: newSlidingWindowCounter(),
	}
}

// IncrementRequests increments the total request counter and 24h window counter
func (m *ServerMetrics) IncrementRequests() {
	atomic.AddUint64(&m.requestsTotal, 1)
	if m.requests24h != nil {
		m.requests24h.increment()
	}
}

// IncrementSearches increments the search counter
func (m *ServerMetrics) IncrementSearches() {
	atomic.AddUint64(&m.searchesTotal, 1)
	prommetric.SearchQueriesTotal.Inc()
	if m.searches24h != nil {
		m.searches24h.increment()
	}
}

// IncrementCacheHits increments the cache hit counter
func (m *ServerMetrics) IncrementCacheHits() {
	atomic.AddUint64(&m.cacheHitsTotal, 1)
}

// RecordSearchResult updates the project-specific duration and result metrics.
func (m *ServerMetrics) RecordSearchResult(duration time.Duration, results int) {
	prommetric.SearchDuration.Observe(duration.Seconds())
	prommetric.SearchResultsTotal.Observe(float64(results))
}

// IncrementSearchErrors increments the search error counter
func (m *ServerMetrics) IncrementSearchErrors() {
	atomic.AddUint64(&m.searchErrors, 1)
}

// IncrementAPIRequests increments the API request counter
func (m *ServerMetrics) IncrementAPIRequests() {
	atomic.AddUint64(&m.apiRequestsTotal, 1)
}

// GetRequestsTotal returns total request count
func (m *ServerMetrics) GetRequestsTotal() uint64 {
	return atomic.LoadUint64(&m.requestsTotal)
}

// GetSearchesTotal returns total search count
func (m *ServerMetrics) GetSearchesTotal() uint64 {
	return atomic.LoadUint64(&m.searchesTotal)
}

// GetSearchErrors returns total search error count
func (m *ServerMetrics) GetSearchErrors() uint64 {
	return atomic.LoadUint64(&m.searchErrors)
}

// GetAPIRequestsTotal returns total API request count
func (m *ServerMetrics) GetAPIRequestsTotal() uint64 {
	return atomic.LoadUint64(&m.apiRequestsTotal)
}

// GetRequests24h returns request count in the last 24 hours per AI.md PART 13
func (m *ServerMetrics) GetRequests24h() uint64 {
	if m.requests24h != nil {
		return m.requests24h.count()
	}
	return 0
}

// GetSearches24h returns search count in the last 24 hours
func (m *ServerMetrics) GetSearches24h() uint64 {
	if m.searches24h != nil {
		return m.searches24h.count()
	}
	return 0
}

// GetCacheHitsTotal returns the total cache hit count
func (m *ServerMetrics) GetCacheHitsTotal() uint64 {
	return atomic.LoadUint64(&m.cacheHitsTotal)
}

// AnalyticsSummary holds aggregated analytics for the admin dashboard
type AnalyticsSummary struct {
	SearchesTotal  uint64  `json:"searches_total"`
	Searches24h    uint64  `json:"searches_24h"`
	Requests24h    uint64  `json:"requests_24h"`
	CacheHitsTotal uint64  `json:"cache_hits_total"`
	CacheHitPct    float64 `json:"cache_hit_pct"`
	UptimeSeconds  float64 `json:"uptime_seconds"`
}

// GetAnalyticsSummary returns a privacy-safe analytics summary for the admin dashboard
func (m *ServerMetrics) GetAnalyticsSummary() AnalyticsSummary {
	total := atomic.LoadUint64(&m.searchesTotal)
	hits := atomic.LoadUint64(&m.cacheHitsTotal)
	hitPct := 0.0
	if total > 0 {
		hitPct = float64(hits) / float64(total) * 100
	}
	return AnalyticsSummary{
		SearchesTotal:  total,
		Searches24h:    m.GetSearches24h(),
		Requests24h:    m.GetRequests24h(),
		CacheHitsTotal: hits,
		CacheHitPct:    hitPct,
		UptimeSeconds:  time.Since(m.startTime).Seconds(),
	}
}

// IncrementActiveConnections increments active connections counter
func (m *ServerMetrics) IncrementActiveConnections() {
	atomic.AddInt64(&m.activeConnections, 1)
}

// DecrementActiveConnections decrements active connections counter
func (m *ServerMetrics) DecrementActiveConnections() {
	atomic.AddInt64(&m.activeConnections, -1)
}

// GetActiveConnections returns current active connections count
func (m *ServerMetrics) GetActiveConnections() int64 {
	return atomic.LoadInt64(&m.activeConnections)
}
