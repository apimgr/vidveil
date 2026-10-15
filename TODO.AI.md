# TODO.AI.md

Beta-test finding (2026-08-15): production `net::ERR_FAILED` on
`/search?q=...` under load, second/residual cause after the redirect-encoding
fix (951a480b3ae7) was confirmed still working.

- Reproduced a genuine server-side condition using a Dockerized `devel`
  build (`casjaysdev/go:latest` build, `alpine:latest` run, real internet
  engine searches, age-verified session cookie): bursting 150 concurrent
  requests at `/search?q=...` produced a handful of client-side "Empty
  reply from server" failures (curl error 52 — the same network-layer
  symptom as browser `net::ERR_FAILED`), caused by the Go HTTP server's
  `Server.Limits.WriteTimeout` (default 30s) firing mid-response-write.
  80-concurrent did not reproduce it; 150-concurrent did, both before and
  after the fix below.
- Root-caused one real gap and fixed it: `EngineManager.batchDeadline()`
  (`src/server/service/engine/manager.go`) computed the fan-out wait budget
  purely from the largest per-engine timeout (`Search.EngineTimeout`/
  `EngineTimeouts`, default 15s) plus a 2s grace, with nothing tying that
  budget to the server's actual `Server.Limits.WriteTimeout`. Under load,
  or with a higher configured `EngineTimeout`, total handler time could
  approach/exceed `WriteTimeout` with no safety margin. Fixed by capping
  `batchDeadline()` at `WriteTimeout - writeTimeoutSafetyMargin` (5s),
  falling back to the 30s default when `WriteTimeout` is unset/invalid.
  Covered by `TestBatchDeadline_CappedByWriteTimeout`,
  `TestBatchDeadline_UnderWriteTimeout_Unaffected`,
  `TestBatchDeadline_InvalidWriteTimeout_FallsBackToDefault` in
  `engine_manager_coverage_test.go`.
- Resolved after the fan-out semaphore and rate-limit handling landed. A
  150-request search burst in Incus (direct container loopback, without
  Docker port forwarding) produced 150 HTTP responses (4 × 200, 146 × 429)
  and zero empty replies. It exposed a separate overload-page bug: the HTML
  429 response omitted required pagination fields, causing a template error
  and showing the generic empty-results state. Search overload rendering now
  supplies the complete template model and displays the localized busy
  message. The regression test and full `make test` pass; after rebuilding,
  a second Incus burst again returned 150 responses (4 × 200, 146 × 429),
  with no empty replies and no overload-template errors in the journal.
- Follow-up implemented (2026-08-15): added a `searchSem` concurrency
  semaphore in `EngineManager` (`src/server/service/engine/manager.go`)
  bounding simultaneous `SearchWithOperators` fan-outs (wires up the
  previously-dead `Search.ConcurrentRequests` config field), with a 2s
  `searchQueueTimeout` before returning the canonical
  `{"ok":false,"error":"RATE_LIMITED",...}` envelope
  (`overloadedSearchResponse`) instead of queueing indefinitely. Wired that
  envelope through to actual HTTP `429 Too Many Requests` + `Retry-After: 2`
  responses (AI.md PART 12 "Rate Limiting") in every handler calling
  `SearchWithOperators`: `SearchPage` (both the `text/html` branch and the
  non-browser content-negotiation branch), `APISearch` (also guarded against
  caching the transient overload envelope), `SearchRSSFeed`, and
  `SearchAtomFeed` (`src/server/handler/handlers.go`, new
  `isSearchOverloaded`/`writeSearchOverloadJSON` helpers; `response.go` split
  `renderResponse` into a thin wrapper over new `renderResponseStatus` so an
  explicit status code can be threaded through content negotiation).
  `BatchSearch` needed no change — its per-item response array already
  carries the RATE_LIMITED envelope per sub-query correctly.
- Follow-up implemented (2026-08-15): `handleSearchSSE`'s
  `SearchStreamWithOperators` fan-out (`manager.go`) now shares the same
  `searchSem` guard as `SearchWithOperators`. Since SSE cannot set an HTTP
  status after streaming starts, overload is signaled through the stream
  itself: a new `StreamResult.Overloaded` field, set when the semaphore
  wait hits `searchQueueTimeout`/`ctx.Done()`, causes `handleSearchSSE` to
  emit an `event: error` frame with the canonical RATE_LIMITED envelope and
  stop. `app.js`'s existing `eventSource.onerror` handler already covers
  this (SSE named "error" events route through the same handler as
  connection failures) with no frontend changes needed. Covered by
  `TestSearchStreamWithOperators_SaturatedSem_ContextCancelled_EmitsOverload`,
  `TestSearchStreamWithOperators_SaturatedSem_QueueTimeout_EmitsOverload`,
  `TestSearchStreamWithOperators_UnsaturatedSem_AcquiresSlotAndReleases` in
  `engine_searchsem_coverage_test.go`.
- Resolved: the rate-limiter cleanup goroutine now has an explicit stop signal;
  `Server.Shutdown` stops and joins it after HTTP shutdown, and endpoint
  limiter collections expose the same lifecycle hook.

Findings from the PART 11/31 security compliance pass (2026-08-20), flagged
but not fixed — each is architecture-sized or spec-contradicted and needs a
decision before implementation:

- Tor hidden-service architecture — RESOLVED (verified 2026-09-28, stale entry
  from 2026-08-20). The architecture is now torrc-driven exactly as AI.md
  PART 31.1 requires, NOT `AddOnion`. `buildTorrc`
  (`src/server/service/tor/service.go:1189`) emits `ControlPort 127.0.0.1:auto`,
  `HiddenServiceDir`, `HiddenServicePort 80 127.0.0.1:{port}` and
  `HiddenServiceExportCircuitID haproxy`; `newTorBackendListener()`
  (`tor/service.go:1330`) allocates the dedicated PROXY-protocol loopback
  listener on a fresh random 64000-64999 port every process start and wraps
  it in `&proxyproto.Listener{}` (`tor/service.go:1347`).
  `github.com/pires/go-proxyproto` v0.15.0 IS in go.mod. `.onion:80` therefore
  maps to the dedicated backend port, never the clearnet listener.
- torrc persistence — RESOLVED. `ensureTorrc`
  (`src/server/service/tor/service.go:1301`) unconditionally rewrites the file
  on every startup (it only uses the `os.Stat` to report whether the file
  pre-existed), and re-chowns/chmods 0600 on non-Windows.
- Tor rate-limit/blocklist keying — RESOLVED. `urlvar.TorClientLabel(req)`
  (`src/server/service/urlvar/urlvar.go:675`) returns `tor:{circuit_id}` when
  `req.RemoteAddr` falls in the fc00::/8 range Tor uses to encode the rendezvous
  circuit ID, and the literal sentinel `tor` when circuit-ID export is
  unavailable — never `127.0.0.1`, never deanonymizing. Callers substitute this
  for the client IP in access logs, audit trails and rate-limit keys.
- Resolved: Tor status reports `enabled=false` and omits the onion address
  when the Tor binary is missing, matching PART 31's instruction to disable
  Tor features and continue. The generated key remains available internally.
- Resolved: CSP now uses fixed default directives with per-directive `*_extra`
  extension fields, a limited `*_override` surface, learned origins, and an
  enforce/report-only mode. See `src/config/config.go` and
  `src/server/server.go`.
- Output sanitization is now applied by `WriteJSON` and by the canonical
  `SendOK`, `SendError`, and detailed-error envelope helpers. Direct JSON
  response paths in Grafana and Loki metrics output now pass through the same
  sanitizer. A static inventory found no other direct public JSON encoder in
  server source. CSRF token comparisons now hash both values to fixed-length
  inputs and apply the required timing floor to both accepted and rejected
  mutating browser requests. A common response middleware now delays
  state-changing responses until the same floor before committing headers or
  body bytes, including handlers that return without writing; operator-token
  diagnostic reads are padded on success and failure. Regression tests and the
  full Go suite plus browser E2E suite pass.

Findings from the PART 16 frontend/PWA compliance pass (2026-08-20),
flagged but not fixed:

- Resolved: `csrfBypass` no longer skips validation based on a session cookie;
  it follows the closed AI.md list (safe methods, bearer/API token, WebSocket,
  configured exempt paths). CSRF JSON failures also use the shared sanitized
  response writer.
- Resolved: cookie-consent message, policy link, and button labels are read
from `server.privacy.consent` settings, including the sold/not-sold
message selection required by PART 12.

Finding from the app.js hardcoded-strings audit fix (2026-08-22, task 7)
— RESOLVED. `addResultCard()` in `src/server/static/js/app.js` and its
favorite-toggle/clipboard-copy call sites (menu aria-labels, "Open in
new tab", "Copy link", "Add/Remove favorites", "Download", "Swipe to
preview", " views" suffix, "Untitled" title fallback, "Video result"
aria-label fallback, fetch-failure/connection-error/retry paths, "Link
copied to clipboard", "Added/Removed from favorites" toasts) now read
from `#app-i18n`/`#favorites-i18n` data islands via `getAppI18n()`/
`getFavoritesI18n()` with English-literal fallbacks. Added new locale
keys `favorites.added`, `favorites.removed`, `search.card_link_copied`
(all 7 locale files) and reused `search.card_untitled` for the
favorites-list page via a new `untitled` key in the `#favorites-i18n`
island (`favorites.tmpl`).

Resolved 2026-10-14: compared all seven current locale files (817 keys each)
and translated every remaining identical-to-English human-readable value in
Arabic, German, Spanish, French, Japanese, and Simplified Chinese. Identical
values remaining are product names, proper names, service-manager names,
standard resolution labels, technical identifiers, punctuation, units, or
placeholder-only content. Locale parity and placeholder validation pass as
part of `make test` (72% overall coverage). The translation pass has not had
independent native-speaker review.

Resolved: all six GitHub/Gitea/Forgejo daily and beta workflows now compute
version, commit ID, and build epoch in a single `version` job and pass them
through `needs.version` outputs to their matrix build jobs. `act --list` passes
for both GitHub workflows. `act` rejects Gitea/Forgejo-specific context names
(`gitea`/`forgejo`), so those workflows require provider-native validation;
the same command cannot validate them.

AI.md diff implementation follow-ups (2026-09-03), flagged during the
Tor Circuit-ID/PROXY-protocol implementation pass, not yet fixed:

- Resolved: the runtime account identity comes from
`system.ServiceUserName()` and is frozen as `vidveil`; `NewServiceManager`
also stores the frozen identity for service, user, and group paths. The raw
executable name is used only for the user-facing service description.
- Resolved: Compose files use `${TZ:-America/New_York}` and no env files.
  Production Compose no longer overrides `MODE`; per AI.md PART 26 it sets
  neither `DEBUG` nor `MODE`, allowing the binary's production default to apply.
- Resolved: verified the original items #6-#12 against the current imports,
  CSS, JavaScript, templates, and handlers: UI/a11y styles and `showToast()`
  are present; site-banner dismissal includes CSRF and same-site return
  fields; theme switching has its form and JS support; conditional 304
  responses are implemented; dead 410 engines are excluded; proxy failures
  use the canonical 500/404 errors. `make test` and browser E2E pass.

## Browser E2E suite (AI.md PART 28)

- Resolved: E2E dependencies are in `go.mod`, and the complete
  `tests/e2e.sh` suite passes in the Go/Chromium containers.
- Fixed stale selectors and routes in the E2E tests, including the actual
  `resultsPerPage` preference control and API routes.
- Fixed browser form submission tests that typed Enter before clicking, which
  triggered navigation before the click action.
- Fixed the development HTTP CSP to emit `upgrade-insecure-requests` only when
  SSL is enabled or the request is HTTPS, as specified by PART 11. The prior
  unconditional directive upgraded HTTP pages' CSS, API calls, and form
  submissions to HTTPS.
- Fixed the browser startup exception from exporting undefined `updateSort`,
  and constrained mobile horizontal overflow from the closed navigation panel.

## Decisions needed (surfaced during the PART 17-22 features audit)

- Resolved: PART 21 now has an operator-token-protected multipart restore API
  (`POST /api/{api_version}/server/restore`) and a no-JS WebUI upload form at
  `/server/restore`; encrypted API restores without the separate backup
  password return the specified 400 `VALIDATION_FAILED` response. Browser
  writes use the configured double-submit CSRF middleware. The UI uses an
  explicit operator-token field because PART 16 says HTML routes remain
  public and there are no admin sessions; see the exact PART 16/PART 21
  tension recorded in `AUDIT.AI.md`.
- Resolved: skipped overlapping scheduler runs remain in task history and
  scheduler state, but no longer use a third Prometheus status label. PART 20
  restricts the metric labels to `success` and `error`.
- Resolved: maintenance managers created by the API and scheduler now
  serialize full/incremental backup and restore operations with a process-wide
  mutex and an OS-backed lock file in a service-owned directory adjacent to
  the data root, so restore replacement cannot remove the active lock. Server,
  scheduler, and standalone CLI processes sharing that directory now coordinate
  before creating backups or replacing shared roots. Lock contention and
  release are covered by a regression test; `make test` passes at 72% coverage.
- Resolved: backup path resolution now prefers the system backup location when
  it or its nearest existing ancestor is writable; otherwise system mode uses
  `{data_dir}/backup/` and user mode uses its user backup path, per AI.md PART 8.
  The resolved backup directory is created and assigned to the service account
  before privilege drop, so scheduled backups remain writable.
- Resolved by AI.md PART 21 update (2026-10-15): `server.restore` now defines
  archive (1G), expanded-size (4G), entry-count (10000), per-file (512M), and
  compression-ratio (100:1) defaults. Configuration validates and normalizes
  these values; restore checks archive and tar metadata before each write,
  counts decompressed bytes while streaming, and removes staged entries after a
  limit failure.
- Resolved: CLI restore stops and restarts a detected managed service and refuses
  to alter live data when the PID file identifies an unmanaged running server.
  Successful API/WebUI restores now drain every HTTP listener and restart the
  server in-process so all service managers and foreground deployments reload
  restored configuration; see `AUDIT.AI.md`.

## Decisions needed (surfaced during the CI/CD audit)

- Resolved: added the AI.md PART 27 CycloneDX SBOM generation step to the
  Gitea release, beta, and daily workflows, matching the existing Forgejo
  workflows.
- AI.md PART 27 has no Gitea/Forgejo `ci.yml` spec block (only release, beta,
  daily, docker), yet `cicd-rules.md` requires `ci.yml` everywhere. Both files
  exist and are sound but have no authority to be judged against.
- Jenkins identifies timer-triggered builds by their UTC start
  hour. The 3am build runs the normal build, test, daily release, and standard
  Docker stages while skipping the devel image; the 4am timer skips those
  stages and runs only the devel image; pushes and other non-timer builds keep
  both Docker stages. This uses Jenkins' documented `currentBuild.getBuildCauses`
  and `startTimeInMillis` APIs. It has not been exercised on a Jenkins
  controller; long queue delays crossing an hour can misclassify a scheduled
  run. Jenkins core's `TimerTriggerCause` exposes no scheduled-time method
  ([Javadoc](https://javadoc.jenkins.io/hudson/triggers/TimerTrigger.TimerTriggerCause.html));
  parameterizing separate cron lines requires the optional Parameterized
  Scheduler plugin ([plugin documentation](https://plugins.jenkins.io/parameterized-scheduler/)).
  Resolve the controller/plugin contract before replacing the current heuristic.
- Resolved: `.gitlab-ci.yml` now derives the daily version with POSIX
  `printf | cut` instead of Bash substring syntax. The daily source archive,
  artifact path, and release link use the `RELEASE_VERSION` dotenv value, so
  the artifact filename matches the binaries and SBOM without relying on
  unsupported substring expansion. YAML parsing and the version extraction
  under `/usr/bin/sh` passed.

## Decisions needed (surfaced during the service/Docker/Tor audit)

- Resolved: `.claude/rules/docker-rules.md` now follows AI.md PART 26 and
  prohibits creating a user or setting `USER` in the Dockerfile; the binary
  owns service-account creation and privilege dropping.
- Resolved compose host-port and fixed-name conflicts using the AI.md PART 26
  update (2026-10-15): production, development, and test compose files use the
  same fixed app/cache container names. The spec explicitly says dev and test
  never run on a production host, and their project/network names isolate their
  stacks. Production uses port 64893, development 64580, and testing 64581.
- Resolved Makefile target discrepancy using AI.md PART 0 precedence: FINAL is
  reference-only, while PART 25 explicitly defines exactly six targets (`dev`,
  `local`, `build`, `test`, `release`, `docker`). Obsolete FINAL checklist
  entries for `clean` and `all` do not add targets. The Makefile and rule file
  follow the PART 25 contract.
- Resolved locale-validation target discrepancy using AI.md PART 0 precedence:
  FINAL is reference-only, PART 25 allows exactly six targets, and PART 30
  requires build-time locale-key validation. Locale integrity checks run in
  the i18n package tests under `make test`; no separate `i18n-validate` target
  is required.
- Resolved: `--service` output and Unix/Windows privilege-escalation prompts
  use locale keys in all seven locale files.
- Resolved: user-level service control now matches user-level installation:
  Linux uses `systemctl --user`, and macOS uses the LaunchAgents plist path
  for status/control. Unit tests, `go vet`, all eight cross-builds, and Incus
  systemd integration pass.

## Frontend (PART 16) items deferred during the CSS/JS/a11y verification pass

- Resolved: AI.md PART 16 branding images now use the secure fetcher, decode
  supported raster formats, generate the required logo/favicon/OpenGraph
  variants, and store them in the data directory. Same-origin routes serve
  cached files; templates use the mobile logo and scaled favicon variants;
  failed refreshes retain prior files and embedded defaults provide fallback.
  Startup and the internal daily scheduler refresh sources, with a configurable
  `server.branding.refresh_interval`; config hot-reload refreshes the cache too.
- Resolved for the known app.js strings: search loading/errors, all-sources,
  related-search toggles, end-of-results, offline status, and the update banner
  now use locale keys through the global or search-page i18n data islands.
- Resolved: the search status bar uses translated result counts, streaming status, engine response counts, and minimum-duration text with named tokens.

## Resolved: named i18n interpolation (AI.md PART 30)

- Translation values and all seven locale files use literal `{token}`
  placeholders. Template calls pass token/value pairs, and
  `Translator.TranslateFormat` supports both the AI.md `map[string]string`
  form and the template-friendly pair form without treating translations as
  Go format strings.
- Locale validation now checks named-token parity. Browser-side count and
  search updates use the same named placeholders. Verified by `make test`
  in the project Go container (75% coverage).

## Decisions needed (surfaced during the handler/status-code audit)

- Proxy error statuses changed to stay inside AI.md's canonical error-code
  table: `/api/{v}/proxy/thumbnails` and `/api/{v}/proxy/videos` now return
  500 `SERVER_ERROR` for upstream fetch failures (was 502) and 404
  `NOT_FOUND` for a non-200 upstream (was the upstream's verbatim status).
  This is spec-conformant but is an observable API change. If a
  `BAD_GATEWAY` code is wanted, it has to be added to the AI.md table first
  — AI.md is read-only, so that is a human decision.
- `/sw.js`, `/manifest.json`, and `/offline.html` still return 404 when their
  `go:embed` read fails. Those reads cannot realistically fail; 500 would
  arguably be more correct, but the status was left unchanged rather than
  altering behavior unprompted.

## Resolved finding: GUI toolkit selection (PART 32)

- AI.md PART 32 now selects `gogpu/ui` + `gogpu/gogpu` for Linux/macOS/Windows
  and specifies TUI/CLI fallback on BSD. `src/client/gui/gui.go` and
  `.claude/rules/binary-rules.md` now follow that contract; Gio and Fyne are
  not compliant choices.
- Full transitive-dependency cgo verification (Dockerized `go mod download
  all` + `grep -rl '^import "C"'` across every module, then reading each
  hit's `//go:build` tag) was run against two further candidates:
  - `github.com/gogpu/ui@v0.1.54` — CLEAN. Own source and its entire
    transitive graph (gpucontext, wgpu, gg, gputypes, gogpu core, naga,
    go-webgpu/goffi, go-webgpu/webgpu) contain no cgo import active on a
    default `CGO_ENABLED=0` Linux build. Only cgo hit found anywhere in the
    graph is `go-webgpu/goffi`'s `dl_android_cgo.go`, gated
    `android && cgo && arm64` (irrelevant to Linux). This is the strongest
    verified candidate so far.
  - `github.com/go-gui-org/go-gui@v0.75.0` — DISQUALIFIED. Its hard,
    unconditional dependency `gopxl/beep/v2` -> `ebitengine/oto/v3`
    (`speaker.go` imports oto unconditionally) has an ALSA-backed Linux
    driver (`driver_unix.go`) that carries `#cgo pkg-config: alsa` +
    `import "C"` under the build tag
    `!android && !darwin && !js && !windows && !nintendosdk && !playstation5`
    — satisfied on default Linux, and NOT gated behind the `cgo` build
    constraint itself (unlike `ebitengine/purego`'s internal cgo files,
    which are safely gated `cgo && (...)` and excluded under
    `CGO_ENABLED=0`). No way to exclude this via build tags without patching
    upstream.
  - `github.com/go-gtk/gtk4` and `modernc.org/tk9.0` were not deep-verified
    for cgo because both fail on a more basic ground first: each requires a
    system GTK4 or Tcl/Tk runtime installed on the target machine, which
    conflicts with AI.md's global "Single self-contained binary" /
    "First-run works with zero config" defaults — a disqualifying
    architectural tradeoff independent of cgo status.
- No implementation work remains for this decision.

## Resolved finding: `version.BuildTime` naming (was flagged by go-lint, 2026-09-11)

- `src/common/version/version.go:22` was flagged as "rename `BuildTime` ->
  `BuildEpoch` per AI.md PART 7". That suggestion is a **false positive** and
  is NOT to be applied. Do not rename.
- The flag conflated two different variables in two different packages:
  - `main.BuildEpoch` — the Unix-seconds ldflags target, the single embedded
    time source (Makefile:42,51 and docker/Dockerfile{,dev}:26 all pass
    `-X 'main.BuildEpoch=...'`).
  - `version.BuildTime` — a *derived display string*, assigned exactly once at
    `src/main.go:82` from the already-formatted `main.BuildDate`
    (`time.Unix(n,0).UTC().Format("2006-01-02T15:04:05Z")`). It is never an
    ldflags target; `go build -X` cannot and does not reach it.
- Renaming it to `BuildEpoch` would make the code strictly wrong: the value
  it holds is an RFC 3339 date string, not a Unix timestamp. The name would
  then contradict the value at all 11 read sites
  (`version.go:96` `--version` output, `version.go:125` `build_time` map key,
  and `handlers.go:963,2016,2076,2237,2270,3502,3612,3654`). Two of those
  (`BuildDateTime()` at `handlers.go:962` and the `build.date` YAML emitter at
  `handlers.go:2076`/`3612`) parse it back as a date.
- AI.md never names `version.BuildTime` or the `build_time` JSON key anywhere;
  the PART 7 constraint it was flagged against governs the *ldflags* surface
  only, and the `version` package is outside that surface.
- No action required. The naming is intentional: `BuildEpoch` (embedded
  input) vs `BuildDate`/`BuildTime` (derived display output).

## Maintenance CLI implementation status

Resolved since the 2026-09-30 audit: `--maintenance secret`, `token`, and
`data` now have dispatch paths and are listed in `--maintenance --help`.
`api_tokens` follows the schema in AI.md PART 5, and token list/revoke commands
are operator-authorized. Secret rotation and personal-data export/erasure have
initial implementations in `src/maint_secret.go` and `src/maint_data.go`.

PART 5/11 review update: secret rotation has typed confirmation and sensitive-
operation authorization; installation-secret rotation re-encrypts the PGP key
and preserves the prior secret for seven days; encryption-key rotation
re-encrypts API-token digests and AES-encrypted security reports transactionally
and preserves the prior key. Startup upgrades legacy plaintext token digests
before serving. Data export uses fixed explicit column lists, omits secret
material and encrypted report bodies, and now records correlated compliance
request / completion events. Erasure records compliance events and redacts
retained audit-log user IDs.

Remaining decision: IDEA.md says VidVeil has no user accounts and favorites
belong to an opaque visitor cookie, but AI.md PART 11 describes per-subject
export/deletion requests. The maintenance commands take no subject identifier;
`data delete` therefore affects every visitor's favorites and every resource
owner token. Do not change that scope until the product defines how a request
identifies its subject.

Resolved using AI.md's explicit data-protection and rotation requirement:
`api_tokens.token_hash` contains an authenticated-encryption encoding of the
SHA-256 digest, never the raw token. Legacy plaintext digests are upgraded at
startup, and key rotation re-encrypts them in the same transaction as the
AES-backed security-report bodies. SHA-256 remains the token digest algorithm.
