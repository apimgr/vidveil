# Authentication

## Public Endpoints

Most search and engine endpoints are public and don't require authentication.

## Operator-Protected API

Operator endpoints require the server-wide operator token configured in
`secrets.yml` as `server.token`. Public search endpoints do not require a token.
Anonymous owner tokens are separate from the operator token. A primary token
manages its owner's linked tokens; linked tokens are intended for automation
and cannot manage tokens. API token-management routes accept Bearer credentials
only and ignore cookies. Raw owner tokens are encrypted for explicit reveal;
their SHA-256 digests are used for authentication.

Favorites are private owner resources. The first API favorite creation without
a token issues a primary token and returns it once in `data.owner_token`;
save it securely. Later favorite API requests must send that token or a linked
token as a Bearer credential. API routes ignore the browser owner cookie. Web
forms use the HttpOnly, Secure, SameSite=Strict cookie named
`vidveil_owner_token_ab5213f6`; the optional localStorage copy only enhances
JavaScript clients. The configured `server.token` has operator access to every
owner's favorites and must not be used by MCP clients.

### Token Authentication

Include the operator token in the `Authorization` header:

```
Authorization: Bearer YOUR_API_TOKEN
```

### Using the Operator Token

Read the operator token from the protected secrets file and provide it as a
bearer credential. The server does not provide an admin web login. Use
the CLI maintenance commands for operator token listing and revocation and
secret operations; see the [CLI reference](../cli.md).

### Owner Token Management

The browser token page is `/server/preferences/tokens`. A visitor without a
valid primary owner cookie can create a new primary identity with
`POST /server/preferences/token/create`; the server sets the owner cookie and
shows the raw token once. Save it securely. To recover an existing owner on a
new browser, restore its primary token. Token management is available when the
primary owner cookie is valid. The owner-token management API is available under
`/api/{api_version}/server/preferences/tokens`. Send the primary owner token
as `Authorization: Bearer …` for list, create, reveal, rotate, update, and
revoke operations. Linked tokens receive `403` on token-management routes.
List responses contain masked values only. Reveal and rotation responses use
`Cache-Control: no-store` and return a raw token only for that explicit action.
Use a separately named linked token for each automation or MCP client; linked
tokens can access their owner's resources but cannot manage tokens. Native API
credentials are accepted only through `Authorization: Bearer …`; alternate
credential headers, cookies, and `?token=` are ignored.

## Rate Limiting

Default: 120 requests per minute per IP.

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 115
X-RateLimit-Reset: 1704067200
```
