# Integrations

VidVeil is intentionally narrow: it is a privacy-respecting meta search
engine that aggregates results from upstream video sites. It does not act
as an identity provider, does not host user accounts, and does not persist
search history.

This page describes the small set of external dependencies VidVeil does
talk to, plus the integration surfaces it exposes for clients.

## What VidVeil Does **Not** Integrate With

These integrations exist in the `AI.md` template but are **not adopted**
by VidVeil:

- **OIDC / LDAP / external identity providers.** VidVeil has no end-user
  accounts, and operators do not log in to a web interface. Operators
  administer the server through local configuration, CLI commands, and
  explicitly protected API operations.
- **OAuth providers (GitHub, Google, …).** Same reason as above.
- **Email user-onboarding flows.** No registration, no verification email
  loop. The email subsystem exists only for admin notifications.
- **Payment processors / billing systems.** VidVeil is fully free; there
  are no premium tiers, license keys, or paid features.
- **Custom user / org domains.** These are outside VidVeil's product scope.

These are excluded by VidVeil's project definition in `IDEA.md`; they are
not features left pending implementation from the current `AI.md`.

## External Services VidVeil Calls

| Direction | Target                                        | Purpose                                                                 | Failure mode                              |
|-----------|-----------------------------------------------|-------------------------------------------------------------------------|-------------------------------------------|
| Outbound  | The 33 upstream video search sites            | Fan-out search at request time. Results stream back via SSE.            | Per-engine timeout; partial results returned. |
| Outbound  | Let's Encrypt ACME (`acme-v02.api.letsencrypt.org`) | TLS certificate issuance and renewal when ACME is enabled.              | Falls back to existing cert; alerts admin. |
| Outbound  | GeoIP / blocklist / CVE feeds (configurable)  | Periodic refresh by the internal scheduler.                             | Last good DB stays in place; next refresh retries. |
| Outbound  | GitHub Releases for `apimgr/vidveil`            | Scheduled update checks and CLI release metadata; CLI downloads are proxied on request. | Existing version remains installed; failed checks/downloads report an error. |
| Outbound  | SMTP server (configurable)                    | Security-report notifications and researcher acknowledgements.         | Email delivery reports an error; the server keeps running. |
| Inbound   | Tor hidden service (when `tor` is present)    | Same routes as the public surface.                                      | Hidden service disabled; clearnet unaffected. |

Analytics are disabled unless configured by the operator, and analytics
scripts are served only after visitor consent. VidVeil does not contact a
vendor license server. Outbound requests are limited to searches initiated by visitors,
configured ACME issuance/renewal, scheduled data and release checks, CLI
release downloads requested by an operator, configured SMTP notifications,
and analytics requests enabled by the operator and visitor consent.

## Integration Surfaces VidVeil Exposes

These are the well-defined entry points clients can integrate against.

### REST API

- Versioned under `/api/v1/...`. See [`api/rest.md`](api/rest.md).
- Content negotiation: HTML for browsers, JSON for `Accept: application/json`,
  text for non-interactive HTTP tools.
- Standard error envelope: `{ ok, error, message, details }`.

### GraphQL

- POST `/api/v1/server/graphql` (also `/api/graphql` alias).
- GraphiQL UI at `/server/docs/graphql`.
- See [`api/graphql.md`](api/graphql.md).

### OpenAPI / Swagger

- Spec served at `/api/v1/server/swagger`.
- Swagger UI at `/server/docs/swagger`.

### MCP and AI-agent discovery

- MCP Streamable HTTP is exposed at `/api/{api_version}/server/mcp`, `/api/mcp`,
  and `/mcp`. Public tools allow anonymous access; use a linked owner token
  for owner-scoped tools. Never use the primary owner token or `server.token`
  for an MCP client. MCP can be disabled with
  `server.mcp.enabled: false`.
- `/server/help/mcp` explains how to connect and lists the live anonymous tool
  set; `/api/{api_version}/server/help/mcp` returns its JSON metadata.
- The generated endpoint guide is available at `/.well-known/llms.txt` and
  `/llms.txt`. See [configuration](configuration.md) for settings.

### CLI Client (`vidveil-cli`)

- Talks to the configured server over the REST API.
- See [`cli.md`](cli.md) for command reference and config layout.

### Health / Status

- `GET /server/healthz` — content-negotiated public status.
- `GET /api/v1/server/healthz` — JSON-only equivalent.
- `GET /metrics` — Prometheus exposition when the root alias is enabled.
  Metrics are internal-only and require a per-service bearer token by default.
  Operators can disable authentication only for a firewalled deployment with
  `server.metrics.auth.allow_unauthenticated: true`.

### Well-Known

- `GET /.well-known/security.txt` — security contact metadata.
- `GET /.well-known/pgp-key.asc` — the configured public security-reporting key,
  when one is available.
- `GET /.well-known/llms.txt` (also `/llms.txt`) — configurable agent
  discovery, generated from the current API description.
- `GET /humans.txt` — project and attribution information.

Other `/.well-known/*` paths return 404 unless explicitly defined. The server
also publishes `/robots.txt`, `/sitemap.xml`, `/manifest.json`, and `/sw.js`;
the sitemap omits visitor-specific pages.

## Tor Hidden Service

When the `tor` binary is present at startup, VidVeil enables a hidden
service automatically. Operators can inspect its status and address through
the server's documented status interfaces. Clearnet routing is unaffected. See
[`admin-guide/security.md`](admin-guide/security.md) for operator detail.
