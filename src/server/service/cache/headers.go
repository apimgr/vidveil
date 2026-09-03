// SPDX-License-Identifier: MIT
// AI.md PART 9: HTTP Cache Headers

package cache

import (
	"net/http"

	"github.com/apimgr/vidveil/src/common/version"
)

// ContentType for cache header selection
type ContentType string

const (
	// ContentStatic for JS/CSS/images - immutable only with a matching ?v= stamp
	ContentStatic ContentType = "static"
	// ContentHTML always fetch fresh
	ContentHTML ContentType = "html"
	// ContentAPI for public API - 60s cache
	ContentAPI ContentType = "api"
	// ContentSW for /sw.js and /manifest.json - no-cache + build-stamp ETag
	ContentSW ContentType = "sw"
	// ContentPrivate for authenticated - no cache
	ContentPrivate ContentType = "private"
	// ContentError for error pages - no cache
	ContentError ContentType = "error"
)

// SetCacheHeaders sets appropriate Cache-Control headers per AI.md PART 9
// | Content Type | Cache-Control Header | Description |
// |--------------|---------------------|-------------|
// | Static assets with matching ?v= stamp | public, max-age=31536000, immutable | 1 year, URL changes every release |
// | Static assets without/mismatched ?v= | no-cache + ETag | Always revalidated |
// | HTML pages | no-store | Always fetch fresh |
// | /sw.js and /manifest.json | no-cache + ETag | New SW must be seen on next update check |
// | API responses (public) | public, max-age=60 | Short cache for CDN |
// | API responses (private) | private, no-store | User-specific data |
// | Authenticated pages | private, no-store | Never cache |
// | Error pages | no-store | Don't cache errors |
func SetCacheHeaders(w http.ResponseWriter, r *http.Request, contentType ContentType, isAuthenticated bool) {
	// Authenticated requests always get no-store
	if isAuthenticated {
		w.Header().Set("Cache-Control", "private, no-store")
		return
	}

	switch contentType {
	case ContentStatic:
		// Immutable ONLY when the request's ?v= equals the running build stamp
		// per PART 9 asset version-busting; otherwise revalidate every time.
		if r != nil && r.URL.Query().Get("v") == version.AssetStamp() {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("ETag", `"`+version.AssetStamp()+`"`)
		}
	case ContentAPI:
		// Public API responses - short cache for CDN
		w.Header().Set("Cache-Control", "public, max-age=60")
	case ContentHTML:
		// HTML pages - always fresh
		w.Header().Set("Cache-Control", "no-store")
	case ContentSW:
		// /sw.js and /manifest.json - never long-cached, or updates stall
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", `"`+version.AssetStamp()+`"`)
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
func SetStaticCacheHeaders(w http.ResponseWriter, r *http.Request) {
	SetCacheHeaders(w, r, ContentStatic, false)
}

// SetAPICacheHeaders sets headers for public API responses
func SetAPICacheHeaders(w http.ResponseWriter) {
	SetCacheHeaders(w, nil, ContentAPI, false)
}

// SetNoCache ensures content is never cached
func SetNoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
