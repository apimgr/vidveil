# Makefile Rules (PART 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Invoke Makefile targets from CI/CD; workflows must use explicit provider commands.
- Run Go build, test, or execution commands directly on the host.
- Copy or symlink generated binaries into tracked source locations.
- Guess version, commit, architecture, or build metadata that a command can provide.

## CRITICAL - ALWAYS DO
- Provide the required local build, release, Docker, test, and development targets with the exact semantics in PART 25.
- Run local toolchain work in the maintained Go container with isolated/project-scoped build caches.
- Build all eight required server and client platform targets with CGO disabled.
- Keep debug artifacts, production release artifacts, metadata, checksums, and versioning in their specified locations and formats.
- Make clean/remove behavior narrowly scoped to project-generated artifacts.

## KEY RULES
- Local Makefile version precedence and CI version derivation differ; follow the defined precedence exactly.
- `make test` is the local quality gate and must include every check required by PART 25 and PART 28.

For complete details, see AI.md PART 25.
