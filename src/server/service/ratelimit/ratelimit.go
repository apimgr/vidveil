// SPDX-License-Identifier: MIT
// AI.md PART 1: Rate Limiting (endpoint-specific limits)
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/service/logging"
)

// Endpoint types for rate limiting per AI.md PART 1
const (
	EndpointLogin          = "login"
	EndpointPasswordReset  = "password_reset"
	EndpointAPIAuth        = "api_authenticated"
	EndpointAPIUnauth      = "api_unauthenticated"
	EndpointRegistration   = "registration"
	EndpointFileUpload     = "file_upload"
	EndpointDefault        = "default"
)

// Default rate limits per AI.md PART 1
// | Endpoint Type | Limit | Window |
// |---------------|-------|--------|
// | Login attempts | 5 | 15 min |
// | Password reset | 3 | 1 hour |
// | API (authenticated) | 100 | 1 min |
// | API (unauthenticated) | 20 | 1 min |
// | Registration | 5 | 1 hour |
// | File upload | 10 | 1 hour |
var DefaultLimits = map[string]struct {
	Requests int
	Window   time.Duration
}{
	EndpointLogin:         {5, 15 * time.Minute},
	EndpointPasswordReset: {3, time.Hour},
	EndpointAPIAuth:       {100, time.Minute},
	EndpointAPIUnauth:     {20, time.Minute},
	EndpointRegistration:  {5, time.Hour},
	EndpointFileUpload:    {10, time.Hour},
	EndpointDefault:       {100, time.Minute},
}

// EndpointLimiters holds multiple rate limiters for different endpoint types per AI.md PART 1
type EndpointLimiters struct {
	limiters map[string]*Limiter
	logger   *logging.Logger
	mu       sync.RWMutex
}

// NewEndpointLimiters creates endpoint-specific rate limiters per AI.md PART 1
func NewEndpointLimiters(enabled bool) *EndpointLimiters {
	el := &EndpointLimiters{
		limiters: make(map[string]*Limiter),
	}

	for endpoint, limits := range DefaultLimits {
		el.limiters[endpoint] = New(enabled, limits.Requests, int(limits.Window.Seconds()))
	}

	return el
}

// SetLogger sets the logger for all endpoint limiters
func (el *EndpointLimiters) SetLogger(logger *logging.Logger) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.logger = logger
	for _, l := range el.limiters {
		l.SetLogger(logger)
	}
}

// Get returns the rate limiter for a specific endpoint type
func (el *EndpointLimiters) Get(endpoint string) *Limiter {
	el.mu.RLock()
	defer el.mu.RUnlock()
	if l, ok := el.limiters[endpoint]; ok {
		return l
	}
	return el.limiters[EndpointDefault]
}

// AllowLogin checks rate limit for login attempts per AI.md PART 1 (5 per 15 min)
func (el *EndpointLimiters) AllowLogin(ip string) bool {
	return el.Get(EndpointLogin).Allow(ip)
}

// AllowPasswordReset checks rate limit for password reset per AI.md PART 1 (3 per hour)
func (el *EndpointLimiters) AllowPasswordReset(ip string) bool {
	return el.Get(EndpointPasswordReset).Allow(ip)
}

// AllowAPIAuth checks rate limit for authenticated API per AI.md PART 1 (100 per min)
func (el *EndpointLimiters) AllowAPIAuth(ip string) bool {
	return el.Get(EndpointAPIAuth).Allow(ip)
}

// AllowAPIUnauth checks rate limit for unauthenticated API per AI.md PART 1 (20 per min)
func (el *EndpointLimiters) AllowAPIUnauth(ip string) bool {
	return el.Get(EndpointAPIUnauth).Allow(ip)
}

// AllowRegistration checks rate limit for registration per AI.md PART 1 (5 per hour)
func (el *EndpointLimiters) AllowRegistration(ip string) bool {
	return el.Get(EndpointRegistration).Allow(ip)
}

// AllowFileUpload checks rate limit for file uploads per AI.md PART 1 (10 per hour)
func (el *EndpointLimiters) AllowFileUpload(ip string) bool {
	return el.Get(EndpointFileUpload).Allow(ip)
}

// Limiter implements a sliding window rate limiter per PART 1
type Limiter struct {
	mu      sync.RWMutex
	enabled bool
	// Max requests per window
	requests int
	// Time window
	window  time.Duration
	clients map[string]*clientInfo
	// Logger for security events per AI.md PART 11
	logger *logging.Logger
}

type clientInfo struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// New creates a new rate limiter
// Default: 100 requests per 60 seconds per AI.md PART 1
func New(enabled bool, requests int, windowSeconds int) *Limiter {
	// Default per AI.md PART 1 (API authenticated)
	if requests <= 0 {
		requests = 100
	}
	// Default per AI.md PART 1
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

// SetLogger sets the logger for security event logging per AI.md PART 11
func (l *Limiter) SetLogger(logger *logging.Logger) {
	l.logger = logger
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

		// Set rate limit headers per PART 1
		w.Header().Set("X-RateLimit-Limit", itoa(l.requests))
		w.Header().Set("X-RateLimit-Remaining", itoa(l.Remaining(ip)))
		w.Header().Set("X-RateLimit-Reset", itoa(int(l.Reset(ip).Unix())))

		if !l.Allow(ip) {
			// Log security event per AI.md PART 11 line 11400: security.rate_limit_exceeded
			if l.logger != nil {
				l.logger.Security("rate_limit_exceeded", ip, map[string]interface{}{
					"endpoint": r.URL.Path,
					"method":   r.Method,
					"limit":    l.requests,
					"window":   int(l.window.Seconds()),
				})
			}
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
