// SPDX-License-Identifier: MIT
// AI.md PART 20: Prometheus Metrics
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

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

	// Cache metrics per AI.md PART 20
	CacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "vidveil_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	CacheMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "vidveil_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	CacheEvictions = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "vidveil_cache_evictions_total",
			Help: "Total number of cache evictions",
		},
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

	// Rate limit metrics per AI.md PART 20 (REQUIRED)
	RateLimitHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_rate_limit_hits_total",
			Help: "Total number of rate limit triggers",
		},
		[]string{"endpoint_class", "ip"},
	)

	RateLimitBlockedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_rate_limit_blocked_total",
			Help: "Total number of requests blocked by rate limiter",
		},
		[]string{"ip"},
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
		path := r.URL.Path
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
