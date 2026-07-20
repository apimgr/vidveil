// SPDX-License-Identifier: MIT
// AI.md PART 20: Prometheus Metrics
package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// uuidPathSegment matches a UUID path segment (any version).
var uuidPathSegment = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// numericPathSegment matches a purely numeric path segment (resource IDs).
var numericPathSegment = regexp.MustCompile(`^\d+$`)

// longHexPathSegment matches long hex tokens (SHA hashes, opaque IDs).
var longHexPathSegment = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)

// normalizePath replaces high-cardinality path segments (UUIDs, numeric IDs,
// long hex tokens) with ":id" so Prometheus label cardinality stays bounded
// per AI.md PART 20 (path label must be normalized, never raw r.URL.Path).
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		if uuidPathSegment.MatchString(seg) || numericPathSegment.MatchString(seg) || longHexPathSegment.MatchString(seg) {
			segments[i] = ":id"
		}
	}
	return strings.Join(segments, "/")
}

var (
	// Application metrics per AI.md PART 20
	AppInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_app_info",
			Help: "Application info (always 1, labels carry data)",
		},
		[]string{"version", "commit", "build_date", "go_version"},
	)

	AppUptimeSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_app_uptime_seconds",
			Help: "Seconds since application start",
		},
	)

	AppStartTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_app_start_timestamp",
			Help: "Unix timestamp of application start",
		},
	)

	// HTTP metrics per AI.md PART 20
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vidveil_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	HTTPRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vidveil_http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "path"},
	)

	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vidveil_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "path"},
	)

	HTTPActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_http_active_requests",
			Help: "Number of active HTTP requests",
		},
	)

	// Database metrics per AI.md PART 20
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table"},
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vidveil_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"operation", "table"},
	)

	DBConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	DBConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	DBErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_db_errors_total",
			Help: "Total number of database errors",
		},
		[]string{"operation", "error_type"},
	)

	// Authentication metrics per AI.md PART 20
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"method", "status"},
	)

	AuthSessionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_auth_sessions_active",
			Help: "Number of active authentication sessions",
		},
	)

	// Cache metrics per AI.md PART 20 — label "cache" = cache name/purpose
	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache"},
	)

	CacheEvictions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_cache_evictions_total",
			Help: "Total number of cache evictions",
		},
		[]string{"cache"},
	)

	CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_cache_size",
			Help: "Current number of items in cache",
		},
		[]string{"cache"},
	)

	CacheBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_cache_bytes",
			Help: "Current cache size in bytes",
		},
		[]string{"cache"},
	)

	// Search metrics
	SearchQueriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "vidveil_search_queries_total",
			Help: "Total number of search queries",
		},
	)

	SearchResultsTotal = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vidveil_search_results_total",
			Help:    "Number of results per search",
			Buckets: []float64{0, 1, 5, 10, 20, 50, 100, 200, 500},
		},
	)

	SearchDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vidveil_search_duration_seconds",
			Help:    "Search duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 20, 30},
		},
	)

	// Engine metrics
	EngineRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_engine_requests_total",
			Help: "Total number of engine requests",
		},
		[]string{"engine"},
	)

	EngineErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_engine_errors_total",
			Help: "Total number of engine errors",
		},
		[]string{"engine"},
	)

	EngineResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vidveil_engine_response_time_seconds",
			Help:    "Engine response time in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 20, 30},
		},
		[]string{"engine"},
	)

	// Rate limiting metrics per AI.md PART 20.
	// label "limit"  = global | per_ip | per_user | per_endpoint
	// label "status" = allowed | limited
	// NEVER use raw client IP as a label — unbounded cardinality; log IPs to structured logs instead.
	RateLimitRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_ratelimit_requests_total",
			Help: "Total rate-limited requests by limit type and outcome",
		},
		[]string{"limit", "status"},
	)

	RateLimitBlockedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_ratelimit_blocked_total",
			Help: "Requests blocked by rate limiter",
		},
		[]string{"limit"},
	)

	// Scheduler metrics per AI.md PART 20 (required when using PART 18 scheduler)
	SchedulerTasksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_scheduler_tasks_total",
			Help: "Total scheduled task executions",
		},
		[]string{"task", "status"},
	)

	SchedulerTaskDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vidveil_scheduler_task_duration_seconds",
			Help:    "Scheduled task execution duration",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300, 600},
		},
		[]string{"task"},
	)

	SchedulerTasksRunning = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_scheduler_tasks_running",
			Help: "Currently running task instances",
		},
		[]string{"task"},
	)

	SchedulerLastRunTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_scheduler_last_run_timestamp",
			Help: "Unix timestamp of last task execution",
		},
		[]string{"task"},
	)

	// System metrics per AI.md PART 20 (enabled when include_system: true in config)
	SystemCPUUsagePercent = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_system_cpu_usage_percent",
			Help: "Current CPU usage percentage (0-100)",
		},
	)

	SystemMemoryUsagePercent = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_system_memory_usage_percent",
			Help: "Current memory usage percentage (0-100)",
		},
	)

	SystemMemoryUsedBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_system_memory_used_bytes",
			Help: "Memory currently in use",
		},
	)

	SystemMemoryTotalBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_system_memory_total_bytes",
			Help: "Total system memory",
		},
	)

	SystemDiskUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_system_disk_usage_percent",
			Help: "Disk usage percentage for a given path",
		},
		[]string{"path"},
	)

	SystemDiskUsedBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_system_disk_used_bytes",
			Help: "Disk space used",
		},
		[]string{"path"},
	)

	SystemDiskTotalBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_system_disk_total_bytes",
			Help: "Total disk space",
		},
		[]string{"path"},
	)

	// Tor metrics per AI.md PART 20 (enabled when Tor hidden service is active)
	TorEnabled = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_tor_enabled",
			Help: "1 if Tor is enabled, 0 otherwise",
		},
	)

	TorRunning = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_tor_running",
			Help: "1 if Tor process is running, 0 otherwise",
		},
	)

	TorCircuitEstablished = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "vidveil_tor_circuit_established",
			Help: "1 if a Tor circuit is established, 0 otherwise",
		},
	)

	TorRequestsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "vidveil_tor_requests_total",
			Help: "Total requests received via Tor hidden service",
		},
	)
)

// InitMetricsAppInfo initialises application-level metric values and starts the uptime updater.
// Call once from main after build variables are resolved.
func InitMetricsAppInfo(ver, commit, buildDate, goVer string) {
	AppInfo.WithLabelValues(ver, commit, buildDate, goVer).Set(1)
	start := time.Now()
	AppStartTimestamp.Set(float64(start.Unix()))
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			AppUptimeSeconds.Set(time.Since(start).Seconds())
		}
	}()
}

// statusWriter wraps http.ResponseWriter to capture the status code and bytes written.
type statusWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

// WriteHeader intercepts the status code so we can record it.
func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// Write counts bytes written so HTTPResponseSize can observe them.
func (sw *statusWriter) Write(b []byte) (int, error) {
	n, err := sw.ResponseWriter.Write(b)
	sw.written += int64(n)
	return n, err
}

// InstrumentMiddleware records per-request labeled Prometheus metrics.
// It wraps each handler to observe latency, request/response sizes, and status.
func InstrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		HTTPActiveRequests.Inc()
		next.ServeHTTP(sw, r)
		HTTPActiveRequests.Dec()

		method := r.Method
		// Normalize the path so IDs/UUIDs collapse to :id — prevents unbounded
		// label cardinality per AI.md PART 20.
		path := normalizePath(r.URL.Path)
		status := strconv.Itoa(sw.status)

		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())

		if r.ContentLength > 0 {
			HTTPRequestSize.WithLabelValues(method, path).Observe(float64(r.ContentLength))
		}
		if sw.written > 0 {
			HTTPResponseSize.WithLabelValues(method, path).Observe(float64(sw.written))
		}
	})
}
