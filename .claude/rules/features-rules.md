# Features Rules (PART 17, 18, 19, 20, 21, 22)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-31; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never queue or fake-log email when SMTP is not configured — "no SMTP = no email, period"
- Never surface operator events (update available, backup failed, SSL expiry, scheduler errors) in the public WebUI — operator channels only (Logs, Email, `--status`/CLI)
- Never use an external scheduler (cron, systemd timers, Task Scheduler, launchd, Kubernetes CronJob, cloud schedulers) — the built-in scheduler handles ALL scheduled tasks, no exceptions
- Never treat GeoIP/country as the sole access-control gate — always advisory, alongside rate limiting and real auth, never a replacement for either
- Never block a request because a GeoIP lookup failed or the database is missing/stale — fail-open for GeoIP, fail-closed only for real auth
- Never use `geoip2-golang` for GeoIP — it rejects ip-location-db's custom `database_type` strings; use `github.com/oschwald/maxminddb-golang`
- Never label the ASN `autonomous_system_organization` field (or anything else) "WHOIS" — no combined WHOIS database or field exists in ip-location-db; the `geo-whois-asn-country` package name is misleading, it exposes only `country_code`
- Never use a raw client IP as a metric label value (Prometheus or otherwise) — unbounded cardinality is a memory-DoS vector; log per-IP detail instead
- Never serve a metrics route without a bearer token — there is no unauthenticated default; an empty per-service token disables that service (403), it never falls open
- Never accept a metrics token via query string — header-only (`Authorization: Bearer`), query tokens leak into access logs and proxies
- Never redirect any `/metrics`/`/server/metrics`/API metrics alias route — every alias invokes the SAME handler, same rule as the healthz root alias
- Never expose metrics publicly — INTERNAL ONLY, firewalled or NetworkPolicy-restricted, per-service bearer tokens are defense in depth, not a substitute for network isolation
- Never store a backup encryption password — operator must remember it; no recovery if lost
- Never delete old backups unless the new backup passes ALL verification checks
- Never allow backups to run in compliance mode without an encryption password set
- Never accept a password via CLI flag for encrypted backup/restore — interactive prompt only (shell history / process list leak)
- Never skip SHA256 checksum verification against the release's `sha256.txt` asset before installing a self-update
- Never auto-install updates by default — `update.auto_install` defaults to `false`; the scheduled task only notifies unless explicitly enabled
- Never let `defer_days` block a manual `--update check`/`--update yes` — the defer window only gates the scheduled task

## CRITICAL - ALWAYS DO

- Always support custom email template overrides in `{config_dir}/template/email/` layered over embedded defaults
- Always auto-detect SMTP via a priority-ordered host list; strictly no email path exists without it
- Always use `{variable}` template syntax (`Subject:` / `---` / body) with vars like `{app_name}`, `{app_url}`, `{fqdn}`, `{onion_url}`, `{timestamp}`, `{year}`
- Always keep the built-in scheduler always-running, with persistent state in `server.db`, and automatic catch-up for missed tasks within `catch_up_window`
- Always download GeoIP MMDB databases from sapics/ip-location-db on first run and update via scheduler — never embed them in the binary
- Always show the DB-IP and NRO CC BY 4.0 attribution notices together, visibly, wherever GeoIP-derived data is shown, plus in `LICENSE.md`
- Always prefix metrics with `{project_name}_`, snake_case, unit-suffixed (`_seconds`, `_bytes`), counters end `_total`
- Always mount `/server/metrics[/{service}]`, the versioned/unversioned API aliases, and the root `/metrics[/{service}]` alias to the identical handler per service
- Always verify every backup immediately after creation (file exists, size>0, checksum, decrypt test, manifest parse, content extraction, DB integrity) — delete on any failure, keep existing valid backups
- Always require operator token authorization for restore when running as service user; deny random users
- Always verify a backup fully (format, decrypt, checksum, manifest, version) before proceeding with restore
- Always replace the update binary per-platform (Unix: atomic rename over running exe; Windows: rename-to-`.old` + `MOVEFILE_DELAY_UNTIL_REBOOT`)

## Key Rules Summary

### Email & Notifications (PART 17)

| Aspect | Detail |
|--------|--------|
| Channels | Public WebUI (toast/banner), Logs, Email — strictly separated; operator events never hit the public WebUI |
| Templates | Embedded defaults + optional override files in `{config_dir}/template/email/`; syntax `{variable}`, `Subject:` line, `---` separator, then body |
| SMTP | Auto-detected via priority-ordered host list; no SMTP configured = no email sent, ever (no queue, no fake log substitute) |
| Suppression | Related failure notifications suppress each other for the same execution (e.g. `backup_failed` suppresses `scheduler_error`) |
| Banner state | No server-side storage — dismissal tracked client-side via `dismissed_announcements` cookie |
| Example vars | `{app_name}`, `{app_url}`, `{fqdn}`, `{onion_url}`, `{onion_address}`, `{i2p_url}`, `{i2p_address}`, `{notification_reply_to}`, `{timestamp}`, `{year}` |
| Event matrix (examples) | Backup failed → ERROR + email; SSL expiring 30/14d → WARN log-only; SSL expiring 7/3/1d → WARN + email; Update installed → INFO + email; Update available → WARN log + email (off by default) |

### Scheduler (PART 18)

| Aspect | Detail |
|--------|--------|
| Rule | Built-in scheduler only — NEVER cron/systemd timers/Task Scheduler/launchd/K8s CronJob/cloud schedulers, on any platform |
| State | Persistent in `server.db`: `task_id`, `schedule`, `last_run`, `last_status`, `last_error`, `next_run`, `run_count`, `fail_count`, `enabled` |
| Startup | Loads state → checks missed tasks within `catch_up_window` (default 1h) → queues them in original scheduled order → starts loop |
| Shutdown | Stop new executions → wait up to 30s for running tasks → release locks → save state; timeout force-releases and marks interrupted tasks for retry |
| Implementation | Go `time`/ticker — no external cron library required |
| Required built-in tasks | `ssl_renewal` (daily 03:00), `geoip_update` (weekly Sun 03:00), `blocklist_update` (daily 04:00), `cve_update` (daily 05:00), `update_check` (daily 06:00, notify-only unless `auto_install`), `token_cleanup` (@every 15m), `log_rotation` (daily 00:00), `backup_daily` (daily 02:00), `backup_hourly` (hourly, disabled by default), `healthcheck_self` (@every 5m), `tor_health` (@every 10m, if Tor installed) |
| Schedule formats | Cron (`0 2 * * *`), `@hourly`, `@daily`, `@weekly`, `@monthly`, `@every Xm`/`@every Xh` |
| Retry policy | `max_retries: 3`, `retry_delay: 5m`, exponential backoff |
| CLI | `{project_name} scheduler list \| show <id> \| run <id> \| enable <id> \| disable <id> \| history <id>` |
| If user asks for cron | Explain built-in scheduler covers it via `{setting}.schedule` in config; no exceptions ("more reliable", "already set up", "need complex schedules" all rejected — full cron syntax is supported natively) |

### GeoIP (PART 19)

| Aspect | Detail |
|--------|--------|
| Source | `sapics/ip-location-db`, via jsDelivr CDN (`@ip-location-db` npm scope, no API key/account) — downloaded on first run + updated weekly via scheduler, never embedded |
| Go library | `github.com/oschwald/maxminddb-golang` (NOT `geoip2-golang`, which rejects ip-location-db's custom `database_type` strings) |
| Storage dir | `{data_dir}/security/geoip` (config: `server.geoip.dir`) |
| Databases | ASN → `asn.mmdb` (`autonomous_system_number`, `autonomous_system_organization`); Country → `geo-whois-asn-country.mmdb` (`country_code` only); City IPv4/IPv6 → `dbip-city-ipv4.mmdb`/`dbip-city-ipv6.mmdb` (`city`, `country_code`, `state1`, `state2`, `postcode`, `latitude`, `longitude`, `timezone`) |
| No WHOIS DB | There is no separate combined WHOIS lookup/field — the `geo-whois-asn-country` name is misleading; org-name data comes only from ASN's `autonomous_system_organization` (a BGP/RIR AS-holder name, not an RDAP/WHOIS registrant record) |
| Never use GeoLite2 | MaxMind GeoLite2 requires EULA acceptance in addition to attribution — not zero-config-compatible, never used as fallback |
| License | All three DBs are CC BY 4.0 — attribution is a license condition: DB-IP link + NRO notice, both required together, visible (not click-through), on every page showing GeoIP data and in `LICENSE.md` |
| Country blocking | `deny_countries: []` (block listed, allow rest) or `allow_countries: []` (allow only listed); if both set, `allow_countries` wins; empty = no blocking |
| Codes | ISO 3166-1 alpha-2, uppercase |
| Bypass | `server.security.allowlist` IPs always bypass country blocking; RFC 1918/RFC 4193/loopback IPs never country-blocked |
| Missing DB | Country blocking skipped with a warning if the Country database is absent/disabled |
| Risk-signal rule | Never sole access gate; always runs alongside/after rate limiting + auth; fail-open on lookup error, never fail-closed for GeoIP alone |

### Metrics (PART 20)

**Metrics live under `/server` (mirrors healthz), with three per-service sub-endpoints and full 4-way alias mounting — every alias invokes the SAME handler, never a redirect.**

| Endpoint | Auth | Description |
|----------|------|-------------|
| `/server/metrics` | Bearer (`prometheus` token) | Same as `/server/metrics/prometheus` |
| `/server/metrics/prometheus` | Bearer (`prometheus` token) | Full Prometheus text exposition (`text/plain; version=0.0.4`) |
| `/server/metrics/grafana` | Bearer (`grafana` token) | Complete importable Grafana dashboard JSON, all metric categories, datasource templated |
| `/server/metrics/loki` | Bearer (`loki` token) | Recent structured logs as Loki push-API JSON streams, bounded by `loki.max_entries`/`max_age`, credentials always redacted |
| `/api/{api_version}/server/metrics[/{service}]` | Same per-service token | Versioned API path, same handlers |
| `/api/metrics[/{service}]` | Same per-service token | Unversioned alias for current `{api_version}` |
| `/metrics[/{service}]` | Same per-service token | Root alias, `server.metrics.root.enabled` (default **true** — Prometheus scrapers expect `/metrics`) |

| Aspect | Detail |
|--------|--------|
| Library | `github.com/prometheus/client_golang` |
| Auth model | Per-service bearer tokens (`server.metrics.auth.tokens.{prometheus,grafana,loki}`), header-only, constant-time compare; empty token = that service disabled (403); `auth.allow_unauthenticated: true` (default false) is a firewalled-network-only escape hatch |
| Access | INTERNAL ONLY regardless of auth — firewall/NetworkPolicy restricted; tokens are defense in depth, not the primary control |
| Naming | `{project_name}_` prefix, snake_case, unit suffix (`_seconds`, `_bytes`), counters end `_total`; base units (seconds not ms, bytes not KB) |
| Labels | snake_case, lowercase, low cardinality only — never raw IPs/user IDs/request IDs/timestamps; normalize paths (`:id`); `per_ip` is a `limit` label *value*, never a per-address label |
| Required metric groups | App info/uptime/start timestamp; HTTP (requests/duration/size/active); DB (queries/duration/connections/errors) if DB used; Auth (attempts/active sessions) |
| Optional groups | Cache, Scheduler, System (`include_system`), Go runtime (`include_runtime`), Tor, rate limiting, business metrics |
| Config | `enabled`, `root.enabled`, `auth.allow_unauthenticated`, `auth.tokens.{prometheus,grafana,loki}`, `include_system`, `include_runtime`, `loki.max_entries`, `loki.max_age`, `duration_buckets`, `size_buckets` |

### Backup & Restore (PART 21)

| Aspect | Detail |
|--------|--------|
| Command | `{project_name} --maintenance backup [filename]` / `--maintenance restore <file>` |
| Always included | `server.yml`, `server.db`, `{config_dir}/template/` (if exists), `{config_dir}/theme/` (if exists) |
| Optional | `{config_dir}/ssl/` (`--include-ssl`), `{data_dir}/` (`--include-data`) |
| Format | `.tar.gz` (or `.tar.gz.enc` if encrypted) with `manifest.json` (version, created_at, app_version, contents, encrypted, encryption_method, checksum) |
| Filenames | Manual/timestamped: `{project_name}_backup_YYYY-MM-DD_HHMMSS.tar.gz[.enc]`; scheduled daily full: `{project_name}_backup_YYYY-MM-DD.tar.gz[.enc]`; daily incremental: `{project_name}-daily.tar.gz[.enc]`; hourly incremental: `{project_name}-hourly.tar.gz[.enc]` |
| Encryption | AES-256-GCM, key via Argon2id from password; password NEVER stored; unencrypted archive never touches disk |
| Compliance mode | If `server.compliance.enabled: true`, backups blocked until encryption password set; scheduled backups skip with audit warning |
| Password entry | Interactive prompt only (CLI/WebUI/API) — never a CLI flag |
| Retention | `max_backups` (default 1, daily fulls), `keep_weekly`/`keep_monthly`/`keep_yearly` (default 0=disabled), `max_total_size` (default `"10%"`, hard cap overrides all count limits, 0=disabled) |
| Falsey values | `0`, `false`, `no`, `none`, `disable`, `disabled`, `off` |
| Priority order | Yearly > Monthly > Weekly > Daily (max_backups), oldest deleted first within a tier |
| Verification (mandatory, all must pass) | File exists, size>0, checksum valid, decrypt test (if encrypted), manifest readable, content extraction, DB integrity — any failure deletes the new backup, keeps old ones, retries next run |
| Disk guard | Skip and log `backup.skipped_disk_full` (ERROR) if free space < 2× last backup size or disk usage > threshold (default 90%) |
| Restore authorization | First-run (DB empty): allowed; root: allowed with confirmation; service user: requires `server.token`; random user: denied |
| Restore verification | File exists/readable, valid tar.gz[.enc] format, decrypt test, checksum, manifest valid, version compatibility warning — restore proceeds only if ALL pass |
| Restore limits | `server.restore.max_archive_size` (`1G`), `max_expanded_size` (`4G`), `max_files` (`10000`), `max_file_size` (`512M`), and `max_ratio` (`100`); validate headers before each write and enforce while streaming |

### Update Command (PART 22)

| Aspect | Detail |
|--------|--------|
| Command | `--update [check\|yes\|branch {stable\|beta\|devel}]`; default (no subcommand) = `yes`; alias `--maintenance update [cmd]` |
| Exit codes | 0 = updated or already current; 1 = error; GitHub API 404 = no updates available |
| Channels | `stable` (full releases only), `beta` (beta pre-releases + all stable, cumulative), `devel` (rolling `devel` release + beta + stable, cumulative); `daily`, `rolling`, `dev`, and `development` normalize to `devel` |
| Config | `server.update.branch` (default `stable`), `auto_install` (default `false`), `defer_days` (default `0`, 0-365), `cli_min_version` (empty disables forced minimum; returned in autodiscover) |
| `defer_days` | Release eligible once `now - published_at >= defer_days` (GitHub `published_at`, UTC); gates ONLY the scheduled `update_check` task — manual `--update check`/`yes` always see/install the true latest |
| Scheduled task | `update_check` (daily 06:00): runs `--update check` filtered by `defer_days`; `auto_install:false` → notify only (WARN log + `update_available` email, off by default, fires once per version); `auto_install:true` → runs full `--update yes` |
| Surfacing | Operators only — WARN log, email event, `--update check`/`--status` CLI; NEVER injected into API responses/headers/public endpoints (Tier 3 info) |
| Update flow | Check GitHub Releases API → download to temp → verify SHA256 against the release's `sha256.txt` asset (mandatory) → replace binary (platform-specific) → restart service or re-exec |
| Unix replace | Atomic `os.Rename` over running binary; preserves permissions |
| Windows replace | Rename current to `.old` → move new binary into place → `MoveFileEx` with `MOVEFILE_DELAY_UNTIL_REBOOT` to clean up `.old` |
| Service restart | Linux: `systemctl restart` (fallback `service restart`); macOS: `launchctl kickstart -k system/{plist_name}`; BSD: `service {project_name} restart`; Windows: `sc stop` + `sc start` |

For complete details, see AI.md PART 17, 18, 19, 20, 21, 22
