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
  — VidVeil does not require Go to be installed on your local machine.
- Optional: [Incus](https://linuxcontainers.org/incus/) for the full systemd
  and OpenRC integration suite. Basic tests always use Docker.

### Clone

```bash
git clone https://github.com/apimgr/vidveil.git
cd vidveil
```

## Build, Test, Run

VidVeil ships a Makefile for local development only. CI/CD never invokes the
Makefile — it runs explicit commands in the provider workflows. The repository
ships GitHub Actions (`.github/workflows/`), GitLab CI (`.gitlab-ci.yml`),
Gitea Actions (`.gitea/workflows/`), Forgejo Actions (`.forgejo/workflows/`),
and Jenkins (`Jenkinsfile`). Only the provider configured for a given mirror runs its
workflows.

| Command           | What it does                                                        |
|-------------------|---------------------------------------------------------------------|
| `make dev`        | Quick development build into `${TMPDIR}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX/`. |
| `make local`      | Production-style build with version info into `binaries/`.          |
| `make build`      | Native-platform server/CLI plus cross-platform server/CLI release binaries (8 targets) into `binaries/`. |
| `make test`       | Run unit tests and coverage checks in `casjaysdev/go:latest`.         |
| `make release`    | Build, stage, and publish a stable release.                           |
| `make docker`     | Build and push the container image.                                 |

To run integration tests:

```bash
./tests/run_tests.sh         # Docker-only basic integration tests
./tests/docker.sh            # ephemeral Docker binary checks
./tests/incus.sh              # full systemd and OpenRC integration tests
```

## Branching and PRs

1. Create a topic branch off `main` (`feat/...`, `fix/...`, `docs/...`).
2. Keep commits small and focused — one logical change per commit.
3. Open a pull request to `main`. The PR template will prompt you for the
   summary, motivation, test evidence, and security/privacy impact.
4. The configured git provider's CI workflow must pass before merge. GitHub CI runs
   lint, tests with coverage, build, vulnerability scanning, secret scanning,
   image scanning, and workflow policy checks; the Docker workflow builds and
   scans container images separately.

## Tests and Documentation Updates Are Required

If your change touches behavior, you must update the relevant tests and
documentation in the same PR. See `AI.md` PART 28 (Testing & Development) and
PART 29 (ReadTheDocs Documentation) for the rules. In particular:

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
