# Server Configuration

VidVeil has no server settings page. Configure the server in `server.yml` and
restart or reload the service as supported by the setting. See the
[configuration guide](../configuration.md) for keys, defaults, and environment
variable precedence, and the [CLI reference](../cli.md) for service commands.

MCP is always enabled at `/api/{api_version}/server/mcp`, `/api/mcp`, and
`/mcp`. Every MCP request requires a valid API Bearer token, including calls to
public API tools. Use a linked owner token for owner-scoped tools; the operator
token (`server.token`) also reveals operator-only tools. Never give MCP clients
the primary owner token or the operator token. AI-agent discovery is served at `/.well-known/llms.txt` and
`/llms.txt`; its endpoint list is generated from OpenAPI and can be adjusted
with `web.llms` settings. See the [REST API guide](../api/rest.md) for routes
and the [configuration guide](../configuration.md) for YAML options.
The MCP connection guide is at `/server/help/mcp`; its VidVeil-specific
instructions come from `server.pages.help.mcp`.
The `/server/help` guide content is configured with `server.pages.help.content`
as shared Markdown or a language map. Locale selection uses the exact request
language, then `default`, then the first language entry in YAML order. The same
localized Markdown source is returned by `GET /api/{api_version}/server/help`;
see the [configuration guide](../configuration.md) for an example.

The standard server pages are server-rendered at `/server/about`,
`/server/privacy`, `/server/dpo`, `/server/help`, `/server/help/mcp`,
`/server/terms`, `/server/security`, `/server/security/policy`,
`/server/security/thanks`, `/server/security/report/{tracking_id}`,
`/server/contact`, `/server/preferences`, `/server/preferences/tokens`,
`/server/preferences/sync`, and `/server/healthz`.
Interactive API references are at
`/server/docs/swagger` and `/server/docs/graphql`. Their API routes and
configuration are documented in the [REST API guide](../api/rest.md),
[integrations guide](../integrations.md), and [configuration guide](../configuration.md).
