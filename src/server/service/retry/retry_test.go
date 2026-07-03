// SPDX-License-Identifier: MIT
package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}

	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay 100ms, got %v", cfg.InitialDelay)
	}

	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay 30s, got %v", cfg.MaxDelay)
	}

	if cfg.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier 2.0, got %f", cfg.Multiplier)
	}

	if cfg.Jitter != 0.1 {
		t.Errorf("Expected Jitter 0.1, got %f", cfg.Jitter)
	}
}

func TestExecuteWithRetrySuccess(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := ExecuteWithRetry(ctx, nil, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestExecuteWithRetryRetryThenSuccess(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
	}

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		if attempts < 3 {
			return ErrTemporary
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestExecuteWithRetryMaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
	}

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		return ErrTemporary
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestExecuteWithRetryNonRetryableError(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	nonRetryable := errors.New("non-retryable error")
	cfg := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		Jitter:          0,
		RetryableErrors: []error{ErrTemporary},
	}

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		return nonRetryable
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should not retry non-retryable errors
	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestExecuteWithRetryContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       0,
	}

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		return ErrTemporary
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestExecuteWithRetryResult(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	result, err := ExecuteWithRetryResult(ctx, nil, func() (string, error) {
		attempts++
		if attempts < 2 {
			return "", ErrTemporary
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestBackoff(t *testing.T) {
	cfg := &RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		// No jitter for predictable testing
		Jitter: 0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
	}

	for _, tt := range tests {
		result := Backoff(tt.attempt, cfg)
		if result != tt.expected {
			t.Errorf("Backoff(%d) = %v, want %v", tt.attempt, result, tt.expected)
		}
	}
}

func TestBackoffMaxDelay(t *testing.T) {
	cfg := &RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
	}

	// After many attempts, should cap at MaxDelay
	result := Backoff(10, cfg)
	if result != 500*time.Millisecond {
		t.Errorf("Backoff should cap at MaxDelay, got %v", result)
	}
}

func TestIsRetryableEmpty(t *testing.T) {
	// When no retryable errors defined, all errors are retryable
	if !isRetryable(errors.New("any error"), nil) {
		t.Error("Expected all errors to be retryable when list is empty")
	}
}

func TestIsRetryableSpecific(t *testing.T) {
	retryable := []error{ErrTemporary, ErrTimeout}

	if !isRetryable(ErrTemporary, retryable) {
		t.Error("ErrTemporary should be retryable")
	}

	if !isRetryable(ErrTimeout, retryable) {
		t.Error("ErrTimeout should be retryable")
	}

	if isRetryable(ErrNetworkError, retryable) {
		t.Error("ErrNetworkError should not be retryable")
	}
}

func TestIsTemporaryError(t *testing.T) {
	// nil should return false
	if IsTemporaryError(nil) {
		t.Error("nil should not be temporary")
	}

	// Standard errors are not temporary
	if IsTemporaryError(errors.New("standard error")) {
		t.Error("Standard error should not be temporary")
	}
}

func TestAddJitter(t *testing.T) {
	d := 100 * time.Millisecond

	// No jitter
	result := addJitter(d, 0)
	if result != d {
		t.Errorf("No jitter should return original, got %v", result)
	}

	// With jitter, result should be within range
	for i := 0; i < 100; i++ {
		result := addJitter(d, 0.5)
		min := time.Duration(float64(d) * 0.5)
		max := time.Duration(float64(d) * 1.5)
		if result < min || result > max {
			t.Errorf("Jittered result %v outside range [%v, %v]", result, min, max)
		}
	}
}

// TestBackoffNilConfig ensures Backoff works when cfg is nil (uses DefaultRetryConfig).
func TestBackoffNilConfig(t *testing.T) {
	// nil cfg must not panic and must return a positive duration
	d := Backoff(1, nil)
	if d <= 0 {
		t.Errorf("Backoff with nil config returned non-positive duration: %v", d)
	}
}

// TestExecuteWithRetryContextCancelledDuringWait covers the select branch inside the
// wait loop (ctx.Done fires while sleeping between retries).
func TestExecuteWithRetryContextCancelledDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 200 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0,
	}

	// Cancel after the first attempt has returned an error but before the sleep ends.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		return ErrTemporary
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Should have only run once before the cancellation fired during the wait
	if attempts != 1 {
		t.Errorf("Expected 1 attempt before cancel, got %d", attempts)
	}
}

// TestExecuteWithRetryResultContextCancelledBeforeStart covers the upfront
// ctx.Done() check when the context is already cancelled before the first attempt.
func TestExecuteWithRetryResultContextCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := ExecuteWithRetryResult(ctx, nil, func() (string, error) {
		return "should not run", nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if result != "" {
		t.Errorf("Expected zero-value result, got %q", result)
	}
}

// TestExecuteWithRetryResultNonRetryableError covers the early-return path when the
// operation returns an error that is not in RetryableErrors.
func TestExecuteWithRetryResultNonRetryableError(t *testing.T) {
	ctx := context.Background()
	nonRetryable := errors.New("permanent failure")
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		Jitter:          0,
		RetryableErrors: []error{ErrTemporary},
	}

	result, err := ExecuteWithRetryResult(ctx, cfg, func() (int, error) {
		attempts++
		return 0, nonRetryable
	})

	if !errors.Is(err, nonRetryable) {
		t.Errorf("Expected nonRetryable error, got %v", err)
	}

	if result != 0 {
		t.Errorf("Expected zero result, got %d", result)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry for non-retryable), got %d", attempts)
	}
}

// TestExecuteWithRetryResultContextCancelledDuringWait covers the wait-select path
// inside ExecuteWithRetryResult when the context is cancelled between retries.
func TestExecuteWithRetryResultContextCancelledDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := &RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 200 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0,
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := ExecuteWithRetryResult(ctx, cfg, func() (string, error) {
		return "", ErrTemporary
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled during wait, got %v", err)
	}
}

// TestExecuteWithRetryContextCancelledBeforeStart covers the ctx.Done() check at the
// very top of the loop when the context is already cancelled. We run many iterations
// because the select { case <-ctx.Done(): ... default: } is nondeterministic when
// both are ready; repeated invocations guarantee the branch is exercised.
func TestExecuteWithRetryContextCancelledBeforeStart(t *testing.T) {
	cfg := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   1.0,
		Jitter:       0,
	}

	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := ExecuteWithRetry(ctx, cfg, func() error {
			return ErrTemporary
		})

		if !errors.Is(err, context.Canceled) && !errors.Is(err, ErrTemporary) {
			t.Errorf("iter %d: expected Canceled or ErrTemporary (after exhaustion), got %v", i, err)
		}
	}
}

// TestExecuteWithRetryDelayCapExceeded covers the `if delay > cfg.MaxDelay` branch
// inside ExecuteWithRetry by setting MaxDelay below what the multiplier would produce.
func TestExecuteWithRetryDelayCapExceeded(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     11 * time.Millisecond,
		Multiplier:   100.0,
		Jitter:       0,
	}

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		return ErrTemporary
	})

	if err == nil {
		t.Error("Expected error after exhausting retries")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestExecuteWithRetryContextCancelledAtLoopTop covers the ctx.Done() check at the
// TOP of the retry loop (second+ iteration) — context is cancelled during sleep but
// the cancel fires just before the loop re-evaluates the guard.
func TestExecuteWithRetryContextCancelledAtLoopTop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   1.0,
		Jitter:       0,
	}

	err := ExecuteWithRetry(ctx, cfg, func() error {
		attempts++
		// Cancel immediately after the first attempt so the context is already
		// done when the second iteration checks ctx.Done() at the top of the loop.
		if attempts == 1 {
			cancel()
		}
		return ErrTemporary
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestExecuteWithRetryResultMaxAttemptsExhausted covers the break-then-return path
// (attempt == MaxAttempts, then `return result, lastErr`), and also the delay-cap.
func TestExecuteWithRetryResultMaxAttemptsExhausted(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     11 * time.Millisecond,
		Multiplier:   100.0,
		Jitter:       0,
	}

	result, err := ExecuteWithRetryResult(ctx, cfg, func() (string, error) {
		attempts++
		return "", ErrTemporary
	})

	if err == nil {
		t.Error("Expected error after exhausting retries")
	}

	if result != "" {
		t.Errorf("Expected empty result, got %q", result)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestExecuteWithRetryResultContextCancelledAtLoopTop covers the ctx.Done() check at
// the TOP of the loop inside ExecuteWithRetryResult (second+ iteration).
func TestExecuteWithRetryResultContextCancelledAtLoopTop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	cfg := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   1.0,
		Jitter:       0,
	}

	_, err := ExecuteWithRetryResult(ctx, cfg, func() (int, error) {
		attempts++
		if attempts == 1 {
			cancel()
		}
		return 0, ErrTemporary
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// temporaryError is a test helper that implements the temporary interface.
type temporaryError struct {
	isTemp bool
}

func (e *temporaryError) Error() string   { return "temporary error" }
func (e *temporaryError) Temporary() bool { return e.isTemp }

// timeoutError is a test helper that implements the timeout interface but NOT temporary.
type timeoutError struct {
	isTimeout bool
}

func (e *timeoutError) Error() string { return "timeout error" }
func (e *timeoutError) Timeout() bool { return e.isTimeout }

// TestIsTemporaryErrorTemporaryInterface covers both true and false returns from
// an error implementing the Temporary() bool interface.
func TestIsTemporaryErrorTemporaryInterface(t *testing.T) {
	if !IsTemporaryError(&temporaryError{isTemp: true}) {
		t.Error("Expected true for Temporary()=true error")
	}

	if IsTemporaryError(&temporaryError{isTemp: false}) {
		t.Error("Expected false for Temporary()=false error")
	}
}

// TestIsTemporaryErrorTimeoutInterface covers the Timeout() bool interface path,
// including the case where Timeout() returns false (falls through to return false).
func TestIsTemporaryErrorTimeoutInterface(t *testing.T) {
	if !IsTemporaryError(&timeoutError{isTimeout: true}) {
		t.Error("Expected true for Timeout()=true error")
	}

	if IsTemporaryError(&timeoutError{isTimeout: false}) {
		t.Error("Expected false for Timeout()=false error")
	}
}
