// Package authpad implements AI.md PART 11's "Identical auth response
// timing" rule and Output Sanitization Pipeline stage 6 ("constant-time
// finalize"): failed-auth and other sensitive-operation responses are
// padded to a fixed minimum duration so attackers cannot distinguish
// internal code paths (e.g. "user exists, wrong password" vs "no such
// user") by response latency.
package authpad

import (
	"net/http"
	"time"
)

// Floor is the minimum response time for auth-failure and other
// timing-sensitive responses, per AI.md PART 11 (Authentication & Identity
// Rules: "Identical auth response timing" — pad to a fixed floor, ≥100ms).
const Floor = 100 * time.Millisecond

// Wait blocks until Floor has elapsed since start, if it hasn't already.
// Call it immediately before writing the response so slow paths (real
// lookups, hashing) are never penalized twice and fast paths (early
// rejects) never leak their speed.
func Wait(start time.Time) {
	if elapsed := time.Since(start); elapsed < Floor {
		time.Sleep(Floor - elapsed)
	}
}

// ResponseWriter delays the first response commit until the timing floor has
// elapsed. Finish also pads handlers that return without writing a response.
type ResponseWriter struct {
	http.ResponseWriter
	start     time.Time
	committed bool
}

// WrapResponse creates a response writer that enforces the timing floor.
func WrapResponse(w http.ResponseWriter, start time.Time) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, start: start}
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.waitBeforeCommit()
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriter) Write(body []byte) (int, error) {
	w.waitBeforeCommit()
	return w.ResponseWriter.Write(body)
}

// Finish ensures even an empty response does not escape before the floor.
func (w *ResponseWriter) Finish() {
	w.waitBeforeCommit()
}

func (w *ResponseWriter) waitBeforeCommit() {
	if !w.committed {
		Wait(w.start)
		w.committed = true
	}
}
