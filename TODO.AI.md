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

### 3. `vidveil-cli engines`/`bangs` still show blank Bang/Method/Preview/Download columns after the data-envelope fix
Beta-testing `vidveil-cli` against a live server found `engines`/`bangs`/`search`
all silently returned empty/zero-valued output because the client's response
structs (`src/client/api/client.go` `SearchResponse`, `src/client/cmd/engines.go`
`EnginesListResponse`) expected a flat top-level JSON shape while the server
actually wraps everything in `{"ok":bool,"data":...}` (confirmed via direct
curl against `/api/v1/search`, `/api/v1/engines`). Fixed the envelope mismatch
(custom `UnmarshalJSON` on `SearchResponse`; retagged `EnginesListResponse.
Engines` to `json:"data"`) and the `SearchResult.Engine` field tag (was
`json:"engine"`, server sends `"source"`) and `VersionResponse.Built` (was
`json:"built"`, server sends `"build_date"`).

**Not fixed — needs a product decision:** `EngineInfo` (`src/client/cmd/
engines.go:32-40`) still declares `Bang`, `Method`, `HasPreview`, `HasDownload`
fields that `GET /api/v1/engines` does not send at all (actual server engine
object has `name`, `display_name`, `enabled`, `available`, `features[]`,
`tier`, `privacy{requires_js,sets_cookies,has_tracking}` — verified live).
`RunBangsCommand` (`engines.go:248-310`) derives bangs from this same engines
response (reading `engine.Bang`), even though the server has a dedicated
`GET /api/v1/bangs` endpoint (`{"count":N,"data":[{"bang","engine_name",
"display_name","short_code"}]}`) that already carries real bang data — the
CLI never calls it. Net effect after today's fix: `engines` table's BANG,
METHOD, PREVIEW, DOWNLOAD columns still render empty/blank for every engine,
and `bangs` still returns zero rows (`engine.Bang` is always `""`, so nothing
passes into `BangInfo`). Needs a decision: (a) point `RunBangsCommand` at the
real `/api/v1/bangs` endpoint (straightforward, has all needed fields) and (b)
either drop the METHOD/PREVIEW/DOWNLOAD columns from the `engines` table (data
doesn't exist server-side) or add those fields to the server's `/api/v1/engines`
response (server-side scope change) — whichever the operator/spec intends.
