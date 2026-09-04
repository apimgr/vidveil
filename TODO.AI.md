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
- This fix is real and worth keeping, but it did NOT eliminate the
  reproduced 150-concurrent failures: re-testing after the fix still showed
  ~4/150 empty replies at ~30-31s, while server access logs for those exact
  failing requests showed the handler itself completed in 3.9s-20.7s with
  small but COMPLETE bodies (~1.8KB, not truncated) — well inside the new
  cap. That means the residual bottleneck is NOT inside
  `SearchWithOperators`/`batchDeadline`/the search handler at all; it is
  connection-level queueing or congestion happening either before the
  handler starts (accept/dispatch delay under 150 simultaneous new TCP
  connections) or after the handler finishes (writing response bytes over a
  congested/backed-up socket). Needs follow-up: re-test without Docker
  port-forwarding in the loop (Incus, per AI.md PART 28 preference) to rule
  out the test rig itself as a contributor; consider a max-concurrent-
  connections / listener backlog tuning pass, and/or raising
  `Server.Limits.WriteTimeout` together with connection-level rate
  limiting, as the leading fix candidates once the true bottleneck layer is
  confirmed.
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

Findings from the PART 11/31 security compliance pass (2026-08-20), flagged
but not fixed — each is architecture-sized or spec-contradicted and needs a
decision before implementation:

- Tor hidden-service architecture: hidden service is published via bine
  `AddOnion` mapping onion:80 → the clearnet listener
  (`src/server/service/tor/service.go:213-423`, `src/main.go:752`), but
  AI.md line ~41397 explicitly requires torrc `HiddenServiceDir` +
  `HiddenServicePort` (NOT ADD_ONION) with a dedicated PROXY-protocol
  backend (`github.com/pires/go-proxyproto`, not in go.mod) on a
  64000-64999 port and `HiddenServiceExportCircuitID haproxy`. The
  committed `.claude/rules/backend-rules.md` says the opposite ("via
  ADD_ONION") — the condensed rules file is stale and must be regenerated
  from AI.md. Multi-file rewrite (tor service, main.go wiring, new
  dependency, circuit-ID plumbing into logging/rate limiting); needs a
  live-Tor verification run.
- torrc persistence: `ensureTorrc` (`tor/service.go:1207-1229`) only writes
  torrc when absent; spec says regenerate on every startup. Tied to the
  torrc-driven architecture item above.
- Tor rate-limit/blocklist keying: Tor traffic keys per-IP on the loopback
  address (single shared bucket). Correct fix per AI.md ~16013 is
  per-circuit-ID keying, which depends on `HiddenServiceExportCircuitID`
  from the architecture item above.
- Tor key-only mode reporting: `GetInfo()` (`tor/service.go:972`) reports
  `enabled=true` for `TorServiceStatusNoTorBinary` (keys generated, no
  binary). Spec: binary absent → INFO log, disable Tor features, continue.
  `IsEnabled()` is already correct; only the info surface disagrees —
  changing it alters the reported API contract, needs sign-off.
- CSP extension model: `src/config/config.go:611-621` exposes a
  full-replacement `csp` string key (violates "extend via `*_extra`, never
  replace") and `src/server/server.go:202-305` hardcodes the policy — no
  `script_src_extra` family, `connect-src` lacks `{learned_origins}`, no
  dev `Content-Security-Policy-Report-Only` mode. Needs a decision on the
  `*_extra` config key set (operator-facing config surface redesign).
- Output Sanitization Pipeline: only log-side redaction exists
  (`logging.go:392` `SanitizeLogFields`). The PART 11 six-stage RESPONSE
  pipeline (allow-list → query-param redaction → internal IP/path strip →
  truncation → dev_only strip → ~100ms constant-time finalize) is not
  implemented; no auth-failure timing floor found. Large cross-cutting
  feature touching the response/error-envelope layer.

Findings from the PART 16 frontend/PWA compliance pass (2026-08-20),
flagged but not fixed:

- `src/server/csrf.go:117-120`: CSRF validation is bypassed when no
  session cookie is present (a "session_id" cookie that is never set
  anywhere), effectively disabling browser CSRF enforcement. AI.md PART
  16's bypass list is closed (Bearer header, safe methods, WS upgrade,
  exempt_paths — no session-presence bypass), but AI.md's own threat-model
  row for this open, unauthenticated API says "n/a (no auth, nothing to
  abuse)". Business-logic decision: either remove the session-cookie
  bypass and enforce double-submit on all mutating browser requests, or
  document the deliberate exemption. Consent forms already include the
  CSRF hidden input either way.
- Cookie-consent banner text uses i18n keys rather than the
  `CookieConsentConfig.Message/PolicyText/PolicyURL` config fields —
  consistent with the hardcoded-strings prohibition (PART 29/30), but
  decide whether those config fields should feed the template (and be
  removed if not).

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

Newly discovered while doing the above (2026-08-22) — pre-existing, NOT
introduced by this session's edits, and far larger in scope than the
finding it was found alongside: comparing every non-English locale file
in `src/common/i18n/locales/` against `en.json` line-for-line shows a
large fraction of values are byte-identical to the English text, i.e.
silently untranslated content that still passes the build-time
key-parity check (PART 30) because the *keys* match — only the
*values* don't. Counts (identical-to-English / 651 total keys):
es=125, zh=116, fr=137, ar=119, de=137, ja=118 — roughly 18-21% of
every non-English locale file, not limited to any one section (the
`favorites.*` section was 100% untranslated before this fix). This
needs a dedicated translation pass per language (a native/reviewed
translation, not a mechanical script) and is too large and
judgment-laden to fold into an unrelated fix — needs its own scoped
session: `grep -Fxf <(comm -12 <(jq -r 'to_entries[]|"\(.key)=\(.value)"' en.json|sort) <(jq -r 'to_entries[]|"\(.key)=\(.value)"' {lang}.json|sort))` (or the diff approach already used to find the counts above) to enumerate the untranslated keys per language, then translate each.

Found during the "make code match AI.md" compliance sweep (2026-08-29),
NOT fixed in that sweep — deliberately deferred as too large/risky to
apply as a mechanical inline edit: `.github/workflows/daily.yml`,
`.gitea/workflows/daily.yml`, `.forgejo/workflows/daily.yml`, and the
matching `beta.yml` in all three providers each compute
`VERSION`/`COMMIT_ID`/`BUILD_EPOCH` inside the "Set build info" step of
the `build` job, which runs under an 8-way platform `strategy.matrix` —
meaning all three values are recomputed redundantly on every matrix leg
instead of once. `cicd-rules.md` (PART 27) requires a dedicated `version`
job that computes these once and exposes them via job outputs, consumed
by the matrix job via `needs:`. Fixing this requires restructuring the
job graph in six workflow files (add a `version` job, wire
`needs: version` + `${{ needs.version.outputs.* }}` into the existing
`build` job, matching the pattern used for `release`/`daily` version
jobs elsewhere) and MUST be validated with `act --list -W {file}` per
file before committing, per the CI/CD workflow-gate rule — not something
to do as a blind sed/Edit pass. Confirmed present via
`grep -n "strategy:\|Set build info" .github/workflows/daily.yml`
(Set build info at line 52 sits inside the matrixed `build` job, line 23).

AI.md diff implementation follow-ups (2026-09-03), flagged during the
Tor Circuit-ID/PROXY-protocol implementation pass, not yet fixed:

- `src/main.go` (two occurrences, ~line 400 and ~line 1295): system
  service-user name and service-manager identity are derived from
  `filepath.Base(os.Args[0])` (the actual, possibly-renamed binary
  filename), with a normalize-to-"vidveil" fallback that only triggers
  when the name contains a `-` and doesn't start with `vidveil-`. A
  binary renamed to something with no dash (e.g. `myapp`) would create
  a system user/group named `myapp` instead of the frozen
  `{internal_name}` (`vidveil`), violating binary-rules.md ("Never
  hardcode... to the actual (possibly renamed) binary filename — always
  the compiled-in `{project_name}`") and service-rules.md ("Always use
  `{internal_name}` (frozen) for service identity... never
  `{project_name}`"). AI.md's diff also renamed `{project_name}` →
  `{internal_name}` in the Tor privilege-drop narrative (PART 31,
  "Tor Process Ownership" step 2), reinforcing this. Left unfixed this
  pass because the existing code's stated intent (a renamed
  `vidveil-cli`/`vidveil-agent` binary installing a matching *service*)
  may be intentional multi-binary behavior worth preserving for
  dash-prefixed names — needs a decision on whether ALL binary renames
  should fall back to the frozen `vidveil` identity, or only non-`vidveil*`
  ones, before editing.
- Verify Docker Compose files (`docker/docker-compose*.yml`) against the
  `${VAR:-default}` inline-fallback env-var syntax vs. hardcoded values
  mentioned in the AI.md diff — not yet checked this pass.
- Verify items #6-#12 from the original diff review (Go stdlib import
  list additions, CSS variable/button/toggle/empty-state/skip-link/
  reduced-motion additions, `showToast()` JS reference implementation,
  Site Banner dismissal form `csrf_token`+`return_to` hidden fields,
  Theme Toggle JS, HTTP 304/410/502 status handling) against actual
  `static/css/`, `static/js/app.js`, and template files — not yet
  checked this pass.

## Browser E2E suite (AI.md PART 28)

- Add the E2E test dependencies to `go.mod`:
  `github.com/chromedp/chromedp` and `github.com/chromedp/cdproto`.
  `tests/e2e/*_test.go` (build tag `e2e`) import them and will not compile
  until `go mod tidy` is run in Docker (`casjaysdev/go:latest`). Subagents
  are not permitted to edit `go.mod`, so this must be done by the main
  session. The suite is on-demand only (`./tests/e2e.sh`) and is never part
  of `make test`, so this does not block the commit gate.
- Once the dependencies resolve, run `./tests/e2e.sh` once and reconcile any
  selector assumptions in `tests/e2e/nojs_test.go` /
  `tests/e2e/browser_test.go` (e.g. `select[name="results_per_page"]`,
  `form input[name="q"]`) against the real templates.

## Decisions needed (surfaced during the PART 17-22 features audit)

- PART 21 defines a `400 VALIDATION_FAILED` response for a restore request
  that supplies a password by a non-interactive channel, but vidveil exposes
  restore only through the CLI (`--maintenance restore`, interactive prompt
  via `term.ReadPassword`). There is no WebUI or API restore surface to
  attach that error path to. Decide whether an API/WebUI restore surface is
  in scope for this project, or whether the CLI-only path satisfies PART 21.
- The scheduler emits a third `status` label value `"skipped"`
  (`src/server/service/scheduler/scheduler.go:780`) in addition to the
  `success`/`error` pair PART 20 enumerates. Confirm whether `"skipped"` is
  an accepted extension or the metric should fold skipped runs into one of
  the two spec'd values.

## Decisions needed (surfaced during the CI/CD audit)

- SBOM on Gitea/Forgejo: `.claude/rules/cicd-rules.md` prose says Gitea and
  Forgejo carry no SBOM step, but AI.md PART 27's own Gitea spec blocks
  (`gt-release.yml:145`, `gt-beta.yml:146`, `gt-daily.yml:148`) include the
  `cyclonedx-gomod` step. The repo is split: `.forgejo/{release,beta,daily}.yml`
  have SBOM steps, `.gitea/*` do not. Neither side was changed. The
  "platform capability gap" argument applies only to attestation (a
  GitHub-only API); `cyclonedx-gomod` runs anywhere. Rule one way and square
  the `.gitea` vs `.forgejo` divergence.
- AI.md PART 27 has no Gitea/Forgejo `ci.yml` spec block (only release, beta,
  daily, docker), yet `cicd-rules.md` requires `ci.yml` everywhere. Both files
  exist and are sound but have no authority to be judged against.
- `Jenkinsfile`: the `Docker` and `Docker: Devel` stages have no `when {}`
  guard, so both cron triggers (3am, 4am) run both stages and the standard
  image rebuilds in the devel slot. Jenkins cannot cleanly tell which cron
  line fired, so fixing it means choosing a mechanism (a `TimerTriggerCause`
  hour check, or splitting into two jobs) — a design call.
- `.gitlab-ci.yml` uses the bashism `${CI_COMMIT_SHA:0:7}` in `script:`
  blocks. Safe under the bash-defaulting images in use, but it breaks under a
  strict `sh`. AI.md carries the same construct.

## Decisions needed (surfaced during the service/Docker/Tor audit)

- Spec contradiction: `.claude/rules/docker-rules.md` says to create a
  non-root user in the runtime stage and set `USER app`, but AI.md PART 26
  (line 33313, authoritative) says the opposite — no `USER` directive and no
  user creation in the Dockerfile. The Dockerfiles follow AI.md, so the rules
  file is the side that needs correcting.
- Port drift: the project compose files use 64893/64581 while AI.md's example
  uses 64580. Left as-is on the assumption the spec value is a template
  placeholder.
- Every `--service` command output string in `src/main.go` is hardcoded
  English rather than an i18n key, which the no-hardcoded-strings rule
  forbids. Needs a pass converting them to `i18n.Translate` keys added to all
  7 locale files.

## Frontend (PART 16) items deferred during the CSS/JS/a11y verification pass

- AI.md PART 16 "Image Scaling" / "Remote URL Fetching" (lines 25821-25840)
  is unimplemented. `server.branding.logo`/`.favicon` are now wired through
  to the templates, but there is no multi-size generation (logo 200/50px,
  favicon 16/32/48/180/192/512, OG 1200x630), no local cache of scaled
  variants, no daily re-fetch of remote URLs, and no
  `src/common/urlutil/fetch.go` with the specified `FetchRemoteImageConfig`
  (10MB max, 30s timeout, image/png|jpeg|gif|webp|x-icon, https-only). This
  needs a new package plus an image-resize dependency, so it requires a
  `go.mod` change.
- `src/server/static/js/app.js` passes hardcoded English strings to
  `showToast(...)` in several places (around lines 1802, 1934, 1941, 1949,
  2073, 2089, 2701, 2720 and in `handleDownloadClick`), violating the
  no-hardcoded-strings rule. They should be moved into the `#app-i18n` data
  island in `partial/public/head.tmpl`.
- `src/server/static/js/app.js` (around line 2545) still builds the status
  bar from hardcoded English: `allResults.length + ' results (streaming...)'`,
  `' results found'`, `' (min ' + N + ' min)'`, and
  `enginesWithResults.size + ' engines responding'`. The `search.streaming`
  key already exists; the rest need new keys in all 7 locales plus entries in
  the `#search-i18n` data island in `src/server/template/page/search.tmpl`.

## Decisions needed (i18n interpolation scheme)

- AI.md 40804-40816 mandates LITERAL `{token}` interpolation and explicitly
  forbids using a translation as a `fmt` format string ("no %s/%d verbs, no
  positional {0}"), but `src/common/i18n/i18n.go:329` implements
  `TranslateFormat` as `fmt.Sprintf(t.Translate(locale, key), args...)`. The
  whole codebase follows the code, not the spec. Converting to the spec'd
  form touches 37 `%`-verb keys x 7 locale files (~259 value edits), 34
  template `tf` call sites, and 12 Go `TranslateFormat` call sites, and
  AI.md's own example (`{{tf .Lang "health.ssl_expires_in" "days" .Days}}`)
  shows the target call shape is variadic key/value pairs. A mis-converted
  site fails silently in 7 languages, so this needs a build+test cycle in
  Docker — main session decision, not an unverifiable bulk edit.

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

## Decisions needed (surfaced during the PART 0-4 compliance audit)

- `.claude/rules/admin-rules.md` exists but is not one of the 13 rule files
  AI.md PART 0 enumerates, and its header claims "PART 17" while AI.md PART
  17 is EMAIL & NOTIFICATIONS (admin-panel content has no matching PART).
  Deleting a file is never done without confirmation, and its content is
  clearly in use, so the mapping question is left for the main session:
  either fold the admin content into an existing rule file or accept a 14th
  file that AI.md's table does not list.
- `.claude/rules/binary-rules.md` carries an "Agent Binary (PART 33,
  OPTIONAL)" section, but AI.md PART 33 is the IDEA.md reference and no
  agent-binary PART exists in AI.md. The file header/footer PART numbers
  were corrected to 7, 8, 32, but whether the agent section itself should
  stay, move, or be dropped needs a human call.
- 10 of the 14 `.claude/rules/*.md` files predate AI.md's current mtime
  (2026-09-03). PART 0's trigger says "AI.md modified more recently than
  rule files -> update all files", so a full regeneration pass of the rule
  set against the current AI.md is owed. Regenerating 14 summary files is
  not a mechanical edit and is deliberately not attempted here.
