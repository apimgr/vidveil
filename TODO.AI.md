# TODO.AI.md — vidveil Outstanding Items

## [ ] Fix Makefile cross-compile targets (build/dev/local)
`casjaysdev/go:latest` entrypoint wraps only `arg[0]`, so `$(GO_DOCKER) sh -c "GOOS=linux GOARCH=arm64 go build ..."` becomes `bash --login -c "sh"` — the `sh -c "..."` string is dropped. Affects `make build`, `make dev`, `make local` (all use `sh -c` to set GOOS/GOARCH inline). `make test` is NOT affected — it uses `$(GO_DOCKER) go test` directly and passes.
Fix: replace inline `sh -c "GOOS=$$OS GOARCH=$$ARCH go build ..."` with `-e GOOS=$$OS -e GOARCH=$$ARCH` env flags directly in the `docker run` command.
Read: AI.md PART 26

## [x] Create GitHub Actions CI/CD workflows
Created:
- `.github/workflows/ci.yml` — lint, test (≥60% coverage), build, vuln-check, secret-scan
- `.github/workflows/release.yml` — 8-platform matrix release on tag push
- `Jenkinsfile` — full parallel build (8 platforms), conditional CLI build, daily/beta/stable triggers
All Actions pinned to full commit SHA. Go project: `casjaysdev/go:latest` used directly (no build-toolchain.yml).
Read: AI.md PART 28

## [x] Verify SSE streaming search endpoint is complete
`/api/v1/search` streams SSE via `handleSearchSSE` (handlers.go:1796). Sets correct headers (`text/event-stream`, `Cache-Control: no-cache`). Results emitted as `data: {...}\n\n` with final `data: {"done":true,...}\n\n` sentinel. `?format=json` fallback returns synchronous JSON. 43 engines registered in manager.go matching IDEA.md.
Read: AI.md PART 14

## [x] Verify privilege drop (root → vidveil user) is implemented
`privilege_unix.go:20–76`: `DropPrivileges` does Setgroups → Setgid → Setuid then verifies `os.Getuid() != 0`. Creates system user if missing. Called from `main.go:653–671` after `srv.Listen()` (port bind) and before server goroutine starts — correct sequence. `--service --install` creates all dirs with `MkdirAll(0755)` and `chown -R vidveil:vidveil`.
Read: AI.md PART 23

## [x] Verify `server.yml` first-run random port selection
`config.go:1134–1148`: when `server.yml` absent, `DefaultAppConfig()` calls `findUnusedPort()` (line 799) which probes 64000–64999 via `net.Listen` and returns the first free port. Config saved to `/etc/apimgr/vidveil/server.yml` (root) or `~/.config/apimgr/vidveil/server.yml` (non-root) via `paths.go:70–72`.
Read: AI.md PART 5

## [x] Verify Makefile `make test` target works correctly
`make test` passes — uses `$(GO_DOCKER) go test -v -cover ./...` directly (not `sh -c`), so the entrypoint wrapping does not affect it. All packages pass. Note: coverage output goes to container stdout; no `-coverprofile` written to disk (acceptable for `make test`; CI uses `$GITHUB_ENV` COVDIR pattern).
Read: AI.md PART 29
