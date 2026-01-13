# REST API

Base URL: `/api/v1`

All responses are JSON unless otherwise specified.

## Authentication

Public endpoints require no authentication. Admin endpoints require a Bearer token:

```bash
curl -H "Authorization: Bearer YOUR_API_TOKEN" \
  https://your-server.com/api/v1/admin/server/settings
```

Or use the `X-API-Token` header:

```bash
curl -H "X-API-Token: YOUR_API_TOKEN" \
  https://your-server.com/api/v1/admin/server/settings
```

---

## Search

### Search Videos

```
GET /api/v1/search
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `q` | string | Yes | Search query (supports bangs like `!ph amateur`) |
| `engines` | string | No | Comma-separated engine list |
| `page` | int | No | Page number (default: 1) |

**Example Request:**

```bash
# Basic search
curl "https://your-server.com/api/v1/search?q=example"

# Search with bang
curl "https://your-server.com/api/v1/search?q=!ph+amateur"

# Search with specific engines
curl "https://your-server.com/api/v1/search?q=tutorial&engines=pornhub,xvideos"

# Paginated results
curl "https://your-server.com/api/v1/search?q=test&page=2"
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "query": "!ph amateur",
    "search_query": "amateur",
    "has_bang": true,
    "bang_engines": ["pornhub"],
    "results": [
      {
        "id": "abc123",
        "title": "Video Title",
        "url": "https://example.com/video",
        "thumbnail": "/api/v1/proxy/thumbnails?url=...",
        "duration": "10:30",
        "duration_seconds": 630,
        "views": "1.2M",
        "views_count": 1200000,
        "source": "pornhub",
        "source_display": "PornHub",
        "quality": "1080p"
      }
    ],
    "engines_used": ["pornhub"],
    "engines_failed": [],
    "search_time_ms": 450
  },
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 100,
    "pages": 5
  }
}
```

### Stream Search (SSE)

```
GET /api/v1/search/stream
```

Returns Server-Sent Events with results as they arrive from each engine.

**Example Request:**

```bash
curl -N "https://your-server.com/api/v1/search/stream?q=test"
```

**Example Response (SSE):**

```
event: result
data: {"title":"Video 1","url":"...","source":"pornhub"}

event: result
data: {"title":"Video 2","url":"...","source":"xvideos"}

event: done
data: {"total":2,"engines_completed":["pornhub","xvideos"]}
```

**JavaScript Example:**

```javascript
const evtSource = new EventSource('/api/v1/search/stream?q=test');

evtSource.addEventListener('result', (e) => {
  const result = JSON.parse(e.data);
  console.log('New result:', result.title);
});

evtSource.addEventListener('done', (e) => {
  const summary = JSON.parse(e.data);
  console.log('Search complete:', summary.total, 'results');
  evtSource.close();
});

evtSource.onerror = () => evtSource.close();
```

### Plain Text Search

```
GET /api/v1/search.txt
```

Returns plain text results (useful for CLI tools).

---

## Bangs

### List All Bangs

```
GET /api/v1/bangs
```

**Example Response:**

```json
{
  "success": true,
  "data": [
    {
      "bang": "!pornhub",
      "engine_name": "pornhub",
      "display_name": "PornHub",
      "short_code": "!ph"
    },
    {
      "bang": "!xvideos",
      "engine_name": "xvideos",
      "display_name": "XVideos",
      "short_code": "!xv"
    }
  ],
  "count": 54
}
```

### Autocomplete

```
GET /api/v1/bangs/autocomplete
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `q` | string | Yes | Partial bang input (e.g., `!po`) |

**Example:**

```bash
curl "https://your-server.com/api/v1/bangs/autocomplete?q=!po"
```

**Response:**

```json
{
  "success": true,
  "suggestions": [
    {"bang": "!pornhub", "short_code": "!ph", "display_name": "PornHub"},
    {"bang": "!pornmd", "short_code": "!pmd", "display_name": "PornMD"}
  ],
  "type": "bang_start"
}
```

---

## Engines

### List Engines

```
GET /api/v1/engines
```

**Example Response:**

```json
{
  "success": true,
  "data": [
    {
      "name": "pornhub",
      "display_name": "PornHub",
      "enabled": true,
      "available": true,
      "tier": 1,
      "features": ["api", "pagination", "hd"]
    },
    {
      "name": "xvideos",
      "display_name": "XVideos",
      "enabled": true,
      "available": true,
      "tier": 1,
      "features": ["pagination", "hd"]
    }
  ]
}
```

### Engine Details

```
GET /api/v1/engines/{name}
```

**Example:**

```bash
curl "https://your-server.com/api/v1/engines/pornhub"
```

---

## Stats

### Server Statistics

```
GET /api/v1/stats
```

**Response:**

```json
{
  "success": true,
  "data": {
    "engines_count": 51,
    "engines_enabled": 47
  }
}
```

---

## Health

### Health Check (JSON)

```
GET /api/v1/healthz
```

**Response:**

```json
{
  "status": "ok",
  "engines_enabled": 47
}
```

### Simple Health (Root)

```
GET /healthz
```

Also supports `/healthz.json` and `/healthz.txt` extensions.

---

## Admin API

All admin endpoints require Bearer token authentication. Generate tokens at `/admin/server/security/tokens`.

### Token Authentication

```bash
# Using Authorization header
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://your-server.com/api/v1/admin/server/settings"

# Using X-API-Token header
curl -H "X-API-Token: YOUR_TOKEN" \
  "https://your-server.com/api/v1/admin/server/settings"
```

### Server Settings

```bash
# Get current settings
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/settings"

# Update settings
curl -X PUT -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"server": {"port": "8080"}}' \
  "https://your-server.com/api/v1/admin/server/settings"
```

### Engine Management

```bash
# List all engines with admin details
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/engines"
```

### Backup & Restore

```bash
# Create backup
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/system/backup"

# List backups
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/system/backup"

# Restore from backup
curl -X POST -H "Authorization: Bearer TOKEN" \
  -F "file=@backup.tar.gz" \
  "https://your-server.com/api/v1/admin/server/system/backup/restore"
```

### Maintenance Mode

```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}' \
  "https://your-server.com/api/v1/admin/maintenance"
```

### Scheduler

```bash
# List scheduled tasks
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/scheduler"

# Run task manually
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/scheduler/{id}/run"
```

### Logs

```bash
# Access logs
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/logs/access"

# Error logs
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/logs/error"
```

---

## Error Responses

All errors follow this format:

```json
{
  "success": false,
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_QUERY` | 400 | Missing or invalid search query |
| `ENGINE_NOT_FOUND` | 404 | Requested engine doesn't exist |
| `RATE_LIMITED` | 429 | Too many requests |
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Rate Limiting

Default limits:
- **Search**: 60 requests/minute
- **API**: 120 requests/minute
- **Admin**: 30 requests/minute

Rate limit headers in responses:

```
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 115
X-RateLimit-Reset: 1704067200
```

When rate limited:

```json
{
  "success": false,
  "error": "Rate limit exceeded",
  "code": "RATE_LIMITED",
  "retry_after": 30
}
```

---

## Documentation Endpoints

| Endpoint | Description |
|----------|-------------|
| `/openapi` | Swagger UI |
| `/openapi.json` | OpenAPI 3.0 spec (JSON) |
| `/graphql` | GraphQL endpoint |
| `/graphiql` | GraphQL playground |
| `/graphql/schema` | GraphQL schema definition |
