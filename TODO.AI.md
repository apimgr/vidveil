# TODO.AI.md

Findings from the full Phase 2 beta-test pass (commit 0d33f978f556), logged for
triage/fix. Remove each item individually once resolved and committed.

Findings from CSS/theming compliance re-audit against AI.md PART 16 (commit
1015f92625d7).

- GOOS string uses "macos" instead of the correct Go GOOS term "darwin" in
  three places in `src/config/config.go` (line ~1001 OS config comment, line
  ~1021 and ~1076 `UserAgentConfig`) and one test data string in
  `src/config/config_coverage_test.go` (line ~358). Found by go-lint during
  the Go package directory rename pass (task 5); unrelated to that change,
  out of scope for that commit, needs its own fix pass.

Findings from the full audit pass (2026-08-12). The Aug 11 AI.md commit
(e19f7f1e5ec3, "Updated the SPEC for API servers", 335 lines changed)
substantially rewrote the API-server spec — chiefly PART 20 (Metrics),
plus PART 13 (Health), Root-Level Endpoints, HTTP Cache Headers, and
Scheduler status. Code and the 13 rule-file mirrors predate it. Gaps:

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

Findings from the engine data-standardization audit (2026-08-19), minor
items not fixed in that pass — each needs its own small follow-up:

- `src/server/service/engine/pornmd.go` (~line 68-92): quality string is
  folded into `Description` instead of the `Quality` field (engine declares
  `HasQuality:false`, so tolerated, but diverges from redtube/pornhub which
  set `Quality` directly). Decide: populate `Quality` + flip `HasQuality`,
  or keep as-is deliberately. Note `engine_pure_coverage_test.go`
  `TestPornMDConvertToResult_DescriptionWithQuality` asserts the current
  behavior and must change with it.
- `src/server/service/parser/xvideos.go` (~line 79-86): `PreviewURL` is set
  from `data-pvv` (a JPG thumbnail sprite), which `sanitizePreviewURL`
  (`manager.go:1401`, drops image extensions) always discards — so xvideos
  previews never survive despite `HasPreview:true` in `xvideos.go:28`.
  Either find a real MP4 rollover source or set `HasPreview:false` and stop
  assigning the dead value.
- `src/server/service/engine/xnxx.go:28`: `HasPreview:false`, but xnxx
  shares the xvideos markup family — investigate whether a usable rollover
  preview attr (`data-preview`/`data-mediabook`) exists and harvest it.
- `src/server/service/parser/redtube.go:64` and
  `src/server/service/parser/youporn.go:89`: redundant manual
  `strings.ReplaceAll(preview, "&amp;", "&")` — goquery already decodes
  attribute entities; remove for consistency with other parsers.

Finding from the AI.md spec update sync (2026-08-20, commit bb89070dd479
"Updated the SPEC for API servers" — SW guaranteed-Response, JS necessity
gate, error-path guarantee):

- New PART 16 "Error Pages" bullet requires every request to terminate in a
  rendered response even when the error path itself fails: the panic/recover
  middleware and template-render failures must fall back to a minimal
  HARDCODED error response with content negotiation (HTML for browsers,
  JSON for API clients). Current state: chi `middleware.Recoverer` is wired
  (`src/server/server.go:189`) so panics do produce a 500, but it is chi's
  stock plain-text 500 (no content negotiation, no themed-fallback
  distinction), and there is no audited template-render-failure fallback
  path. Needs its own pass: replace/wrap Recoverer with a custom recover
  middleware emitting the canonical JSON envelope for API/JSON clients and
  a minimal hardcoded HTML page for browsers, and audit `renderResponse`/
  template execution error paths for the same guaranteed-response fallback.

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

Finding from go-lint pass during the API-version cross-cutting fix
(2026-08-15, task 8 completion).

- `BuildDate` is embedded directly via ldflags instead of `BuildEpoch`
  (AI.md PART 25: "Always embed `BuildEpoch` (Unix timestamp), `Version`,
  `CommitID`, `OfficialSite` via `-ldflags -X` at build time — `BuildDate`
  is derived at runtime from `BuildEpoch`, never embedded directly as its
  own ldflag"). Violations: `Makefile` line 31 (`LDFLAGS`) and line 39
  (`CLI_LDFLAGS`) both set `main.BuildDate` via `-X`; `docker/Dockerfile`
  line 25 does the same. `src/main.go` line 57 and
  `src/client/cmd/root.go` line 28 declare a `BuildDate` var populated by
  ldflags instead of a `BuildEpoch` var derived at runtime. Found
  unrelated to and out of scope for the API-version fix; needs its own fix
  pass: replace `BUILD_DATE`/`main.BuildDate` with
  `BUILD_EPOCH`/`main.BuildEpoch` (Unix timestamp) across Makefile and
  Dockerfile, then compute `BuildDate` at runtime from `BuildEpoch` in
  `main.go`/`root.go`.
