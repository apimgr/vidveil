// SPDX-License-Identifier: MIT
// AI.md PART 21: Prometheus Metrics
package metrics

import (
"github.com/prometheus/client_golang/prometheus"
"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
// HTTP metrics per AI.md PART 21
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

// Database metrics per AI.md PART 21
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

// Cache metrics per AI.md PART 21
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
)
