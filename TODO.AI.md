# TODO.AI.md

Findings from the full Phase 2 beta-test pass (commit 0d33f978f556), logged for
triage/fix. Remove each item individually once resolved and committed.

- `filter-preview-first` checkbox in `src/server/template/partial/public/filters.tmpl`
  has no `name=` attribute, so it can't submit via a no-JS GET/POST form — unlike
  `results_per_page`/`open_new_tab`, the "preview first" preference has no
  server-side cookie fallback (`resultsPerPageCookieName`/`openNewTabCookieName`
  pattern in handlers.go) for JS-disabled clients. Needs a `preview_first` cookie
  + form field + `PreferencesSave` handler support, and `previewFirst` threaded
  into the no-JS HTML render path in `handlers.go` (currently hardcoded `false`
  at the `text/html` no-JS/text-browser branch and CLI/curl JSON fallback).

Findings from CSS/theming compliance re-audit against AI.md PART 16 (commit
1015f92625d7).

- CLI/TUI theming gap: AI.md PART 16 "Themes" specifies a
  `TerminalPalette`/`TerminalPaletteDark`/`TerminalPaletteLight` ANSI-index
  struct that CLI/TUI must consume as their baseline (literal hex is only
  an opt-in "additional enhancement" for true-color terminals on top of
  the ANSI baseline). This struct does not exist anywhere in the codebase
  — `src/client/tui/styles.go` and `src/client/cmd/tui.go` consume the
  literal hex `theme.ColorPalette`/`theme.Dark`/`theme.Light`/
  `theme.GetColorPalette` directly instead. Needs a `TerminalPalette` type
  in `src/common/theme/` plus ANSI-index dark/light vars matching AI.md,
  and `src/client/tui/styles.go`/`src/client/cmd/tui.go` updated to use it
  as the baseline with hex as an opt-in enhancement.

- Go package directory naming: `src/server/service/metrics`,
  `src/server/service/secrets`, `src/server/service/urlvars`, and
  `src/server/service/utls` are plural directory names; per Go convention
  (AI.md/project-rules.md) Go package directories must be singular to
  match their package names — should be `metric/`, `secret/`, `urlvar/`,
  `utl/` respectively. Found by go-lint during the CSS/theming
  pre-commit gate; out of scope for that commit, needs its own rename
  pass (package rename + all import path updates across the codebase).

Findings from the full audit pass (2026-08-12). The Aug 11 AI.md commit
(e19f7f1e5ec3, "Updated the SPEC for API servers", 335 lines changed)
substantially rewrote the API-server spec — chiefly PART 20 (Metrics),
plus PART 13 (Health), Root-Level Endpoints, HTTP Cache Headers, and
Scheduler status. Code and the 13 rule-file mirrors predate it. Gaps:

- Metrics endpoints incomplete vs updated AI.md PART 20. Only a single
  endpoint is mounted (`src/server/server.go:647`, at
  `Server.Metrics.Endpoint`, default `/metrics`). The updated spec now
  requires the full set with per-service sub-paths and identical handlers:
  `/server/metrics[/{service}]`, root alias `/metrics[/{service}]`,
  `/api/{api_version}/server/metrics[/{service}]`, and unversioned alias
  `/api/metrics[/{service}]` — each `{service}` gated by its own per-service
  bearer token, with Prometheus text / Grafana dashboard JSON / Loki stream
  outputs. No `{service}` path support exists anywhere in `src/server`.
  Needs: per-service metrics config + token model, handler dispatch on the
  `{service}` path segment, and the 4-way alias mounting (same handler, no
  redirects). Cross-cutting; do as one change.

- API version is hardcoded `v1` (AI.md PART 14: "Never hardcode `v1` —
  always use `{api_version}` / `APIBasePath()`"). No `APIVersion` config
  field and no `APIBasePath()`/`APIVersion()` accessor exist on
  `AppConfig` (only `AdminAPIPrefix()` at `src/config/config.go:2220`,
  whose leader is hardcoded). `src/swagger/swagger.go` hardcodes `/api/v1`
  (11 occurrences) and `src/config/config.go` defaults a Deny path to
  `/api/v1/server/admin`. The client side IS versioned
  (`src/client/api/client.go GetAPIBaseURL`, configurable api version).
  Needs: add `APIVersion` config field + `APIBasePath()` accessor, thread
  through swagger and all `src/server` route registration, delete hardcoded
  `v1`. Cross-cutting; do as one change.

- `.claude/rules/*.md` (13 mirror files) are stale relative to AI.md.
  AI.md was last modified 2026-08-11 (commit e19f7f1e5ec3); the rule
  mirrors were last regenerated 2026-05-25. Per ai-rules.md ("create/update
  `.claude/rules/*.md` at session start if missing or if AI.md is newer"),
  regenerate all 13 from the current AI.md so the mirrors reflect the Aug 11
  API-server spec update (esp. api-rules.md PART 13/14 and features-rules.md
  PART 20).

- `.video-card-fav-btn` (favorite star button, used in
  `src/server/template/page/favorites.tmpl` and now also
  `src/server/template/page/search.tmpl`) has no CSS rule anywhere under
  `src/server/static/css/` — renders as an unstyled native button with a
  raw `&#9733;` glyph instead of the designed star icon/active state. Needs
  a `.video-card-fav-btn`/`.video-card-fav-btn--active` rule (hover/focus
  states, WCAG AA contrast, 44x44px touch target per AI.md PART 16/28).

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
- Remaining gap, explicitly out of scope for the above: `handleSearchSSE`
  uses `SearchStreamWithOperators` (`manager.go`), a materially separate
  fan-out implementation NOT gated by `searchSem` and not covered by the
  429 wiring above. Needs the same concurrency-guard + overload-signaling
  treatment (SSE equivalent, e.g. an `event: error` frame with a
  RATE_LIMITED payload) as its own follow-up.
