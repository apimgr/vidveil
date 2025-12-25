# Authentication

## Public Endpoints

Most search and engine endpoints are public and don't require authentication.

## Admin API

Admin API endpoints require an API token.

### Token Authentication

Include the token in the `Authorization` header:

```
Authorization: Bearer YOUR_API_TOKEN
```

Or use the `X-API-Token` header:

```
X-API-Token: YOUR_API_TOKEN
```

### Generating Tokens

1. Log in to the admin panel at `/admin`
2. Go to Profile > API Tokens
3. Click "Generate New Token"
4. Copy and securely store the token (shown only once)

## Rate Limiting

Default: 120 requests per minute per IP.

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 115
X-RateLimit-Reset: 1704067200
```
