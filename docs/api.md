# API Reference

Vidveil exposes a versioned REST API plus a GraphQL endpoint. Every
REST route supports content negotiation (`application/json`,
`text/plain`, and a `.txt` extension) and returns a standard error
envelope.

When using the public deployment, the base URL is `https://x.scour.li`.

## Detailed Pages

- [REST API](api/rest.md) — endpoints, request/response formats, status codes
- [GraphQL](api/graphql.md) — schema and query examples
- [Authentication](api/authentication.md) — tokens and admin authorization
