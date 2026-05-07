# Security

VidVeil is built around a privacy-first design. This page describes the
public-facing security model, public endpoints, and how to report
vulnerabilities.

## Threat Model Summary

VidVeil is internet-facing by default and assumes hostile traffic.

- The server **does not** track, log, or analyze user search queries beyond
  what is required to serve a single in-flight request.
- The server **does not** store user accounts. There is no registration,
  no profiles, no per-user history. (PART 34 of `AI.md` is intentionally
  not adopted by this project — see `IDEA.md`.)
- Server admins manage configuration via the admin panel; their accounts
  use Argon2id password hashing and support TOTP.
- Tor hidden service support auto-enables when the `tor` binary is
  present on the host or in the container.

## Public Status Endpoints

These endpoints are public and safe to monitor. They never expose secrets,
internal IPs, query strings, or stack traces.

| Endpoint                             | Purpose                                       |
|--------------------------------------|-----------------------------------------------|
| `GET https://x.scour.li/server/healthz`               | Health, version, build info, uptime.          |
| `GET https://x.scour.li/api/v1/server/healthz`        | Same data, JSON-only.                         |
| `GET https://x.scour.li/.well-known/security.txt`     | Machine-readable security contact metadata.   |
| `GET https://x.scour.li/server/contact`               | Public contact page.                          |

`/metrics` is **internal only** — it is not exposed to the public internet
unless the operator deliberately opens it, and even then it is gated by an
optional bearer token. Detailed performance telemetry never appears on the
public health endpoints.

## Reporting a Vulnerability

Use the **private** reporting flow:

- Preferred: [open a private security advisory][advisory] on GitHub.
- Alternative: contact the project owner via the GitHub profile at
  <https://github.com/apimgr>.

[advisory]: https://github.com/apimgr/vidveil/security/advisories/new

Please **do not** file a public issue for security problems. Public bug
reports for vulnerabilities are explicitly discouraged in
`.github/SECURITY.md` and `/server/contact`.

What to include:

- Affected version (`/server/healthz` `version` field, or `vidveil --version`).
- Steps to reproduce or a minimal proof-of-concept.
- Whether the issue has been disclosed elsewhere.

You should hear back within **3 working days**. We follow coordinated
disclosure: fix, advisory, release notes, version bump.

## Defense-in-Depth Controls

| Layer            | Control                                                            |
|------------------|--------------------------------------------------------------------|
| TLS              | TLS 1.2 minimum; auto-detected Let's Encrypt cert paths.           |
| Validation       | All inbound paths run through `SafePath` / path-traversal middleware. |
| CSRF             | Tokens on every state-changing form.                               |
| Headers          | CSP, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. |
| Rate limiting    | Per-IP, per-endpoint, configurable in the admin panel.             |
| Country blocking | `deny_countries` / `allow_countries` via GeoIP (PART 20).          |
| Blocklists       | Domain and IP blocklists, refreshed by the internal scheduler.     |
| Argon2id         | Server admin password hashing.                                     |
| TOTP             | Optional MFA for admins (suggested at first login, never forced).  |
| Tor              | Auto-enabled hidden service when `tor` is present.                 |
| Logging          | Structured logs; secrets and tokens are never logged plaintext.    |

## Supported Versions

VidVeil follows semantic versioning. Security fixes target the latest minor
version on `main`. Older minor versions do not receive backports unless
explicitly noted in the release notes.

## Operator Checklist

When self-hosting, follow this minimum baseline:

- Run behind HTTPS (Let's Encrypt or your own CA).
- Configure `server.fqdn` to your real domain so emitted links match.
- Enable TOTP for the primary admin account.
- Enable rate limiting (defaults are sane; tighten for high-risk surfaces).
- Configure GeoIP / blocklist updates (defaults run via the internal
  scheduler).
- Subscribe to release notifications on the GitHub repo so you see security
  releases promptly.

For full operator detail, see [`admin-guide/security.md`](admin-guide/security.md).
