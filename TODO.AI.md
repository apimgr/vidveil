# TODO.AI.md — vidveil Outstanding Items

## [x] Fix Makefile cross-compile targets (build/dev/local)
Rewrote Makefile per AI.md PART 25: spec variable names (GO_CACHE, GO_BUILD, OFFICIALSITE, PROJECTNAME/PROJECTORG), spec mount paths (/app, /usr/local/share/go/pkg/mod, /usr/local/share/go/cache), spec targets (build: clean, local: clean), 80% coverage enforcement in test with temp-dir isolation, dev writes to $TMPDIR/$PROJECTORG/$PROJECTNAME-XXXXXX. Cross-compile uses -e GOOS/-e GOARCH env flags (not sh -c which the entrypoint drops); test and dev use -v $$DIR:$$DIR volume mounts. GO_DOCKER defined per spec (includes image); _GO_OPTS is internal helper for cases needing extra flags before image.
make test passes: 80% coverage ✓, darwin/arm64 cross-compile confirmed ✓
Read: AI.md PART 25

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
