# REST API

Base URL: `/api/v1`

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

**Response:**

```json
{
  "success": true,
  "query": "search term",
  "page": 1,
  "results": [
    {
      "title": "Video Title",
      "url": "https://example.com/video",
      "thumbnail": "https://example.com/thumb.jpg",
      "duration": "10:30",
      "views": "1.2M",
      "engine": "pornhub"
    }
  ],
  "total": 100
}
```

### Stream Search (SSE)

```
GET /api/v1/search/stream
```

Returns Server-Sent Events with results as they arrive from each engine.

### Plain Text Search

```
GET /api/v1/search.txt
```

Returns plain text results.

## Engines

### List Engines

```
GET /api/v1/engines
```

### Engine Details

```
GET /api/v1/engines/{name}
```

## Health

```
GET /api/v1/healthz
```

## Stats

```
GET /api/v1/stats
```
