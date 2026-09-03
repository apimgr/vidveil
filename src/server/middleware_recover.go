// SPDX-License-Identifier: MIT
package server

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
)

// responseTracker wraps http.ResponseWriter so guaranteedRecoverer can tell
// whether a handler already started writing a response before it panicked.
// Without this, a recovered panic could clobber a partially-written body or
// send a second WriteHeader, corrupting the response on the wire.
type responseTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rt *responseTracker) WriteHeader(status int) {
	if rt.wroteHeader {
		return
	}
	rt.wroteHeader = true
	rt.ResponseWriter.WriteHeader(status)
}

func (rt *responseTracker) Write(b []byte) (int, error) {
	if !rt.wroteHeader {
		rt.WriteHeader(http.StatusOK)
	}
	return rt.ResponseWriter.Write(b)
}

// guaranteedRecoverer replaces chi's stock middleware.Recoverer. chi's
// Recoverer only sets a bare 500 status with an empty body on panic - the
// "blank body" / "dropped connection" failure AI.md PART 16 explicitly
// forbids ("Always terminate every request in a rendered response...never a
// blank body, dropped connection, or leaked stack trace"). This renders a
// minimal, hardcoded, content-negotiated error body instead - deliberately
// independent of html/template and the app's own error-page templates,
// since either of those could be the source of the panic being recovered
// from, and a fallback that depends on the failing subsystem is no fallback.
func guaranteedRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt := &responseTracker{ResponseWriter: w}
		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}
			if rvr == http.ErrAbortHandler {
				// Client disconnected mid-stream; nothing to render, this
				// sentinel must keep propagating so net/http can settle it.
				panic(rvr)
			}
			// Full detail (panic value + stack trace) server-side only, per
			// AI.md PART 11 Tier 1 - never exposed in the client response.
			log.Printf("panic recovered: %v\n%s", rvr, debug.Stack())
			if !rt.wroteHeader {
				writeGuaranteedError(rt, r, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(rt, r)
	})
}

// writeGuaranteedError writes a minimal hardcoded error body, content
// negotiated between JSON (API routes / Accept: application/json) and HTML
// (everything else), matching the canonical error envelope and standard
// error-page shape without depending on any template rendering.
func writeGuaranteedError(w http.ResponseWriter, r *http.Request, status int) {
	const message = "Something went wrong on our end. Please try again later."
	accept := r.Header.Get("Accept")
	wantsJSON := strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(accept, "application/json")
	if wantsJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"ok":false,"error":"SERVER_ERROR","message":%q,"details":{}}`+"\n", message)
		return
	}
	if strings.Contains(accept, "text/plain") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprintf(w, "%d Server Error: %s\n", status, message)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>%d Server Error</title></head><body><h1>%d Server Error</h1><p>%s</p></body></html>`,
		status, status, message)
}
