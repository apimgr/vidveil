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
  accounts. Server admins authenticate locally with Argon2id + optional
  TOTP. PART 34 of `AI.md` is not implemented for this project — see
  `IDEA.md`.
- **OAuth providers (GitHub, Google, …).** Same reason as above.
- **Email user-onboarding flows.** No registration, no verification email
  loop. The email subsystem exists only for admin notifications.
- **Payment processors / billing systems.** VidVeil is fully free; there
  are no premium tiers, license keys, or paid features.
- **Custom user / org domains (PART 36).** Not implemented.

If you operate VidVeil and need any of the above, you would need to fork
and adopt the corresponding `AI.md` PART (and update `IDEA.md` to match).

## External Services VidVeil Calls

| Direction | Target                                        | Purpose                                                                 | Failure mode                              |
|-----------|-----------------------------------------------|-------------------------------------------------------------------------|-------------------------------------------|
| Outbound  | The 43 upstream video search sites            | Fan-out search at request time. Results stream back via SSE.            | Per-engine timeout; partial results returned. |
| Outbound  | Let's Encrypt ACME (`acme-v02.api.letsencrypt.org`) | TLS certificate issuance and renewal when ACME is enabled.              | Falls back to existing cert; alerts admin. |
| Outbound  | GeoIP / blocklist / CVE feeds (configurable)  | Periodic refresh by the internal scheduler.                             | Last good DB stays in place; next refresh retries. |
| Outbound  | SMTP server (configurable)                    | Admin notifications only.                                               | Notifications drop; server keeps running.  |
| Inbound   | Tor hidden service (when `tor` is present)    | Same routes as the public surface.                                      | Hidden service disabled; clearnet unaffected. |

VidVeil does **not** phone home, send telemetry, or contact a vendor
license server. The only outbound network the binary makes on its own
authority are ACME renewals and the scheduled refresh feeds you have
configured.

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

### CLI Client (`vidveil-cli`)

- Talks to the configured server over the REST API.
- See [`cli.md`](cli.md) for command reference and config layout.

### Health / Status

- `GET /server/healthz` — content-negotiated public status.
- `GET /api/v1/server/healthz` — JSON-only equivalent.
- `GET /metrics` — Prometheus exposition. **Internal only.** Optionally
  bearer-token-gated.

### Well-Known

- `GET /.well-known/security.txt` — security contact metadata.

Other `/.well-known/*` paths return 404 unless explicitly defined.

## Tor Hidden Service

When the `tor` binary is present at startup, VidVeil enables a hidden
service automatically. The `.onion` address is shown on `/server/healthz`
and in admin tooling. Clearnet routing is unaffected. See
[`admin-guide/security.md`](admin-guide/security.md) for operator detail.
