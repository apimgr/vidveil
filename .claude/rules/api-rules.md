# API Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-31; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never create sub-routes under healthz (no `/server/healthz/db`, etc.) — one health endpoint per scope
- Never expose sensitive data in `/server/healthz`: DB connection strings, API keys/tokens, passwords/secrets, internal IPs, file paths, env vars, config contents, stack traces, SMTP/email details
- Never wrap the health response in the `{ok,data}` envelope — healthz is the one endpoint that returns its object bare, at every content type
- Never keep legacy endpoints — DELETE old/changed/removed routes completely, no backwards-compat shims, no deprecation periods (e.g. old root `/openapi`, `/openapi.json`, `/graphql` paths are gone, no redirects)
- Never layer new routes on top of old code — migrate handlers/templates/tests/clients to the new route tree and delete the old one
- Never use singular resource names, uppercase, underscores, trailing slashes, or verbs in API routes (`/item` ✗, `/Items` ✗, `/api_keys` ✗, `/items/` ✗, `/getItems` ✗)
- Never hardcode `v1` — always use `{api_version}` / `APIBasePath()`
- Never emit a bare top-level JSON array — always wrap in `{ "data": [...] }`
- Never invent an ad-hoc error shape — every 4xx/5xx uses the canonical `{ok, error, message, details}` envelope; never put the HTTP status code, a bare string, or a legacy `{error: "msg"}`/`{success:false}` shape in the body
- Never manually edit generated `openapi.json` or GraphQL schema files — both are build-time generated from code, never by hand
- Never place swagger/graphql files outside `src/swagger/` and `src/graphql/` (never project root)
- Never redirect unversioned API aliases (`/api/swagger`, `/api/graphql`, `/api/healthz`, `/api/metrics`) to the versioned path — mount the SAME handler at both paths
- Never enable `/healthz` or `/metrics` root aliases by default — each gated by its own `server.*.root.enabled` config flag
- Never do business logic, data transformation, or required-for-core-function work in client-side JavaScript — server does the work
- Never implement full external-API route cloning unless the user explicitly requests route/API/client compatibility — default is feature compatibility only
- Never partially implement an RFC-defined protocol (DNS, DHCP, SMTP, HTTP, FTP, NTP, WebDAV) — full RFC compliance is mandatory, not optional
- Never show icons/ASCII art/colors in log output — logs are always raw plain text
- Never let the app auto-renew certs under `/etc/letsencrypt/live/**` (system/certbot-managed) or `{config_dir}/ssl/local/{fqdn}/` (user-managed, manual only)
- Never reuse a version number; never use a `v` prefix inside the version string itself (git tags DO get `v` prefix); never derive the `devel` version from a timestamp — it is the short commit hash

## CRITICAL - ALWAYS DO

- Always provide `/server/healthz` (HTML/frontend) and `/api/{api_version}/server/healthz` (JSON API); both use content negotiation (HTML/JSON/text), body always bare (no `ok`/`data` wrapper)
- Always keep healthz data public-safe — safe: version, status, uptime, enabled public features, check status (ok/error only), aggregate stat counts, mode, pending-restart flag/reason
- Always version ALL API routes: `/api/{api_version}/...`; always use plural, lowercase, hyphenated nouns
- Always give project-specific API features a matching frontend route (CRUD parity); admin/system-only APIs stay file/API-only, no web route
- Always keep Swagger and GraphQL specs auto-generated, build-time regenerated, and in sync with each other and the live API
- Always use the canonical error envelope for every error response: `{ "ok": false, "error": "CODE", "message": "...", "details": {} }`
- Always use the canonical success envelope for actions: `{ "ok": true, "data": {...} }`; single-item GET returns the item directly, no wrapper
- Always support `.txt` extension + `Accept: text/plain` + non-interactive client detection on every `/api/**` route
- Always research the target service's actual API docs before implementing external compatibility — never guess
- Always implement built-in Let's Encrypt support (HTTP-01, TLS-ALPN-01, DNS-01) in every project
- Always resolve `{fqdn}` via reverse-proxy headers first, then `DOMAIN` env, then hostname, then public IP, then `localhost`
- Always check certificate lookup order on startup: `/etc/letsencrypt/live/domain/` → `/etc/letsencrypt/live/{fqdn}/` → `{config_dir}/ssl/letsencrypt/{fqdn}/` → `{config_dir}/ssl/local/{fqdn}/` → request new cert
- Always auto-renew app-managed certs (`{config_dir}/ssl/letsencrypt/{fqdn}/`) 7 days before expiry, checked daily at 03:00

## Key Rules Summary

**Health endpoints:**
| Route | Format | Notes |
|-------|--------|-------|
| `/server/healthz` | HTML (content negotiation) | Frontend, canonical |
| `/api/{api_version}/server/healthz` | JSON default | API, text via `.txt`/Accept/CLI |
| `/api/healthz` | JSON | Unversioned alias, same handler, no redirect |
| `/healthz` | Same as `/server/healthz` | Optional, `server.healthz.root.enabled: true` only, no redirect |

Body is always bare (no `{ok,data}` envelope) at every content type — the one deliberate exception to the standard envelope rule.

**HealthResponse struct field order:** `Project` (name/tagline/description) → `Status`/`PendingRestart`/`RestartReason` → `Version`/`GoVersion`/`Build` → `Uptime`/`Mode`/`Timestamp` → `Features` (public only) → `Checks` (ok/error only, no details — includes `database`/`cache`/`scheduler`/`tor`/`disk`) → `Stats` (public aggregates) → app-specific fields.

**Health status → HTTP code:**
| Status | HTTP |
|--------|------|
| healthy | 200 |
| degraded | 200 |
| restart_required | 200 |
| unhealthy | 503 |
| maintenance | 503 |
| shutting_down | 503 |

`restart_required` carries `pending_restart: true` + `restart_reason` (e.g. listen address/port/DB driver changed in `server.yml`).

**Versioning (SemVer):** MAJOR = breaking API/config/DB schema; MINOR = new backward-compatible features; PATCH = bug fixes/security/docs. Start at `1.0.0`, never `0.x.x`. Pre-release: `1.0.0-rc1`. Git tags: `v1.0.0`. Version sources priority: `release.txt` → git tag → `dev`.

**Version formats:** Stable `MAJOR.MINOR.PATCH`; Beta `YYYYMMDDHHMMSS-beta`; Devel = short commit hash (`git rev-parse --short=7 HEAD`) — never `release.txt` or a timestamp, for the rolling `devel` release.

**`--version` output:**
```
{project_name} {project_version}
Built: {build_date}
Go: {go_version}
OS/Arch: {GOOS}/{GOARCH}
```

**Route rules table:**
| Rule | Wrong | Correct |
|------|-------|---------|
| Versioned | `/api/items` | `/api/{api_version}/items` |
| Plural | `/api/{api_version}/item` | `/api/{api_version}/items` |
| Lowercase | `/Items` | `/items` |
| Hyphens | `/api_keys` | `/api-keys` |
| No trailing slash | `/items/` | `/items` |
| No verbs | `/getItems` | `GET /items` |

**Route scopes:** `/server/*` ↔ `/api/{api_version}/server/*` (server-owned, public). `/*` ↔ `/api/{api_version}/*` (project resources).

**Path vs query params:** identity (IDs, slugs, categories) → path; modifiers (pagination, sorting, filtering) → query.

**Response envelopes:**
- Single item: raw object, no wrapper
- Action (create/update/delete): `{ "ok": true, "data": { "id": "...", "message": "..." } }`
- Error: `{ "ok": false, "error": "CODE", "message": "...", "details": {} }` — forbidden shapes: bare status code, `{error:"msg"}`, `{success:false}`, HTML error pages for API routes
- Pagination (default 250/page): `{ "data": [], "pagination": { "page", "limit", "total", "pages" } }`
- Health: bare object, no envelope (see above)

**Formatting:** JSON/YAML/HTML/CSS/JS = 2-space indent; Go/Makefiles = tabs; every response and file ends with exactly one trailing newline; JSON via `MarshalIndent(v, "", "  ")` + `\n`.

**Content negotiation priority (API routes):** `.txt` extension → `Accept: text/plain` → non-interactive client (curl/wget) → default JSON.
**Content negotiation priority (frontend routes):** `Accept: text/html` → `Accept: text/plain` → User-Agent browser detection → CLI/curl → default HTML.

**Client types:** Our CLI (`{project_name}-cli/`) → JSON, renders own TUI. Text browsers (lynx/w3m/links/elinks) → no-JS HTML, forms via POST. HTTP tools (curl/wget/httpie, or no UA) → `HTML2TextConverter()` plain text (forms/inputs/scripts skipped).

**API types required:** REST (primary) + Swagger/OpenAPI (JSON only, no YAML) + GraphQL — all three, always in sync, auto-generated at build time, themed to match the project's dark/light theme system. Locations: `src/swagger/{swagger,theme,annotations}.go`, `src/graphql/{graphql,schema,resolvers,theme}.go`.

**Root-level endpoints:**
| Endpoint | Auth |
|----------|------|
| `/`, `/server/healthz`, `/server/docs/swagger`, `/server/docs/graphql` | None |
| `/server/metrics[/{service}]` (services: `prometheus`, `grafana`, `loki`) | Bearer token per service |
| `/metrics[/{service}]` | Optional root alias, `server.metrics.root.enabled: true`, same auth as above |
| `/api/autodiscover`, `/api/swagger`, `/api/graphql`, `/api/healthz`, `/api/metrics[/{service}]` | None on discovery/docs/health; metrics still needs its bearer token — unversioned aliases, same handler as versioned, never a redirect |
| `/api/{api_version}/server/swagger`, `/graphql`, `/healthz`, `/metrics[/{service}]` | None except metrics (token) |
| `/api/{api_version}/server/*` | Mixed — info public, operator endpoints need operator token (PART 11) |

**MCP (PART 14):** `POST /api/{api_version}/server/mcp` is the canonical Streamable HTTP JSON-RPC endpoint; `/api/mcp` and `/mcp` are direct aliases to the same handler, never redirects. It uses existing API Bearer validation and authorization, ignores cookies, and has no MCP-specific credentials or sessions. Token-management routes are never exposed as MCP tools. `/.well-known/llms.txt` and `/llms.txt` expose generated endpoint discovery.

Legacy root paths `/openapi`, `/openapi.json`, and bare `/graphql` GET/POST are removed entirely — no redirects, no shims.

**External compatibility:** default = feature/behavior compatibility using our own routes; only add external route surface when user explicitly asks for route/API/client compatibility. Full protocols (Matrix, ActivityPub, WebDAV/RFC-defined) require complete spec compliance — not optional.

**SSL/TLS challenge types:** HTTP-01 (port 80, default), TLS-ALPN-01 (port 443), DNS-01 (wildcard, any DNS provider via `server.tls.dns_provider` + encrypted `server.tls.dns_credentials.*`, validated on startup and before each cert request).

**Certificate directory structure:**
```
{config_dir}/ssl/
├── letsencrypt/{fqdn}/{fullchain.pem,privkey.pem}   (app-managed, auto-renew)
└── local/{fqdn}/{cert.pem,key.pem}                  (user-managed, manual)
```
Mirrors certbot's `/etc/letsencrypt/live/{fqdn}/` layout.

**Cert ownership:** `/etc/letsencrypt/live/**` = system/certbot, app never renews. `{config_dir}/ssl/letsencrypt/{fqdn}/` = app auto-renews 7 days before expiry (daily 03:00 check). `{config_dir}/ssl/local/{fqdn}/` = manual only.

**FQDN resolution order:** reverse-proxy headers (`X-Forwarded-Host`/`X-Real-Host`/`X-Original-Host`) → `DOMAIN` env (comma-separated list, first = primary, multi-domain infers wildcard) → `os.Hostname()` → `$HOSTNAME` → public IPv6 → public IPv4 → `localhost`. Dev TLDs (`{project_name}`, `.local`, `.test`, `.example`, `.invalid`, etc.) fall back to the global IP for display/access, not the dev name.

**Port rules:** single port = HTTP (except 443 = HTTPS-only); dual = first HTTP, second HTTPS; `ssl.enabled` can override. Always strip `:80`/`:443` from displayed URLs.

**Overlay networks (Tor/I2P):** HTTP by default; inherit HTTPS-only from clearnet if clearnet is port-443-only; self-signed certs only (LE doesn't support `.onion`/`.i2p`).

For complete details, see AI.md PART 13, 14, 15
