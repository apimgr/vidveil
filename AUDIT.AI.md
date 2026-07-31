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
- [x] metrics query-param token — FIXED (PART 20 specifies `Authorization: Bearer` as the sole auth channel; the `?token=` fallback leaked the secret into access logs and was never in spec — removed, now header-only; test updated to assert 401)
- [ ] install.sh downloads binary without checksum/signature verification — needs published checksums to verify against
- [x] audit.log perms — FIXED (spec check corrected the flag: PART 11 mandates 0640, not 0600; code created ALL logs at 0644, so audit.log was world-readable — now 0640, others stay 0644)

## Pass 2: Code Quality (flagged — dead/unwired code)
- [ ] src/notify webhook package is not imported anywhere (dead subsystem). It also carries internal bugs that only matter once wired: pushover drops token/user fields, telegram request body shape mismatch, logWebhookFailed is a no-op. Decision needed: wire it in or remove the package.
- [ ] ~80 exported symbols across packages have no external callers (dead public API). Unexporting is mechanical but touches many files; batch it deliberately rather than mid-audit.
- [x] maintenance.go NO_COLOR — FIXED (PART 8: the four `fmt.Println("✅ ...")` calls now go through terminal.StatusIcon(true), which returns "✅" or the "[OK]" fallback when NO_COLOR/no-emoji is set, matching the rest of the codebase)

## Pass 5b: Cache backend unwired (flagged — unimplemented spec subsystem)
- [ ] The entire configurable Valkey/Redis cache backend from PART 12 is unwired. Deeper analysis (correcting an earlier, wrong "compose typo" reading — the compose `CACHE_URL: valkey://...` and its `depends_on: service_healthy` on the Valkey container actually match the PART 26 template and show clear intent for the app to USE Valkey):
  - `config.CacheConfig` implements only a subset of the PART 12 schema — it has Type/Host/Port/Password/DB/Prefix/TTL but is MISSING `url`, `username`, `tls`, `tls_skip_verify`, `pool_size`, `min_idle`, `timeout` (PART 12 lines ~17000-17030; spec says "url takes precedence if both specified").
  - Nothing consumes `appConfig.Server.Cache` to select a backend. `cache.NewSearchResultCache` (the valkey/redis-capable constructor) and `database.NewValkeySyncChannel` are both DEAD — never called outside tests. The only search cache built is handlers.go:323 `cache.NewSearchCache(5*time.Minute, 1000)` — hardcoded in-memory, ignoring config entirely. `server.cache` is read only by debug.go:91 for display.
  - Net effect: the app ALWAYS uses in-memory cache regardless of `server.cache`, `CACHE_URL`, or `VIDVEIL_CACHE_*`; production compose gates startup on a Valkey container the app never connects to.
  - This is not a quick fix — it is a fully-specified but unimplemented subsystem (same category as the notify-webhook item above). Wiring it (add the missing CacheConfig fields + url parsing, map config->cache backend, change the handler's `searchCache` field from `*cache.SearchCache` to the `SearchResultCache` interface, build via NewSearchResultCache, read CACHE_URL into cache.url, add tests) would make production deployments start actually connecting to Valkey — a real user-visible/runtime behavior change introducing a hard external dependency. Left for the maintainer to schedule as a feature-completion, not silently changed mid-audit. Compose files intentionally left as-is (they match the PART 26 template).

## Pass 3: Logic (flagged)
- [ ] scheduler has no self-execution guard (a task can in principle re-trigger while still running); needs a design decision on overlap policy
- [ ] cache layer has no operation timeout; unbounded under a stalled backend
- [ ] engine manager fans out under a single RLock held across all engine calls — a slow engine can stall the batch; perf/locking redesign

## Pass 4: Documentation (flagged)
- [x] env-var inventory vs docs gap — FIXED (full read-vs-documented reconciliation done. User-facing server vars added to docs/configuration.md: BASEURL, DOMAIN, HOSTNAME, TZ, plus a documented VIDVEIL_{SECTION}_{KEY} config-override mechanism with cache/database/smtp examples verified against src/config/env.go. Missing CLI legacy aliases added to docs/cli.md: VIDVEIL_TIMEOUT, VIDVEIL_FORMAT, VIDVEIL_COLOR. The ~24 remaining undocumented vars are system auto-detection reads (DISPLAY, WAYLAND_DISPLAY, SSH_*, container, KUBERNETES_*, APPDATA, XDG_CONFIG_HOME, etc.) and are correctly left undocumented — not user-configurable. VIDVEIL_LANG is internal plumbing (set from --lang, never read as user input). The CACHE_URL discrepancy is logged separately under Pass 5b as a real compose defect.)
- [x] IDEA.md container-root note — FIXED (added a row to "Security decisions & exceptions" documenting that the container ENTRYPOINT execs as root to bind port 80, then the binary drops to the vidveil user via DropPrivileges after the listener is bound, per PART 23 — the transient-root form of the no-permanent-root default)

## Pass 5: Spec Compliance (flagged — not-yet-implemented spec subsystems)
- [ ] analytics/tracking subsystem described in spec is not implemented
- [ ] privacy consent configuration + GPC (Global Privacy Control) handling not implemented
- [ ] reporting endpoints described in spec not implemented
- [x] llms.txt — FIXED (PART 14: served at both /.well-known/llms.txt and /llms.txt, text/plain; charset=utf-8, auto-generated from routes with public+authenticated endpoints only, metrics never advertised, every URL resolved per-request via BuildURL; added to age-verify/content-restriction skip lists like robots.txt)
- [x] unified auth-header chain — RESOLVED as not-applicable (PART 8/14: the multi-header chain requirement applies only to "every auth-protected API endpoint". vidveil exposes NONE: every /api route is a public GET (search/bangs/engines/healthz/swagger) or a public POST (age-verify, content-restricted, preferences/save, contact, search/batch); the sole token-gated endpoint /metrics is spec-mandated Bearer-only per PART 20; and server.token is a CLI/operator credential (gates --maintenance restore/mode/setup, never verified over HTTP). The PART 14 requirement is therefore vacuously satisfied. Concrete fix: corrected the misleading llms.txt Authentication line that advertised a nonexistent "resource owner token issued on resource creation" — now states the public endpoints require no token)
- [x] /server/terms — FIXED (PART 16: added TermsPage HTML handler + server-terms.tmpl default template with Acceptance/Acceptable Use/Liability/Changes/Governing Law sections per PART 16 spec table; added APITerms JSON+text handler with content negotiation per PART 14; registered /server/terms and /api/v1/server/terms routes; added to sitemap. Footer left unchanged — PART 16 footer list is About/Privacy/Contact/Help only)
- [ ] token format / IsValidHost validation differ from spec's described format — rewrite would change accepted inputs
- [x] health check always returns OK regardless of subsystem state — FIXED (PART 13: checkDatabase now PingContext's the real *sql.DB with a 2s timeout, checkScheduler reflects the always-running scheduler's IsRunning() state, checkDisk uses maintenance.DiskSpace and fails at >=99% usage; cache is process-local so stays "ok". Wired via SetHealthDB/SetScheduler in server.go; a stopped scheduler now drives status=unhealthy/503. Test helpers start the scheduler to mirror production PART 18 always-running invariant)
- [ ] GeoIP has no IPv6 database wired; IPv6 lookups unsupported
- [ ] SSL DNS-01 dynamic provider selection not implemented (lego providers present but not surfaced)
- [ ] server CLI uses manual flag parsing rather than the spec-described structured parser
- [~] Go singular-directory convention — RE-EXAMINED, largely a false positive. The original rationale ("dirs plural but packages singular") is factually wrong: in all four (`secrets`, `metrics`, `urlvars`, `utls`) the package name already equals the directory name, so the AI.md "match package name" rule (lines 948-949) is satisfied. `utls` is the proper name of the uTLS library wrapper (renaming to `utl` would be nonsensical); `urlvars` is a compound utility name ("URL vars"), not a plural of a resource concept. Only `secrets`/`metrics` are borderline against the "singular directory names" rule (lines 1135/1146) — but both are idiomatic Go collective package names and the AI.md violation examples are all pluralized resource types (`handler`->`handlers`, `model`->`models`), not collective nouns. Left as a maintainer judgment call, NOT an auto-fixed rename — an incorrect rename here would break the build and the uTLS semantic.

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
