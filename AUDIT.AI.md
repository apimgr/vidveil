# Project Audit — Full AI.md PART 0–33 Compliance

Started: 2026-07-24

Scope: deep gap audit of every AI.md PART against actual src/, config, docker/,
tests/, workflows. Findings verified against AI.md line numbers and file:line.
Fix directly, one commit per finding (or per tightly-coupled subsystem) via
gitcommit. Delete this file when all resolved.

Priority key: HIGH = broken/missing core behavior · MED = non-conforming ·
LOW = minor/cleanup.

## Pass: Config & Structure (PART 2-6)
- [x] MED path/paths.go: GetDefaultBackupDir wrong macOS/Windows backup paths — FIXED
  (darwin sys=/Library/Backups/{org}/{name}, user=~/Library/Backups/...; windows
  sys=%ProgramData%\Backups\..., user=%LocalAppData%\Backups\... per PART 8 systemBackupDir/userBackupDir)
- [~] LOW path/paths.go: GetDatabaseDir nests db under data/ on privileged macOS/Windows —
  WONTFIX: current `filepath.Join(dataDir,"db")` matches AI.md 12133-12142 reference
  implementation exactly. Spec path table (6700) shows db as data sibling but the
  authoritative reference impl nests it; code follows the reference impl. Not a bug.
- [~] LOW main.go:321-325,465-469: PORT/LISTEN env honored every run vs first-run-only.
  WONTFIX-ambiguous: AI.md self-conflicts — 7705-7706 lists PORT/LISTEN as Init-Only,
  but the config-precedence table 12107-12108 explicitly maps PORT->--port / LISTEN->
  --address as runtime env fallbacks, and 10606-10607 defines the runtime port chain.
  Current code implements the 12107-table (runtime) reading. Flipping to init-only would
  break the documented flag<->env mapping. Left as-is; needs a spec de-conflict (AI.md is
  READ-ONLY) — not a code bug against the authoritative precedence table.

## Pass: Binary & CLI (PART 7-8)
- [ ] HIGH main.go handleMaintenanceCommand: `pgp` subcommand missing (spec 14659-14673)
- [ ] HIGH main.go handleMaintenanceCommand: `token` subcommand missing (spec 11815)
- [ ] MED main.go handleMaintenanceCommand: `data` (GDPR export/delete) missing (spec 15291-15340)
- [ ] MED main.go handleMaintenanceCommand: `compliance` report missing (spec 15444)
- [ ] LOW main.go:1604-1622: --maintenance help stale (advertises removed --password,
  omits pgp/token/data/compliance)

## Pass: Backend (PART 9-12)
- [x] LOW database.go HandleQueryError: used direct `==` comparison; spec (AI.md 13476)
  uses errors.Is — fixed to errors.Is (now matches wrapped errors) — FIXED
- [x] LOW database.go: pool settings hardcoded; added PoolConfig (max_open/max_idle/
  max_lifetime/max_idle_time) to DatabaseConfig with spec defaults (25/5/5m/1m),
  wired into NewAppDatabase per AI.md PART 10 Pool Configuration — FIXED

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
- [x] MED maintenance.go + client/cmd/update.go: beta channel now cumulative
  (newest of {beta,stable} via matchesBranch/matchesCLIBranch) (spec 29514) — FIXED
- [x] MED maintenance.go + client: daily channel now cumulative
  (newest of {daily,beta,stable}) (spec 29515) — FIXED
- [x] MED maintenance.go + client: daily tag detection now 14 digits (len==14, no dot);
  main.go/update.go help text YYYYMMDDHHMMSS (spec 29911) — FIXED
- [x] MED maintenance.go + client: checksum now from `checksums.txt` asset
  (parsed by asset filename), client checksum mandatory (spec 29818) — FIXED
- [ ] LOW/MED maintenance.go:1238: server ApplyUpdate no re-exec/restart (spec 29554/29929)
- [ ] LOW main.go:671-675: defer_days skips instead of newest-eligible-older fallback (29523-29526)

## Pass: Service/Docker/CI (PART 23-27)
- [x] MED docker/Dockerfile:72: ENV MODE=development baked in; spec (31799) = not set — FIXED
- [ ] LOW Makefile:69: extra targets `clean`+`i18n-validate` beyond mandated 6 (spec 30967)

## Pass: PART 28-33
- [x] MED tor/service.go: Start() only used exec.LookPath("tor"); added findTorBinary
  helper (config server.tor.binary -> PATH -> OS common locations per spec 39563-39572) — FIXED
- [ ] MED mkdocs.yml:100-127: nav points to nested files; spec-required flat pages
  (installation/configuration/api/development.md) orphaned (spec 37134 nav template)
- [ ] LOW tor/service.go buildTorrc: omits `ControlPort 127.0.0.1:auto` (spec 40036);
  bine-managed so functionally OK — parity-only, likely WONTFIX
- [~] LOW coverage threshold: testing-rules.md + Makefile enforce 80%; AI.md floor is
  60% (31358) with upward override via IDEA.md coverage_minimum. NOT-A-BUG: 80% is a
  stricter, allowed target and is internally consistent. Only nit: IDEA.md lacks the
  formal coverage_minimum:80 declaration — needs USER confirmation to add (rule 17).
  Makefile comment cites a non-existent "SERVER gate" — harmless. WONTFIX without user.

## LARGE subsystems — need dedicated implementation (flagged, tracked)
These are genuine spec gaps but multi-hour builds and/or carry regression/
deployment risk; listed precisely so work persists.
- [ ] HIGH TLS/ACME serving (PART 13-15): server.go ServeOn has no TLSConfig/ServeTLS;
  ssl.GetTLSConfig/GetHTTPHandler unused; no HTTP->HTTPS redirect; ACME handler
  unmounted. Wiring changes live deployment behavior.
- [ ] HIGH maintenance subcommands (PART 8): pgp (14659-14673), token (11815),
  data/GDPR (15291-15340), compliance (15444) all missing from dispatcher.
- [ ] HIGH frontend CSP+inline (PART 16): extract inline <script>/on* handlers from
  ~13 templates AND tighten CSP script-src to 'self'. Must ship together; UI-regression risk.

## Completed
- response.go: added Details field + SendErrorWithDetails helper (bfdfc2f6f39c)
