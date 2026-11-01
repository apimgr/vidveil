# Makefile Rules (PART 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-31; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never add Makefile targets beyond the six core ones: `dev`, `local`, `build`, `test`, `release`, `docker`
- Never run `go` directly on the host — every target shells out through Docker (`casjaysdev/go:latest`)
- Never hardcode `PROJECT_NAME`/`PROJECT_ORG` in the Makefile — infer from `git remote get-url origin`, fallback to directory path
- Never add a `v` prefix to a non-numeric version tag (`dev`, `beta`, `devel`, timestamps, commit ids) — `v` prefix ONLY for numeric semver (`1.2.3` → `v1.2.3`)
- Never double a `v` prefix (`vv1.2.3` is always wrong)
- Never guess or infer `site.txt` / `{official_site}` from project name or domain — it must be explicitly created by the user or come from `OFFICIAL_SITE` env/CI secret
- Never symlink or copy binaries out of `binaries/` (to `/usr/local/bin`, `/tmp`, `releases/` manually, etc.) — run directly from `binaries/`; the only exception is the CI/CD release process
- Never skip `make test` coverage gate — 60% minimum (`go test -cover`) enforced, override upward only in IDEA.md, never downward, never bypass with `-short`
- Never let `make release` be used for anything but manual local stable releases — beta/devel releases are CI/CD only
- Never allow more than 1 "devel" release to exist — each devel build deletes and replaces the previous one (single rolling tag literally named `devel`)

## CRITICAL - ALWAYS DO

- Always embed `BuildEpoch` (Unix timestamp), `Version`, `CommitID`, `OfficialSite` via `-ldflags -X` at build time — `BuildDate` is derived at runtime from `BuildEpoch`, never embedded directly as its own ldflag
- Always use `CGO_ENABLED=0` and `GOFLAGS=-buildvcs=false` for all Go Docker builds (mounted `.git` UID mismatch otherwise fails with exit 128)
- Never add a cleanup target or delete build/release output as a side effect of `build` or `local`
- Always output `dev` builds to `${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX/` (isolated, random) — never to `binaries/`
- Always strip binaries (`strip`) before copying into `releases/` during `make release`
- Always prefer `release.txt` (if present) as version source of truth over `VERSION` env var over `devel` fallback
- Always generate both `sha256.txt` and `sha512.txt` checksum files alongside every release artifact set
- Always use `./tests/incus.sh` (preferred, full OS/systemd) or `./tests/run_tests.sh` (auto-detect) for integration testing after `make dev`/`make local`

## Key Rules Summary

**Six targets:**
| Target | Purpose | Output | Notes |
|--------|---------|--------|-------|
| `dev` | quick dev build | `${TMPDIR}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX/` | local platform only, no ldflags |
| `local` | prod test build | `binaries/` | local platform, full ldflags |
| `build` | full release build | `binaries/` | all 8 platforms |
| `test` | unit tests | coverage report | 60% minimum gate |
| `release` | manual local release | `releases/` | strips binaries, source tar.gz + checksums, `gh release create` |
| `docker` | build+push container | `$REGISTRY` | `docker buildx`, multi-arch, `--push`, `DOCKER_MEM ?= 4g`, `DOCKER_CPUS ?= 2` |

**Build matrix (8 platforms):** linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, freebsd/arm64.

**Binary naming:** server `{project_name}[-{os}-{arch}][.exe]` → for vidveil: `vidveil`, `vidveil-linux-amd64`, `vidveil-windows-amd64.exe`, etc. CLI (if `src/client/` exists): `vidveil-cli-{os}-{arch}`.

**Version precedence:** 1) `release.txt` file (wins if present) 2) `VERSION` env var 3) `devel` fallback.

**Version tag `v` prefix rule:**
| Input | Add `v`? | Result |
|-------|----------|--------|
| `0.2.0`, `1.2.3` (numeric semver) | YES | `v0.2.0`, `v1.2.3` |
| `v1.2.0` (already has v) | NO (unchanged) | `v1.2.0` |
| `dev`, `beta`, `devel` (text) | NO | `dev`, `beta`, `devel` |
| `20251218060432` (beta timestamp) | NO | `20251218060432` |
| `a1b2c3d` (devel short commit id) | NO | `a1b2c3d` |

**site.txt (optional):** single-line official site URL, wins over IDEA.md/README/env/CI. Sources in order: `site.txt` file → `OFFICIAL_SITE` env var → CI secret → empty (self-hosted, requires `--server` flag). Never guess.

**Go module caching:** `GO_CACHE` (`~/go/pkg/mod` → `/usr/local/share/go/pkg/mod`), `GO_BUILD` (`~/.cache/go-build/{project_name}` → `/usr/local/share/go/cache`) — persistent across builds, shared across projects.

**Release types:**
| Type | Trigger | Version format | v prefix | Max releases |
|------|---------|-----------------|----------|---------------|
| Stable | tag push `v*`/`*.*.*` | `X.Y.Z` | YES | unlimited |
| Beta | push to `beta` branch | `{YYYYMMDDHHMMSS}-beta` | NO | unlimited |
| Devel | 3am UTC schedule + push to main | short commit id (`git rev-parse --short=7 HEAD`) | NO | **1** (single rolling tag `devel`, replaces previous) |

Devel identity is the commit being built, not `release.txt` and not a timestamp — this differs from beta, which still uses a UTC timestamp version.

**Release artifacts (every GitHub release):** all 8 server binaries + all 8 CLI binaries (if CLI exists) + `version.txt` (version string only) + `sha256.txt` + `sha512.txt` (checksums of every binary artifact) + `{project_name}-{version}-source.tar.gz` (excludes `.git`, `.github`, `binaries/`, `releases/`).

**Local dev workflow:** `make dev` (code) → `make test` (Phase 1 toolchain gate, pre-commit) → `./tests/run_tests.sh` (Phase 2 binary validation) → `make local` (prod test) → `./tests/incus.sh` (preferred full OS test) → `make build` (cross-platform, pre-release).

**Never copy/symlink binaries:** run via `./binaries/vidveil` directly; the only exception is CI/CD's release process (build → strip → checksum → upload).

For complete details, see AI.md PART 25
