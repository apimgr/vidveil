# TODO (AI)

## Progressive-enhancement / no-JS search results

AI.md PART 14 (~L17864 "If it works without JavaScript, ship it") and ~L20562
("JavaScript enhances, it does not enable") require the search page to show
real results without JS. Currently `search.tmpl` renders the server-computed
`.Results` only inside `<noscript>`, so JS browsers start with an empty
`#video-grid` behind a "Connecting to engines..." spinner and app.js re-runs
the whole search via SSE — search runs twice and a hung engine leaves visitors
stuck forever on the spinner despite server-side results existing.

- [ ] Render `.Results` into the visible `#video-grid` unconditionally; keep
      `<noscript>` only for genuine no-JS-specific variants (e.g. no
      infinite-scroll controls). Server default order = existing relevance sort;
      no cookie-based sorting (prefs are localStorage-only per IDEA.md).
- [ ] Remove the "Connecting to engines..." spinner as the default first-paint
      state; it may only show when JS chooses to fetch more.
- [ ] app.js: on load with JS, read localStorage prefs via
      `applySearchFiltersAndSort()` and re-sort/re-filter the already
      server-rendered `.video-card` DOM in place — do not rebuild from an empty
      SSE state.
- [ ] Repurpose SSE/EventSource as true enhancement only (infinite scroll /
      additional results), never as the source of the first visible result set.
- [ ] Leave video hover/swipe preview as JS-only (confirmed correct).
- [ ] Verify: JS-disabled (curl / text-browser) shows real results immediately
      with no SSE dependency; JS-enabled shows server results immediately then
      client re-sort/filter, SSE only appends.
