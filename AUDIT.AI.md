# Project Audit — Full AI.md PART 0–33 Compliance

Started: 2026-07-24

Scope: deep gap audit of every AI.md PART against actual src/, config, docker/,
tests/, workflows. Findings verified against AI.md line numbers and file:line.
Fix directly, one commit per finding (or per tightly-coupled subsystem) via
gitcommit. Delete this file when all resolved.

Priority key: HIGH = broken/missing core behavior · MED = non-conforming ·
LOW = minor/cleanup.

## Pass: Config & Structure (PART 2-6)
- [ ] MED path/paths.go: GetDefaultBackupDir wrong macOS/Windows backup paths
- [ ] LOW path/paths.go: GetDatabaseDir nests db under data/ on privileged macOS/Windows
- [ ] LOW main.go:321-325,465-469: PORT/LISTEN env honored every run; spec (AI.md
  7705-7712) = first-run-only seeding

## Pass: Binary & CLI (PART 7-8)
- [ ] HIGH main.go handleMaintenanceCommand: `pgp` subcommand missing (spec 14659-14673)
- [ ] HIGH main.go handleMaintenanceCommand: `token` subcommand missing (spec 11815)
- [ ] MED main.go handleMaintenanceCommand: `data` (GDPR export/delete) missing (spec 15291-15340)
- [ ] MED main.go handleMaintenanceCommand: `compliance` report missing (spec 15444)
- [ ] LOW main.go:1604-1622: --maintenance help stale (advertises removed --password,
  omits pgp/token/data/compliance)

## Pass: Backend (PART 9-12)
- [ ] LOW database.go HandleQueryError (296-312): dead code, non-sentinel non-%w errors
- [ ] LOW database.go: pool settings hardcoded (25/5/5m/1m), no DatabaseConfig fields (verify spec)

## Pass: API & SSL (PART 13-15)
- [ ] HIGH server.go ServeOn (683-695): no TLSConfig/ServeTLS — SSL never served;
  ssl.GetTLSConfig/GetHTTPHandler defined but never called. COUPLED w/ redirect+ACME.
- [ ] HIGH: no HTTP→HTTPS redirect when SSL enabled (spec PART 15)
- [ ] MED: ACME challenge handler never mounted in serve path
- [ ] MED: port-based listen behavior vs spec
- [x] MED response.go: error envelope missing `details` field — FIXED bfdfc2f6f39c

## Pass: Frontend (PART 16) — COUPLED cluster (CSP+inline must ship together)
- [ ] HIGH templates: inline <script> blocks (footer.tmpl:44-101 etc.)
- [ ] HIGH templates: inline on* handlers across ~13 templates
- [ ] HIGH server.go:202 CSP script-src has 'unsafe-inline'; spec (AI.md 14169) = 'self' only
- [ ] MED: alert()/confirm() usage in templates
- [ ] MED server.go: /server/terms HTML page missing
- [ ] MED: /api/v1/server/terms missing
- [ ] LOW server.go: /server → /server/about redirect missing
- [ ] LOW templates: hardcoded English strings (should be i18n keys)

## Pass: Features (PART 17-22) — all in PART 22 Update
- [ ] MED maintenance.go:1061 + client/cmd/update.go: beta channel non-cumulative
  (must pick newest of {beta,stable}) (spec 29514)
- [ ] MED maintenance.go:1079 + client: daily channel non-cumulative
  (must pick newest of {daily,beta,stable}) (spec 29515)
- [ ] MED maintenance.go:1077 + client: daily tag regex `^\d{12}$`, spec = 14 digits (29911)
- [ ] MED maintenance.go:1117 + client: checksum from `.sha256` sidecar; spec = `checksums.txt` asset (29818)
- [ ] LOW/MED maintenance.go:1238: server ApplyUpdate no re-exec/restart (spec 29554/29929)
- [ ] LOW main.go:671-675: defer_days skips instead of newest-eligible-older fallback (29523-29526)

## Pass: Service/Docker/CI (PART 23-27)
- [x] MED docker/Dockerfile:72: ENV MODE=development baked in; spec (31799) = not set — FIXED
- [ ] LOW Makefile:69: extra targets `clean`+`i18n-validate` beyond mandated 6 (spec 30967)

## Pass: PART 28-33
- [ ] (pending agent a7b6ae595a00aa22d)

## Completed
- response.go: added Details field + SendErrorWithDetails helper (bfdfc2f6f39c)
