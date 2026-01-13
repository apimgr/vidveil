// SPDX-License-Identifier: MIT
// AI.md PART 5: Path Security Functions
package paths

import (
	"errors"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrPathTraversal = errors.New("path traversal attempt detected")
	ErrInvalidPath   = errors.New("invalid path characters")
	ErrPathTooLong   = errors.New("path exceeds maximum length")

	// Valid path segment: lowercase alphanumeric, hyphens, underscores
	validPathSegment = regexp.MustCompile(`^[a-z0-9_-]+$`)
)

// normalizePath cleans a path for safe use per AI.md PART 5
// - Strips leading/trailing slashes
// - Collapses multiple slashes (// → /)
// - Removes path traversal (.., .)
// - Returns empty string for invalid input
func normalizePath(input string) string {
	if input == "" {
		return ""
	}

	// Use path.Clean to handle .., ., and //
	cleaned := path.Clean(input)

	// Strip leading/trailing slashes
	cleaned = strings.Trim(cleaned, "/")

	// Reject if still contains .. after cleaning
	if strings.Contains(cleaned, "..") {
		return ""
	}

	return cleaned
}

// validatePathSegment checks a single path segment per AI.md PART 5
func validatePathSegment(segment string) error {
	if segment == "" {
		return ErrInvalidPath
	}
	if len(segment) > 64 {
		return ErrPathTooLong
	}
	if !validPathSegment.MatchString(segment) {
		return ErrInvalidPath
	}
	if segment == "." || segment == ".." {
		return ErrPathTraversal
	}
	return nil
}

// validatePath checks an entire path per AI.md PART 5
func validatePath(p string) error {
	if len(p) > 2048 {
		return ErrPathTooLong
	}

	// Check for traversal attempts before normalization
	if strings.Contains(p, "..") {
		return ErrPathTraversal
	}

	// Check each segment
	segments := strings.Split(strings.Trim(p, "/"), "/")
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if err := validatePathSegment(seg); err != nil {
			return err
		}
	}

	return nil
}

// SafePath normalizes and validates - returns error if invalid per AI.md PART 5
func SafePath(input string) (string, error) {
	if err := validatePath(input); err != nil {
		return "", err
	}
	return normalizePath(input), nil
}

// SafeFilePath ensures path stays within base directory per AI.md PART 5
func SafeFilePath(baseDir, userPath string) (string, error) {
	safe, err := SafePath(userPath)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(baseDir, safe)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	// Verify path is still within base
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", ErrPathTraversal
	}

	return absPath, nil
}

// PathSecurityMiddleware normalizes paths and blocks traversal attempts per AI.md PART 5
// This middleware MUST be first in the chain - before auth, before routing.
func PathSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original := r.URL.Path

		// Check both raw path and URL-decoded for traversal
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}

		// Block path traversal attempts (encoded and decoded)
		// %2e = . so %2e%2e = ..
		if strings.Contains(original, "..") ||
			strings.Contains(rawPath, "..") ||
			strings.Contains(strings.ToLower(rawPath), "%2e") {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Normalize the path
		cleaned := path.Clean(original)

		// Ensure leading slash
		if !strings.HasPrefix(cleaned, "/") {
			cleaned = "/" + cleaned
		}

		// Preserve trailing slash for directory paths
		if original != "/" && strings.HasSuffix(original, "/") && !strings.HasSuffix(cleaned, "/") {
			cleaned += "/"
		}

		// Update request
		r.URL.Path = cleaned

		next.ServeHTTP(w, r)
	})
}
