# API Reference

## Base URL

- Production: `https://x.scour.li/api/v1`
- Local: `http://localhost:64580/api/v1`

## Authentication

Most endpoints are public. Admin endpoints require Bearer token:

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://x.scour.li/api/v1/admin/engines
```

## Search

### Search Videos

```http
GET /api/v1/search?q={query}&page={page}&limit={limit}
```

**Parameters:**

- `q` - Search query (1-500 characters, supports bang shortcuts)
- `page` - Page number (default: 1, max: 100)
- `limit` - Results per page (default: 20, max: 100)

**Response:**

```json
{
  "success": true,
  "data": {
    "query": "test",
    "results": [
      {
        "id": "abc123",
        "title": "Video Title",
        "url": "https://...",
        "thumbnail": "/proxy/thumbnail?url=...",
        "duration": "12:34",
        "duration_seconds": 754,
        "views": "1.2M",
        "views_count": 1200000,
        "source": "pornhub",
        "source_display": "PornHub"
      }
    ],
    "engines_used": ["pornhub", "xhamster"],
    "engines_failed": [],
    "search_time_ms": 1234
  },
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 142,
    "pages": 8
  }
}
```

### SSE Streaming Search

```http
GET /api/v1/search/stream?q={query}
```

Returns Server-Sent Events with real-time results as engines respond.

## Engines

### List Engines

```http
GET /api/v1/engines
```

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "name": "pornhub",
      "display_name": "PornHub",
      "enabled": true,
      "available": true,
      "features": ["api", "preview"],
      "tier": 1
    }
  ]
}
```

## Bang Shortcuts

### List Bang Shortcuts

```http
GET /api/v1/bangs
```

### Autocomplete

```http
GET /api/v1/bangs/autocomplete?q=!p
```

## Health Check

```http
GET /healthz
```

Returns `200 OK` if service is healthy.

## OpenAPI

- Swagger UI: `/openapi`
- OpenAPI Spec: `/openapi.json`

## GraphQL

- GraphQL Endpoint: `/graphql`
- GraphiQL UI: `/graphiql`
