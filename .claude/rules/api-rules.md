# API Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Skip either `/healthz` or `/api/{api_version}/healthz`
- Use mismatched success and error response shapes
- Put OpenAPI endpoints under `/api/{api_version}/`
- Hardcode the public domain
- Skip built-in Let's Encrypt support

## CRITICAL - ALWAYS DO

- Expose both web and API health endpoints independently
- Use unified response envelopes
- Follow content negotiation rules
- Serve `/openapi` as HTML and `/openapi.json` as JSON
- Resolve FQDN from proxy headers, env, hostname, then public IP fallback

## Endpoint Pattern

- Web routes return HTML for browsers
- API routes return JSON for automation
- Every important web page has a corresponding API route
- GraphQL and OpenAPI stay aligned with implemented routes

## Health Rules

- `/healthz` and `/api/{api_version}/healthz` both exist
- Health output includes status, version, build info, mode, features, and checks
- Do not expose sensitive internal details in health data

For complete details, see AI.md PART 13, PART 14, PART 15
