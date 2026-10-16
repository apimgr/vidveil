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
token. VidVeil has no admin web UI or web-based settings API; configuration is
managed through `server.yml` and the CLI. See
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

MCP is enabled by default and uses the same API handlers and Bearer token as
REST. The canonical route and both direct aliases accept Streamable HTTP
JSON-RPC POST requests; GET and DELETE return 405.

```http
POST https://x.scour.li/api/v1/server/mcp
POST https://x.scour.li/api/mcp
POST https://x.scour.li/mcp
Authorization: Bearer {server.token}
Content-Type: application/json
Accept: application/json, text/event-stream
```

Set `server.mcp.enabled: false` to disable all three routes. MCP ignores
cookies and validates any supplied `Origin` against the request origin or an
explicitly configured CORS origin.

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
