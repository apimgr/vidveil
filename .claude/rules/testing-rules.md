# Testing Rules (PART 29, 30, 31)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Testing & Development, ReadTheDocs Documentation, I18N & A11Y

## CRITICAL - NEVER DO

- Run forbidden host commands during tests/debugging (reboot, network, mount, package mgmt, etc.) - container/VM only
- Mock the database for integration tests - hit a real DB
- Pin Go version - latest stable always
- Skip translation when adding user-facing text

## CRITICAL - ALWAYS DO

- `tests/run_tests.sh` auto-detects incus vs docker
- Incus preferred for full systemd / persistent integration testing
- Docker for ephemeral quick checks
- AI is a beta tester: try edge cases, break it, fix it, verify
- ReadTheDocs site_url matches RTD project format
- Translation parity across all locale files (en/es/fr/de/zh/ar/ja)
- When adding user-facing text, add translation keys to en.json + at least Spanish example, then note other locales need it
- <html lang="{{.Lang}}" dir="{{.Dir}}"> - never hardcoded lang="en"

---
For complete details, see AI.md PART 29, 30, 31
