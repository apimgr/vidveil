// SPDX-License-Identifier: MIT
// AI.md PART 9: HTTP Cache Headers

package cache

import (
	"net/http"
)

// ContentType for cache header selection
type ContentType string

const (
	// ContentStatic for JS/CSS/images - 1 year immutable
	ContentStatic ContentType = "static"
	// ContentHTML always fetch fresh
	ContentHTML ContentType = "html"
	// ContentAPI for public API - 60s cache
	ContentAPI ContentType = "api"
	// ContentPrivate for authenticated - no cache
	ContentPrivate ContentType = "private"
	// ContentError for error pages - no cache
	ContentError ContentType = "error"
)

// SetCacheHeaders sets appropriate Cache-Control headers per AI.md PART 9
// | Content Type | Cache-Control Header | Description |
// |--------------|---------------------|-------------|
// | Static assets | public, max-age=31536000, immutable | 1 year, fingerprinted |
// | HTML pages | no-store | Always fetch fresh |
// | API responses (public) | public, max-age=60 | Short cache for CDN |
// | API responses (private) | private, no-store | User-specific data |
// | Authenticated pages | private, no-store | Never cache |
// | Error pages | no-store | Don't cache errors |
func SetCacheHeaders(w http.ResponseWriter, contentType ContentType, isAuthenticated bool) {
	// Authenticated requests always get no-store
	if isAuthenticated {
		w.Header().Set("Cache-Control", "private, no-store")
		return
	}

	switch contentType {
	case ContentStatic:
		// Static assets with fingerprinted URLs - cache forever
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case ContentAPI:
		// Public API responses - short cache for CDN
		w.Header().Set("Cache-Control", "public, max-age=60")
	case ContentHTML:
		// HTML pages - always fresh
		w.Header().Set("Cache-Control", "no-store")
	case ContentPrivate:
		// Private/authenticated content
		w.Header().Set("Cache-Control", "private, no-store")
	case ContentError:
		// Error pages - never cache
		w.Header().Set("Cache-Control", "no-store")
	default:
		// Default to no-store for safety
		w.Header().Set("Cache-Control", "no-store")
	}
}

// SetStaticCacheHeaders is a convenience for static assets
func SetStaticCacheHeaders(w http.ResponseWriter) {
	SetCacheHeaders(w, ContentStatic, false)
}

// SetAPICacheHeaders sets headers for public API responses
func SetAPICacheHeaders(w http.ResponseWriter) {
	SetCacheHeaders(w, ContentAPI, false)
}

// SetNoCache ensures content is never cached
func SetNoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
