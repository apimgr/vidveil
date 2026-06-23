// SPDX-License-Identifier: MIT
package handler

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
	// Register promauto metrics with the default Prometheus registry.
	_ "github.com/apimgr/vidveil/src/server/service/metrics"
)

// slidingWindowCounter tracks counts in a 24-hour sliding window using hourly buckets.
// Each bucket represents one hour of counts. The ring rotates hourly.
type slidingWindowCounter struct {
	mu sync.RWMutex
	// 24 hourly buckets
	buckets [24]uint64
	// Current bucket index (0-23)
	currentHour int
	// Last rotation timestamp
	lastRotate time.Time
}

// newSlidingWindowCounter creates a new 24h sliding window counter
func newSlidingWindowCounter() *slidingWindowCounter {
	return &slidingWindowCounter{
		lastRotate: time.Now().Truncate(time.Hour),
	}
}

// increment adds one to the current bucket, rotating if needed
func (s *slidingWindowCounter) increment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotateLocked()
	s.buckets[s.currentHour]++
}

// rotateLocked rotates buckets if an hour has passed. Must be called with lock held.
func (s *slidingWindowCounter) rotateLocked() {
	now := time.Now().Truncate(time.Hour)
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
	} else {
		// Rotate through the buckets, clearing expired ones
		for i := 0; i < hoursPassed; i++ {
			s.currentHour = (s.currentHour + 1) % 24
			s.buckets[s.currentHour] = 0
		}
	}
	s.lastRotate = now
}

// count returns the total count across all 24 buckets
func (s *slidingWindowCounter) count() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Rotate to ensure stale buckets are cleared before counting
	s.rotateLocked()

	var total uint64
	for _, b := range s.buckets {
		total += b
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
	// cacheHitsTotal tracks how many searches were served from cache
	cacheHitsTotal uint64
	// activeConnections tracks current active connections
	activeConnections int64

	// 24h sliding window counter per AI.md PART 13 (stats.requests_24h)
	requests24h *slidingWindowCounter
	// searches24h tracks searches in the last 24 hours
	searches24h *slidingWindowCounter
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
	if m.searches24h != nil {
		m.searches24h.increment()
	}
}

// IncrementCacheHits increments the cache hit counter
func (m *ServerMetrics) IncrementCacheHits() {
	atomic.AddUint64(&m.cacheHitsTotal, 1)
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

// GetCacheHitsTotal returns total cache hit count
func (m *ServerMetrics) GetCacheHitsTotal() uint64 {
	return atomic.LoadUint64(&m.cacheHitsTotal)
}

// GetSearches24h returns search count in the last 24 hours
func (m *ServerMetrics) GetSearches24h() uint64 {
	if m.searches24h != nil {
		return m.searches24h.count()
	}
	return 0
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

// isLoopbackRequest reports whether the request originates from localhost.
// Used to enforce internal-only access when no bearer token is configured.
func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// Handler returns the Prometheus metrics HTTP handler.
// Per AI.md PART 20: metrics are internal-only.
// When a token is configured, it is required for all requests.
// When no token is configured, access is restricted to loopback (127.x/::1).
// Responses are served via promhttp.Handler() from the default registry, which
// includes all promauto-registered vidveil_* metrics (PART 20).
func (m *ServerMetrics) Handler() http.HandlerFunc {
	promHandler := promhttp.Handler()
	return func(w http.ResponseWriter, r *http.Request) {
		if m.appConfig.Server.Metrics.Token != "" {
			// Token configured: require it from all clients
			header := r.Header.Get("Authorization")
			expected := "Bearer " + m.appConfig.Server.Metrics.Token
			// Constant-time comparison prevents token timing side-channels (PART 1, PART 11)
			if subtle.ConstantTimeCompare([]byte(header), []byte(expected)) != 1 {
				query := r.URL.Query().Get("token")
				if subtle.ConstantTimeCompare([]byte(query), []byte(m.appConfig.Server.Metrics.Token)) != 1 {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
		} else {
			// No token: restrict to loopback only (internal-only per PART 14/20)
			if !isLoopbackRequest(r) {
				http.Error(w, "Forbidden: metrics are internal-only", http.StatusForbidden)
				return
			}
		}
		promHandler.ServeHTTP(w, r)
	}
}

// MetricsMiddleware creates middleware that tracks request metrics per AI.md PART 13
func (m *ServerMetrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.IncrementRequests()
		m.IncrementActiveConnections()
		defer m.DecrementActiveConnections()
		next.ServeHTTP(w, r)
	})
}
