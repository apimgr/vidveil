// SPDX-License-Identifier: MIT
package server

import (
	"expvar"
	"net/http"
	"net/http/pprof"
	"runtime"

	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/go-chi/chi/v5"
)

// registerDebugRoutes registers debug endpoints (--debug/DEBUG=true only)
func (s *Server) registerDebugRoutes(r chi.Router) {
	if !mode.IsDebugEnabled() {
		return
	}

	r.Route("/debug", func(r chi.Router) {
		// pprof endpoints
		r.HandleFunc("/pprof/", pprof.Index)
		r.HandleFunc("/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/pprof/profile", pprof.Profile)
		r.HandleFunc("/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/pprof/trace", pprof.Trace)
		r.Handle("/pprof/heap", pprof.Handler("heap"))
		r.Handle("/pprof/goroutine", pprof.Handler("goroutine"))
		r.Handle("/pprof/allocs", pprof.Handler("allocs"))
		r.Handle("/pprof/block", pprof.Handler("block"))
		r.Handle("/pprof/mutex", pprof.Handler("mutex"))
		r.Handle("/pprof/threadcreate", pprof.Handler("threadcreate"))

		// expvar
		r.Handle("/vars", expvar.Handler())

		// Custom debug endpoints
		r.Get("/config", s.handleDebugConfig)
		r.Get("/routes", s.handleDebugRoutes)
		r.Get("/cache", s.handleDebugCache)
		r.Get("/db", s.handleDebugDB)
		r.Get("/scheduler", s.handleDebugScheduler)
		r.Get("/memory", s.handleDebugMemory)
		r.Get("/goroutines", s.handleDebugGoroutines)
	})
}

func (s *Server) handleDebugConfig(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]interface{}{
		"server": map[string]interface{}{
			"address": s.cfg.Server.Address,
			"port":    s.cfg.Server.Port,
			"mode":    s.cfg.Server.Mode,
			"debug":   mode.IsDebugEnabled(),
		},
		"database": map[string]interface{}{
			"driver":     s.cfg.Server.Database.Driver,
			"sqlite_dir": s.cfg.Server.Database.SQLite.Dir,
		},
		"search": map[string]interface{}{
			"concurrent_requests":  s.cfg.Search.ConcurrentRequests,
			"engine_timeout":       s.cfg.Search.EngineTimeout,
			"results_per_page":     s.cfg.Search.ResultsPerPage,
			"min_duration_seconds": s.cfg.Search.MinDurationSeconds,
		},
	}

	// Per AI.md PART 14: Use 2-space indent JSON with trailing newline
	handler.WriteJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleDebugRoutes(w http.ResponseWriter, r *http.Request) {
	routes := []map[string]string{}

	chi.Walk(s.router, func(method string, route string, h http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, map[string]string{
			"method": method,
			"route":  route,
		})
		return nil
	})

	handler.WriteJSON(w, http.StatusOK, routes)
}

func (s *Server) handleDebugCache(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"enabled": true,
		"entries": 0,
	}

	handler.WriteJSON(w, http.StatusOK, stats)
}

func (s *Server) handleDebugDB(w http.ResponseWriter, r *http.Request) {
	// Get database stats from admin service if available
	db := s.adminSvc.GetDB()
	if db == nil {
		data := map[string]interface{}{
			"status": "database not available",
		}
		handler.WriteJSON(w, http.StatusOK, data)
		return
	}

	stats := db.Stats()
	data := map[string]interface{}{
		"open_connections":    stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration.String(),
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}

	handler.WriteJSON(w, http.StatusOK, data)
}

func (s *Server) handleDebugScheduler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"status": "running",
		"tasks":  []string{},
	}

	handler.WriteJSON(w, http.StatusOK, data)
}

func (s *Server) handleDebugMemory(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	data := map[string]interface{}{
		"alloc_mb":       m.Alloc / 1024 / 1024,
		"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
		"sys_mb":         m.Sys / 1024 / 1024,
		"num_gc":         m.NumGC,
		"goroutines":     runtime.NumGoroutine(),
	}

	handler.WriteJSON(w, http.StatusOK, data)
}

func (s *Server) handleDebugGoroutines(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"count": runtime.NumGoroutine(),
	}

	handler.WriteJSON(w, http.StatusOK, data)
}
