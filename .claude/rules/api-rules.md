# API Rules (PART 13, 14, 15)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Health & Versioning, API Structure, SSL/TLS & Let's Encrypt

## CRITICAL - NEVER DO

- Expose /metrics publicly - internal only
- Return raw stack traces in API errors - generic messages only
- Return ad-hoc top-level fields in errors - use canonical {ok, error, message, details}
- Mix Retry-After into response body - it is an HTTP header (RFC 9110)

## CRITICAL - ALWAYS DO

- Both web (HTML) and API (JSON) for every feature
- Content negotiation: HTML for browsers, JSON for API, text for CLI/HTTP tools
- Mount /healthz only as direct handler when server.healthz.root.enabled=true (NEVER redirect)
- Use `curl -q -LSsf {url}` in all docs/tests/scripts
- TLS1.2+ minimum; auto-detect Lets Encrypt cert paths

---
For complete details, see AI.md PART 13, 14, 15
