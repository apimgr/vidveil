# Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-17; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never write inline YAML comments — all comments go ABOVE the setting (exception: GitHub Actions SHA-pin `# vX.Y.Z` annotations)
- Never use `strconv.ParseBool()` directly — always `config.ParseBool()` / `config.IsTruthy()` (accepts yes/no, on/off, oui/non, si/non, etc.)
- Never fail startup on invalid config — warn and replace with default
- Never store user accounts or operator-editable configuration in the database — `server.yml` is the sole config source of truth; database only holds resource state, owner tokens, audit log
- Never bypass `server.token`/root authorization for sensitive ops (`--maintenance setup/restore/mode/pgp/secret rotate`) — file/directory ownership is NOT authorization
- Never let `--debug`/`DEBUG=true` disable authentication or security checks — debug affects verbosity/diagnostics ONLY, in every mode including production
- Never let `MODE=debug`/`--mode debug` be implied or auto-enabled — it is explicit opt-in only; it defaults the debug flag on, but an explicit `--debug`/`DEBUG` still wins (`MODE=debug DEBUG=false` = debug mode with `/debug/*` off)
- Never honor `X-Forwarded-*` headers from a peer IP outside `trusted_proxies` — drop them, fall back to `r.Host`/`r.RemoteAddr`
- Never trust-check the rewritten client IP for `isTrustedPeer()` — real-IP middleware must preserve the original TCP peer in context and trust-check that, never the resolved value
- Never leak clearnet FQDN, clearnet email, or `Preferred-Languages` into a Tor (`.onion`) response — Tor detection is priority 0, before proxy headers, no IP check
- Never auto-default `server.contact.abuse.email` to `abuse@{fqdn}` — operator must opt in (unprovisioned mailbox would bounce reports); `security@{fqdn}` IS auto-defaulted (RFC 2142)
- Never introduce flat aliases or duplicate contact-key names — use only `server.contact.{admin,security,abuse,general}.email`
- Never put `Retry-After`/maintenance/rate-limit wait-time info as a top-level JSON body field — headers only (`Retry-After`, `X-Maintenance-*`)
- Never bind ports <1024 without either being already elevated or escalating first — user-mode processes are restricted to >1024
- Never keep a project permanently root/Administrator unless the core function requires ongoing kernel/device operations (firewall control, packet capture, TUN/TAP/VPN, mount/filesystem management, package/service management) and only with an explicit justification in IDEA.md
- Never re-resolve the backup directory at cleanup time — use the same path resolved and cached at startup step 7

## CRITICAL - ALWAYS DO

- Always normalize and validate every path (config, HTTP request, file, API param) via `SafePath()`/`normalizePath()`/`validatePath()` — reject `..`, uppercase, invalid chars, >2048 chars, segments >64 chars
- Always run `PathSecurityMiddleware` at execution position #3 (after URL normalize + RequestID, before auth/routing) — see 10-step middleware order
- Always self-heal on critical errors (DB connection failure, file write failure) — retry with backoff (default 30s, configurable), enter maintenance mode (read-only + 503 for writes), auto-recover when resolved
- Always treat only 2 error classes as critical: database connection errors and file write errors — everything else is recoverable
- Always auto-migrate `server.yaml` → `server.yml` on startup if found
- Always persist the selected port (random or specified) to `server.yml` after first run — random selection happens ONLY on first run with no configured port
- Always drop privileges to the dedicated `{project_name}` system user as the FINAL root-phase action (after binding ports, chown-ing dirs, and spawning any managed children like Tor with service-user creds) — verify with a post-drop UID check
- Always check both AUTHORIZATION (not just file access) before sensitive ops — `server.token` OR root OR first-run/empty-DB as applicable per operation
- Always resolve `{fqdn}`/`{proto}`/`{port}`/`{baseurl}` via the documented reverse-proxy header priority chains, gated by `trusted_proxies`
- Always sign outbound webhooks with `X-Webhook-Signature` (HMAC-SHA256 per-webhook auto-generated secret), `X-Webhook-Timestamp`, `X-Webhook-ID` (UUIDv7), `X-Webhook-Event`
- Always retry failed webhook delivery with backoff 1m, 5m, 15m, 1h, 6h, 24h then drop, reusing the same `X-Webhook-ID`

## Key Rules Summary

### Mode & Debug Detection Priority

| | Priority order |
|---|---|
| **Mode** | `--mode` CLI flag → `MODE` env var → default `production` |
| **Debug** | `--debug` CLI flag → `DEBUG` env var (truthy or falsy, explicit wins) → `--mode debug`/`MODE=debug` alias (defaults debug on) → default `false` |

`MODE=debug`/`--mode debug` = explicit opt-in, never implied. It defaults the debug flag to on for that process, but an explicitly-set `DEBUG`/`--debug` (true or false) still wins — `MODE=debug DEBUG=false` → debug mode, `/debug/*` off.

Shortcuts: `dev`/`devel`/`development` → development; `prod`/`production` → production; `debug` → development-family mode + debug defaulted on.

| State | Mode | Debug | Debug/pprof endpoints | Logging | Caching |
|---|---|---|---|---|---|
| Production | production | false | 404 | info | on |
| Production+Debug | production | true | enabled | full | on |
| Development | development | false | 404 (need `--debug`) | debug | off (hot reload) |
| Development+Debug | development | true | enabled | full | off |
| Debug | debug | false | 404 (`MODE=debug DEBUG=false`) | debug | off |
| Debug+Endpoints | debug | true | enabled (default when `MODE=debug`, `DEBUG` unset) | full | off |

Debug endpoints (`--debug`/`DEBUG=true` only, else 404): `/debug/pprof/*` (index, heap, goroutine, allocs, block, mutex, threadcreate, cmdline, profile, symbol, trace), `/debug/vars` (expvar), `/debug/config`, `/debug/routes`, `/debug/cache`, `/debug/db`, `/debug/scheduler`, `/debug/memory`, `/debug/goroutines`. `server.debug.*` config keys: `pprof`, `log_queries`, `log_cache`, `log_bodies`, `max_body_log_size`, `block_profile_rate`, `mutex_profile_fraction`, `runtime_endpoints`. Operator token auth is NEVER bypassed by debug mode; credentials/keys/tokens/secrets stay redacted even at maximum verbosity.

### Config File

| Rule | Value |
|---|---|
| Location (root) | `/etc/{internal_org}/{internal_name}/server.yml` |
| Location (user) | `~/.config/{internal_org}/{internal_name}/server.yml` |
| Format | YAML, never `.yaml` (auto-migrated) |
| Comments | Above the setting, single-line, <140 chars |
| Design | Clean, everything configurable, sane built-in defaults, no admin web UI |
| Runtime mutation | None — edit `server.yml` directly, hot-reload on save except listen address/port/DB driver (need restart) |

### Boolean Parsing (`config.ParseBool()` / `config.IsTruthy()`)

Truthy: `1 y t yes true on enable enabled yep yup yeah aye si oui da hai affirmative accept allow grant sure totally ok`
Falsy: `0 n f no false off disable disabled nope nah nay nein non niet iie lie negative reject block revoke deny never noway`
Case-insensitive. Empty → default value. Invalid → error (never silent default).

### Environment Variables

**Runtime (always checked):** `NO_COLOR`, `TERM` (`dumb` forces CLI mode, no ANSI), `DOMAIN`, `MODE`, `DEBUG`, `DATABASE_DRIVER` (`sqlite`/`sqlite2`/`sqlite3`, `libsql`/`turso`), `DATABASE_URL`, `SMTP_HOST/PORT/USERNAME/PASSWORD/FROM_NAME/FROM_EMAIL/TLS`.

**Init-only (first run, then ignored):** `CONFIG_DIR`, `DATA_DIR`, `LOG_DIR`, `DATABASE_DIR`, `BACKUP_DIR`, `PORT`, `LISTEN`, `APPLICATION_NAME`, `APPLICATION_TAGLINE`.

URL vars resolution order: `{fqdn}`: Tor (priority 0) → proxy headers → `DOMAIN` → `os.Hostname()` → `$HOSTNAME` → global IP → `localhost`. `{proto}`: Tor forces `http` → `X-Forwarded-Proto` → `X-Forwarded-Ssl` → `X-Url-Scheme` → TLS detect → `http`. `{port}`: `X-Forwarded-Port` → Host header → server port → proto default. `{baseurl}`: `X-Forwarded-Prefix` → `X-Forwarded-Path` → `X-Script-Name` → `server.baseurl`/`--baseurl` → `/`.

### Port Rules

| Scenario | Behavior |
|---|---|
| First run, no config | Random unused port in **64000-64999**, saved to `server.yml` |
| Config specifies port | Use exact port |
| Port in use | Fail with clear error |
| Privileged (<1024) | Escalate if possible, else fallback >1024 |
| Port `80` | Enables Let's Encrypt HTTP-01 |
| Port `443` | Enables TLS-ALPN-01, auto-enables SSL (HTTPS-only unless `ssl.enabled` overridden) |
| Port `0` | OS assigns any available |
| Dual port | `"8090,8443"` (HTTP,HTTPS comma-separated); config-file only, requires restart |

Port display: strip `:80`/`:443` from displayed URLs, always show non-standard ports. Port change is config-file only — no runtime API.

### Privilege / Service Model

| Mode | Started | Port restriction | Privilege |
|---|---|---|---|
| Service (escalated) | `sudo {project_name} --service --install` | any | root → drops to `{project_name}` user after binding |
| User (`$USER`) | `{project_name}` | >1024 only | no escalation |

Unix startup sequence (root): create system user → create dirs/chown → bind privileged ports → spawn any managed children (e.g. Tor) with service-user setuid/setgid credentials → **drop privileges** (final root-phase action, verified with a post-drop UID check) → init config/DB → serve. Windows uses Virtual Service Account (`NT SERVICE\{internal_name}`) — no privilege drop needed.

`{project_name}` system user: shell `/usr/sbin/nologin`, home `/var/lib/{internal_org}/{internal_name}`, system UID (<1000 Linux). Can read/write config/data/cache/logs/backups, bind >1024, run scheduled tasks. Cannot bind <1024, modify service files, create users, write `/usr`/`/bin`, modify/update the binary.

Escalation symbols: ❌ No (never) · ⚠️ Check (binary determines) · 🔐 Auth (needs `server.token`/root/typed confirmation). Sensitive 🔐 ops: `--maintenance setup` (first-run OR root), `--maintenance restore` (token/root/empty-DB), `--maintenance mode` (token/root), `--maintenance pgp export private/import/delete` (token/root + confirmation), `--maintenance secret rotate installation_secret|encryption_key` (token/root + confirmation).

### server.yml Structure (key sections)

```yaml
server:
  port: {random 64xxx}
  fqdn: {hostname}
  address: "[::]"
  mode: production
  api_version: v1
  baseurl: /
  healthz:
    root:
      enabled: false
  branding: { title, tagline, description }
  seo: { keywords: [] }
  user: {auto}
  group: {auto}
  pidfile: true
  daemonize: false
  ssl:
    enabled: false
    cert: ""
    key: ""
    min_version: "TLS1.2"
    letsencrypt: { enabled: false, email, challenge: http-01, staging: false }
  scheduler:
    enabled: true
    tasks: { geoip_update, blocklist_update, cve_update, log_rotation,
             token_cleanup, backup, ssl_renewal, health_check, tor_health }
  tokens:
    max_linked: 25
    cookie_max_age: 1y
    idle_purge_days: 0
  rate_limit:
    enabled: true
    read: { requests: 120, window: 60 }
    write: { requests: 10, window: 60 }
    health: { requests: 120, window: 60 }
    global_burst: 240
  database:
    driver: sqlite
    url: "{data_dir}/db/server.db"
  cors:
    allowed_origins: ["*"]
  limits: { max_body_size: 10MB, read_timeout: 30s, write_timeout: 30s, idle_timeout: 120s }
  compression: { enabled: true, level: 5, types: [...] }
  trusted_proxies: { additional: [] }
  i18n: { default_language: en, supported: [en] }
  contact: { admin, security, abuse, general }  # each: email + webhooks{telegram,discord,slack,mattermost,pushover,gotify,generic}
  tracking: { type: "", id: "", url: "" }
  privacy: { data, consent, ... }
  cache: { type: memory, url: "", host, port, username, password, db, tls, pool_size, prefix, ttl }
  debug: { pprof, log_queries, log_cache, log_bodies, max_body_log_size,
           block_profile_rate, mutex_profile_fraction, runtime_endpoints }
  maintenance:
    self_healing: { enabled: true, retry_interval: 30s, max_attempts: 0 }
    cleanup: { disk_threshold: 90, log_retention_days: 7, backup_keep_count: 5 }
    notify: { on_enter: true, on_exit: true }
tor:
  onion_address: ""
  contact_email: ""
web:
  ui:
    theme: dark
```

### Maintenance Mode

Two critical error types only: DB connection failure, file write failure (disk full/permissions). All others recoverable. States: Normal, Maintenance (read-only, writes → 503), Starting. Self-healing retries every `retry_interval` (default 30s, configurable), `max_attempts` default 0 = unlimited, auto-exits when resolved, sends notification if configured. API error body: `{"ok":false,"error":"MAINTENANCE","message":"...","details":{"reason":...,"self_healing":true}}` + headers `Retry-After`, `X-Maintenance-Mode`, `X-Maintenance-Reason`. Backup-dir cleanup reuses the path resolved and cached at startup step 7 — never re-resolved mid-run.

### Trusted Proxies

Always trusted (no config): loopback, RFC1918 (`10/8`,`172.16/12`,`192.168/16`), `fc00::/7`, link-local, same `/24` as listen address. `server.trusted_proxies.additional` accepts IP/CIDR/DNS name (DNS refreshed every 5min). Gated headers: FQDN (`X-Forwarded-Host`, `X-Real-Host`, `X-Original-Host`), Proto (`X-Forwarded-Proto`, `X-Forwarded-Ssl`, `X-Url-Scheme`), Port (`X-Forwarded-Port`), Base path (`X-Forwarded-Prefix`, `X-Forwarded-Path`, `X-Script-Name`), Client IP (`X-Real-IP`, `X-Forwarded-For`, `CF-Connecting-IP`, `True-Client-IP`, `X-Client-IP`). Real-IP middleware must preserve the ORIGINAL TCP peer in request context before any header rewrite — `isTrustedPeer()` always trust-checks that preserved original peer, never the header-resolved value, so a spoofed forwarded-for chain can never self-authorize.

### Tor (`tor.onion_address`)

Priority-0 detection (before proxy headers, no trust check needed — the hidden service listener is inherently local). FQDN = onion address, proto always `http://`, port stripped. Rules: email shows only `tor.contact_email` (else omitted, never clearnet fallback); all absolute URLs use the onion address, never the clearnet FQDN; CORS auto-adds the onion origin, never clearnet; OAuth redirect URIs must use onion or the provider is disabled (503); `Preferred-Languages` omitted from `.well-known/security.txt` on the Tor variant. Server owns the full Tor lifecycle (binary discovery, torrc generation, hidden-service keys) — see PART 31 / backend-rules.md.

### Rate Limiting

Sliding window per-IP, stored in `server.db` (or external cache if configured). Defaults: read 120/min, write 10/min, health 120/min, global burst 240/min ceiling. `429` + `Retry-After` header; body `{"ok":false,"error":"RATE_LIMITED","message":"..."}` (wait time in header only).

### Contact Roles (`server.contact.*`)

Four roles: `admin` (required, never public, fallback target for others), `security` (default `security@{fqdn}` RFC 2142, public in security.txt), `abuse` (default `""`, opt-in only, falls back to `general` then `admin`), `general` (public contact form, falls back to `admin`). Each role: `email` + `webhooks.{telegram,discord,slack,mattermost,pushover,gotify,generic}`. Webhook signing: `X-Webhook-Signature` (HMAC-SHA256, per-webhook auto-generated secret), `X-Webhook-Timestamp`, `X-Webhook-ID` (UUIDv7), `X-Webhook-Event`, `User-Agent`. Retry backoff: 1m,5m,15m,1h,6h,24h then drop, reusing the same `X-Webhook-ID`.

### Analytics (`server.tracking`)

Types: `google`, `matomo`, `piwik`, `owa`, `fathom`, `plausible`, `umami`, `simple`, `cloudflare`. Empty/`none` = disabled. `url` required for matomo/piwik/owa/umami; optional for fathom/plausible (self-hosted); unused for google/simple/cloudflare.

### Cache (`server.cache`)

Types: `none`, `memory` (default), `valkey`, `redis`. Connect via `url` (takes precedence) OR `host`/`port`/`username`/`password`/`db`. URL formats: `redis://`, `valkey://`, `rediss://` (TLS). Used for: sessions, rate-limit counters, optional API response caching. Key prefix: `"{project_name}:"`.

### Base URL (`server.baseurl` / `--baseurl`)

Resolution priority: `X-Forwarded-Prefix` → `X-Forwarded-Path` → `X-Script-Name` → config `server.baseurl` / CLI `--baseurl` → default `/`. Used to build every absolute link (`BuildURL(r, ...)`) so the app works correctly behind a reverse-proxy path prefix.

### Privacy & Consent

Cookie categories: Essential (always on), Preferences (optional), Analytics (optional, only if tracking configured). Consent stored client-side as JSON cookie `cookie_consent`. `ConsentMiddleware` gates non-essential cookie writes; `CanSetPreferenceCookie()`/`CanLoadAnalytics()` check consent state. `server.privacy.data.sold` (default `false`) drives dynamic consent/privacy-policy wording and gates the CCPA "Do Not Sell" opt-out section — only rendered when `true`.

For complete details, see AI.md PART 5, 6, 12
