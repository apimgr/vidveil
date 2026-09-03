# Contributing to VidVeil

Thanks for your interest in contributing. This document explains how to work
with the codebase day-to-day. The full implementation specification lives in
`AI.md`; the project plan and feature scope live in `IDEA.md`. Both are the
source of truth — please read the relevant PART of `AI.md` before changing
behavior.

## Local Setup

### Prerequisites

- A POSIX shell (`bash`).
- Docker (or a compatible runtime). All builds and tests run inside containers
  — VidVeil does not require Go to be installed on your host machine.
- Optional: [Incus](https://linuxcontainers.org/incus/) for full-stack
  integration tests with systemd. Falls back to Docker if not present.

### Clone

```bash
git clone https://github.com/apimgr/vidveil.git
cd vidveil
```

## Build, Test, Run

VidVeil ships a Makefile for local development only. CI/CD never invokes the
Makefile — it runs explicit commands per `.github/workflows/build.yml`.

| Command           | What it does                                                        |
|-------------------|---------------------------------------------------------------------|
| `make dev`        | Quick development build into `${TMPDIR}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX/`. |
| `make local`      | Production-style build with version info into `binaries/`.          |
| `make build`      | Cross-platform release build (8 targets) into `binaries/`.          |
| `make test`       | Run `go test` inside `golang:alpine`.                               |
| `make release`    | Build and stage release artifacts in `releases/`.                   |
| `make docker`     | Build and push the container image.                                 |

To run integration tests:

```bash
./tests/run_tests.sh         # auto-detects incus or docker
./tests/incus.sh             # full systemd integration (preferred)
./tests/docker.sh            # ephemeral docker-only smoke tests
```

## Branching and PRs

1. Create a topic branch off `main` (`feat/...`, `fix/...`, `docs/...`).
2. Keep commits small and focused — one logical change per commit.
3. Open a pull request to `main`. The PR template will prompt you for the
   summary, motivation, test evidence, and security/privacy impact.
4. CI must pass before merge. Required checks: `Build`, `Security`,
   `Docker Build`, plus any release/release-builder workflows.

## Tests and Documentation Updates Are Required

If your change touches behavior, you must update the relevant tests and
documentation in the same PR. See `AI.md` PART 29 (Testing & Development) and
PART 30 (ReadTheDocs Documentation) for the rules. In particular:

- Add or update `*_test.go` next to the code you touched.
- Update `docs/*.md` (ReadTheDocs) for any user-, admin-, operator-, or
  integration-facing change.
- Update `IDEA.md` when feature scope, data models, or roles change.
- Update the OpenAPI annotations and GraphQL schema when the API surface
  changes.

## Code Style

- `gofmt`-clean (CI enforces this).
- `go vet`-clean.
- Comments go **above** the code line, never inline (see `AI.md` PART 0).
- Pure Go only — `CGO_ENABLED=0` is required.

## Reporting Vulnerabilities

Please **do not** file public issues for security problems. Use the private
reporting path described in `.github/SECURITY.md`.

## License

By contributing you agree that your contributions are licensed under the MIT
License (see `LICENSE.md`).
