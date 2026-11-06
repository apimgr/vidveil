# Server Configuration

VidVeil has no server settings page. Configure the server in `server.yml` and
restart or reload the service as supported by the setting. See the
[configuration guide](../configuration.md) for keys, defaults, and environment
variable precedence, and the [CLI reference](../cli.md) for service commands.

MCP is enabled by default at `/api/{api_version}/server/mcp`, `/api/mcp`, and
`/mcp`. Public tools allow anonymous requests. Use a linked owner token for
owner-scoped tools; do not give MCP clients the primary owner token or the
operator token (`server.token`). Set `server.mcp.enabled: false` to disable
MCP and its help page. AI-agent discovery is served at `/.well-known/llms.txt` and
`/llms.txt`; its endpoint list is generated from OpenAPI and can be adjusted
with `web.llms` settings. See the [REST API guide](../api/rest.md) for routes
and the [configuration guide](../configuration.md) for YAML options.
The MCP connection guide is at `/server/help/mcp`; its VidVeil-specific
instructions come from `server.pages.help.mcp`.

The standard server pages are server-rendered at `/server/about`,
`/server/privacy`, `/server/contact`, `/server/help`, `/server/help/mcp`,
`/server/terms`, and `/server/healthz`. Interactive API references are at
`/server/docs/swagger` and `/server/docs/graphql`. Their API routes and
configuration are documented in the [REST API guide](../api/rest.md),
[integrations guide](../integrations.md), and [configuration guide](../configuration.md).
