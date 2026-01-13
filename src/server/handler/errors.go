// SPDX-License-Identifier: MIT
// AI.md PART 9: Error Handling

package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"syscall"
	"time"
)

// AppError represents a standardized API error per PART 9
// This is for internal use - external responses use Response struct
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	RequestID  string
	Internal   error
}

// Error implements error interface
func (e *AppError) Error() string {
	return e.Message
}

// NewAppError creates a new AppError for internal use
func NewAppError(code string, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: ErrorCodeToHTTP(code),
	}
}

// WithInternal wraps an internal error (for logging, never exposed)
func (e *AppError) WithInternal(err error) *AppError {
	e.Internal = err
	return e
}

// WithRequestID adds request ID for tracing
func (e *AppError) WithRequestID(id string) *AppError {
	e.RequestID = id
	return e
}

// Write sends the error as JSON response
func (e *AppError) Write(w http.ResponseWriter) {
	SendError(w, e.Code, e.Message)
}

// LogError logs an error with context per AI.md PART 9
func LogError(ctx context.Context, err *AppError, logger *slog.Logger) {
	attrs := []any{
		slog.String("error_code", err.Code),
		slog.Int("http_status", err.HTTPStatus),
	}

	if err.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", err.RequestID))
	}

	// Include internal error for debugging (never in response)
	if err.Internal != nil {
		attrs = append(attrs, slog.String("internal", err.Internal.Error()))
	}

	// Log level based on HTTP status per PART 9
	if err.HTTPStatus >= 500 {
		logger.Error(err.Message, attrs...)
	} else if err.HTTPStatus >= 400 {
		logger.Warn(err.Message, attrs...)
	}
}

// WithRetry implements exponential backoff per AI.md PART 9
// Backoff: 0s, 1s, 2s, 4s, 8s (max 30s)
func WithRetry(ctx context.Context, fn func() error) error {
	backoff := []time.Duration{0, 1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	maxBackoff := 30 * time.Second

	var lastErr error
	for attempt := 0; attempt < len(backoff); attempt++ {
		if attempt > 0 {
			wait := backoff[attempt]
			if wait > maxBackoff {
				wait = maxBackoff
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		if err := fn(); err != nil {
			if !IsRetryable(err) {
				return err
			}
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

// IsRetryable checks if an error is retryable per AI.md PART 9
// Network errors, timeouts, 503s are retryable
// 4xx errors are NOT retryable
func IsRetryable(err error) bool {
	// Context errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false // Don't retry if context was canceled
	}

	// Network errors
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	// AppError with 503
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus == 503
	}

	return false
}

// Common pre-defined errors for convenience
var (
	ErrBadRequest      = NewAppError(CodeBadRequest, MsgBadRequest)
	ErrUnauthorized    = NewAppError(CodeUnauthorized, MsgUnauthorized)
	ErrForbidden       = NewAppError(CodeForbidden, MsgForbidden)
	ErrNotFound        = NewAppError(CodeNotFound, MsgNotFound)
	ErrConflict        = NewAppError(CodeConflict, MsgConflict)
	ErrRateLimited     = NewAppError(CodeRateLimited, MsgRateLimited)
	ErrServerError     = NewAppError(CodeServerError, MsgServerError)
	ErrMaintenance     = NewAppError(CodeMaintenance, MsgMaintenance)
)
