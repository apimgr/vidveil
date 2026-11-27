# Temporary AI.md Alignment Audit

This file tracks findings from the explicit repository alignment audit. Remove it after every item is implemented and verified.

- [ ] Run staged update verification immediately after path resolution, before root startup work.
- [ ] Route all production database reads/writes through the shared store API; remove raw writer handle access from app, services, and handlers.
- [ ] Use bounded store transactions for maintenance paths, token cleanup, privacy deletion, and PGP/key rotation.
- [ ] Make Jenkins secret scanning use range mode for branch/change builds and verified-only full-history mode for scheduled builds.
- [ ] Stage and validate both managed TLS files before replacement and restore the previous pair if replacement fails.
- [ ] Set the macOS service account home to the resolved data directory.
- [ ] Reconcile all server-rendered pages, docs, deployment workflows, and client surfaces with their AI.md requirements.
- [ ] Run every required local verification gate and resolve failures before commit.
