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

Admin endpoints accept either a Bearer token or `X-API-Token`:

```bash
curl -q -LSsf -H "Authorization: Bearer YOUR_API_TOKEN" \
  https://x.scour.li/api/v1/admin/server/settings
```

```bash
curl -q -LSsf -H "X-API-Token: YOUR_API_TOKEN" \
  https://x.scour.li/api/v1/admin/server/settings
```

## Search

### JSON Search

```http
GET https://x.scour.li/api/v1/search?q={query}&page={page}
```

```bash
curl -q -LSsf "https://x.scour.li/api/v1/search?q=!ph+amateur&page=1"
```

### SSE Search

The same endpoint switches to Server-Sent Events when `Accept: text/event-stream` is sent:

```bash
curl -q -LSsfN -H "Accept: text/event-stream" \
  "https://x.scour.li/api/v1/search?q=test"
```

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
```

## Health

```http
GET https://x.scour.li/api/v1/healthz
```

The frontend health page is available separately at `https://x.scour.li/healthz`.

## API Documentation

- OpenAPI UI: `https://x.scour.li/openapi`
- OpenAPI JSON: `https://x.scour.li/openapi.json`
- GraphQL endpoint: `https://x.scour.li/graphql`
- GraphiQL explorer: `https://x.scour.li/graphiql`
