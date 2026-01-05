package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// AppError represents a standardized API error per PART 9
type AppError struct {
	Code       string       `json:"code"`
	Message    string       `json:"message"`
	Details    []FieldError `json:"details,omitempty"`
	RequestID  string       `json:"request_id"`
	HTTPStatus int          `json:"-"`
	Internal   error        `json:"-"`
}

// FieldError represents a field-specific validation error
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse wraps AppError for JSON response
type ErrorResponse struct {
	Error *AppError `json:"error"`
}

// NewError creates a new AppError
func NewError(code string, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: errorCodeToHTTP(code),
		RequestID:  generateRequestID(),
	}
}

// WithField adds a field-level error
func (e *AppError) WithField(field, code, message string) *AppError {
	e.Details = append(e.Details, FieldError{
		Field:   field,
		Code:    code,
		Message: message,
	})
	return e
}

// WithInternal wraps an internal error
func (e *AppError) WithInternal(err error) *AppError {
	e.Internal = err
	return e
}

// WriteError writes an error response with proper JSON formatting per PART 14
func WriteError(w http.ResponseWriter, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	response := ErrorResponse{Error: err}
	// Use MarshalIndent with 2-space indentation per PART 14
	data, marshalErr := json.MarshalIndent(response, "", "  ")
	if marshalErr != nil {
		w.Write([]byte(`{"error":{"code":"ERR_INTERNAL","message":"Failed to encode error"}}`))
		w.Write([]byte("\n"))
		return
	}
	w.Write(data)
	// Single trailing newline (NON-NEGOTIABLE per PART 14)
	w.Write([]byte("\n"))
}

// errorCodeToHTTP maps error codes to HTTP status codes per PART 9
func errorCodeToHTTP(code string) int {
	switch code {
	case "ERR_BAD_REQUEST":
		return http.StatusBadRequest
	case "ERR_VALIDATION":
		return http.StatusBadRequest
	case "ERR_UNAUTHORIZED":
		return http.StatusUnauthorized
	case "ERR_SESSION_EXPIRED":
		return http.StatusUnauthorized
	case "ERR_SESSION_INVALID":
		return http.StatusUnauthorized
	case "ERR_2FA_REQUIRED":
		return http.StatusUnauthorized
	case "ERR_2FA_INVALID":
		return http.StatusUnauthorized
	case "ERR_FORBIDDEN":
		return http.StatusForbidden
	case "ERR_ACCOUNT_LOCKED":
		return http.StatusForbidden
	case "ERR_NOT_FOUND":
		return http.StatusNotFound
	case "ERR_METHOD_NOT_ALLOWED":
		return http.StatusMethodNotAllowed
	case "ERR_CONFLICT":
		return http.StatusConflict
	case "ERR_UNPROCESSABLE":
		return http.StatusUnprocessableEntity
	case "ERR_RATE_LIMIT":
		return http.StatusTooManyRequests
	case "ERR_INTERNAL":
		return http.StatusInternalServerError
	case "ERR_SERVICE_UNAVAILABLE":
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func generateRequestID() string {
	return "req_" + uuid.New().String()[:16]
}

// Common errors
var (
	ErrBadRequest     = NewError("ERR_BAD_REQUEST", "Invalid request format")
	ErrUnauthorized   = NewError("ERR_UNAUTHORIZED", "Please log in to continue")
	ErrForbidden      = NewError("ERR_FORBIDDEN", "You don't have permission to do this")
	ErrNotFound       = NewError("ERR_NOT_FOUND", "The requested resource was not found")
	ErrInternal       = NewError("ERR_INTERNAL", "Something went wrong. Please try again later")
	ErrRateLimit      = NewError("ERR_RATE_LIMIT", "Too many requests. Please wait and try again")
)
