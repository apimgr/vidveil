# REST API

Base URL: `https://x.scour.li/api/v1`

Public endpoints are available without authentication unless noted otherwise.

## Response Format

JSON responses use the unified envelope:

```json
{
  "ok": true,
  "data": {}
}
```

Errors use:

```json
{
  "ok": false,
  "error": "CODE",
  "message": "Human-readable message"
}
```

## Authentication

Public search and information endpoints do not require authentication.
Operator-protected endpoints accept the server-wide `server.token` as a Bearer
token. Owner-token management is available at
`/api/{api_version}/server/preferences/tokens` and accepts only a primary owner
token as Bearer authorization. The preferences API stores browser preferences
in cookies; it does not change server configuration. See
[authentication](authentication.md) for token scope.

## Search

### JSON Search

```http
GET https://x.scour.li/api/v1/search?q={query}&page={page}
```

```bash
curl -q -LSsf "https://x.scour.li/api/v1/search?q=!ph+amateur&page=1"
```

Browser clients may request `display=human` to add localized presentation
values under `data.display` (`search_time`, `result_count`,
`result_count_text`, and `result_count_formats`). The raw `search_time_ms`,
result, and pagination fields retain their machine-readable values. Ordinary
API clients can omit this option.

### SSE Search

The same endpoint switches to Server-Sent Events when `Accept: text/event-stream` is sent:

```bash
curl -q -LSsfN -H "Accept: text/event-stream" \
  "https://x.scour.li/api/v1/search?q=test"
```

`display_offset` is used by the browser when appending an SSE page so the
localized `display_count` continues from the already-rendered result count.
Direct SSE clients can omit it; it affects presentation only.

### Plain Text Search

Plain text is available with `Accept: text/plain`:

```bash
curl -q -LSsf -H "Accept: text/plain" \
  "https://x.scour.li/api/v1/search?q=test"
```

### Batch Search

```http
POST https://x.scour.li/api/v1/search/batch
```

### Supporting search endpoints

```http
GET https://x.scour.li/api/v1/bangs
GET https://x.scour.li/api/v1/bangs/autocomplete?q={partial}
GET https://x.scour.li/api/v1/engines
GET https://x.scour.li/api/v1/engines/health
GET https://x.scour.li/api/v1/engines/{name}
GET https://x.scour.li/api/v1/stats
GET https://x.scour.li/api/v1/proxy/thumbnails?url={url}
GET https://x.scour.li/api/v1/proxy/videos?url={url}
```

The proxy endpoints validate upstream destinations and return proxied media.
Use the thumbnail endpoint for result images; video proxy availability depends
on the source engine and its supported preview data.

## Bangs

```http
GET https://x.scour.li/api/v1/bangs
GET https://x.scour.li/api/v1/bangs/autocomplete?q={partial}
```

## Engines

```http
GET https://x.scour.li/api/v1/engines
GET https://x.scour.li/api/v1/engines/health
GET https://x.scour.li/api/v1/engines/{name}
GET https://x.scour.li/api/v1/stats
GET https://x.scour.li/api/v1/version
```

## Health

```http
GET https://x.scour.li/api/v1/server/healthz
GET https://x.scour.li/server/healthz
```

`/server/healthz` is the canonical frontend health route. `/healthz` is an
optional root alias controlled by `server.healthz.root.enabled`.

## Guest preferences

Guest preferences are cookie based and need no API token. The API can read and
partially update preferences, or export/import them for another device:

```http
GET  https://x.scour.li/api/v1/server/preferences
POST https://x.scour.li/api/v1/server/preferences
GET  https://x.scour.li/api/v1/server/preferences/export
GET  https://x.scour.li/api/v1/server/preferences/import?theme=dark&lang=en
```

`POST` accepts JSON or form fields. Portable values are `theme`, `lang`, and
`vidveil_pref_*` cookies. Consent, build-stamp, session, and other credential
cookies are never exported or imported. API updates return the effective
preference state plus `applied` and `ignored` parameter names.

## MCP

MCP is enabled by default and uses the same API handlers and Bearer
authentication as REST. For owner-scoped tools, use a linked owner token
created for that MCP client; never use the primary owner token or the operator
token (`server.token`). Public tools work anonymously. The canonical route and
both direct aliases accept Streamable HTTP
JSON-RPC POST requests; GET and DELETE return 405.

```http
POST https://x.scour.li/api/v1/server/mcp
POST https://x.scour.li/api/mcp
POST https://x.scour.li/mcp
Authorization: Bearer {linked-owner-token}
Content-Type: application/json
Accept: application/json, text/event-stream
```

Set `server.mcp.enabled: false` to disable all three routes. MCP ignores
cookies and validates any supplied `Origin` against the request origin or an
explicitly configured CORS origin.

Connection instructions are available at `GET /server/help/mcp`; the JSON
equivalent is `GET /api/{api_version}/server/help/mcp`. Both return `404` when
MCP is disabled. The VidVeil-specific examples are configured by
`server.pages.help.mcp`.

## Owner token management

The browser token page is `/server/preferences/tokens`. It supports restoring
an existing primary token when opened in a new browser; management actions are
available only when the primary owner cookie is valid. API management routes
require the primary token in `Authorization: Bearer …`; linked tokens cannot
manage tokens. The first anonymous favorite created through the API returns a
new primary token once in `data.owner_token`. Save it securely and use the
token page to create client-specific linked tokens.

## Favorites

Favorites are private to an anonymous owner. API requests use a Bearer owner
token; the browser owner cookie is not accepted by API routes.

```http
GET    https://x.scour.li/api/v1/favorites
POST   https://x.scour.li/api/v1/favorites
DELETE https://x.scour.li/api/v1/favorites/{id}
DELETE https://x.scour.li/api/v1/favorites
POST   https://x.scour.li/api/v1/favorites/import
```

The first favorite created without an existing owner token issues a primary
token in the response. Store it securely. Use a linked token for automation;
only the primary token can manage linked tokens.

## Server information and operator endpoints

```http
GET  https://x.scour.li/api/v1/server/about
GET  https://x.scour.li/api/v1/server/privacy
GET  https://x.scour.li/api/v1/server/dpo
GET  https://x.scour.li/api/v1/server/help
GET  https://x.scour.li/api/v1/server/help/mcp
GET  https://x.scour.li/api/v1/server/terms
GET  https://x.scour.li/api/v1/server/security
GET  https://x.scour.li/api/v1/server/security/policy
GET  https://x.scour.li/api/v1/server/security/thanks
POST https://x.scour.li/api/v1/server/contact
POST https://x.scour.li/api/v1/server/reports/default
POST https://x.scour.li/api/v1/server/reports/nel
POST https://x.scour.li/api/v1/server/reports/csp
GET  https://x.scour.li/api/v1/version
GET  https://x.scour.li/api/autodiscover
GET  https://x.scour.li/cli/binaries/{filename}
POST https://x.scour.li/api/v1/server/restore
```

The DPO endpoint always returns `200` with `name`, `email`, and
`privacy_url: "/server/privacy"`. An empty DPO email falls back to
`server.contact.general.email`; if neither is configured, `email` is an empty
string. The HTML page is available at `/server/dpo` and always links to the
privacy page and contact form.

The three public security JSON endpoints mirror the overview, disclosure policy,
and researcher acknowledgments. The token-bearing
`/server/security/report/{tracking_id}` page has no API mirror.

`POST /api/v1/server/contact` accepts `name`, `email`, `subject`, `message`,
and a CAPTCHA response. With `simple` (default), send the one-use `captcha_id`
and `captcha_answer` shown by `/server/contact`; with `recaptcha` or `hcaptcha`,
send the corresponding `g-recaptcha-response` or `h-captcha-response` from the
provider widget. The CAPTCHA is required for browser and API submissions. Tor
and I2P always use the built-in challenge. Restore requires the operator token. CLI
binary downloads are public unless `cli.binary_download.require_auth` is enabled. Browser security-report
ingestion routes return `204` after recording a sanitized summary; they do not
return submitted report bodies.

## Metrics

When metrics are enabled, the server exposes `/server/metrics/{service}` and
`/api/v1/server/metrics/{service}` for `prometheus`, `grafana`, and `loki`.
`/metrics/{service}` and `/api/metrics/{service}` are aliases; the root alias
is controlled by `server.metrics.root.enabled`. Every metrics service requires
its own bearer token by default. Only enable
`server.metrics.auth.allow_unauthenticated: true` on a firewalled deployment.

## Agent discovery

The generated discovery document is served at both locations:

```http
GET https://x.scour.li/.well-known/llms.txt
GET https://x.scour.li/llms.txt
```

Configure it with `web.llms.enabled`, `include_endpoints`, `include_schemas`,
and `custom_sections`. Its endpoint list comes from the OpenAPI document and
does not advertise metrics or operator-only operations.

## API Documentation

- OpenAPI UI: `https://x.scour.li/server/docs/swagger`
- OpenAPI JSON: `https://x.scour.li/api/v1/server/swagger` (alias: `/api/swagger`)
- GraphQL endpoint: `https://x.scour.li/api/v1/server/graphql` (alias: `/api/graphql`)
- GraphiQL explorer: `https://x.scour.li/server/docs/graphql`
