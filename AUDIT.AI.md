# Project Audit

Started: 2026-07-30

All directly-fixable findings have been fixed and verified (build + vet + full
test suite pass in Docker). The items still checked below require a design
decision, change user-visible behavior, or implement spec subsystems that are
not yet built — they are left for the maintainer to decide and are NOT bugs
introduced by this audit. Delete an entry when resolved; delete this file when
all are resolved.

## Pass 1: Security (flagged — design/behavior decisions)
- [ ] handler CSP: policy still allows `unsafe-inline` for styles/scripts; removing it requires refactoring inline handlers/templates to nonces or hashes (would break current UI until templates migrate)
- [ ] setup_token stored in plaintext on disk; spec implies hashing — needs a decision on migration/one-time-display flow
- [ ] metrics endpoint accepts token via query param (`?token=`) which lands in access logs; moving to header-only is a client-facing contract change
- [ ] install.sh downloads binary without checksum/signature verification — needs published checksums to verify against
- [ ] audit.log written with 0640; confirm intended perms vs spec (0600) before tightening

## Pass 2: Code Quality (flagged — dead/unwired code)
- [ ] src/notify webhook package is not imported anywhere (dead subsystem). It also carries internal bugs that only matter once wired: pushover drops token/user fields, telegram request body shape mismatch, logWebhookFailed is a no-op. Decision needed: wire it in or remove the package.
- [ ] ~80 exported symbols across packages have no external callers (dead public API). Unexporting is mechanical but touches many files; batch it deliberately rather than mid-audit.

## Pass 3: Logic (flagged)
- [ ] scheduler has no self-execution guard (a task can in principle re-trigger while still running); needs a design decision on overlap policy
- [ ] cache layer has no operation timeout; unbounded under a stalled backend
- [ ] engine manager fans out under a single RLock held across all engine calls — a slow engine can stall the batch; perf/locking redesign

## Pass 4: Documentation (flagged)
- [ ] Several env vars are read in code but not documented in README/docs/configuration.md (env-var inventory vs docs gap). Needs the full read-vs-documented reconciliation before publishing a complete table.
- [ ] IDEA.md: document that the Docker runtime starts as root only to bind port 80, then drops to the `vidveil` user (intentional, worth recording as a deployment note)

## Pass 5: Spec Compliance (flagged — not-yet-implemented spec subsystems)
- [ ] analytics/tracking subsystem described in spec is not implemented
- [ ] privacy consent configuration + GPC (Global Privacy Control) handling not implemented
- [ ] reporting endpoints described in spec not implemented
- [ ] llms.txt not served
- [ ] unified auth-header chain (spec describes a single precedence chain) not implemented as specified
- [ ] /server/terms page not implemented
- [ ] token format / IsValidHost validation differ from spec's described format — rewrite would change accepted inputs
- [ ] health check always returns OK regardless of subsystem state — spec implies real dependency checks
- [ ] GeoIP has no IPv6 database wired; IPv6 lookups unsupported
- [ ] SSL DNS-01 dynamic provider selection not implemented (lego providers present but not surfaced)
- [ ] server CLI uses manual flag parsing rather than the spec-described structured parser

## Pass 6: Code Flow Trace (flagged)
- [ ] see Pass 2 dead-export and src/notify findings (call-graph dead ends)

## Completed (fixed and verified this audit)
- Pass 1 handler SSRF: privateCIDRs missing `0.0.0.0/8` and `::/128` (0.0.0.0 loopback bypass on Linux) — added both — src/server/handler/handlers.go
- Pass 1 generate-licenses.sh: unquoted `$PWD` in docker -v (word-splitting on spaces) — quoted — scripts/generate-licenses.sh
- Pass 1 server debug routes: /api/v1/debug/engines always registered — gated behind mode.IsDebugEnabled() — src/server/server.go
- Pass 2 system service: IsRunningAsRoot Windows branch leaked the os.Open handle — close before return — src/server/service/system/service.go
- Pass 2 logging: rotate-reopen assigned a possibly-nil file handle — guarded — src/server/service/logging/logging.go
- Pass 3 database Begin/Query/QueryRow: deferred cancel fired before the returned tx/rows/row was consumed (flaky use-after-cancel; caused intermittent TestAppDatabase_Version failure); rewrote to keep the PART 10 timeouts (30s tx / 5s reads) via a timeout timer instead of an immediate defer cancel — src/server/service/database/database.go
- Pass 3 engine manager: division-by-zero when resultsPerPage <= 0 — guarded default 50 — src/server/service/engine/manager.go
- Pass 3 main torTargetPort: strconv.Atoi error ignored — checked, warns and returns on invalid port — src/main.go
- Pass 3 scheduler: data race reading task.Enabled/NextRun outside the lock in checkAndRunTasks and runMissedTasks — snapshot under RLock — src/server/service/scheduler/scheduler.go
- Pass 3 config SaveAppConfig: non-atomic write — temp+rename — src/config/config.go
- Pass 3 maintenance backup: non-atomic write — temp+rename — src/server/service/maintenance/maintenance.go
- Pass 3 pgp private key: non-atomic write — temp+rename — src/server/service/pgp/pgp.go
- Pass 3 handler response: CSRF_FAILED mapped to 500 instead of 403 — added to 403 case — src/server/handler/response.go
- Pass 4 en.json: missing `privacy.summary` key broke locale parity (i18n-validate) — added — src/common/i18n/locales/en.json
- Pass 4 README badges: build.yml/security.yml badges referenced non-existent workflows — repointed to ci.yml — README.md
- Pass 4 LICENSE.md: removed stale robfig/cron/v3 (not a dep); relabeled phantom go-chi/cors to actual rs/cors; modernc.org/mathutil Unknown -> BSD-3-Clause; added missing linked deps ProtonMail/go-crypto (BSD-3-Clause) and go-acme/lego/v4 (MIT) — LICENSE.md
