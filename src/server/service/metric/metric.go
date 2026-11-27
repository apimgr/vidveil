// SPDX-License-Identifier: MIT
// AI.md PART 20: Prometheus Metrics
package metric

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/apimgr/vidveil/src/common/authpad"
	"github.com/apimgr/vidveil/src/common/dbstore"
	"github.com/apimgr/vidveil/src/common/outsan"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/logging"
	gocpu "github.com/shirou/gopsutil/v4/cpu"
	godisk "github.com/shirou/gopsutil/v4/disk"
	gomem "github.com/shirou/gopsutil/v4/mem"
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

func ConfigureHTTPBuckets(duration, size []float64) error {
	if err := validateBuckets(duration); err != nil {
		return fmt.Errorf("invalid HTTP duration buckets: %w", err)
	}
	if err := validateBuckets(size); err != nil {
		return fmt.Errorf("invalid HTTP size buckets: %w", err)
	}
	registry := prometheus.DefaultRegisterer
	registry.Unregister(HTTPRequestDuration)
	registry.Unregister(HTTPRequestSize)
	registry.Unregister(HTTPResponseSize)
	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "vidveil_http_request_duration_seconds", Help: "HTTP request duration in seconds", Buckets: append([]float64(nil), duration...),
	}, []string{"method", "path"})
	HTTPRequestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "vidveil_http_request_size_bytes", Help: "HTTP request size in bytes", Buckets: append([]float64(nil), size...),
	}, []string{"method", "path"})
	HTTPResponseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "vidveil_http_response_size_bytes", Help: "HTTP response size in bytes", Buckets: append([]float64(nil), size...),
	}, []string{"method", "path"})
	for _, collector := range []prometheus.Collector{HTTPRequestDuration, HTTPRequestSize, HTTPResponseSize} {
		if err := registry.Register(collector); err != nil {
			return fmt.Errorf("register configured HTTP histogram: %w", err)
		}
	}
	return nil
}

func validateBuckets(buckets []float64) error {
	if len(buckets) == 0 {
		return errors.New("at least one bucket is required")
	}
	for i, bucket := range buckets {
		if bucket <= 0 || math.IsNaN(bucket) || math.IsInf(bucket, 0) || i > 0 && bucket <= buckets[i-1] {
			return fmt.Errorf("buckets must be finite, positive, and strictly increasing")
		}
	}
	return nil
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

	GoGoroutines          = promauto.NewGauge(prometheus.GaugeOpts{Name: "vidveil_go_goroutines", Help: "Current number of goroutines"})
	GoMemAllocBytes       = promauto.NewGauge(prometheus.GaugeOpts{Name: "vidveil_go_mem_alloc_bytes", Help: "Bytes allocated and in use on the Go heap"})
	GoMemSysBytes         = promauto.NewGauge(prometheus.GaugeOpts{Name: "vidveil_go_mem_sys_bytes", Help: "Bytes obtained from the system by the Go runtime"})
	GoGCRunsTotal         = promauto.NewCounter(prometheus.CounterOpts{Name: "vidveil_go_gc_runs_total", Help: "Total number of completed Go garbage collections"})
	GoGCPauseTotalSeconds = promauto.NewCounter(prometheus.CounterOpts{Name: "vidveil_go_gc_pause_total_seconds", Help: "Total time spent in Go garbage collection pauses"})

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

	DBBusyTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vidveil_db_busy_total",
		Help: "SQLite writes that hit a busy or locked error and were retried",
	})

	DBWriteWaitSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "vidveil_db_write_wait_seconds",
		Help:    "Time SQLite writes wait for a writer connection or file lock",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	// Authentication metrics per AI.md PART 20
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vidveil_auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"method", "status"},
	)

	OwnerTokensActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vidveil_owner_tokens_active",
			Help: "Number of active owner tokens by kind",
		},
		[]string{"kind"},
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
			Name:    "vidveil_search_results",
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

	ContactBlockedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vidveil_contact_blocked_total",
		Help: "Contact form submissions blocked by antispam reason",
	}, []string{"reason"})

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

func init() {
	OwnerTokensActive.WithLabelValues("primary").Set(0)
	OwnerTokensActive.WithLabelValues("linked").Set(0)
}

// RefreshOwnerTokenMetrics publishes counts of unrevoked, unexpired primary
// and linked owner tokens. Both series exist even before the first token.
type metricsReadStore interface {
	Read(context.Context, func(context.Context, dbstore.Reader) error) error
}

func RefreshOwnerTokenMetrics(db metricsReadStore) error {
	if db == nil {
		return nil
	}
	counts := map[string]float64{"primary": 0, "linked": 0}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.Read(ctx, func(readCtx context.Context, readDB dbstore.Reader) error {
		rows, err := readDB.QueryContext(readCtx, `SELECT kind, COUNT(*) FROM api_tokens WHERE revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?) GROUP BY kind`, time.Now().Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var kind string
			var count int64
			if err := rows.Scan(&kind, &count); err != nil {
				return err
			}
			if _, expected := counts[kind]; expected {
				counts[kind] = float64(count)
			}
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	OwnerTokensActive.WithLabelValues("primary").Set(counts["primary"])
	OwnerTokensActive.WithLabelValues("linked").Set(counts["linked"])
	return nil
}

// InitMetricsAppInfo initialises application-level metric values and starts the uptime updater.
// Call once from main after build variables are resolved.
var initAppMetricsOnce sync.Once
var initSystemMetricsOnce sync.Once
var runtimeMetricsEnabled = true
var systemMetricsRegistered = true

func ConfigureRuntimeMetrics(enabled bool) {
	collectors := []prometheus.Collector{GoGoroutines, GoMemAllocBytes, GoMemSysBytes, GoGCRunsTotal, GoGCPauseTotalSeconds}
	if enabled == runtimeMetricsEnabled {
		return
	}
	for _, collector := range collectors {
		if enabled {
			if err := prometheus.DefaultRegisterer.Register(collector); err != nil {
				log.Printf("metrics: unable to enable Go runtime collector: %v", err)
			}
		} else {
			prometheus.DefaultRegisterer.Unregister(collector)
		}
	}
	runtimeMetricsEnabled = enabled
}

func InitMetricsAppInfo(ver, commit, buildDate, goVer string) {
	initAppMetricsOnce.Do(func() {
		AppInfo.WithLabelValues(ver, commit, buildDate, goVer).Set(1)
		start := time.Now()
		AppStartTimestamp.Set(float64(start.Unix()))
		var lastGC uint32
		var lastPause uint64
		updateRuntime := func() {
			if !runtimeMetricsEnabled {
				return
			}
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			GoGoroutines.Set(float64(runtime.NumGoroutine()))
			GoMemAllocBytes.Set(float64(stats.Alloc))
			GoMemSysBytes.Set(float64(stats.Sys))
			if stats.NumGC > lastGC {
				GoGCRunsTotal.Add(float64(stats.NumGC - lastGC))
				lastGC = stats.NumGC
			}
			if stats.PauseTotalNs > lastPause {
				GoGCPauseTotalSeconds.Add(float64(stats.PauseTotalNs-lastPause) / float64(time.Second))
				lastPause = stats.PauseTotalNs
			}
		}
		updateRuntime()
		go func() {
			t := time.NewTicker(5 * time.Second)
			defer t.Stop()
			for range t.C {
				AppUptimeSeconds.Set(time.Since(start).Seconds())
				updateRuntime()
			}
		}()
	})
}

// ConfigureSystemMetrics starts the platform-aware host collector when enabled.
func ConfigureSystemMetrics(enabled bool, dataPath string) {
	collectors := []prometheus.Collector{SystemCPUUsagePercent, SystemMemoryUsagePercent, SystemMemoryUsedBytes, SystemMemoryTotalBytes, SystemDiskUsagePercent, SystemDiskUsedBytes, SystemDiskTotalBytes}
	if !enabled {
		if systemMetricsRegistered {
			for _, collector := range collectors {
				prometheus.DefaultRegisterer.Unregister(collector)
			}
			systemMetricsRegistered = false
		}
		return
	}
	if !systemMetricsRegistered {
		for _, collector := range collectors {
			if err := prometheus.DefaultRegisterer.Register(collector); err != nil {
				log.Printf("metrics: unable to enable system collector: %v", err)
			}
		}
		systemMetricsRegistered = true
	}
	initSystemMetricsOnce.Do(func() {
		refresh := func() {
			if usage, err := gocpu.Percent(0, false); err == nil && len(usage) > 0 {
				SystemCPUUsagePercent.Set(usage[0])
			} else if err != nil {
				log.Printf("metrics: unable to read system CPU usage: %v", err)
			}
			if memory, err := gomem.VirtualMemory(); err == nil {
				SystemMemoryUsagePercent.Set(memory.UsedPercent)
				SystemMemoryUsedBytes.Set(float64(memory.Used))
				SystemMemoryTotalBytes.Set(float64(memory.Total))
			} else {
				log.Printf("metrics: unable to read system memory usage: %v", err)
			}
			if dataPath != "" {
				if disk, err := godisk.Usage(dataPath); err == nil {
					SystemDiskUsagePercent.WithLabelValues(dataPath).Set(disk.UsedPercent)
					SystemDiskUsedBytes.WithLabelValues(dataPath).Set(float64(disk.Used))
					SystemDiskTotalBytes.WithLabelValues(dataPath).Set(float64(disk.Total))
				} else {
					log.Printf("metrics: unable to read data filesystem usage: %v", err)
				}
			}
		}
		refresh()
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				refresh()
			}
		}()
	})
}

// boolGauge maps a boolean state onto the 1/0 gauge encoding AI.md PART 20
// specifies for the Tor metrics.
func boolGauge(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

// SetTorState refreshes the PART 20 Tor gauges. Called by the tor_health
// scheduled task and by the Tor lifecycle transitions in the server.
func SetTorState(enabled, running, circuitEstablished bool) {
	TorEnabled.Set(boolGauge(enabled))
	TorRunning.Set(boolGauge(running))
	TorCircuitEstablished.Set(boolGauge(circuitEstablished))
}

// RecordTorRequest counts one request received over the Tor hidden service
// listener (AI.md PART 20 vidveil_tor_requests_total).
func RecordTorRequest() {
	TorRequestsTotal.Inc()
}

// Handler returns the Prometheus text exposition HTTP handler for the
// registered default metrics registry (AI.md PART 20 "prometheus" service).
// Used directly by ServiceHandler and kept exported for callers that only
// ever want the raw Prometheus registry (e.g. tests).
func Handler() http.Handler {
	return promhttp.Handler()
}

// serviceFromRequest resolves the {service} path segment (last segment of the
// request path), defaulting to "prometheus" per AI.md PART 20 ("/server/metrics"
// and the root "/metrics" alias with no sub-path both serve prometheus).
func serviceFromRequest(r *http.Request) string {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) == 0 {
		return "prometheus"
	}
	last := segments[len(segments)-1]
	switch last {
	case "prometheus", "grafana", "loki":
		return last
	case "metrics":
		return "prometheus"
	default:
		return ""
	}
}

// ServiceHandler returns the AI.md PART 20 metrics HTTP handler, dispatching on
// the {service} path segment (prometheus/grafana/loki) with mandatory per-service
// bearer-token authentication (constant-time compare, header only). An empty
// token disables that service (403). logger supplies recent structured log
// entries for the loki service; nil is treated as an empty log stream.
// The SAME handler is mounted at every alias route — callers never redirect.
func ServiceHandler(appConfig *config.AppConfig, logger *logging.AppLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if appConfig == nil || !appConfig.Server.Metrics.Enabled {
			http.NotFound(w, r)
			return
		}
		cfg := appConfig.Server.Metrics

		service := serviceFromRequest(r)

		var token string
		switch service {
		case "prometheus":
			token = cfg.Auth.Tokens.Prometheus
		case "grafana":
			token = cfg.Auth.Tokens.Grafana
		case "loki":
			token = cfg.Auth.Tokens.Loki
		default:
			http.NotFound(w, r)
			return
		}

		if !cfg.Auth.AllowUnauthenticated {
			start := time.Now()
			response := authpad.WrapResponse(w, start)
			w = response
			defer response.Finish()
			deny := func() {
				authpad.Wait(start)
				w.WriteHeader(http.StatusForbidden)
			}
			// Empty token = that service disabled per AI.md PART 20.
			if token == "" {
				AuthAttemptsTotal.WithLabelValues("bearer_token", "disabled").Inc()
				deny()
				return
			}
			// Header only — query-string tokens are forbidden (they leak into
			// access logs and reverse-proxy logs).
			header := r.Header.Get("Authorization")
			expected := "Bearer " + token
			providedHash := sha256.Sum256([]byte(header))
			expectedHash := sha256.Sum256([]byte(expected))
			if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
				AuthAttemptsTotal.WithLabelValues("bearer_token", "failure").Inc()
				deny()
				return
			}
			AuthAttemptsTotal.WithLabelValues("bearer_token", "success").Inc()
		}

		switch service {
		case "prometheus":
			promhttp.Handler().ServeHTTP(w, r)
		case "grafana":
			serveGrafanaDashboard(w)
		case "loki":
			serveLokiStreams(w, cfg.Loki, logger)
		}
	}
}

// serveGrafanaDashboard writes a complete importable Grafana dashboard JSON
// definition covering every metric category, datasource left as a template
// variable, per AI.md PART 20.
func serveGrafanaDashboard(w http.ResponseWriter) {
	dashboard := buildGrafanaDashboard()
	writeSanitizedJSON(w, dashboard)
}

func writeSanitizedJSON(w http.ResponseWriter, data any) {
	output, err := json.MarshalIndent(outsan.Sanitize(data, config.IsProductionMode()), "", "  ")
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(append(output, '\n')); err != nil {
		log.Printf("metrics: failed to write JSON response: %v", err)
	}
}

// grafanaPanel is a minimal Grafana dashboard panel definition.
type grafanaPanel struct {
	ID         int               `json:"id"`
	Title      string            `json:"title"`
	Type       string            `json:"type"`
	Datasource map[string]string `json:"datasource"`
	Targets    []grafanaTarget   `json:"targets"`
	GridPos    grafanaGridPos    `json:"gridPos"`
}

type grafanaTarget struct {
	Expr         string `json:"expr"`
	LegendFormat string `json:"legendFormat"`
	RefID        string `json:"refId"`
}

type grafanaGridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

// buildGrafanaDashboard assembles the panel set, one per metrics category
// (App, HTTP, Database, Cache, Scheduler, System, Business) per AI.md PART 20.
func buildGrafanaDashboard() map[string]interface{} {
	type panelSpec struct {
		title string
		expr  string
	}
	specs := []panelSpec{
		{"Application Uptime", "vidveil_app_uptime_seconds"},
		{"HTTP Requests Total", "rate(vidveil_http_requests_total[5m])"},
		{"HTTP Request Duration", "histogram_quantile(0.95, rate(vidveil_http_request_duration_seconds_bucket[5m]))"},
		{"HTTP Response Bytes", "rate(vidveil_http_response_size_bytes_sum[5m])"},
		{"Active HTTP Requests", "vidveil_http_active_requests"},
		{"Database Queries Total", "rate(vidveil_db_queries_total[5m])"},
		{"Database Errors Total", "rate(vidveil_db_errors_total[5m])"},
		{"Database Connections Open", "vidveil_db_connections_open"},
		{"Database Connections In Use", "vidveil_db_connections_in_use"},
		{"Database Write Wait", "histogram_quantile(0.95, rate(vidveil_db_write_wait_seconds_bucket[5m]))"},
		{"Database Busy Retries", "rate(vidveil_db_busy_total[5m])"},
		{"Cache Hit Ratio", "rate(vidveil_cache_hits_total[5m]) / (rate(vidveil_cache_hits_total[5m]) + rate(vidveil_cache_misses_total[5m]))"},
		{"Cache Size", "vidveil_cache_size"},
		{"Cache Bytes", "vidveil_cache_bytes"},
		{"Cache Evictions", "rate(vidveil_cache_evictions_total[5m])"},
		{"Scheduler Tasks Total", "rate(vidveil_scheduler_tasks_total[5m])"},
		{"Scheduler Tasks Running", "vidveil_scheduler_tasks_running"},
		{"System CPU Usage", "vidveil_system_cpu_usage_percent"},
		{"System Memory Usage", "vidveil_system_memory_usage_percent"},
		{"System Disk Usage", "vidveil_system_disk_usage_percent"},
		{"Go Goroutines", "vidveil_go_goroutines"},
		{"Go Heap Allocation", "vidveil_go_mem_alloc_bytes"},
		{"Go GC Pause Time", "rate(vidveil_go_gc_pause_total_seconds[5m])"},
		{"Authentication Attempts", "rate(vidveil_auth_attempts_total[5m])"},
		{"Active Owner Tokens", "vidveil_owner_tokens_active"},
		{"Rate Limited Requests", "rate(vidveil_ratelimit_requests_total{status=\"limited\"}[5m])"},
		{"Tor State", "vidveil_tor_running"},
		{"Search Queries Total", "rate(vidveil_search_queries_total[5m])"},
		{"Search Duration", "histogram_quantile(0.95, rate(vidveil_search_duration_seconds_bucket[5m]))"},
		{"Search Results (p95)", "histogram_quantile(0.95, rate(vidveil_search_results_bucket[5m]))"},
	}

	panels := make([]grafanaPanel, 0, len(specs))
	for i, s := range specs {
		panels = append(panels, grafanaPanel{
			ID:         i + 1,
			Title:      s.title,
			Type:       "timeseries",
			Datasource: map[string]string{"type": "prometheus", "uid": "${DS_PROMETHEUS}"},
			Targets: []grafanaTarget{
				{Expr: s.expr, LegendFormat: s.title, RefID: "A"},
			},
			GridPos: grafanaGridPos{H: 8, W: 12, X: (i % 2) * 12, Y: (i / 2) * 8},
		})
	}

	return map[string]interface{}{
		"__inputs": []map[string]interface{}{
			{
				"name":        "DS_PROMETHEUS",
				"label":       "Prometheus",
				"description": "",
				"type":        "datasource",
				"pluginId":    "prometheus",
				"pluginName":  "Prometheus",
			},
		},
		"title":         "vidveil",
		"uid":           "vidveil-metrics",
		"schemaVersion": 39,
		"version":       1,
		"timezone":      "browser",
		"editable":      true,
		"panels":        panels,
		"annotations":   map[string]interface{}{"list": []interface{}{}},
		"refresh":       "30s",
		"time": map[string]string{
			"from": "now-6h",
			"to":   "now",
		},
	}
}

// serveLokiStreams writes recent structured log entries as Loki push-API JSON
// streams, bounded by loki.max_entries/max_age, per AI.md PART 20. Credentials
// are always redacted by the logging subsystem before entries are captured.
func serveLokiStreams(w http.ResponseWriter, lokiCfg config.MetricsLokiConfig, logger *logging.AppLogger) {
	if logger == nil {
		writeSanitizedJSON(w, map[string]interface{}{"streams": []interface{}{}})
		return
	}

	maxAge := time.Hour
	if parsed, err := time.ParseDuration(lokiCfg.MaxAge); err == nil && parsed > 0 {
		maxAge = parsed
	}

	entries := logger.RecentEntries(lokiCfg.MaxEntries, maxAge)

	// Group entries by their label set (level + output) into Loki streams.
	type streamKey struct {
		level  string
		output string
	}
	grouped := make(map[streamKey][][2]string)
	order := make([]streamKey, 0)
	for _, e := range entries {
		key := streamKey{level: e.Level, output: e.Output}
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
		}
		ns := strconv.FormatInt(e.Timestamp.UnixNano(), 10)
		grouped[key] = append(grouped[key], [2]string{ns, e.Message})
	}

	streams := make([]map[string]interface{}, 0, len(order))
	for _, key := range order {
		streams = append(streams, map[string]interface{}{
			"stream": map[string]string{
				"level":  key.level,
				"output": key.output,
				"app":    "vidveil",
			},
			"values": grouped[key],
		})
	}

	writeSanitizedJSON(w, map[string]interface{}{"streams": streams})
}

// statusWriter wraps http.ResponseWriter to capture the status code and bytes written.
type statusWriter struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

// WriteHeader intercepts the status code so we can record it.
func (sw *statusWriter) WriteHeader(code int) {
	if sw.wroteHeader {
		return
	}
	sw.status = code
	sw.wroteHeader = true
	sw.ResponseWriter.WriteHeader(code)
}

// Write counts bytes written so HTTPResponseSize can observe them.
func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.WriteHeader(http.StatusOK)
	}
	n, err := sw.ResponseWriter.Write(b)
	sw.written += int64(n)
	return n, err
}

func (sw *statusWriter) Flush() {
	if !sw.wroteHeader {
		sw.WriteHeader(http.StatusOK)
	}
	if flusher, ok := sw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := sw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (sw *statusWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := sw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (sw *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if !sw.wroteHeader {
		sw.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := sw.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(r)
		sw.written += n
		return n, err
	}
	n, err := io.Copy(struct{ io.Writer }{sw.ResponseWriter}, r)
	sw.written += n
	return n, err
}

func (sw *statusWriter) Unwrap() http.ResponseWriter { return sw.ResponseWriter }

type countedRequestBody struct {
	io.ReadCloser
	read int64
}

func (body *countedRequestBody) Read(p []byte) (int, error) {
	n, err := body.ReadCloser.Read(p)
	body.read += int64(n)
	return n, err
}

// InstrumentMiddleware records per-request labeled Prometheus metrics.
// It wraps each handler to observe latency, request/response sizes, and status.
func InstrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		HTTPActiveRequests.Inc()
		requestBody := &countedRequestBody{ReadCloser: r.Body}
		r.Body = requestBody
		completed := false
		defer func() {
			HTTPActiveRequests.Dec()
			if !completed && !sw.wroteHeader {
				sw.status = http.StatusInternalServerError
			}
			method := r.Method
			path := normalizePath(r.URL.Path)
			status := strconv.Itoa(sw.status)
			HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
			HTTPRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
			requestSize := requestBody.read
			if r.ContentLength > requestSize {
				requestSize = r.ContentLength
			}
			if requestSize > 0 {
				HTTPRequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
			}
			if sw.written > 0 {
				HTTPResponseSize.WithLabelValues(method, path).Observe(float64(sw.written))
			}
		}()
		next.ServeHTTP(sw, r)
		completed = true
	})
}
