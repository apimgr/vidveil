# API Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Leave a defined API route unimplemented, undocumented, or inconsistent with its contract.
- Expose private diagnostics on public endpoints or bypass base URL, validation, authentication, or rate-limit rules.
- Weaken TLS or certificate validation contrary to PART 15.
- Let generated OpenAPI/Swagger output drift from the actual route registry and handlers.

## CRITICAL - ALWAYS DO
- Implement required health, readiness, version, and operational endpoints with their exact public/private data classifications.
- Use the API versioning, route structure, status codes, pagination, error, and content-negotiation rules in PART 14.
- Keep IDEA.md’s route inventory, handler behavior, route registry, Swagger/OpenAPI, and docs synchronized.
- Validate TLS configuration, ACME behavior, certificate lifecycle, and challenge inputs per PART 15.
- Apply the request limits, trusted-proxy, Tor semantics, and base URL rules from PART 12.

## KEY RULES
- Public health output must not leak Tier 1 or Tier 3 information.
- Route changes require matching API documentation and tests for supported response formats.

For complete details, see AI.md PART 13, 14, 15.
