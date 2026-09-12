// SPDX-License-Identifier: MIT
// AI.md PART 32: typed API client errors so callers can map failures onto the
// required CLI exit codes (3 connection, 4 authentication, 5 not found)
package api

import "fmt"

// ConnectionError reports a failure to reach the configured server.
type ConnectionError struct {
	BaseURL string
	Err     error
}

// Error renders the operator-facing connection failure message.
func (e *ConnectionError) Error() string {
	return fmt.Sprintf("cannot connect to server at %s: %v", e.BaseURL, e.Err)
}

// Unwrap exposes the underlying transport error for errors.Is/As.
func (e *ConnectionError) Unwrap() error { return e.Err }

// StatusError reports a non-success HTTP response from the server.
type StatusError struct {
	StatusCode int
	Body       string
}

// Error renders the operator-facing server error message.
func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("server returned %d", e.StatusCode)
	}
	return fmt.Sprintf("server returned %d: %s", e.StatusCode, e.Body)
}
