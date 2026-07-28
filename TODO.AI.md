# TODO.AI.md

## Open Items

### 1. `/search?q=...` intermittently fails with `net::ERR_FAILED` in browser
Reported by user against `https://x.scour.li/search?q=lesbian%20teen`. Investigated:
- No panic, `os.Exit`, or hijack-misuse found in the `/search` handler pipeline.
- Blocklist logic is IP/domain-only; does not match query text, so it is not
  rejecting this specific query.
- Each engine's `http.Client` has a bounded timeout (`EngineTimeout` config,
  default 15s, per-engine override via `EngineTimeouts`) — not an unbounded hang
  in application code.
- Could not reproduce locally: sandboxed build environment has no outbound
  internet, so a live end-to-end request against the real search engines
  couldn't be exercised.
- Leading hypothesis: a reverse-proxy or edge-layer timeout in front of
  `x.scour.li` that is shorter than the engine fetch time, or a TLS/SNI
  mismatch at the proxy — both are infrastructure/deployment concerns, not
  something fixable by editing this repo's Go code.
- **Next step**: needs production server logs (request ID + timing for the
  failing request) or reverse-proxy access logs to confirm/deny the timeout
  hypothesis. Ask the user for proxy config or logs, or reproduce against a
  deployed instance with real network access.

### 2. No-JS `resultsPerPage` / `openNewTab` preferences are stored but not wired to behavior
`PreferencesSave` (handlers.go) now persists `resultsPerPage` and `openNewTab`
as cookies for the no-JS fallback path (`nojs/preferences.tmpl`), fixing the
preferences form's previous 404. However, no page-size concept exists anywhere
in the search results rendering (JS or no-JS), and no-JS result links don't
read the `open_new_tab` cookie to set `target="_blank"`. This is a pre-existing
feature gap larger than "fix the broken preferences template" — needs its own
implementation pass:
- Wire `getRequestResultsPerPage()` into the no-JS search handler's page-size logic.
- Wire `getRequestOpenNewTab()` into no-JS result-link `target` attribute rendering.
