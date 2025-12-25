// SPDX-License-Identifier: MIT
package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

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

func TestDoSuccess(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Do(ctx, nil, func() error {
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

func TestDoRetryThenSuccess(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	cfg := &Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
	}

	err := Do(ctx, cfg, func() error {
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

func TestDoMaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	cfg := &Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
	}

	err := Do(ctx, cfg, func() error {
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

func TestDoNonRetryableError(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	nonRetryable := errors.New("non-retryable error")
	cfg := &Config{
		MaxAttempts:     3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		Jitter:          0,
		RetryableErrors: []error{ErrTemporary},
	}

	err := Do(ctx, cfg, func() error {
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

func TestDoContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	cfg := &Config{
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

	err := Do(ctx, cfg, func() error {
		attempts++
		return ErrTemporary
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestDoWithResult(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	result, err := DoWithResult(ctx, nil, func() (string, error) {
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
	cfg := &Config{
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
	cfg := &Config{
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
