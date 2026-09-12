// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI exit codes and error classification

package cmd

import (
	"errors"
	"net/http"

	"github.com/apimgr/vidveil/src/client/api"
)

// Exit codes required by AI.md PART 32
const (
	// ExitSuccess indicates the command completed successfully
	ExitSuccess = 0
	// ExitGeneral indicates an unclassified failure
	ExitGeneral = 1
	// ExitConfig indicates a configuration problem
	ExitConfig = 2
	// ExitConnection indicates the server could not be reached
	ExitConnection = 3
	// ExitAuth indicates the supplied credentials were rejected
	ExitAuth = 4
	// ExitNotFound indicates the requested resource does not exist
	ExitNotFound = 5
	// ExitUsage indicates bad arguments or flags
	ExitUsage = 64
)

// ExitError wraps an error with an explicit AI.md PART 32 exit code.
type ExitError struct {
	Code int
	Err  error
}

// Error renders the wrapped error message.
func (e *ExitError) Error() string { return e.Err.Error() }

// Unwrap exposes the wrapped error for errors.Is/As.
func (e *ExitError) Unwrap() error { return e.Err }

// NewExitError wraps err with the given exit code.
func NewExitError(code int, err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: code, Err: err}
}

// ExitCodeForError maps an error onto the AI.md PART 32 exit code table.
func ExitCodeForError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	var connErr *api.ConnectionError
	if errors.As(err, &connErr) {
		return ExitConnection
	}

	var statusErr *api.StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return ExitAuth
		case http.StatusNotFound:
			return ExitNotFound
		}
		return ExitGeneral
	}

	return ExitGeneral
}
