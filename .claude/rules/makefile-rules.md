# Makefile Rules (PART 26)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Makefile (local dev only, NOT CI/CD)

## CRITICAL - NEVER DO

- Use the Makefile in CI/CD - workflows have explicit commands with all env vars
- Run go directly on host - all builds inside Docker `golang:alpine`
- Add targets beyond the six core ones - PART 26 says "DO NOT ADD MORE"

## CRITICAL - ALWAYS DO

- Provide exactly the six core targets: `dev`, `local`, `build`, `test`, `release`, `docker`
- Output binaries to ${TMPDIR}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX (dev) or binaries/ (local/build)
- Tabs for Makefile indentation

---
For complete details, see AI.md PART 26
