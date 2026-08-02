# TODO.AI.md

Findings from the full Phase 2 beta-test pass (commit 0d33f978f556), logged for
triage/fix. Remove each item individually once resolved and committed.

## Medium

- [ ] HTML numeric entity `&#039;` (apostrophe) decodes incorrectly to the
  literal string `&#'` in scraped titles — reproducible directly from the
  JSON API. Example: `GET /api/v1/search?q=test&engines=youjizz&limit=20`
  returns titles like `"Mother&#'s Test Full"`. Points to a bug in shared
  title/text cleanup (generic parser or an engine-specific extractor).
- [ ] JSON-LD `VideoObject` metadata enrichment added in commit 0d33f978f556
  (`parseJSONLDVideos`/`mergeLDVideoInfo` in helpers.go) has no observable
  effect on any tested engine in practice. Checked 6 genericSearch()-based
  engines with live traffic (tnaflix, drtuber, sunporno, youjizz, eporner,
  tube8) — description/tags/performer/rating all null/empty, Published zero
  on tnaflix/drtuber/sunporno/youjizz. Grepped raw search-page HTML on 10
  engines (eporner, sunporno, tnaflix, drtuber, youjizz, pornmd, hqporner,
  alphaporno, empflix, txxx, nuvid, hellporno) for
  `application/ld+json`/`VideoObject` — only eporner and sunporno emit any
  JSON-LD on the search page, and both are `BreadcrumbList`, never
  `VideoObject`. The merge logic itself is safe (non-panicking, never
  clobbers existing fields), but schema.org VideoObject JSON-LD appears to
  only exist on individual video detail pages, not search/listing pages, on
  every site tested — so the new fields are effectively dead code against
  real upstream listing pages. Needs a decision: either target detail pages
  (extra fetch per result — cost/latency tradeoff) or drop the JSON-LD path
  in favor of per-engine specific selectors for these fields.

## Low

- [ ] `vidveil-cli` fails with an opaque low-level error when run as root on
  the same host as a server that was also started as root: `creating client
  directories: path /root/.config/apimgr/vidveil owned by uid=899 gid=899,
  want uid=0 gid=0`. Root cause: server (run via `sudo vidveil`) drops
  privileges to the `vidveil` system user (uid 899) and creates
  `~root/.config/apimgr/vidveil/` owned by that uid; CLI run as
  `sudo vidveil-cli` resolves the same default path but expects root
  ownership. Either avoid the path collision, or make the CLI's error
  message explain the conflict and suggest `--config <name>` as a
  workaround.
