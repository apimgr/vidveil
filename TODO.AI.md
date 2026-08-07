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
