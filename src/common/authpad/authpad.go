// Package authpad implements AI.md PART 11's "Identical auth response
// timing" rule and Output Sanitization Pipeline stage 6 ("constant-time
// finalize"): failed-auth and other sensitive-operation responses are
// padded to a fixed minimum duration so attackers cannot distinguish
// internal code paths (e.g. "user exists, wrong password" vs "no such
// user") by response latency.
package authpad

import "time"

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
