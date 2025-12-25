# REST API

Base URL: `/api/v1`

All responses are JSON unless otherwise specified.

## Authentication

Public endpoints require no authentication. Admin endpoints require a Bearer token:

```bash
curl -H "Authorization: Bearer YOUR_API_TOKEN" \
  https://your-server.com/api/v1/admin/stats
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
| `q` | string | Yes | Search query |
| `engines` | string | No | Comma-separated engine list |
| `page` | int | No | Page number (default: 1) |
| `safe` | bool | No | Safe search filter |

**Example Request:**

```bash
# Basic search
curl "https://your-server.com/api/v1/search?q=funny+cats"

# Search with specific engines
curl "https://your-server.com/api/v1/search?q=tutorial&engines=pornhub,xvideos"

# Paginated results
curl "https://your-server.com/api/v1/search?q=music&page=2"
```

**Example Response:**

```json
{
  "success": true,
  "query": "funny cats",
  "page": 1,
  "results": [
    {
      "title": "Video Title",
      "url": "https://example.com/video",
      "thumbnail": "https://example.com/thumb.jpg",
      "duration": "10:30",
      "views": "1.2M",
      "engine": "pornhub",
      "quality": "1080p"
    }
  ],
  "total": 100,
  "engines_searched": ["pornhub", "xvideos", "xhamster"]
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
data: {"title":"Video 1","url":"...","engine":"pornhub"}

event: result
data: {"title":"Video 2","url":"...","engine":"xvideos"}

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

**Example:**

```bash
curl "https://your-server.com/api/v1/search.txt?q=test"
```

---

## Engines

### List Engines

```
GET /api/v1/engines
```

**Example:**

```bash
curl "https://your-server.com/api/v1/engines"
```

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "name": "pornhub",
      "enabled": true,
      "priority": 100,
      "categories": ["general", "amateur"]
    },
    {
      "name": "xvideos",
      "enabled": true,
      "priority": 90,
      "categories": ["general"]
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

## Health

### Health Check

```
GET /api/v1/healthz
```

**Example:**

```bash
curl "https://your-server.com/api/v1/healthz"
```

**Response:**

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "24h15m",
  "database": "ok",
  "cache": "ok"
}
```

### Simple Health (Plain Text)

```
GET /healthz
```

Returns `OK` if healthy.

---

## Stats

### Server Statistics

```
GET /api/v1/stats
```

**Example:**

```bash
curl "https://your-server.com/api/v1/stats"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "total_searches": 15420,
    "engines_enabled": 47,
    "cache_hits": 8234,
    "uptime_seconds": 86400
  }
}
```

---

## Admin API

All admin endpoints require Bearer token authentication.

### Get API Token

1. Log into admin panel at `/admin`
2. Go to Profile → API Token
3. Copy or regenerate your token

### Statistics

```bash
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/stats"
```

### Engine Management

```bash
# List all engines with admin details
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/engines"

# Enable/disable engine
curl -X PUT -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}' \
  "https://your-server.com/api/v1/admin/engines/pornhub"
```

### Server Configuration

```bash
# Get current config
curl -H "Authorization: Bearer TOKEN" \
  "https://your-server.com/api/v1/admin/server/settings"

# Update settings
curl -X PUT -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"server": {"port": "8080"}}' \
  "https://your-server.com/api/v1/admin/server/settings"
```

---

## Error Responses

All errors follow this format:

```json
{
  "success": false,
  "error": "Error message",
  "code": "ERROR_CODE",
  "status": 400
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

When rate limited, you'll receive:

```json
{
  "success": false,
  "error": "Rate limit exceeded",
  "code": "RATE_LIMITED",
  "status": 429,
  "retry_after": 30
}
```

---

## Code Examples

### Python

```python
import requests

# Search
response = requests.get(
    "https://your-server.com/api/v1/search",
    params={"q": "test", "page": 1}
)
data = response.json()

for result in data["results"]:
    print(f"{result['title']} - {result['duration']}")
```

### JavaScript (Node.js)

```javascript
const fetch = require('node-fetch');

async function search(query) {
  const response = await fetch(
    `https://your-server.com/api/v1/search?q=${encodeURIComponent(query)}`
  );
  const data = await response.json();
  return data.results;
}

search('test').then(results => {
  results.forEach(r => console.log(r.title));
});
```

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
)

type SearchResponse struct {
    Success bool `json:"success"`
    Results []struct {
        Title string `json:"title"`
        URL   string `json:"url"`
    } `json:"results"`
}

func main() {
    resp, _ := http.Get("https://your-server.com/api/v1/search?q=" +
        url.QueryEscape("test"))
    defer resp.Body.Close()

    var data SearchResponse
    json.NewDecoder(resp.Body).Decode(&data)

    for _, r := range data.Results {
        fmt.Println(r.Title)
    }
}
```
