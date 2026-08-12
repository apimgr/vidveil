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
