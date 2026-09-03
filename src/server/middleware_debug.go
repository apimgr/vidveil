// SPDX-License-Identifier: MIT
package server

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/apimgr/vidveil/src/mode"
)

// debugMiddleware logs detailed request/response info (--debug/DEBUG=true only)
func (s *Server) debugMiddleware(next http.Handler) http.Handler {
	// No-op unless debug enabled
	if !mode.IsDebugEnabled() {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture request body for logging (limit to 10KB)
		if r.Body != nil && r.ContentLength > 0 && r.ContentLength < 10*1024 {
			requestBody, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Wrap response writer to capture status and size
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(rw, r)

		// Log after request completes
		s.debugLog(r, rw.status, time.Since(start), rw.size)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}
