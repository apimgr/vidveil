// SPDX-License-Identifier: MIT
package handler

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// ServerMetrics holds application metrics
type ServerMetrics struct {
	appConfig *config.AppConfig
	engineMgr *engine.EngineManager
	startTime time.Time

	// Counters
	requestsTotal    uint64
	searchesTotal    uint64
	searchErrors     uint64
	apiRequestsTotal uint64
}

// NewMetrics creates a new metrics collector
func NewMetrics(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *ServerMetrics {
	return &ServerMetrics{
		appConfig: appConfig,
		engineMgr: engineMgr,
		startTime: time.Now(),
	}
}

// IncrementRequests increments the total request counter
func (m *ServerMetrics) IncrementRequests() {
	atomic.AddUint64(&m.requestsTotal, 1)
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

		// Write metrics in Prometheus format
		fmt.Fprintf(w, "# HELP vidveil_info Application information\n")
		fmt.Fprintf(w, "# TYPE vidveil_info gauge\n")
		fmt.Fprintf(w, "vidveil_info{version=\"%s\",go_version=\"%s\"} 1\n", version.GetVersion(), runtime.Version())
		fmt.Fprintf(w, "\n")

		// Uptime
		fmt.Fprintf(w, "# HELP vidveil_uptime_seconds Time since application start\n")
		fmt.Fprintf(w, "# TYPE vidveil_uptime_seconds counter\n")
		fmt.Fprintf(w, "vidveil_uptime_seconds %.2f\n", time.Since(m.startTime).Seconds())
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

// MetricsMiddleware creates middleware that tracks request metrics
func (m *ServerMetrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.IncrementRequests()
		next.ServeHTTP(w, r)
	})
}
