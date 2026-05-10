# Project Audit (Pass 4 — exhaustive PARTs 8/17/18/19/25/33)

Started: 2026-05-09
AI.md md5: d180bc9dd77aac60b310d787a7796d23 (60,825 lines)

## Issues Found

(All findings closed — see "Fixed" section.)

## Out of Scope / Verified Clean

- PART 8 — all other server flags (`--help`, `--version`, `--shell`, `--mode`, `--config`, `--data`, `--cache`, `--log`, `--backup`, `--pid`, `--address`, `--port`, `--daemon`, `--debug`, `--color`, `--service`, `--maintenance`, `--update`, `--status`) are present.
- PART 18 — `autodetectSMTP()` probes `localhost`, `127.0.0.1`, `172.17.0.1`, the gateway, and runs an EHLO handshake; matches the spec's priority order.
- PART 19 — all required built-in tasks are registered: `ssl_renewal`, `geoip_update`, `blocklist_update`, `cve_update`, `session_cleanup`, `token_cleanup`, `log_rotation`, `backup_auto`, `healthcheck_self`, `tor_health`, `cluster_heartbeat`.
- PART 25 — systemd, OpenRC, SysVinit, runit, launchd, rc.d, and Windows Service installers all exist and match the spec.
- PART 33 — `--shell`, `--server`, `--token`, `--token-file`, `--output`, `--config`, `--color`, `--timeout`, `--debug`, `--update [check|yes|branch <name>]` are wired; cluster failover with autodiscover refresh is in place; cli.yml + token file permission checks (0600) are enforced.

## Fixed

- **F-P8-1, F-P8-2** — `--baseurl PATH` and `--lang CODE` flags added to `src/main.go` (parser + env fallbacks BASEURL/LANG + `printHelp()` updates).
- **F-P19-1** — Built-in scheduler task IDs renamed from `xxx.yyy` to `xxx_yyy`. DB migration (`migrateLegacyTaskIDs()` in `scheduler.go`) renames existing rows in `scheduled_tasks` and `task_history`; called once at the start of `RegisterBuiltinTasks()`.
- **F-P25-1, F-P25-2** — `installOpenRC()` and `installSysVInit()` added to `src/server/service/system/service.go` with detection helpers (`hasOpenRC()`, `hasSysVInit()`) and dispatch order systemd → OpenRC → SysVinit → runit.
- **F-P33-1** — `--update [check|yes|branch <name>|--help]` added to `vidveil-cli`. New `src/client/cmd/update.go` implements GitHub-Releases-based check, SHA-256 verification, atomic replace, and `syscall.Exec` re-exec. Branches persisted in `update-branch` file under the CLI config dir.
- **F-P17-A** — `renderAdminNav()` rewritten in `src/server/handler/admin.go` to point at canonical `/server/{admin_path}/config/...` routes. Nav clicks now resolve.
- **F-P17-B** — Admin web routes mounted at the spec-canonical `/server/{admin_path}/...` prefix in `src/server/server.go`. Cookies (`vidveil_admin_session`, `vidveil_setup_token`, `vidveil_csrf_token`) and all redirects updated to use `appConfig.AdminURLPrefix()`. Legacy `/{admin_path}/*` paths return HTTP 308 to the canonical path.
- **F-P17-C** — Admin web subroutes nested under `/config/...` (was `/server/...`). Admin API moved to `/api/v1/server/{admin_path}/config/...` to match the web pattern; setup wizard moved to `/server/{admin_path}/config/setup`. Admin templates' `{{.AdminPath}}` now resolves to `/server/{admin_path}`.

## Known follow-ups (out of audit scope)

- Hardcoded JS fetch URLs inside `renderDashboard()`, `renderEmailPage()`, `renderSchedulerPage()`, `renderBackupPage()`, and the Tor vanity scripts in `admin.go` reference a non-existent `/api/v1/admin/<resource>` shape that pre-dates this migration. They returned 404 before this audit and still return 404 (now under the `/server/admin/config/...` mount). Replacing them with `h.appConfig.AdminAPIPrefix()`-composed paths is a separate UI fix.
