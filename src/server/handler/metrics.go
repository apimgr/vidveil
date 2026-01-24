// SPDX-License-Identifier: MIT
package handler

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// slidingWindowCounter tracks counts in a 24-hour sliding window using hourly buckets.
// Each bucket represents one hour of counts. The ring rotates hourly.
type slidingWindowCounter struct {
	mu          sync.RWMutex
	buckets     [24]uint64    // 24 hourly buckets
	currentHour int           // Current bucket index (0-23)
	lastRotate  time.Time     // Last rotation timestamp
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
	requestsTotal     uint64
	searchesTotal     uint64
	searchErrors      uint64
	apiRequestsTotal  uint64
	// activeConnections tracks current active connections
	activeConnections int64

	// 24h sliding window counter per AI.md PART 13 (stats.requests_24h)
	requests24h *slidingWindowCounter
}

// NewMetrics creates a new metrics collector
func NewMetrics(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *ServerMetrics {
	return &ServerMetrics{
		appConfig:   appConfig,
		engineMgr:   engineMgr,
		startTime:   time.Now(),
		requests24h: newSlidingWindowCounter(),
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

// Handler returns the Prometheus metrics handler
func (m *ServerMetrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for token if metrics require auth
		if m.appConfig.Server.Metrics.Token != "" {
			token := r.Header.Get("Authorization")
			if token != "Bearer "+m.appConfig.Server.Metrics.Token {
				token = r.URL.Query().Get("token")
				if token != m.appConfig.Server.Metrics.Token {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Runtime metrics
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// Write metrics in Prometheus format per AI.md PART 21

		// Application metrics per AI.md PART 21
		fmt.Fprintf(w, "# HELP vidveil_app_info Application information (always 1, labels carry data)\n")
		fmt.Fprintf(w, "# TYPE vidveil_app_info gauge\n")
		fmt.Fprintf(w, "vidveil_app_info{version=\"%s\",commit=\"%s\",build_date=\"%s\",go_version=\"%s\"} 1\n",
			version.GetVersion(), version.CommitID, version.BuildTime, runtime.Version())
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "# HELP vidveil_app_uptime_seconds Seconds since application start\n")
		fmt.Fprintf(w, "# TYPE vidveil_app_uptime_seconds gauge\n")
		fmt.Fprintf(w, "vidveil_app_uptime_seconds %.2f\n", time.Since(m.startTime).Seconds())
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "# HELP vidveil_app_start_timestamp Unix timestamp of application start\n")
		fmt.Fprintf(w, "# TYPE vidveil_app_start_timestamp gauge\n")
		fmt.Fprintf(w, "vidveil_app_start_timestamp %d\n", m.startTime.Unix())
		fmt.Fprintf(w, "\n")

		// Request counters
		fmt.Fprintf(w, "# HELP vidveil_requests_total Total number of HTTP requests\n")
		fmt.Fprintf(w, "# TYPE vidveil_requests_total counter\n")
		fmt.Fprintf(w, "vidveil_requests_total %d\n", atomic.LoadUint64(&m.requestsTotal))
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "# HELP vidveil_searches_total Total number of searches performed\n")
		fmt.Fprintf(w, "# TYPE vidveil_searches_total counter\n")
		fmt.Fprintf(w, "vidveil_searches_total %d\n", atomic.LoadUint64(&m.searchesTotal))
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "# HELP vidveil_search_errors_total Total number of search errors\n")
		fmt.Fprintf(w, "# TYPE vidveil_search_errors_total counter\n")
		fmt.Fprintf(w, "vidveil_search_errors_total %d\n", atomic.LoadUint64(&m.searchErrors))
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "# HELP vidveil_api_requests_total Total number of API requests\n")
		fmt.Fprintf(w, "# TYPE vidveil_api_requests_total counter\n")
		fmt.Fprintf(w, "vidveil_api_requests_total %d\n", atomic.LoadUint64(&m.apiRequestsTotal))
		fmt.Fprintf(w, "\n")

		// Engine metrics
		engineList := m.engineMgr.ListEngines()
		enabledCount := 0
		for _, eng := range engineList {
			if eng.Enabled {
				enabledCount++
			}
		}

		fmt.Fprintf(w, "# HELP vidveil_engines_total Total number of search engines\n")
		fmt.Fprintf(w, "# TYPE vidveil_engines_total gauge\n")
		fmt.Fprintf(w, "vidveil_engines_total %d\n", len(engineList))
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "# HELP vidveil_engines_enabled Number of enabled search engines\n")
		fmt.Fprintf(w, "# TYPE vidveil_engines_enabled gauge\n")
		fmt.Fprintf(w, "vidveil_engines_enabled %d\n", enabledCount)
		fmt.Fprintf(w, "\n")

		// Per-engine status
		fmt.Fprintf(w, "# HELP vidveil_engine_enabled Engine enabled status\n")
		fmt.Fprintf(w, "# TYPE vidveil_engine_enabled gauge\n")
		for _, eng := range engineList {
			enabled := 0
			if eng.Enabled {
				enabled = 1
			}
			fmt.Fprintf(w, "vidveil_engine_enabled{name=\"%s\",tier=\"%d\"} %d\n", eng.Name, eng.Tier, enabled)
		}
		fmt.Fprintf(w, "\n")

		// Memory metrics
		if m.appConfig.Server.Metrics.IncludeSystem {
			fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use\n")
			fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
			fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", memStats.Alloc)
			fmt.Fprintf(w, "\n")

			fmt.Fprintf(w, "# HELP go_memstats_sys_bytes Number of bytes obtained from system\n")
			fmt.Fprintf(w, "# TYPE go_memstats_sys_bytes gauge\n")
			fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", memStats.Sys)
			fmt.Fprintf(w, "\n")

			fmt.Fprintf(w, "# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and still in use\n")
			fmt.Fprintf(w, "# TYPE go_memstats_heap_alloc_bytes gauge\n")
			fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n", memStats.HeapAlloc)
			fmt.Fprintf(w, "\n")

			fmt.Fprintf(w, "# HELP go_memstats_heap_sys_bytes Number of heap bytes obtained from system\n")
			fmt.Fprintf(w, "# TYPE go_memstats_heap_sys_bytes gauge\n")
			fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", memStats.HeapSys)
			fmt.Fprintf(w, "\n")

			fmt.Fprintf(w, "# HELP go_memstats_gc_total_count Total number of GC runs\n")
			fmt.Fprintf(w, "# TYPE go_memstats_gc_total_count counter\n")
			fmt.Fprintf(w, "go_memstats_gc_total_count %d\n", memStats.NumGC)
			fmt.Fprintf(w, "\n")

			fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines currently running\n")
			fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
			fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())
			fmt.Fprintf(w, "\n")

			fmt.Fprintf(w, "# HELP go_threads Number of OS threads created\n")
			fmt.Fprintf(w, "# TYPE go_threads gauge\n")
			fmt.Fprintf(w, "go_threads %d\n", runtime.GOMAXPROCS(0))
			fmt.Fprintf(w, "\n")
		}
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
