# Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use `.yaml` instead of `.yml`
- Hardcode dev-only values into config defaults
- Trust config input without validation
- Put inline comments in YAML
- Expose settings only in files but not in admin UI

## CRITICAL - ALWAYS DO

- Use `server.yml`
- Validate configuration and normalize paths safely
- Detect application mode at runtime
- Keep every server setting editable through the admin panel
- Use production-safe defaults and explicit validation

## Configuration Basics

- Config file name is always `server.yml`
- YAML comments must go above the line they describe
- Server validates configuration instead of silently ignoring bad input
- Runtime detection decides user vs system paths and production vs development mode

## Mode Rules

- Support production, development, and debug behavior as defined by spec
- Prefer runtime detection over hardcoded mode assumptions
- Use environment and filesystem context to determine app mode

## Server Settings

- Validate limits, addresses, URLs, and file paths
- Keep base URL and network settings consistent with request and proxy handling
- Use safe defaults for timeouts, limits, and storage paths

For complete details, see AI.md PART 5, PART 6, PART 12
