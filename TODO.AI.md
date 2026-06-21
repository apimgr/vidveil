# TODO.AI.md — vidveil Outstanding Items

## [ ] Fix Makefile Docker invocation pattern
`casjaysdev/go:latest` entrypoint wraps commands via `bash --login -c "$cmdExec"` where `$cmdExec` is only `arg[0]`, so `go build -o ./binaries/vidveil ./src` becomes just `go` (no subcommand = prints usage, exits 2). All `make dev`, `make build`, `make test`, `make local` targets fail. Fix: either write build scripts to `/tmp` and mount them, or update `GO_DOCKER` to pass a script file via a volume mount. Build via `docker run ... casjaysdev/go:latest /path/to/build.sh` works correctly.
Read: AI.md PART 26

## [x] Create GitHub Actions CI/CD workflows
Created:
- `.github/workflows/ci.yml` — lint, test (≥60% coverage), build, vuln-check, secret-scan
- `.github/workflows/release.yml` — 8-platform matrix release on tag push
- `Jenkinsfile` — full parallel build (8 platforms), conditional CLI build, daily/beta/stable triggers
All Actions pinned to full commit SHA. Go project: `casjaysdev/go:latest` used directly (no build-toolchain.yml).
Read: AI.md PART 28

## [ ] Verify SSE streaming search endpoint is complete
Check that `/api/{api_version}/search` streams SSE results via `text/event-stream` with correct event format (`type:result`, `type:done`). Verify fallback `?format=json` returns full JSON response. Confirm all 43 engines in IDEA.md have corresponding engine files in `src/server/service/engine/`.
Read: AI.md PART 14

## [ ] Verify privilege drop (root → vidveil user) is implemented
`src/server/service/system/privilege_unix.go` exists but confirm it performs: bind port → drop to `vidveil` system user via setuid/setgid. Verify `vidveil service install` creates the `vidveil` system user and sets directory ownership.
Read: AI.md PART 23

## [ ] Verify `server.yml` first-run random port selection
On first run with no `server.yml`, the port should be randomly selected from 64000-64999 and saved to `server.yml`. Confirm this is implemented in `src/config/config.go`.
Read: AI.md PART 5

## [ ] Verify Makefile `make test` target works correctly
After fixing the Docker invocation issue (see above), run `make test` to confirm all unit tests pass. Coverage output must go to `/tmp/coverage.out` inside the container, not the project tree.
Read: AI.md PART 29
