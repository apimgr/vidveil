// Package outsan implements the Output Sanitization Pipeline required by
// AI.md PART 11 "Output Sanitization Pipeline". Every public JSON response
// passes through this single chokepoint before it leaves the server —
// features cannot opt out, only add additional filters upstream.
//
// Stages implemented here (numbering matches AI.md PART 11):
//  2. Redact known-sensitive query params found in any string field.
//  3. Strip internal IPs / filesystem paths from any string field.
//  4. Truncate long fields (URL-like, message-like, stack-like, and a
//     generic hard cap as a DOS backstop).
//  5. Strip dev-only fields (JSON keys with a leading underscore, e.g.
//     "_debug", "_internal_id") when running in production mode.
//
// Stage 1 (allow-list fields) is satisfied by construction: every handler
// in this codebase marshals a purpose-built response struct/map rather than
// a raw DB row, so there is no unlisted field to drop here. Stage 6
// (constant-time finalize) is a response-timing concern, not a payload
// concern, and lives in package authpad instead.
package outsan

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// sensitiveQueryParams are the query parameter names redacted from any
// string field that looks like a URL, per AI.md PART 11 stage 2. Matched
// case-insensitively.
var sensitiveQueryParams = map[string]struct{}{
	"token": {}, "session": {}, "code": {}, "key": {}, "password": {},
	"secret": {}, "auth": {}, "pwd": {}, "api_key": {}, "apikey": {},
	"access_token": {}, "refresh_token": {},
}

// internalIPPattern matches RFC1918 / loopback / link-local IPv4 prefixes
// per the exact regex given in AI.md PART 11 stage 3.
var internalIPPattern = regexp.MustCompile(`\b(10\.|172\.(1[6-9]|2[0-9]|3[01])\.|192\.168\.|127\.|169\.254\.)[0-9.]+`)

// internalPathPattern matches absolute filesystem paths under well-known
// system/source-tree directories per AI.md PART 11 stage 3 ("filesystem
// paths in any string field"). Deliberately scoped to system-style prefixes
// so ordinary application URL paths (e.g. "/search?q=") are never touched.
var internalPathPattern = regexp.MustCompile(`(?:/(?:etc|var|root|home|usr/(?:local/)?(?:share|lib)|proc|opt)/[^\s"'<>]*)|(?:[A-Za-z]:\\[^\s"'<>]*)`)

// Truncation limits per AI.md PART 11 stage 4.
const (
	urlFieldLimit     = 256
	messageFieldLimit = 200
	stackFieldLimit   = 2048
	// genericHardCap is a DOS backstop applied to any string regardless of
	// field name — ordinary product content (titles, descriptions, etc.)
	// never approaches this length, so it never truncates legitimate data.
	genericHardCap = 8192
)

var (
	urlLikeKey     = regexp.MustCompile(`(?i)(url|link|href)`)
	messageLikeKey = regexp.MustCompile(`(?i)(message|error|reason|detail|sample)`)
	stackLikeKey   = regexp.MustCompile(`(?i)(stack|trace)`)
)

// Sanitize runs data through the Output Sanitization Pipeline and returns a
// value safe to hand to json.MarshalIndent. production gates stage 5
// (dev-only field stripping) — dev mode keeps underscore-prefixed fields
// for troubleshooting per AI.md PART 11.
func Sanitize(data interface{}, production bool) interface{} {
	// Round-trip through the generic JSON tree so field-name-based rules
	// (stage 4/5) apply uniformly regardless of whether the caller passed a
	// struct or a map — this is the "single chokepoint" AI.md requires.
	raw, err := json.Marshal(data)
	if err != nil {
		// Marshal failure is handled by the caller (WriteJSON already has a
		// fallback error path) — return the original value unsanitized
		// rather than swallowing the error here.
		return data
	}
	var tree interface{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return data
	}
	return walk("", tree, production)
}

// walk recursively applies stages 2-5 to v. key is the JSON object key that
// held v (empty for array elements and the document root), used to pick the
// field-name-based truncation limit.
func walk(key string, v interface{}, production bool) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			// Stage 5: drop dev-only fields (leading underscore) in production.
			if production && strings.HasPrefix(k, "_") {
				continue
			}
			out[k] = walk(k, child, production)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, child := range val {
			out[i] = walk(key, child, production)
		}
		return out
	case string:
		return sanitizeString(key, val)
	default:
		return v
	}
}

// sanitizeString applies stages 2-4 to a single string field.
func sanitizeString(key, s string) string {
	s = redactQueryParams(s)
	s = internalIPPattern.ReplaceAllString(s, "[redacted]")
	s = internalPathPattern.ReplaceAllString(s, "[redacted]")
	return truncate(key, s)
}

// redactQueryParams parses s as a URL (best-effort) and blanks any query
// parameter named in sensitiveQueryParams; non-URL strings are returned
// unchanged. Per AI.md PART 11 stage 2, matched case-insensitively.
func redactQueryParams(s string) string {
	if !strings.Contains(s, "?") || !strings.Contains(s, "=") {
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.RawQuery == "" {
		return s
	}
	q := u.Query()
	redacted := false
	for name := range q {
		if _, ok := sensitiveQueryParams[strings.ToLower(name)]; ok {
			q.Set(name, "[redacted]")
			redacted = true
		}
	}
	if !redacted {
		return s
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// truncate applies the stage-4 length cap that matches key's role, falling
// back to the generic DOS-backstop cap for everything else.
func truncate(key, s string) string {
	limit := genericHardCap
	switch {
	case urlLikeKey.MatchString(key):
		limit = urlFieldLimit
	case stackLikeKey.MatchString(key):
		limit = stackFieldLimit
	case messageLikeKey.MatchString(key):
		limit = messageFieldLimit
	}
	if len(s) <= limit {
		return s
	}
	// Truncate on a rune boundary to avoid splitting multi-byte UTF-8.
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
