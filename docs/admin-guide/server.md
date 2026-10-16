# Server Configuration

VidVeil has no server settings page. Configure the server in `server.yml` and
restart or reload the service as supported by the setting. See the
[configuration guide](../configuration.md) for keys, defaults, and environment
variable precedence, and the [CLI reference](../cli.md) for service commands.

MCP is enabled by default at `/api/{api_version}/server/mcp`, `/api/mcp`, and
`/mcp`; it uses the existing API Bearer token. Set `server.mcp.enabled: false`
to disable it. AI-agent discovery is served at `/.well-known/llms.txt` and
`/llms.txt`; its endpoint list is generated from OpenAPI and can be adjusted
with `web.llms` settings. See the [REST API guide](../api/rest.md) for routes
and the [configuration guide](../configuration.md) for YAML options.
