// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 16: Rate Limiting
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Limiter implements a sliding window rate limiter per PART 16
type Limiter struct {
	mu      sync.RWMutex
	enabled bool
	// Max requests per window
	requests int
	// Time window
	window  time.Duration
	clients map[string]*clientInfo
}

type clientInfo struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// New creates a new rate limiter
// Default: 120 requests per 60 seconds (from config)
func New(enabled bool, requests int, windowSeconds int) *Limiter {
	// Default per TEMPLATE.md PART 16
	if requests <= 0 {
		requests = 120
	}
	// Default per TEMPLATE.md PART 16
	if windowSeconds <= 0 {
		windowSeconds = 60
	}

	l := &Limiter{
		enabled:  enabled,
		requests: requests,
		window:   time.Duration(windowSeconds) * time.Second,
		clients:  make(map[string]*clientInfo),
	}

	// Start cleanup goroutine
	go l.cleanup()

	return l
}

// Allow checks if a request from the given IP should be allowed
func (l *Limiter) Allow(ip string) bool {
	if !l.enabled {
		return true
	}

	l.mu.Lock()
	client, ok := l.clients[ip]
	if !ok {
		client = &clientInfo{
			timestamps: make([]time.Time, 0, l.requests),
		}
		l.clients[ip] = client
	}
	l.mu.Unlock()

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Remove timestamps outside the window
	valid := make([]time.Time, 0, len(client.timestamps))
	for _, t := range client.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	client.timestamps = valid

	// Check if under limit
	if len(client.timestamps) >= l.requests {
		return false
	}

	// Add new timestamp
	client.timestamps = append(client.timestamps, now)
	return true
}

// Remaining returns how many requests are remaining for an IP
func (l *Limiter) Remaining(ip string) int {
	if !l.enabled {
		return l.requests
	}

	l.mu.RLock()
	client, ok := l.clients[ip]
	l.mu.RUnlock()

	if !ok {
		return l.requests
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	count := 0
	for _, t := range client.timestamps {
		if t.After(cutoff) {
			count++
		}
	}

	return l.requests - count
}

// Reset returns when the rate limit will reset for an IP
func (l *Limiter) Reset(ip string) time.Time {
	if !l.enabled {
		return time.Now()
	}

	l.mu.RLock()
	client, ok := l.clients[ip]
	l.mu.RUnlock()

	if !ok || len(client.timestamps) == 0 {
		return time.Now()
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	// Find oldest timestamp in the window
	if len(client.timestamps) > 0 {
		return client.timestamps[0].Add(l.window)
	}

	return time.Now()
}

// cleanup periodically removes stale entries
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		// Keep entries for 2x window
		cutoff := now.Add(-l.window * 2)

		for ip, client := range l.clients {
			client.mu.Lock()
			// Remove if no recent timestamps
			hasRecent := false
			for _, t := range client.timestamps {
				if t.After(cutoff) {
					hasRecent = true
					break
				}
			}
			if !hasRecent {
				delete(l.clients, ip)
			}
			client.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that enforces rate limiting
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP (use X-Real-IP or X-Forwarded-For if behind proxy)
		ip := r.RemoteAddr
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			ip = realIP
		} else if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			// Use first IP in the chain
			ip = forwarded
			for i, c := range forwarded {
				if c == ',' {
					ip = forwarded[:i]
					break
				}
			}
		}

		// Set rate limit headers per PART 16
		w.Header().Set("X-RateLimit-Limit", itoa(l.requests))
		w.Header().Set("X-RateLimit-Remaining", itoa(l.Remaining(ip)))
		w.Header().Set("X-RateLimit-Reset", itoa(int(l.Reset(ip).Unix())))

		if !l.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SetHeaders sets rate limit response headers
func (l *Limiter) SetHeaders(w http.ResponseWriter, ip string) {
	w.Header().Set("X-RateLimit-Limit", itoa(l.requests))
	w.Header().Set("X-RateLimit-Remaining", itoa(l.Remaining(ip)))
	w.Header().Set("X-RateLimit-Reset", itoa(int(l.Reset(ip).Unix())))
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	if neg {
		buf[i] = '-'
		i--
	}
	return string(buf[i+1:])
}
