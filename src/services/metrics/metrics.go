// Package metrics provides Prometheus-compatible metrics per TEMPLATE.md PART 11
package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apimgr/vidveil/config"
)

// Manager handles Prometheus-compatible metrics collection
type Manager struct {
	cfg     *config.Config
	mu      sync.RWMutex
	enabled bool

	// Application metrics
	requestCount     atomic.Uint64
	errorCount       atomic.Uint64
	searchCount      atomic.Uint64
	latencySum       atomic.Uint64
	latencyCount     atomic.Uint64
	activeRequests   atomic.Int32
	startTime        time.Time

	// Engine metrics
	engineRequests map[string]*atomic.Uint64
	engineErrors   map[string]*atomic.Uint64
	engineMu       sync.RWMutex
}

// New creates a new metrics manager
func New(cfg *config.Config) *Manager {
	return &Manager{
		cfg:            cfg,
		enabled:        cfg.Server.Metrics.Enabled,
		startTime:      time.Now(),
		engineRequests: make(map[string]*atomic.Uint64),
		engineErrors:   make(map[string]*atomic.Uint64),
	}
}

// IsEnabled returns whether metrics collection is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Enable enables metrics collection
func (m *Manager) Enable() {
	m.mu.Lock()
	m.enabled = true
	m.mu.Unlock()
}

// Disable disables metrics collection
func (m *Manager) Disable() {
	m.mu.Lock()
	m.enabled = false
	m.mu.Unlock()
}

// RecordRequest increments the request counter
func (m *Manager) RecordRequest() {
	m.requestCount.Add(1)
}

// RecordError increments the error counter
func (m *Manager) RecordError() {
	m.errorCount.Add(1)
}

// RecordSearch increments the search counter
func (m *Manager) RecordSearch() {
	m.searchCount.Add(1)
}

// RecordLatency records a request latency in milliseconds
func (m *Manager) RecordLatency(ms int64) {
	m.latencySum.Add(uint64(ms))
	m.latencyCount.Add(1)
}

// StartRequest increments active request count
func (m *Manager) StartRequest() {
	m.activeRequests.Add(1)
}

// EndRequest decrements active request count
func (m *Manager) EndRequest() {
	m.activeRequests.Add(-1)
}

// RecordEngineRequest records a request to a specific engine
func (m *Manager) RecordEngineRequest(engine string) {
	m.engineMu.Lock()
	if m.engineRequests[engine] == nil {
		m.engineRequests[engine] = &atomic.Uint64{}
	}
	m.engineMu.Unlock()

	m.engineMu.RLock()
	m.engineRequests[engine].Add(1)
	m.engineMu.RUnlock()
}

// RecordEngineError records an error for a specific engine
func (m *Manager) RecordEngineError(engine string) {
	m.engineMu.Lock()
	if m.engineErrors[engine] == nil {
		m.engineErrors[engine] = &atomic.Uint64{}
	}
	m.engineMu.Unlock()

	m.engineMu.RLock()
	m.engineErrors[engine].Add(1)
	m.engineMu.RUnlock()
}

// Handler returns an HTTP handler that serves Prometheus-formatted metrics
func (m *Manager) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check authentication if token is set
		if m.cfg.Server.Metrics.Token != "" {
			authHeader := r.Header.Get("Authorization")
			expected := "Bearer " + m.cfg.Server.Metrics.Token
			if authHeader != expected {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.writeMetrics(w)
	}
}

// writeMetrics writes Prometheus-formatted metrics
func (m *Manager) writeMetrics(w http.ResponseWriter) {
	// Application info
	fmt.Fprintf(w, "# HELP vidveil_info Application information\n")
	fmt.Fprintf(w, "# TYPE vidveil_info gauge\n")
	fmt.Fprintf(w, "vidveil_info{version=\"%s\"} 1\n\n", config.Version)

	// Uptime
	uptime := time.Since(m.startTime).Seconds()
	fmt.Fprintf(w, "# HELP vidveil_uptime_seconds Time since application started\n")
	fmt.Fprintf(w, "# TYPE vidveil_uptime_seconds counter\n")
	fmt.Fprintf(w, "vidveil_uptime_seconds %.2f\n\n", uptime)

	// Request metrics
	fmt.Fprintf(w, "# HELP vidveil_requests_total Total number of HTTP requests\n")
	fmt.Fprintf(w, "# TYPE vidveil_requests_total counter\n")
	fmt.Fprintf(w, "vidveil_requests_total %d\n\n", m.requestCount.Load())

	fmt.Fprintf(w, "# HELP vidveil_errors_total Total number of errors\n")
	fmt.Fprintf(w, "# TYPE vidveil_errors_total counter\n")
	fmt.Fprintf(w, "vidveil_errors_total %d\n\n", m.errorCount.Load())

	fmt.Fprintf(w, "# HELP vidveil_searches_total Total number of searches\n")
	fmt.Fprintf(w, "# TYPE vidveil_searches_total counter\n")
	fmt.Fprintf(w, "vidveil_searches_total %d\n\n", m.searchCount.Load())

	// Latency
	latencyCount := m.latencyCount.Load()
	if latencyCount > 0 {
		avgLatency := float64(m.latencySum.Load()) / float64(latencyCount)
		fmt.Fprintf(w, "# HELP vidveil_request_latency_avg_ms Average request latency in milliseconds\n")
		fmt.Fprintf(w, "# TYPE vidveil_request_latency_avg_ms gauge\n")
		fmt.Fprintf(w, "vidveil_request_latency_avg_ms %.2f\n\n", avgLatency)
	}

	// Active requests
	fmt.Fprintf(w, "# HELP vidveil_active_requests Current number of active requests\n")
	fmt.Fprintf(w, "# TYPE vidveil_active_requests gauge\n")
	fmt.Fprintf(w, "vidveil_active_requests %d\n\n", m.activeRequests.Load())

	// Engine metrics
	m.engineMu.RLock()
	if len(m.engineRequests) > 0 {
		fmt.Fprintf(w, "# HELP vidveil_engine_requests_total Requests per search engine\n")
		fmt.Fprintf(w, "# TYPE vidveil_engine_requests_total counter\n")
		for engine, count := range m.engineRequests {
			fmt.Fprintf(w, "vidveil_engine_requests_total{engine=\"%s\"} %d\n", engine, count.Load())
		}
		fmt.Fprintf(w, "\n")
	}
	if len(m.engineErrors) > 0 {
		fmt.Fprintf(w, "# HELP vidveil_engine_errors_total Errors per search engine\n")
		fmt.Fprintf(w, "# TYPE vidveil_engine_errors_total counter\n")
		for engine, count := range m.engineErrors {
			fmt.Fprintf(w, "vidveil_engine_errors_total{engine=\"%s\"} %d\n", engine, count.Load())
		}
		fmt.Fprintf(w, "\n")
	}
	m.engineMu.RUnlock()

	// System metrics (if enabled)
	if m.cfg.Server.Metrics.IncludeSystem {
		m.writeSystemMetrics(w)
	}
}

// writeSystemMetrics writes system resource metrics
func (m *Manager) writeSystemMetrics(w http.ResponseWriter) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Memory metrics
	fmt.Fprintf(w, "# HELP vidveil_memory_alloc_bytes Allocated heap memory in bytes\n")
	fmt.Fprintf(w, "# TYPE vidveil_memory_alloc_bytes gauge\n")
	fmt.Fprintf(w, "vidveil_memory_alloc_bytes %d\n\n", memStats.Alloc)

	fmt.Fprintf(w, "# HELP vidveil_memory_sys_bytes Total memory obtained from system\n")
	fmt.Fprintf(w, "# TYPE vidveil_memory_sys_bytes gauge\n")
	fmt.Fprintf(w, "vidveil_memory_sys_bytes %d\n\n", memStats.Sys)

	fmt.Fprintf(w, "# HELP vidveil_memory_heap_objects Number of allocated heap objects\n")
	fmt.Fprintf(w, "# TYPE vidveil_memory_heap_objects gauge\n")
	fmt.Fprintf(w, "vidveil_memory_heap_objects %d\n\n", memStats.HeapObjects)

	// GC metrics
	fmt.Fprintf(w, "# HELP vidveil_gc_runs_total Total number of GC runs\n")
	fmt.Fprintf(w, "# TYPE vidveil_gc_runs_total counter\n")
	fmt.Fprintf(w, "vidveil_gc_runs_total %d\n\n", memStats.NumGC)

	// Goroutines
	fmt.Fprintf(w, "# HELP vidveil_goroutines Current number of goroutines\n")
	fmt.Fprintf(w, "# TYPE vidveil_goroutines gauge\n")
	fmt.Fprintf(w, "vidveil_goroutines %d\n\n", runtime.NumGoroutine())

	// CPU
	fmt.Fprintf(w, "# HELP vidveil_cpu_count Number of CPUs available\n")
	fmt.Fprintf(w, "# TYPE vidveil_cpu_count gauge\n")
	fmt.Fprintf(w, "vidveil_cpu_count %d\n", runtime.NumCPU())
}

// GetStats returns current metrics as a map (for JSON API)
func (m *Manager) GetStats() map[string]interface{} {
	latencyCount := m.latencyCount.Load()
	avgLatency := float64(0)
	if latencyCount > 0 {
		avgLatency = float64(m.latencySum.Load()) / float64(latencyCount)
	}

	stats := map[string]interface{}{
		"requests_total":     m.requestCount.Load(),
		"errors_total":       m.errorCount.Load(),
		"searches_total":     m.searchCount.Load(),
		"active_requests":    m.activeRequests.Load(),
		"avg_latency_ms":     avgLatency,
		"uptime_seconds":     time.Since(m.startTime).Seconds(),
	}

	if m.cfg.Server.Metrics.IncludeSystem {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		stats["memory_alloc_bytes"] = memStats.Alloc
		stats["memory_sys_bytes"] = memStats.Sys
		stats["goroutines"] = runtime.NumGoroutine()
		stats["gc_runs"] = memStats.NumGC
	}

	return stats
}
