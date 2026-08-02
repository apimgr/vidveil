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
