# TODO.AI.md

## Open Items

### 1. `/search?q=...` intermittently fails with `net::ERR_FAILED` in browser — RESOLVED
Root-caused via live `curl` testing against `https://x.scour.li/search?q=Test`
with a mobile Chrome User-Agent: `AgeVerifyMiddleware` and
`ContentRestrictionMiddleware` (src/server/handler/handlers.go) built the
redirect `Location` header by raw string-concatenating the original
path+query (e.g. `/search?q=Test`) directly as the `redirect` query
parameter's value, producing a malformed URL with an ambiguous nested query
string. Chrome's strict URI parser rejects this outright as
`net::ERR_FAILED`; lenient clients like `curl` (without a query string of
their own) still followed it, which is why earlier reproduction attempts in
this file didn't catch it. Confirmed via `git blame`/`git log -L` that this
defect has existed since the project's initial commit (2025-12-15) — not a
regression from any recent session's changes. Fixed both call sites with
`url.QueryEscape()`; the consuming side (`r.URL.Query().Get("redirect")`)
already auto-decodes, and the existing open-redirect guard
(`strings.HasPrefix(redirect, "/")`) is unaffected. Verified via full
build+vet+test+go-lint pass; committed+pushed as `951a480b3ae7`; CI (CI,
Daily Build, Docker Build) verified green for this commit.

Separately, the user flagged the live healthz response as spec-noncompliant.
Audited `TorInfo` (handlers.go) against AI.md PART 13's canonical struct and
found `Hostname` was tagged `json:"hostname,omitempty"` instead of the spec's
required `json:"hostname"` — causing the `hostname` key to silently vanish
from live JSON instead of serializing as `""`. Fixed, verified, committed+
pushed as `d97f831ee38a`; CI verified green for this commit too.

**Still open, NOT fixable from this repo** — see item 7 below: the live
production server was observed reporting `"mode": "development"` in its
healthz JSON with debug mode also enabled (`/debug/*` endpoints reachable),
even though this repo's own `docker/docker-compose.yml` (production) already
sets `MODE=production` with no `DEBUG` override. The discrepancy must be in
the actual deployment/CD configuration outside this repo's visibility.

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

### 8. `/server/security` coordinated-disclosure feature (AI.md PART 12) not implemented
While making the `/server/*` pages accurate, the `/server/contact` page was given
its spec-mandated (AI.md PART 16, line ~26185) "Security Issues" section, which
links to `/server/security` and must NEVER point at the raw
`/.well-known/security.txt`. That link is currently a dangling reference: only
`/.well-known/security.txt` exists (handlers.go `SecurityTxt`) — the whole
`/server/security*` HTML feature tree is unbuilt. AI.md PART 12 (lines ~14681-14684)
mandates:
- `/server/security` — human-readable security overview page (RFC 9116 channels in
  preference order, Expires, Encryption key, plain-language reporting instructions),
  rendered from live config via `BuildURL(r, ...)`.
- `/server/security/policy` — disclosure policy page (default content, API-editable).
- `/server/security/thanks` — researcher acknowledgments/hall of fame.
- `/server/security/report/{tracking_id}` — one-shot-token researcher status page.
- Also the `/server/contact?security_id={id}` contact mode (line ~14681) that ties a
  contact submission to a security report — part of the same feature.
This is a substantial standalone feature (coordinated-disclosure pipeline + GPG
keypair management), intentionally NOT built as part of the `/server/*` page-accuracy
task. Until it lands, the "Security Issues" link on `/server/contact` will 404. Build
per PART 12, then the link resolves.

### 9. Generic PART 16 privacy config-tree (`server.privacy.*`) intentionally not implemented — verify no gap
AI.md PART 16's `/server/privacy` spec is the generic template model driven by a
`server.privacy.*` config tree (data.sold, CCPA opt-out, retention, third_party
services, GetDataUsageContent/GetConsentMessage helpers). VidVeil has NO such config
tree and IDEA.md non-goals explicitly make it inapplicable: no user accounts, stateless,
nothing persisted per user, nothing sold. The `/server/privacy` page and API were made
accurate to VidVeil's actual reality instead of fabricating that config. No code gap —
this is a deliberate, IDEA.md-documented deviation. Logged only so a future reader
doesn't "fix" the privacy page back toward the generic CCPA/data.sold model.

### 7. Live deployment intentionally running MODE=development/DEBUG=true — not an issue
`https://x.scour.li` was observed reporting `mode: development` with
`/debug/*` reachable. **User confirmed this is intentional** — set deliberately
to debug the app while it was broken (see item 1), not a misconfiguration.
No action needed here; this repo's own `docker/docker-compose.yml`
(production) still correctly defaults to `MODE=production`/no `DEBUG` for
normal deploys, which is unaffected by the user's deliberate live override.
