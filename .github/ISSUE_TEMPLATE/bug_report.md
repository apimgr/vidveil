---
name: Bug report
about: Report a problem with VidVeil
title: "[bug] "
labels: ["bug", "needs-triage"]
assignees: []
---

<!--
Before filing: if you suspect a security vulnerability, STOP. Use the private
reporting flow described in SECURITY.md. Do not file a public bug for
security issues.
-->

## Version / Commit

<!-- Output of `vidveil --version`, or the `version` field from
     `GET /server/healthz`. Include the commit ID if you built from source. -->

## Environment / Platform

<!-- OS + arch, install method (Docker, binary, source), reverse proxy in
     front (nginx, Caddy, none), Tor enabled or not. -->

## Expected Behavior

<!-- What should have happened? -->

## Actual Behavior

<!-- What did happen? Include the smallest possible reproduction. -->

## Reproducible Steps

1. ...
2. ...
3. ...

## Logs / Screenshots

<!-- Paste relevant log output (redact tokens, IPs, and any personal data).
     For server-side issues: `journalctl -u vidveil` or the configured
     log_dir. For Docker: `docker logs vidveil`. -->

## Security Impact

- [ ] I believe this report may have a security impact.

<!-- If checked, please STOP and use the SECURITY.md private reporting flow
     instead of filing this issue. -->
