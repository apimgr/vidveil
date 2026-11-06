# Backend Rules (PART 9, 10, 11, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-11-06; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never invent a new error response shape — always `{"ok":true,"data":{}}` / `{"ok":false,"error":"CODE","message":"...","details":{...}}`
- Never expose internal error detail (stack traces, DB errors, file paths, SQL) in the client response — Output Sanitization Pipeline strips it; full detail logged server-side only
- Never retry a 4xx or a non-idempotent write on transient failure — only network errors/timeouts/503 are retryable, with exponential backoff (0s,1s,2s,4s,8s, cap 30s)
- Never use `SELECT *` — name columns explicitly
- Never write a numbered/timestamped migration file or version-tracking table — schema is idempotent (`CREATE TABLE IF NOT EXISTS`) applied on every start
- Never use `github.com/mattn/go-sqlite3` (CGO) — use `modernc.org/sqlite`; remote-only libSQL/Turso uses `github.com/tursodatabase/libsql-client-go`
- Never run a DB query without a context timeout
- Never log or return a cryptographic secret (`installation_secret`, `cookie_signing_key`, `csrf_token_secret`, `server.security.encryption_key`) — Output Sanitization Pipeline redacts any field named `secret`/`key`/`password`/`token`
- Never use non-constant-time comparison (`==`, `bytes.Equal`) for tokens/passwords/HMACs/signatures — always `crypto/subtle.ConstantTimeCompare`
- Never use sequential integer IDs in public-facing URLs — opaque UUID v4/v7 only
- Never emit `Server-Timing` in production (debug-mode only, Tier 3 info)
- Never write ANSI escape codes, emojis, or color codes to any log FILE — plain text only, console output only may be pretty
- Never use a free-form string for `auth.log` `reason=` — always a stable machine code (keeps Fail2ban/SIEM parsers stable)
- Never let the audit log be truncated or edited — append-only, only log rotation may remove entries
- Never accept a plaintext archive/backup password via CLI flag — interactive prompt only; never store the private PGP key unencrypted
- Never use system Tor or a default Tor port (9050/9051) — the binary starts its own isolated Tor instance with `ControlPort 127.0.0.1:auto`
- Never make the Tor hidden service configurable on/off — it is always enabled when the `tor` binary is found, no toggle
- Never auto-enable I2P — it is opt-in only (`server.i2p.enabled: true`), unlike Tor; no provider is contacted and no port allocated unless explicitly enabled
- Never wrap the I2P eepsite backend listener in `go-proxyproto` — neither i2pd nor SAMv3 prepend a PROXY-protocol header, so it is a plain loopback listener, unlike the Tor backend

## CRITICAL - ALWAYS DO

- Always return the canonical error/success envelope; map error codes to HTTP status via the standard table below
- Always log 5xx as `Error` level and 4xx as `Warn` level with `error_code`, `request_id`, `http_status` fields
- Always use idempotent schema application (`EnsureSchema` pattern) — no migration tooling
- Always configure connection pooling (`SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`/`SetConnMaxIdleTime`) and run every query under a context timeout
- Always use `WithTransaction` for multi-statement writes; use serializable isolation + retry (`WithSerializableRetry`) for balance-transfer-style operations requiring strict consistency
- Always include the required security headers on every response, plus HSTS when SSL is enabled
- Always apply Content-Security-Policy with the project default policy, extended (never replaced) via `csp.*_extra` config keys
- Always generate the four project-level secrets on first start, before any user-visible operation, and include them in every backup
- Always write `audit.log` as JSON Lines, one event per line, append-only, `0640` perms
- Always suppress successful 2xx health-check requests from `access.log` (never suppress failures)
- Always sanitize markdown/user-HTML output and force `Content-Disposition: attachment` on active-content MIME types before serving
- Always let the app own the entire Tor lifecycle (binary discovery, directory creation with 0700 perms, torrc generation, process start/stop) — no external scripts

## Key Rules Summary

### Error Handling & Caching (PART 9)

**Response envelope:**
```json
{"ok":true,"data":{}}
{"ok":false,"error":"CODE","message":"...","details":{}}
```
(Canonical shape defined in PART 14; PART 9 only references it.)

**Error code → HTTP status:**
| Code | Status |
|------|--------|
| BAD_REQUEST, VALIDATION_FAILED | 400 |
| UNAUTHORIZED, TOKEN_EXPIRED, TOKEN_INVALID | 401 |
| FORBIDDEN, ACCOUNT_LOCKED, CSRF_FAILED | 403 |
| NOT_FOUND | 404 |
| METHOD_NOT_ALLOWED | 405 |
| CONFLICT | 409 |
| RATE_LIMITED | 429 |
| SERVER_ERROR (default) | 500 |
| MAINTENANCE | 503 |

**Retry policy:** exponential backoff `0s,1s,2s,4s,8s`, capped at 30s. Only network errors, timeouts, and 503 are retryable — never 4xx.

**Caching:** drivers `memory` (dev default) / `valkey` (preferred prod) / `redis`. Hierarchical colon-separated cache keys, prefixed `{project_name}:`. Per-content-type TTL defaults; invalidation by time, event, version, or tag. `Cache-Control` headers set per content type; cache warmed on startup.

**Asset version-busting:** `asset()` template helper appends `?v={version}-{short_commit}`; immutable caching only applies while the query-string stamp matches; HTML is never cached; service worker cache name embeds the version. On version change, a `{project_name}_build` cookie mismatch triggers `Clear-Site-Data: "cache", "storage"` (never `"cookies"` here) — one-shot by design.

### Database (PART 10)

- Schema is idempotent: `CREATE TABLE IF NOT EXISTS` run on every start via `EnsureSchema` — no migration files, no version table; ignore "duplicate column" errors on `ALTER TABLE ADD COLUMN`.
- Column rename = 3-step pattern (add new column → app dual-read/write → old column left unused after upgrade), never an in-place rename.
- Drivers: `modernc.org/sqlite` (pure Go, local, driver name `sqlite`) or `github.com/tursodatabase/libsql-client-go` (remote-only, driver `libsql`/`turso`).
- Connection pooling via `SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`/`SetConnMaxIdleTime` in `NewDB`, verified with `PingContext`. Sizing guide: Dev 5/2, Small 25/5, Medium 50/10, Large 100/20 (open/idle).
- Query timeouts: simple SELECT 5s, complex SELECT/JOIN 15s, INSERT/UPDATE/DELETE 10s, bulk ops 60s, migrations 5m, reports 2m. `handleQueryError` maps `DeadlineExceeded`→TIMEOUT, `sql.ErrNoRows`→NOT_FOUND, `Canceled`→CANCELED, default→SERVER_ERROR.
- Transaction patterns: basic `WithTransaction` (30s timeout), optimistic locking (version column + retry), serializable isolation with `WithSerializableRetry` (backoff on SQLITE_BUSY/"database is locked") for correctness-critical transfers.
- Never `SELECT *` — name columns explicitly.

### Security & Logging (PART 11)

**Public endpoint info tiers:** Tier 1 (never shown, even in debug — DB creds, internal IPs, tokens, PGP private keys, others' PII, filesystem paths, account-existence signals); Tier 2 (always public — app name/version/commit/build date/uptime/mode, aggregate metrics, OpenAPI/GraphQL spec); Tier 3 (debug-only via `--debug` — stack traces, full error chains, CSP/CSRF violation context, rate-limit counts, redacted SQL, pprof).

**Output Sanitization Pipeline (every response, in order):** allow-list fields → redact sensitive query params (`token`,`session`,`code`,`key`,`password`,`secret`,`auth`,`api_key`,`access_token`,`refresh_token`) → strip internal IPs/paths → truncate (256/200/2KB limits) → strip `dev_only` fields in production → constant-time finalize (pad response toward a ~100ms floor).

**Auth rules:** constant-time comparison (`crypto/subtle.ConstantTimeCompare`) for all token/password/HMAC/signature checks; identical error messages and timing (≥100ms floor) regardless of failure cause; opaque UUID v4/v7 resource IDs only, never sequential.

**Owner API tokens:** store `SHA-256(token)` for validation and an AES-256-GCM encrypted copy (`token_enc`, keyed by `server.security.encryption_key`) for owner reveal. Owner and linked tokens are masked by default, never logged or returned by list/detail endpoints, and revealed only through an audited, no-store action. The operator token remains in `server.yml`, is validated by hash, and is never revealed by the server. Token-management routes must never be exposed as MCP tools.

**Cryptographic keys** (all stored server-side, never logged/returned, always in backups):
| Secret | Length | Storage | Rotation |
|--------|--------|---------|---------|
| `installation_secret` | 32B | `server.db` | manual, `--maintenance secret rotate`, 7d grace |
| `cookie_signing_key` | 32B HMAC-SHA256 | `server.db` | auto every 90d, 7d grace |
| `csrf_token_secret` | 32B | `server.db` | auto every 180d + on API key regen |
| `server.security.encryption_key` | 32B AES-256-GCM | `server.yml` | manual, `--maintenance secret rotate`, 30d grace |

Only `installation_secret` and `encryption_key` support manual rotation (sensitive-operation flow: re-prompt operator token, log to `audit.log`).

**Required security headers (every response):** `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, `Referrer-Policy`, `X-Permitted-Cross-Domain-Policies`, `Origin-Agent-Cluster: ?1`, `Cross-Origin-Opener-Policy`/`-Embedder-Policy`/`-Resource-Policy`, `Content-Security-Policy`, `Permissions-Policy`, `Reporting-Endpoints`/`Report-To`/`NEL`, `X-Request-ID`. SSL enabled adds `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` (opt-out via `web.hsts.preload: false`). Token revocation/consent withdrawal adds `Clear-Site-Data: "cache", "cookies", "storage"`.

**Permissions-Policy:** locked-down (`()`) by default for camera/mic/geolocation/USB/sensors/etc.; app-used features (WebAuthn, fullscreen, payment, autoplay) scoped to `(self)`; tracking features (`interest-cohort`, `browsing-topics`, `attribution-reporting`) always disabled. `IDEA.md`-declared features pre-fill the matching key.

**Content-Security-Policy default:**
```
default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';
img-src 'self' data: blob: https:; font-src 'self' https:;
connect-src 'self' {learned_origins}; media-src 'self' blob:;
worker-src 'self' blob:; manifest-src 'self'; frame-src 'self';
frame-ancestors 'self'; base-uri 'self'; form-action 'self';
object-src 'none'; upgrade-insecure-requests;
report-to default; report-uri /api/{api_version}/server/reports/csp
```
Operator extends per-directive (`script_src_extra`, `frame_src_extra`, etc.) — never redefines the whole policy. Dev mode / `mode: report-only` uses `Content-Security-Policy-Report-Only`; reports logged to `security.log` as `security.csp_violation`.

**Well-known & security disclosure:** required `robots.txt`, `security.txt` (RFC 9116); optional `llms.txt`, `pgp-key.asc`, ACME challenge. Security reports use a rotating `{security_id}` (HMAC-SHA256(installation_secret, floor(unix/172800)), 48h rotation) gating `/server/contact?security_id=` — submissions encrypted at rest (PGP or AES-256-GCM), never stored plaintext. GPG keypair managed via `--maintenance pgp` (Ed25519+Curve25519, private key encrypted with installation_secret-derived key, no web UI).

**Logging — log files:**
| Log | Format | Notes |
|-----|--------|-------|
| `access.log` | apache (default) / nginx / json | health checks (2xx) suppressed by default |
| `server.log` | text / json | application events |
| `error.log` | text / json | errors |
| `app.log` | logfmt / json | `key=value` pairs, RFC 3339 time |
| `auth.log` | syslog (RFC 3164) / json | `result=success\|fail reason=<stable_code>` |
| `audit.log` | json only | security/config events, append-only, tamper-evident |
| `security.log` | fail2ban (default) / syslog / cef / json / text | |
| `debug.log` | text / json | dev mode only |

All log FILES: raw ASCII, no ANSI/emoji/color/control chars, one event per line. Console output (stdout/stderr) may use color/emoji, respecting `NO_COLOR`. Defaults: `access.log` rotates monthly, `audit.log` daily, all others `weekly,50MB`; `keep: none` everywhere by default (delete on rotation).

**Audit log:** JSON Lines, required fields `id` (ULID), `time` (ISO 8601 ms UTC), `event`, `category`, `severity` (`info`/`warn`/`error`/`critical`), `actor`, `result` (`success`/`failure`); optional `target`, `details`, `reason`. Covers config changes, security incidents (rate limits, IP blocks, CSP/CSRF failures, secret rotation), backups, and server lifecycle. Never logs private keys, SMTP creds, full API credentials, or financial data; tokens masked to first 8 chars. File perms `0640`, append-only, rotation-only removal.

**Compliance standards** (`server.compliance.*`, all disabled by default): `gdpr`, `ccpa`, `hipaa`, `soc2`, `pci_dss`, `iso27001`, `fedramp`, `lgpd`, `pipeda`, `appi`, `pdpa`. When multiple are enabled and conflict, the strictest wins (longest retention, strongest encryption, shortest breach-notification window, shortest session timeout); right-to-erasure vs retention conflicts are resolved by anonymizing PII rather than deleting rows.

### Overlay Networks — Tor & I2P (PART 31)

**PART 31.1 — Tor Hidden Service (REQUIRED, auto-enabled):**

- All projects MUST support Tor: hidden service (.onion) always enabled if the `tor` binary is found; outbound Tor network routing is optional (`server.tor.use_network`, default `false`).
- App-scoped only — the outbound Tor client may only fetch app/server-determined destinations, never a URL/host/callback supplied by an inbound `.onion` visitor (closes the onion-visitor-as-SSRF-relay path). torrc hardening: `ExitRelay 0`, `ExitPolicy reject *:*`, no `ORPort`, `PublishServerDescriptor 0`, `DirPort 0`, `SocksPort 127.0.0.1:auto` only.
- Integration via external `github.com/cretz/bine` binary (not embedded) — keeps `CGO_ENABLED=0`.
- Server owns its own dedicated Tor process; NEVER uses system Tor. `HiddenServiceVersion 3` (v3 .onion, 56-char, ed25519) declared via torrc `HiddenServiceDir`/`HiddenServicePort` — NEVER via `ADD_ONION` control-protocol calls.
- Hidden service maps to a dedicated random-unused loopback backend port allocated fresh every run (`64000-64999`, never persisted), never the clearnet port: `HiddenServicePort 80 127.0.0.1:{torBackendPort}`.
- `HiddenServiceExportCircuitID haproxy` makes Tor prepend an HAProxy PROXY protocol v1 header to each backend connection; the app runs a dedicated `github.com/pires/go-proxyproto`-wrapped loopback listener on `torBackendPort` that parses the header and feeds the connection into the SAME handler chain as the public HTTP listener (`srv.ServeOn(...)`), never a separate/duplicate route tree.
- Tor itself generates and persists the v3 ed25519 key + `.onion` hostname under `HiddenServiceDir` — the app only reads `{hsDir}/hostname` post-bootstrap; vanity-address generation grinds and writes directly into `{hsDir}/hs_ed25519_secret_key` in Tor's on-disk key format, never through the control port.
- `ControlPort 127.0.0.1:auto` on all OSes — never a fixed/default control port (never 9050/9051). `SocksPort auto` when `server.tor.use_network` is enabled (outbound), `SocksPort 0` otherwise.
- Directories are NOT configurable: `{config_dir}/tor/` (torrc, regenerated every startup, 0700), `{data_dir}/tor/` (runtime data, 0700), `{data_dir}/tor/site/` (hidden service keys, 0700), log to `{log_dir}/tor.log`.
- Tor detection is priority 0 in FQDN resolution (PART 12) — evaluated before reverse-proxy headers, always trusted.
- If Tor binary not found: log INFO, disable Tor features, continue without Tor (no hard failure).
- `--status` includes Tor hidden-service state (Connected + .onion address) alongside node/cluster status.

**PART 31.2 — I2P Eepsite (OPTIONAL, opt-in, default OFF):**

- Unlike Tor, I2P is never auto-enabled — the eepsite is created only when `server.i2p.enabled: true` (or `I2P_ENABLED=true`/`--i2p`) **and** a provider resolves; no port, no `tunnels.conf`, no SAM session otherwise.
- I2P provides ONE capability: `.b32.i2p` eepsite hosting. It is never used for outbound anonymized requests (that's Tor's job), floodfill/relay, or SOCKS/HTTP client proxying.
- Two provider models, resolved in order: **Model A (preferred)** — an `i2pd` binary is found → app spawns/owns a dedicated i2pd process, regenerates its `tunnels.conf` server-tunnel every startup (torrc-regeneration parallel); **Model B (fallback)** — no i2pd binary but a SAMv3 bridge is reachable (default `127.0.0.1:7656`) → app speaks raw SAMv3 over `net.Conn` (no new dependency), creates a `STREAM` session from a persisted destination key and issues `STREAM FORWARD` to the backend port. Neither available → log WARN, disable I2P, continue (not an error).
- Eepsite backend is a **plain loopback listener** (`127.0.0.1:{i2p_backend_port}`) — no `go-proxyproto` wrapping, since neither i2pd nor SAM prepend a PROXY-protocol header (this differs from Tor).
- Destination identity persists under `{data_dir}/i2p/site/` (0600 keys); `.b32.i2p` address = `base32(sha256(destination)) + ".b32.i2p"`, computed with stdlib only. `tunnels.conf` (Model A) is derived state, regenerated each run like torrc.
- Directories NOT configurable: `{config_dir}/i2p/`, `{data_dir}/i2p/`, log `{log_dir}/i2pd.log` (Model A only; Model B logging belongs to the external router).
- A `.b32.i2p` request is trusted for FQDN resolution exactly like `.onion` — no reverse-proxy header or IP check applies.

For complete details, see AI.md PART 9, 10, 11, 31
