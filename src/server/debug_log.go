// SPDX-License-Identifier: MIT
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/apimgr/vidveil/src/mode"
)

// debugLog logs detailed request information (--debug/DEBUG=true only)
func (s *Server) debugLog(r *http.Request, status int, duration time.Duration, size int) {
	if !mode.IsDebugEnabled() {
		return
	}

	slog.Debug("request",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"size", size,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"referer", r.Referer(),
		"request_id", r.Header.Get("X-Request-ID"),
	)
}

// debugLogDB logs database queries (--debug/DEBUG=true only)
func (s *Server) debugLogDB(query string, args []any, duration time.Duration, err error) {
	if !mode.IsDebugEnabled() {
		return
	}

	attrs := []any{
		"query", query,
		"duration_ms", duration.Milliseconds(),
	}

	if len(args) > 0 {
		attrs = append(attrs, "args", args)
	}

	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Debug("db query failed", attrs...)
	} else {
		slog.Debug("db query", attrs...)
	}
}

// debugLogCache logs cache operations (--debug/DEBUG=true only)
func (s *Server) debugLogCache(op string, key string, hit bool, duration time.Duration) {
	if !mode.IsDebugEnabled() {
		return
	}

	slog.Debug("cache",
		"operation", op,
		"key", key,
		"hit", hit,
		"duration_us", duration.Microseconds(),
	)
}
