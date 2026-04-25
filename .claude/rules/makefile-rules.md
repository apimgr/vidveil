# Makefile Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Run Makefile targets in CI/CD when the spec requires explicit commands
- Add extra convenience targets that conflict with the spec
- Run host `go` commands instead of containerized targets

## CRITICAL - ALWAYS DO

- Keep the Makefile limited to the spec-defined local-development targets
- Use Makefile targets for local builds and tests
- Keep version tagging and build output aligned with the spec

## Local Development Targets

- `make dev`
- `make local`
- `make build`
- `make test`

Use explicit commands in CI/CD instead of Makefile wrappers.

For complete details, see AI.md PART 26
