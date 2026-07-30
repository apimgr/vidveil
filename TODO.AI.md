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

### 4. Maintenance mode's spec-shaped JSON body / content negotiation still incomplete (AI.md PART 5/6, lines ~7336-7418)
`MaintenanceModeMiddleware` (src/server/handler/handlers.go) was found, via
live Docker testing (touching `{data_dir}/maintenance.flag`), to block
`/server/healthz`, `/api/v1/server/healthz`, and `/api/v1/version` with the
same hardcoded HTML 503 page as write operations, even with
`Accept: application/json`. The exemption list has been fixed (see this
session's commit) so health/version/static routes now bypass the block
entirely. Still outstanding, and out of scope for a one-line patch:
- Per AI.md lines 7336-7359, blocked write-operation requests should receive
  the canonical `{"ok":false,"error":"MAINTENANCE","message":...,
  "details":{...}}` JSON body (with `Retry-After`/`X-Maintenance-Mode`/
  `X-Maintenance-Reason` headers) for API/text/JSON requests, keeping the
  HTML page only for browser/frontend requests, per PART 14 content
  negotiation rules — currently it's the hardcoded HTML string regardless
  of `Accept`.
- If self-healing state (attempts/last_attempt/next_attempt) is ever
  surfaced through a JSON healthz-during-maintenance body, that tracking
  needs to be built — it doesn't exist yet.

### 5. `/api/v1/*` 404s return HTML instead of negotiated content type
Beta-testing found that unmatched routes under `/api/v1/*` (e.g. a typoed or
removed endpoint) return the generic HTML 404 error page even when the
request sends `Accept: application/json` or `Accept: text/plain`. Per PART
13/14, every route under `/api/v1/` must honor content negotiation, including
the not-found case (`{"error":"message","code":404}` for JSON, plain text for
`text/plain`). This needs the router's `NotFoundHandler`/catch-all for the
`/api/v1` mount to branch on `Accept`/`.txt` the same way real handlers do,
rather than falling through to the generic frontend 404 template.

### 6. Unconditional emoji in `src/server/handler/handlers.go` server-rendered output
go-lint flagged 4 pre-existing (not introduced by any current session's
changes) emoji characters used unconditionally in server-rendered HTML:
"🔧" on the maintenance page (~line 521), and "✅"/"🔴"/"⚠️" as `StatusIcon`
template field values for the healthz template (~lines 1644, 1648, 1652).
Per CLAUDE.md's "no emojis in code or inline tool output unless asked" rule,
these should be replaced with text labels (e.g. "OK"/"DOWN"/"WARN") or made
conditional. Left unfixed here since it's unrelated to the two findings this
session's commits address; logged so it isn't lost.
