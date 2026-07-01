# TODO.AI.md — vidveil Outstanding Items

Generated: 2026-06-30
Source: AI.md compliance audit (full 44k line spec)

---

## HIGH PRIORITY (Core functionality gaps)

### [x] PART 9: Error Handling & Caching — COMPLETE
All implemented in `src/server/handler/response.go` and `src/server/service/cache/cache.go`:
- `APIResponse` struct with `OK`, `Data`, `Error`, `Message` fields
- `SendOK()` and `SendError()` functions
- `ErrorCodeToHTTP()` maps codes to HTTP status
- All error code constants (`CodeBadRequest`, `CodeValidation`, etc.)
- All error message constants (`MsgBadRequest`, `MsgValidation`, etc.)
- Retry with exponential backoff in `src/server/service/retry/retry.go`
- `WarmableCache` interface and `Warm()` method added to cache
Read: AI.md PART 9

### [ ] PART 11: Security & Logging (Critical)
- [x] Sec-Fetch-* validation middleware (secFetchValidationMiddleware in server.go:894)
- [x] Constant-time comparison for metrics token (handler/metrics.go:265)
- [ ] Add `app_secrets` table with `installation_secret`, `cookie_signing_key` columns
- [ ] Generate/store 32-byte `installation_secret` on first startup
- [ ] Use `installation_secret` for HMAC operations (security.txt ID, PGP key encryption)
Note: VidVeil is stateless/privacy-first (no user accounts), so "identical auth response timing" for user auth is N/A.
Read: AI.md PART 11

### [ ] PART 17: Email & Notifications
- Implement WebUI notification center (toast/banner system) with bell icon
- Add notification storage in database table
- Implement SMTP auto-detection (checking multiple hosts/ports in priority order)
Read: AI.md PART 17

### [ ] PART 18: Scheduler — Missing Builtin Tasks
- Add `ssl_renewal` task
- Add `blocklist_update` task
- Add `cve_update` task
- Add `log_rotation` task
- Add `token_cleanup` task
- Add `healthcheck_self` task
- Add `tor_health` task
Read: AI.md PART 18

### [ ] PART 23-24: Privilege Escalation & Service
- Implement smart escalation detection (check if user can escalate via sudoers/wheel)
- Implement multi-platform escalation (sudo/su/pkexec/doas/osascript/runas)
- Fix reserved UID checking to use `reservedIDs` map per spec
- Implement macOS user creation via `dscl`
- Implement Windows Virtual Service Account setup
- Implement OpenRC init script (`installOpenRC()` method)
- Implement s6 init system support
- Fix systemd unit to use `{project_org}` placeholder instead of hardcoded `apimgr`
- Fix launchd plist format (`io.github.{project_org}.{internal_name}` not `apimgr.{appName}`)
Read: AI.md PART 23, PART 24

### [ ] PART 32: Implement native GUI mode for vidveil-cli
AI.md PART 32 requires "Full GUI app (GTK/Cocoa/Win32)" for DisplayModeGUI. Currently only bubbletea TUI is implemented.
- Use fyne.io cross-platform GUI toolkit (pure Go, no CGO required for basic apps)
- Implement same functionality as TUI: server list, search, favorites, settings
- Add --gui flag to launch GUI mode
- Detect DISPLAY/WAYLAND_DISPLAY on Linux, always available on macOS/Windows
Read: AI.md PART 32

---

## MEDIUM PRIORITY (Feature completeness)

### [ ] PART 10: Database — libsql/Turso
- Add libsql/Turso support for remote SQLite
Read: AI.md PART 10

### [ ] PART 15: SSL/TLS & Let's Encrypt
- Complete DNS-01 challenge provider implementations
Read: AI.md PART 15

### [ ] PART 20: Metrics
- Add missing metric: `rate_limit_hits_total`
- Add system metrics (CPU/memory/disk)
Read: AI.md PART 20

### [ ] PART 13: Health & Versioning
- Verify all spec fields populated in health response (project.tagline, stats.requests_total)
Read: AI.md PART 13

### [ ] PART 21: Backup & Restore
- Verify backup verification (checksum, extract test, db integrity)
- Verify retention policy cleanup logic
Read: AI.md PART 21

### [ ] PART 22: Update Command
- Verify full self-update with platform-specific binary replacement
Read: AI.md PART 22

---

## LOW PRIORITY (Polish/edge cases)

### [ ] PART 7: Binary Requirements
- Create `src/data/` directory for embedded application data JSON files
- Add display environment detection files (`detect_unix.go`, `detect_windows.go`)
Read: AI.md PART 7

### [ ] PART 12: Server Configuration
- Verify webhook adapters (Telegram/Discord/Slack sending)
Read: AI.md PART 12

### [ ] PART 31: Tor Hidden Service
- Verify torrc generation against spec
Read: AI.md PART 31

---

## Completed

### [x] Fix Makefile cross-compile targets (build/dev/local)
Rewrote Makefile per AI.md PART 25: spec variable names (GO_CACHE, GO_BUILD, OFFICIALSITE, PROJECTNAME/PROJECTORG), spec mount paths (/app, /usr/local/share/go/pkg/mod, /usr/local/share/go/cache), spec targets (build: clean, local: clean), 80% coverage enforcement in test with temp-dir isolation, dev writes to $TMPDIR/$PROJECTORG/$PROJECTNAME-XXXXXX. Cross-compile uses -e GOOS/-e GOARCH env flags (not sh -c which the entrypoint drops); test and dev use -v $$DIR:$$DIR volume mounts. GO_DOCKER defined per spec (includes image); _GO_OPTS is internal helper for cases needing extra flags before image.
make test passes: 80% coverage ✓, darwin/arm64 cross-compile confirmed ✓
Read: AI.md PART 25

### [x] Create GitHub Actions CI/CD workflows
Created:
- `.github/workflows/ci.yml` — lint, test (≥60% coverage), build, vuln-check, secret-scan
- `.github/workflows/release.yml` — 8-platform matrix release on tag push
- `Jenkinsfile` — full parallel build (8 platforms), conditional CLI build, daily/beta/stable triggers
All Actions pinned to full commit SHA. Go project: `casjaysdev/go:latest` used directly (no build-toolchain.yml).
Read: AI.md PART 28

### [x] Verify SSE streaming search endpoint is complete
`/api/v1/search` streams SSE via `handleSearchSSE` (handlers.go:1796). Sets correct headers (`text/event-stream`, `Cache-Control: no-cache`). Results emitted as `data: {...}\n\n` with final `data: {"done":true,...}\n\n` sentinel. `?format=json` fallback returns synchronous JSON. 43 engines registered in manager.go matching IDEA.md.
Read: AI.md PART 14

### [x] Verify privilege drop (root → vidveil user) is implemented
`privilege_unix.go:20–76`: `DropPrivileges` does Setgroups → Setgid → Setuid then verifies `os.Getuid() != 0`. Creates system user if missing. Called from `main.go:653–671` after `srv.Listen()` (port bind) and before server goroutine starts — correct sequence. `--service --install` creates all dirs with `MkdirAll(0755)` and `chown -R vidveil:vidveil`.
Read: AI.md PART 23

### [x] Verify `server.yml` first-run random port selection
`config.go:1134–1148`: when `server.yml` absent, `DefaultAppConfig()` calls `findUnusedPort()` (line 799) which probes 64000–64999 via `net.Listen` and returns the first free port. Config saved to `/etc/apimgr/vidveil/server.yml` (root) or `~/.config/apimgr/vidveil/server.yml` (non-root) via `paths.go:70–72`.
Read: AI.md PART 5

### [x] Verify Makefile `make test` target works correctly
`make test` passes — uses `$(GO_DOCKER) go test -v -cover ./...` directly (not `sh -c`), so the entrypoint wrapping does not affect it. All packages pass. Note: coverage output goes to container stdout; no `-coverprofile` written to disk (acceptable for `make test`; CI uses `$GITHUB_ENV` COVDIR pattern).
Read: AI.md PART 29

### [x] Fix scripts and tests to adhere to spec
- Deleted `scripts/completions/` (PART 8: completions built INTO binary)
- Fixed `scripts/generate-licenses.sh` to use `casjaysdev/go:latest` and proper temp dir
- Fixed `scripts/install.sh` to install both server AND CLI binaries
- Enhanced `tests/docker.sh` and `tests/incus.sh` with content negotiation tests
Read: AI.md PART 8, PART 26, PART 28

---

## Summary

- **Total items**: 38
- **HIGH priority**: 6 sections (most critical)
- **MEDIUM priority**: 6 sections
- **LOW priority**: 3 sections
- **Completed**: 7 items
