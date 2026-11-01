# Binary Rules (PART 7, 8, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-31; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never hand-roll flag parsing with manual `switch`/`os.Args` loops — server uses stdlib `flag` (single-command, no subcommands); client CLI uses `cobra`/`viper` (multi-command)
- Never hardcode the User-Agent, default paths, config keys, or DB identifiers to the actual (possibly renamed) binary filename — always the compiled-in `{project_name}` / `{project_name}-cli`
- Never show the compiled project name in `--help`/`--version`/error messages — always `filepath.Base(os.Args[0])` (the actual, possibly renamed, binary name)
- Never implement UI mode flags (`--tui`, `--gui`, `--cli`) — display mode is auto-detected, never explicitly selected
- Never create/check a PID file inside a container — skip entirely when `isContainer()` is true (the `pidfile: true` config default is ignored)
- Never resolve `~`/`$HOME` after the privilege drop — system-vs-user path mode is decided ONCE from EUID at startup, before dropping privileges, and cached for the process lifetime (a late `$HOME` lookup would nest user-style paths inside the service account's home, which points at `{data_dir}`)
- Never persist a `--server`/`--token`/other flag value to config when the current config value is already valid — flags only save when config is empty or invalid (session-only override otherwise); never clear an existing valid value
- Never use `strconv.ParseBool()` for CLI boolean flags — always `config.ParseBool()`/`config.IsTruthy()`
- Never require a flag when smart detection can determine the value — minimize flags, args ARE the input where possible
- Never ship separate shell-completion files — completions are generated in-binary via `--shell completions`/`--shell init`, always current
- Never require auth to download the CLI binary by default (`cli.binary_download.require_auth` defaults false) — the CLI is the onboarding path, forcing auth on its own download is circular
- Never build an Agent binary by default — it is OPTIONAL, needed only when the server must reach INTO remote machines (collect metrics/logs, execute commands, run jobs); a purely inbound/pull-based project (content API, upload/hosting, git/chat/wiki) never needs one — check IDEA.md before assuming vidveil needs it
- Never give the Agent a setup token, `--port`, `--address`, or WebUI — it is headless and connects out to the server via `--server`/`--token` (or `agent.yml`), never listens for inbound HTTP
- Never escalate privileges from `--help` at any level (main, subcommand, nested) — help exits immediately, never calls sudo or checks privilege state
- Never add a `config` or `tui` command to the client — no `config` command (edit `cli.yml` directly), no `tui` command (TUI launches automatically with zero args)
- Never omit `--debug` from any binary — it is required, not optional
- Never use OS system directories for CLI runtime state (`/etc/...`, `/var/lib/...`, `/var/log/...`, `C:\ProgramData\`) — CLI config/data/cache/logs are always user-scope, even when the invoking user is root/Administrator
- Never re-resolve the backup directory fallback at cleanup time — cached once at startup; system mode falls back to `{data_dir}/backup/`, never a `$HOME`-derived path

## CRITICAL - ALWAYS DO

- Always show the ACTUAL (possibly renamed) binary name in `--help`, `--version`, and any "run X --help" error message
- Always hardcode `{project_name}` for User-Agent header, default paths, config keys, DB tables, API identifiers — these never change even if the binary is renamed
- Always support `NO_COLOR` (disables ANSI colors + emojis) and `TERM=dumb` (disables ALL ANSI escapes) on every binary
- Always accept both `--flag=value` and `--flag value` syntax on every flag
- Always give every flag a corresponding `cli.yml`/`server.yml` config equivalent to change its default
- Always create all directory flags (`--config`, `--data`, `--cache`, `--log`, `--backup`, `--pid`) if missing, with proper permissions, and validate writable
- Always detect stale PID files (process not running, or PID reused by another process) and remove them — except inside containers (no PID file at all)
- Always create `cli.yml` with `0600` (Unix) / user-only ACL (Windows) permissions — it holds the API token
- Always auto-detect `$SHELL` when `[SHELL]` is omitted from `--shell completions`/`--shell init`
- Always verify SHA-256 on CLI/server self-update downloads before atomic replace
- Always validate everything before use or save, only save valid values, never clear an existing valid config value with an empty/invalid one

## Key Rules Summary

**Binary naming:**

| Binary | Default Name | User-Agent | Key Flags |
|--------|--------------|------------|-----------|
| Server | `{project_name}` | `{project_name}/{version}` | `--config`, `--data`, `--port`, `--mode` |
| Client | `{project_name}-cli` | `{project_name}-cli/{version}` | `--server`, `--token`, `--output` |

Shared flags (ALL binaries): `--help`/`-h`, `--version`/`-v`, `--color {auto|yes|no}`, `--lang CODE`, `--shell {completions,init,help} [SHELL]`, `--debug`. Only `-h`/`-v` have short forms — everything else long-form only.

**Server: single-command, stdlib `flag`.** No subcommands. Complete command set (cannot be changed):

```
--help  --version
--shell completions [SHELL]  --shell init [SHELL]
--mode {production|development|debug}
--config DIR  --data DIR  --cache DIR  --log DIR  --backup DIR  --pid FILE
--address ADDR  --port PORT  --baseurl PATH
--status                              (exit 0=healthy, 1=unhealthy)
--service {start,restart,stop,reload,--install,--uninstall,--disable,--help}
--daemon  --debug  --color {auto|yes|no}  --lang CODE
--maintenance {backup,restore,update,mode,setup,pgp,secret,token,data,compliance,--help} [arg]
--update [check|yes|branch {stable|beta|devel}|--help]
```

**Client: multi-command, `cobra`/`viper`.** No standard commands — project-specific only (`create`/`get`/`list`/`delete`, `lookup`/`search`, etc.). `--version`/`--help` are flags, never commands. No `config` or `tui` command. Running with zero args launches interactive TUI.

**Directory flag defaults (Linux, server):**

| Flag | Root default | User default |
|------|--------------|--------------|
| `--config` | `/etc/{internal_org}/{internal_name}/` | `~/.config/{internal_org}/{internal_name}/` |
| `--data` | `/var/lib/{internal_org}/{internal_name}/` | `~/.local/share/{internal_org}/{internal_name}/` |
| `--cache` | `/var/cache/{internal_org}/{internal_name}/` | `~/.cache/{internal_org}/{internal_name}/` |
| `--log` | `/var/log/{internal_org}/{internal_name}/` | `~/.local/log/{internal_org}/{internal_name}/` |
| `--backup` | `/mnt/Backups/{internal_org}/{internal_name}/` (if writable, else `{data_dir}/backup/`) | `~/.local/share/Backups/{internal_org}/{internal_name}/` |
| `--pid` | `/var/run/{internal_org}/{internal_name}.pid` | `~/.local/share/{internal_org}/{internal_name}/{internal_name}.pid` |

Permissions: root → dirs `0755`, files `0644`, PID `0644`; user → dirs `0700`, files `0600`, PID `0600`. Directory mode (root vs user) is locked from EUID at process start, before any privilege drop, and never re-derived.

**Server startup sequence (summary):** parse flags → handle immediate-exit flags (`--help`/`--version`/`--status`/`--shell --help`) → handle `--service`/`--maintenance`/`--update` subcommands (each executes and exits) → determine context (root/container/service manager) → resolve and cache all paths once → if root: create system user, create/chown/permission all dirs, determine ports, bind privileged (<1024) ports, spawn managed children (Tor) with service-user setuid/setgid credentials, THEN drop privileges → if user: create user-scope dirs directly → init logging → check/write PID file (skipped entirely in containers) → load/generate `server.yml` → init database (idempotent schema) → start scheduler → start Tor (if not already spawned) → start HTTP server (bind remaining ports, register routes) → register signal handlers → enter main loop.

**Signals (Unix):** `SIGTERM`/`SIGINT`/`SIGQUIT`/`SIGRTMIN+3` → graceful shutdown; `SIGHUP` → ignored (config auto-reloads via file watcher); `SIGUSR1` → reopen logs; `SIGUSR2` → status dump. Windows: `os.Interrupt`/Windows Service Control → graceful shutdown only (no SIGHUP/USR1/USR2/QUIT support — use build tags to separate platform code).

**Daemonization (`--daemon`, manual start only):** foreground by default. `--service start` auto-detects the service manager and ignores `daemonize`/`--daemon` entirely (systemd/launchd/runit/s6/container → always foreground; SysV/rc.d → always daemonize). Priority for manual start: `--daemon` flag > `server.daemonize` config > default `false`. Windows ignores `--daemon` with a warning — use `--service --install` instead.

**Container detection:** `/.dockerenv`, `/run/.containerenv`, `/dev/lxc`, `container` env var, `KUBERNETES_SERVICE_HOST`, parent process is `tini`/`dumb-init`/`s6-svscan`/`runsv`/`runsvdir`/`catatonit`, or `/proc/1/cgroup` contains docker/kubepods/lxc.

**NO_COLOR priority (highest→lowest):** CLI flag `--color` → config `output.color` → `NO_COLOR` env → auto-detect (TTY + `TERM`). `NO_COLOR` disables colors+emojis only, not bold/underline/box-drawing/structure. `TERM=dumb` disables ALL ANSI escapes.

**Client-specific flags:**

| Flag | Purpose |
|------|---------|
| `--server URL` | Server base URL (priority: flag > `server.primary` in `cli.yml` > `{official_site}` compiled default > error with setup instructions) |
| `--token TOKEN` / `--token-file FILE` | API token (priority: flag > `{PROJECT_NAME}_TOKEN` env, no save > `cli.yml auth.token`) |
| `--config NAME` | Config profile name (default `cli.yml`); accepts a bare name (`test` → `{config_dir}/test.yml`), a filename, or an absolute/`~` path; extension auto-detects `.yml` then `.yaml`, defaulting to `.yml` for new configs |
| `--output {json\|table\|plain}` | Output format (`cli.yml` `output.format` additionally accepts `yaml`/`csv`) |

Flag→config save rule: a flag value is persisted to `cli.yml` only if the current config value is empty or invalid; otherwise it's session-only. Environment variable pattern: `{PROJECT_NAME}_{SECTION}_{KEY}` or `{PROJECT_NAME}_{KEY}` (e.g. `{PROJECT_NAME}_SERVER_PRIMARY`, `{PROJECT_NAME}_TOKEN`, `{PROJECT_NAME}_OUTPUT_FORMAT`). Config precedence: CLI flag > env var > config file > compiled default.

**CLI directories are always user-scope** (never system dirs, even when invoked as root): `~/.config/{internal_org}/{internal_name}/` (config, `cli.yml`), `~/.local/share/{internal_org}/{internal_name}/` (data), `~/.cache/{internal_org}/{internal_name}/` (cache), `~/.local/log/{internal_org}/{internal_name}/` (logs, `cli.log`); Windows uses `%APPDATA%`/`%LOCALAPPDATA%`. Dirs `0700`, files `0600` (Unix); Windows inherits ACLs from the user profile dir.

**Display modes:** DisplayMode = Headless / CLI / TUI / GUI, auto-detected (TTY, SSH/Mosh — always TUI/CLI, never GUI — `WAYLAND_DISPLAY`/`DISPLAY`, platform checks) — never an explicit flag. Server is headless/daemon by default (status banner only, no GUI/TUI). Client CLI supports full GUI/TUI/CLI/Headless and owns the only built-in setup wizard (first-run/double-click flow). GUI uses `github.com/gogpu/ui` with `github.com/gogpu/gogpu` for Linux/macOS/Windows; BSD reports GUI unsupported and falls back to TUI or CLI. Gio and Fyne are not compliant because they require CGO on Linux. Never use Electron/web-based or another CGO-dependent native toolkit.

**Shell completions (built-in, no separate files):** `--shell completions [SHELL]` prints script to stdout; `--shell init [SHELL]` prints an eval-ready wrapper (e.g. `source <({project_name} --shell completions bash)`). `[SHELL]` optional, auto-detected from `$SHELL`. Supported: bash, zsh, fish (full); sh, dash, ksh (POSIX basic); powershell, pwsh (full).

**Output formats:** `json` (raw), `table` (box-drawing, ┌─┬─┐ style), `plain` (bare value, scripting-friendly); `cli.yml` also accepts `yaml` and `csv` for `output.format`.

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Connection error |
| 4 | Authentication error |
| 5 | Not found |
| 64 | Usage error (bad arguments) |

**Auth model:** server is an open API — no auth to read public endpoints. Tokens are for resource-owner actions (protected endpoints scoped to a single resource) and operator-wide actions (`server.token`, CLI only). Token storage: `cli.yml` `auth.token`, `0600` perms; `--token` flag saves only if config is empty/invalid; `{PROJECT_NAME}_TOKEN` env var never saves to config.

**HTTP client identity:** User-Agent always uses the compiled-in `{project_name}` (or `{project_name}-cli`), never the renamed binary filename — format `{project_name}-cli/{version}`. Binary name (`filepath.Base(os.Args[0])`) is used only for display in `--help`/`--version`/error messages and the config file path stays fixed to the compiled name.

**CLI auto-update:** same pattern as server self-update (PART 22) — check via `/api/autodiscover` (`cli_versions`, `cli_min_version`) on every CLI start, download `{base}/cli/binaries/{project_name}-cli-{os}-{arch}`, verify SHA-256, atomic replace, re-exec with original argv. Below `cli_min_version` → refuse further requests until updated. Binary download is unauthenticated by default (`cli.binary_download.require_auth` opt-in).

**CLI is required for every project** — provides scripting/automation, headless/SSH, CI/CD, and terminal-power-user access. Build: same Makefile, `make build` produces both binaries. Config: `~/.config/{internal_org}/{internal_name}/cli.yml` (Unix) / `%APPDATA%\{internal_org}\{internal_name}\cli.yml` (Windows).

For complete details, see AI.md PART 7, 8, 32
