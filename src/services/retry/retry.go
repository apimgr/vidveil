// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 28: Retry Logic with Exponential Backoff
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts     int           // Maximum number of attempts (default: 3)
	InitialDelay    time.Duration // Initial delay between retries (default: 100ms)
	MaxDelay        time.Duration // Maximum delay between retries (default: 30s)
	Multiplier      float64       // Backoff multiplier (default: 2.0)
	Jitter          float64       // Random jitter factor 0-1 (default: 0.1)
	RetryableErrors []error       // Errors that should trigger retry
}

// DefaultConfig returns default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// Operation is a function that can be retried
type Operation func() error

// OperationWithResult is a function that returns a result and can be retried
type OperationWithResult[T any] func() (T, error)

// Do executes an operation with retry logic
func Do(ctx context.Context, cfg *Config, op Operation) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute operation
		err := op()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err, cfg.RetryableErrors) {
			return err
		}

		// Don't delay after last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate delay with jitter
		jitteredDelay := addJitter(delay, cfg.Jitter)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitteredDelay):
		}

		// Increase delay for next attempt (exponential backoff)
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return lastErr
}

// DoWithResult executes an operation that returns a result with retry logic
func DoWithResult[T any](ctx context.Context, cfg *Config, op OperationWithResult[T]) (T, error) {
	var result T

	if cfg == nil {
		cfg = DefaultConfig()
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Execute operation
		res, err := op()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err, cfg.RetryableErrors) {
			return result, err
		}

		// Don't delay after last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate delay with jitter
		jitteredDelay := addJitter(delay, cfg.Jitter)

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(jitteredDelay):
		}

		// Increase delay for next attempt
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return result, lastErr
}

// isRetryable checks if an error should trigger a retry
func isRetryable(err error, retryableErrors []error) bool {
	// If no specific errors defined, retry all errors
	if len(retryableErrors) == 0 {
		return true
	}

	for _, retryable := range retryableErrors {
		if errors.Is(err, retryable) {
			return true
		}
	}

	return false
}

// addJitter adds random jitter to a duration
func addJitter(d time.Duration, factor float64) time.Duration {
	if factor <= 0 {
		return d
	}

	jitter := float64(d) * factor * (rand.Float64()*2 - 1) // -factor to +factor
	return time.Duration(float64(d) + jitter)
}

// Backoff calculates the backoff duration for a given attempt
func Backoff(attempt int, cfg *Config) time.Duration {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if attempt <= 0 {
		return cfg.InitialDelay
	}

	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return addJitter(time.Duration(delay), cfg.Jitter)
}

// Common retryable errors
var (
	ErrTemporary     = errors.New("temporary error")
	ErrTimeout       = errors.New("timeout")
	ErrRateLimit     = errors.New("rate limited")
	ErrServerError   = errors.New("server error")
	ErrNetworkError  = errors.New("network error")
)

// IsTemporaryError checks if an error is temporary and should be retried
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common temporary error patterns
	type temporary interface {
		Temporary() bool
	}

	if te, ok := err.(temporary); ok {
		return te.Temporary()
	}

	// Check for timeout
	type timeout interface {
		Timeout() bool
	}

	if te, ok := err.(timeout); ok && te.Timeout() {
		return true
	}

	return false
}
