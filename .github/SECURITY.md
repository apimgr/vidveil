# Security Policy

## Reporting a Vulnerability

**Do not file public bug reports for security issues.**

If you discover a vulnerability in VidVeil, please report it privately:

- Preferred: open a private security advisory at <https://github.com/apimgr/vidveil/security/advisories/new>.
- Alternative: email the project owner via the GitHub profile at <https://github.com/apimgr>.

Please include:

- A description of the issue and the impact you observed.
- Steps to reproduce, or a minimal proof-of-concept.
- The affected version (`/server/healthz` or `vidveil --version` output).
- Whether the issue is already known or has been disclosed elsewhere.

## What to Expect

- We aim to acknowledge new reports within **3 working days**.
- We will work with you to validate, scope, and fix the issue.
- Once a fix lands, we will publish a coordinated disclosure: GitHub advisory,
  release notes, and an updated `release.txt` version.

We do not currently offer a paid bug bounty.

## Supported Versions

VidVeil follows semantic versioning. The latest released minor version on the
`main` branch is the supported version for security fixes. Older minor versions
do not receive backports unless explicitly noted.

## Public Endpoints That Help You Stay Secure

- `GET https://x.scour.li/.well-known/security.txt` — machine-readable
  security contact metadata.
- `GET https://x.scour.li/server/healthz` — server status, version, build
  info (no sensitive data).
- `GET https://x.scour.li/server/contact` — public contact page.

## Out of Scope

- Vulnerabilities that require physical access to a self-hosted instance.
- Bugs in third-party dependencies — please report those upstream first; we
  will track them via Dependabot.
- "Self-XSS" requiring the victim to paste attacker-controlled JavaScript into
  the browser console.

## Coordinated Disclosure

Please give us a reasonable window (typically 90 days, or shorter if a fix
ships sooner) before public disclosure. We will credit reporters who request
attribution unless requested otherwise.
