# Authentication

## Public Endpoints

Most search and engine endpoints are public and don't require authentication.

## Operator-Protected API

Operator endpoints require the server-wide operator token configured in
`server.yml` as `server.token`. Public search endpoints do not require a token.
Resource ownership tokens, where supported, authorize only access to resources
owned by that token.

### Token Authentication

Include the operator token in the `Authorization` header:

```
Authorization: Bearer YOUR_API_TOKEN
```

### Using the Operator Token

Read the operator token from the protected server configuration and provide it
as a bearer credential. The server does not provide an admin web login or a
token-generation page. Use the CLI maintenance commands for supported token
and secret operations; see the [CLI reference](../cli.md).

## Rate Limiting

Default: 120 requests per minute per IP.

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 115
X-RateLimit-Reset: 1704067200
```
