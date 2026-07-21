# VidVeil

## Project description

Privacy-respecting meta search engine for adult video content that aggregates results from 42 video sites without tracking, logging, or analytics. Results stream in real-time via SSE as each engine responds.

**Target Users:**
- Privacy-conscious users seeking adult content without tracking
- Self-hosters wanting their own private search instance
- Developers needing a unified API across multiple adult video platforms
- Tor users requiring anonymous access to adult content

## Project variables

project_name:  vidveil
project_org:   apimgr
internal_name: vidveil
app_name:      vidveil
official_site: https://x.scour.li

`{plist_name}` is NOT stored - it is derived at substitution time as `io.github.apimgr.vidveil`.

`internal_name` is FROZEN as of first-time setup on 2026-05-06. It must equal `project_name` at first run and may not be edited after the project ships. A project rename changes `project_name` only; `internal_name` stays.

## Business logic

### Product scope & non-goals

**In scope (the WHAT):**
- Privacy-First Search: No tracking, logging, or analytics - complete privacy
- Multi-Engine Aggregation: 42 engines with bang shortcuts for targeted searches
- Real-Time Streaming: SSE streaming delivers results as each engine responds
- Thumbnail Proxy: All thumbnails proxied through server to prevent tracking
- Video Preview: Hover (desktop) and swipe (mobile) preview support
- Client-Side Preferences: All settings stored in localStorage, no server storage
- Favorites & History: Local-only bookmarks and search history with export/import
- Tor Support: Built-in Tor hidden service support for maximum anonymity
- Geographic Content Restriction: Admin-configurable warnings/blocks for regions with adult content laws
- Static Binary: Single static binary with all assets embedded

**Non-goals:**
- VidVeil has NO user accounts - privacy-first, stateless design.
- No caching of search results - all searches are real-time. Thumbnail proxy may cache images temporarily to reduce repeated fetches.
- No admin web UI. All server configuration is file-only via `server.yml` (per AI.md PART 5).
- Engine availability is best-effort - some sites may block or rate limit, and the app does not guarantee any specific engine remains reachable.

### Roles & permissions

| Role | Storage | Authentication | Permissions |
|------|---------|----------------|-------------|
| **Anonymous visitor** | None (stateless) | None | Search, browse results, set local preferences/favorites/history (localStorage only) |
| **Operator** | None (file-only) | OS-level access to server | Edit `server.yml` and restart the server — no web login, no DB accounts |

There are no Regular User accounts (PART 34 NOT implemented). There are no organizations (PART 35 NOT implemented). There are no custom domains (PART 36 NOT implemented). There is no admin web UI — all configuration is file-only (AI.md PART 5).

### Data model & sensitivity

**Stateless application data (in transit only - never persisted):**

```go
// VideoResult represents a single video search result
type VideoResult struct {
    ID              string    `json:"id"`                      // Video identifier (engine-specific)
    Title           string    `json:"title"`                   // Video title
    URL             string    `json:"url"`                     // URL to video page on source site
    Thumbnail       string    `json:"thumbnail"`               // Proxied thumbnail URL (static image)
    PreviewURL      string    `json:"preview_url,omitempty"`   // Preview video URL for hover/swipe
    DownloadURL     string    `json:"download_url,omitempty"`  // Direct download URL (if available)
    Duration        string    `json:"duration"`                // Formatted duration (e.g., "12:34")
    DurationSeconds int       `json:"duration_seconds"`        // Duration in seconds for filtering
    Views           string    `json:"views"`                   // Formatted view count (e.g., "1.2M")
    ViewsCount      int64     `json:"views_count"`             // Raw view count for sorting
    Rating          float64   `json:"rating,omitempty"`        // Rating percentage (0-100)
    Quality         string    `json:"quality,omitempty"`       // Video quality (HD, 4K, etc.)
    Source          string    `json:"source"`                  // Source engine name
    SourceDisplay   string    `json:"source_display"`          // Source display name
    Published       time.Time `json:"published,omitempty"`     // Publish date
    Description     string    `json:"description,omitempty"`   // Video description
    Tags            []string  `json:"tags,omitempty"`          // Video tags/categories
    Performer       string    `json:"performer,omitempty"`     // Performer/model name (if available)
}

// EngineInfo represents information about a search engine
type EngineInfo struct {
    Name         string              `json:"name"`           // Internal engine name
    DisplayName  string              `json:"display_name"`   // Display name
    Enabled      bool                `json:"enabled"`        // Whether engine is enabled
    Available    bool                `json:"available"`      // Whether engine is available
    Features     []string            `json:"features"`       // Supported features
    Tier         int                 `json:"tier"`           // Engine tier (1=API, 2=JSON, 3+=HTML)
    Capabilities *EngineCapabilities `json:"capabilities,omitempty"`
}

// EngineCapabilities represents engine feature support
type EngineCapabilities struct {
    HasPreview  bool `json:"has_preview"`   // Supports preview URLs
    HasDownload bool `json:"has_download"`  // Supports download URLs
}

// SearchResponse represents the API response for a search
type SearchResponse struct {
    Ok         bool           `json:"ok"`
    Data       SearchData     `json:"data"`
    Pagination PaginationData `json:"pagination"`
    Error      string         `json:"error,omitempty"`
    Message    string         `json:"message,omitempty"`
}

// SearchData holds the search results and metadata
type SearchData struct {
    Query           string                    `json:"query"`
    SearchQuery     string                    `json:"search_query,omitempty"`
    Results         []VideoResult             `json:"results"`
    EnginesUsed     []string                  `json:"engines_used"`
    EnginesFailed   []string                  `json:"engines_failed"`
    SearchTimeMS    int64                     `json:"search_time_ms"`
    HasBang         bool                      `json:"has_bang,omitempty"`
    BangEngines     []string                  `json:"bang_engines,omitempty"`
    Cached          bool                      `json:"cached,omitempty"`
    EngineStats     map[string]EngineStatInfo `json:"engine_stats,omitempty"`
    RelatedSearches []string                  `json:"related_searches,omitempty"`
}

// AutocompleteResponse - actual API response format (map-based, varies by type)
// Response fields:
//   ok:          bool   - always true on success
//   type:        string - "bang", "bang_start", "performer", "search", "popular"
//   suggestions: array  - structure varies by type (see below)
//   replace:     string - (optional) word to replace in query

// Bang suggestions (type: "bang" or "bang_start"):
type BangSuggestion struct {
    Bang       string `json:"bang"`        // "!ph" (with ! prefix)
    EngineName string `json:"engine_name"` // "pornhub"
}

// Performer suggestions (type: "performer"):
// Returns []map[string]string with keys: "term" (e.g., "@mia khalifa"), "type" ("performer")

// Search suggestions (type: "search"):
type CombinedSuggestion struct {
    Term string `json:"term"` // "amateur"
    Type string `json:"type"` // "static", "performer", "popular"
}
```

**Persisted data (server side):**

| Data | Location | Sensitivity | Retention |
|------|----------|-------------|-----------|
| Engine registry | Embedded in binary (`src/server/engine/engines.go`) | Public | Build time |
| Server config | `server.yml` (file-only — no runtime mutation via API) | Mixed (branding public; rate limit values internal) | Until operator edits |
| Runtime config KV | `srv_config` / `config` table (per PART 5) | Mixed | Until operator edits |
| Audit log | `srv_audit_log` / `audit_log` table (per PART 11) | Medium - config changes, security events | Per retention policy |
| Backups | `{backup_dir}` | High - contains DB encryption material when compliance encryption is on | `server.scheduler.tasks.backup.retention` (default 4) |
| GeoIP / blocklist DBs | `{data_dir}/security/` | Public datasets | Auto-refreshed by scheduler |
| SSL certs | `{config_dir}/ssl/` | High - private keys | Until renewal |
| Tor onion key | `{data_dir}/tor/` | High | Lifetime of hidden service |

**Client-side data (localStorage only - never sent to server):**
- `vidveil-theme`: current theme preference
- `vidveil_prefs`: complete preferences object
- `vidveil_history`: search history array
- `vidveil_favorites`: favorites array
- Sensitivity: low (local to browser); never written to server, never logged.

### Trust boundaries & external services

| Boundary | Service / Source | Trust assumption | Failure mode |
|----------|------------------|------------------|--------------|
| Outbound search | 42 third-party adult video sites (HTML/JSON parsing) | UNTRUSTED - response bodies may be malicious or change shape; never executed, only parsed | Engine drops out of `engines_used`, listed in `engines_failed`; search continues with remaining engines |
| Outbound thumbnails | Same 42 sites + CDN hosts (e.g., `ttcache.com`) | UNTRUSTED - byte stream is rewritten through proxy, never linked directly into HTML | Thumbnail falls back to `placeholder.svg` |
| Outbound preview videos | Same engines (where supported) | UNTRUSTED - URL only, never inlined as HTML | Preview not shown |
| Outbound Tor | Local Tor binary if present | TRUSTED inside container/host; auto-detected per AI.md PART 31 | Tor disabled if binary absent; clearnet still works |
| Outbound GeoIP DB feed | Public GeoIP DB endpoint (per scheduler `geoip_update`) | TRUSTED dataset (publicly published) | Stale GeoIP data continues to be used until next successful refresh |
| Outbound blocklist feed | Configured blocklist source(s) (per scheduler `blocklist_update`) | TRUSTED dataset (publicly published) | Stale blocklist data continues to be used until next successful refresh |
| Inbound HTTP | Browsers, HTTP tools, text browsers, Tor clients | UNTRUSTED - all input validated; all responses content-negotiated | Standard 4xx error response per AI.md PART 14 canonical error body |

External integrations and failure modes are NOT to be extended at code time without updating this section.

### Threat model & abuse cases

**Primary assets:**
1. User privacy - the absence of any record that a particular IP queried a particular term (this is the product itself).
2. Server availability - the app must keep working even when individual engines fail.
3. `server.yml` integrity - protect config file from unauthorized edits and rogue restore/setup.
4. The host - VidVeil must never let untrusted engine HTML pivot into XSS, SSRF, or RCE.

**Inputs trusted vs untrusted:**

| Input | Trust |
|-------|-------|
| Anonymous user search query, autocomplete prefix, filter selections | UNTRUSTED |
| Engine response bodies (HTML/JSON), thumbnail bytes, preview URLs | UNTRUSTED |
| CLI flags and maintenance commands from the OS user running the server | TRUSTED-on-invoke but still validated for path safety and known values |
| `server.yml`, environment variables, CLI flags written by the operator | TRUSTED-on-write but normalized via `SafePath()` and `config.ParseBool()` |

**Attacker goals + required defenses:**

| Attacker goal | Defense |
|---------------|---------|
| Correlate IPs to search queries | No request logging of query content; structured logs redact query bodies; rate-limit responses do not echo the query |
| Inject XSS via crafted engine titles / tags / performer names | All result fields are HTML-escaped server-side before render; CSP + escape-on-render templates; no inline JS or CSS |
| SSRF via thumbnail/preview URL | Thumbnail proxy validates scheme + host against an allowlist of known engine CDN hosts; rejects file://, gopher://, and RFC1918 targets |
| Path traversal via static asset paths | `PathSecurityMiddleware` (PART 5) blocks `..`, `%2e%2e`, normalizes // -> / |
| Scraping / abusing the search endpoint as a free unlogged proxy | Configurable rate limiting (`rate_limit.requests`/`window`) + GeoIP allow/deny + IP/domain blocklist (PART 12) |
| Abuse via Tor exit nodes | Tor traffic is allowed by default (privacy goal), but operator can blocklist exit nodes in `server.yml` if abuse is observed |
| Geographic compliance bypass | Operator-configurable restriction modes (off, warn, soft_block, hard_block) via `server.yml`; dismissable acknowledgement cookie (30 days) for soft_block |
| Restore-to-takeover via `--maintenance restore` | PART 0 / PART 5 authorization flow: empty DB OR root OS user (service user must prompt for credentials) |
| Setup-to-takeover via `--maintenance setup` after install | First-run only OR root OS user OR valid one-time setup token |
| Mode change to expose debug endpoints | `--maintenance mode` requires root OS user or service credentials |
| Untrusted engine HTML pivoting into the proxy host | Engine adapters never render engine HTML; they extract specific fields, all of which are escaped at render |

**Project-specific abuse cases that MUST stay defended:**

- AI / deepfake content surfacing: server-side AI Content Filter is on by default. Disable is per-user opt-in (preference toggle), never default-on for the server.
- Performer name autocomplete must never confirm whether a particular performer name exists in the local performer dataset beyond the public 150+ list - it is a static suggestion source, not a directory.
- "Forward IP for geo-targeted results" is admin-controlled; even when admin enables it, users opt-in via a server-side cookie (`forward_ip=1`). It is NOT a localStorage preference, because localStorage is not authoritative for cross-user privacy decisions.

### Security decisions & exceptions

The following are intentional, project-defined deviations or strong defaults. Any code change that touches these MUST update this section.

| Decision | Rationale |
|----------|-----------|
| No user accounts (PART 34 NOT implemented) | Stateless privacy-first design; user accounts would create a record of who searched what, defeating the product purpose. |
| No organizations (PART 35 NOT implemented) | Same reason; multi-tenant search has no product fit here. |
| No custom domains (PART 36 NOT implemented) | No per-user/per-org branding requirement. |
| Tor users bypass GeoIP/restriction by default | Tor anonymizes IP, so geo-checks are not actionable. Operator can flip this on in `server.yml` if local law requires. |
| Server-side AI content filter ON by default | Default-on protects all visitors including ones who never open preferences; user opt-in (preference) overrides. |
| Default `min_duration` = 10 minutes | Reduces accidental thumbnail count for shorter clips - usability default, not a security control. |
| `forward_ip` is server-cookie opt-in, not localStorage | LocalStorage is per-browser; geo-forwarding affects requests the server makes, so it must be readable on the request. |
| Thumbnail proxy is mandatory; cannot be disabled by user | Direct thumbnail URLs would leak the user's IP to source CDNs - core privacy guarantee. The PREFERENCE toggle "Proxy thumbnails through server" is documented as default Yes; disabling it is an operator-level `server.yml` option only — it must NOT silently disable the proxy in normal mode. |
| Engines are HTML-parsed, not iframed | Iframing untrusted adult content origins would leak referrer + cookies to engines and let them frame us. |
| Run as dedicated `vidveil` system user (no permanent root) | Default per AI.md PART 5; permanent-root would require an IDEA.md-justified exception, which this product does not have. |
| Engine registry is operator-only | End users have engine toggles in preferences (client-side), but the engine REGISTRY is operator-only via `server.yml` — prevents tampering with which engines are reachable for everyone. |

---

### Search behavior (reference detail)

- Bang search syntax: `!xx query` where `!xx` is engine shortcut.
- Multiple bangs supported: `!ph !rt query` searches both engines.
- Without bang, search queries all enabled engines.
- SSE streaming delivers results as each engine responds.
- All thumbnails proxied through server to prevent tracking.
- Autocomplete suggests bang shortcuts as user types `!`.
- Results are merged from all queried engines.
- URL deduplication with normalization (removes duplicates across engines).
- Page parameter supports infinite scroll.
- No admin web UI — all server configuration is via `server.yml`.

**Semantic Search & AND-Based Filtering (Server-Side):**
- Server-side filtering before results are sent to client.
- Multi-term queries use AND logic (all terms must match).
- Search term order doesn't matter ("teen blonde" = "blonde teen").
- Filtering matches against title, tags, and performer fields.
- Term normalization expands synonyms automatically:
  - "teen" matches: 18, 19, eighteen, nineteen, barely legal, young, 18yo, 19yo
  - "pregnant" matches: preggo, preggy, expecting, knocked up
  - "lesbian" matches: lesbo, girl on girl, girls, lez, lesbians
  - "milf" matches: mom, mother, mommy, cougar, mature
  - "bbw" matches: chubby, fat, plump, thick, curvy, plus size
  - "asian" matches: oriental, japanese, chinese, korean, thai, filipina
  - 27 category mappings with synonyms.
- Quoted phrases preserved as single term ("big tits blonde").
- Engines parse additional metadata: tags, categories, performer names.

**Smart Related Searches:**
- Generated based on actual query terms (not random suggestions).
- Combines query words with related terms from taxonomy.
- Includes quality modifiers (hd, 4k, amateur, homemade, pov).
- Swaps synonyms to create variations.
- Sub-combinations for multi-word queries.

**Engine Tiers:**
- Tier 1: PornHub, XVideos, XNXX, RedTube, xHamster - major sites.
- Tier 2: Eporner, YouPorn, PornMD - popular sites.
- Tier 3-6: 35 additional engines - HTML parsing.

**Validation:**
- Query must be non-empty.
- Page must be >= 1 (default: 1).
- Bang shortcuts must exist in bangs list.
- Engine names must be valid registered engines.

**Geographic Content Restriction:**
- Admin-configurable restriction modes: off, warn, soft_block, hard_block.
- Default mode: warn (shows dismissable banner).
- Restricted regions configurable by country code or region (e.g., "US:Texas").
- Tor users bypass restriction checks by default (configurable).
- Non-geolocatable IPs (VPN/Tor) are not restricted.
- Acknowledgment cookie (30 days) for soft_block mode.
- Default restricted US states: Texas, Utah, Louisiana, Arkansas, Montana, Mississippi, Virginia, North Carolina.

**AI Content Filter (Server-Side):**
- Filters out AI-generated/deepfake content by default.
- Server-wide default: enabled (AI content blocked).
- Users can override via preference to show AI content.
- Keyword-based detection in titles and tags.
- Configurable keyword list in `server.yml` (`content.ai_filter_keywords`).

**Client-Side Filtering (applied after results load):**
- Duration: Any, Under 10min, 10-30min, Over 30min.
- Quality: Any, 4K (2160p), 1080p HD, 720p.
- Sources: Multi-source selection with "All Sources" toggle.
- Preview filter: Show only videos with preview.
- Minimum duration: 0, 1, 3, 5, 10, 20, 30 minutes.

**Client-Side Sorting:**
- Preview First (toggle): Videos with preview capability sorted to top.
- Relevance (original order from engines).
- Duration descending (longest first).
- Duration ascending (shortest first).
- View count descending (most viewed).
- Quality score (4K=3, 1080p=2, 720p=1).

### Pages & layout (reference detail)

**Home Page (`/`):**
- Large centered search form with bang hints.
- Search history display (up to 8 recent with timestamps).
- Per-item remove button and clear all button.
- Timestamps: "just now", "5m ago", "1h ago", "2d ago".
- Engine statistics (42 engines, no tracking, Tor support).
- Collapsible filters panel.

**Search Results Page (`/search?q={query}`):**
- Inline compact search form for refinement.
- Real-time status bar (streaming status, engine count).
- Collapsible filters panel with filter count badge.
- Related searches section (up to 20 suggestions).
- Video grid with infinite scroll.
- Dynamic loading indicator.

**Preferences Page (`/preferences`):**
- All settings stored in localStorage (`vidveil_prefs` key).
- No server-side storage.
- Reset to defaults button.

**Other Pages:**
- `/server/about` - About page.
- `/server/privacy` - Privacy policy.
- `/server/contact` - Contact information.
- `/server/help` - Help documentation.
- NoJS fallback versions of all pages.

### Video cards / preview / autocomplete / history / favorites / related / infinite scroll / filters

(Preserved exactly from the prior IDEA.md - see git history of IDEA.md.preMigration.bak. No semantic change.)

- Video Cards: thumbnail (16:9), 2-line clamped title, source badge, duration badge bottom-right, quality badge top-left, view count, three-dot menu (open in new tab, copy link, add/remove favorite, download).
- Video Preview: desktop hover with delay (Instant / 200ms / 500ms / 1000ms), mobile swipe-right (50px threshold, auto-stop 8s).
- Autocomplete System: bang mode triggered by `!`, performer mode by `@`, search-term mode otherwise (2+ chars, 150ms debounce, hidden until typing). Multi-bang `!ph !rt lesbian` flow. Suggestion sources: user history (priority 1), static suggestions (priority 2), popular (priority 3).
- Search History: localStorage (`vidveil_history`), max items configurable (default unlimited), auto-clear options (Never / 1d / 7d / 30d), Export/Import JSON.
- Favorites: localStorage (`vidveil_favorites`), each entry url+title+thumbnail+source+added_at, Export/Import JSON, Clear all with confirm.
- Related Searches: server-side rendered into HTML, client adds "Show more" toggle, up to 20 (first 8 visible).
- Infinite Scroll: IntersectionObserver, sentinel 200px, dedup by URL, stops when no more results.
- Filter Panel: collapsible with toggle, filter count badge, multi-source checkbox per engine, "All Sources" toggle, persists during session.

### User preferences (reference detail)

All preferences stored in localStorage (`vidveil_prefs` key). No server-side storage.

**Appearance:**
- Theme: Auto (system), Dark (Dracula), Light - default: Auto.
- Grid density: Comfortable (340px), Default, Compact (220px) - default: Default.
- Thumbnail size: Small, Medium, Large - default: Medium.

**Video Preview:**
- Auto-play preview on hover (toggle) - default: Yes.
- Preview delay: Instant, 200ms, 500ms, 1000ms - default: Instant.

**Search Settings:**
- Results per page: Infinite scroll, 20, 50, 100 - default: Infinite scroll.
- Open links in new tab (toggle) - default: Yes.

**Default Filters (auto-applied to new searches):**
- Show videos with preview first (toggle) - default: Yes (sort priority, not exclusive filter).
- Default duration filter: Any, Under 10min, 10-30min, Over 30min - default: Any.
- Default quality filter: Any, 4K, 1080p HD, 720p - default: Any.
- Minimum quality filter (server-side): Any, 240p+, 360p+, 480p+, 720p+, 1080p+, 4K only - default: 360p+.
- Default sort: Relevance, Longest, Shortest, Most Viewed, Best Quality - default: Relevance.
- Minimum video duration: 0, 1, 3, 5, 10, 20, 30 minutes - default: 10 minutes.

**History:**
- Maximum history items: Unlimited, 10, 25, 50, 100 - default: Unlimited.
- Auto-clear history: Never, 1 day, 7 days, 30 days - default: Never.
- Export/Import/Clear buttons.

**Favorites:**
- Export/Import/Clear buttons.
- Count display.

**Privacy:**
- Use Tor for all searches (toggle) - default: No (stored in localStorage; Tor routing is server-side).
- Proxy thumbnails through server (toggle) - default: Yes.
- Show AI-generated content (toggle) - default: No (AI content filtered out).
- Forward IP for geo-targeted results - admin-controlled feature; when enabled by admin, users opt in via a server-side cookie (`forward_ip=1`); not a localStorage preference.

**Search Engines:**
- Tier-based toggle switches (all tiers enabled by default):
  - Tier 1 - Major Sites (5 engines).
  - Tier 2 - Popular Sites (3 engines).
  - Tier 3-6 - Additional Sites (35 engines).
- Expand/collapse each tier to see individual engines.
- Checkbox for each individual engine within tiers.
- Indeterminate state when some engines in tier are disabled.
- Select All / Select None buttons.

### UI components, accessibility, SSE, error handling

- HTML5/CSS-First: minimize JS, use `<details>/<summary>`, native `loading="lazy"`, no IntersectionObserver for images.
- Toggle switches per AI.md PART 16: hidden checkbox + CSS, keyboard accessible.
- Notifications/toasts: success / error / warning / info; auto-dismiss 3s; top-right.
- Loading states: large spinner initial, compact spinner pagination, "Connecting to engines..." -> "Searching engines..." text updates.
- Status bar: fixed bottom, connection status + engine count, real-time updates.
- Keyboard shortcuts: `/` focus search, `Esc` blur/close dropdowns, Arrow Up/Down navigate autocomplete, Enter/Tab select.
- Responsive: Desktop >1024px (4+ col), Tablet <=768px (2 col), Mobile <=600px (1 col), Extra Small <=380px (adjusted padding/font), 44px touch minimum, hamburger nav.
- A11Y: WCAG 2.1 AA target, ARIA labels, role attributes, aria-expanded, aria-live polite, skip link, semantic HTML5, keyboard nav, focus management.
- SSE Streaming primary (`text/event-stream`), Tier 1 first, status updates with engine count + elapsed ms; fallback to JSON API (`application/json`).
- Error handling: image fallback to `placeholder.svg`, JSON parse handling, SSE -> JSON fallback, network retry, missing-pref defaults applied, graceful UI state.

### Endpoints (reference detail)

**Search:**

| Method | Purpose |
|--------|---------|
| GET | Search videos - see PART 14 |
| GET | Autocomplete (bangs + terms) - see PART 14 |

**Search Content Negotiation:**

| Accept Header | Response |
|---------------|----------|
| `application/json` | JSON with caching |
| `text/event-stream` | SSE streaming |
| `text/plain` | Plain text format |

**Engines:**

| Method | Purpose |
|--------|---------|
| GET | List all engines - see PART 14 |
| GET | Get engine details - see PART 14 |
| GET | List bang shortcuts - see PART 14 |

**Proxy:**

| Method | Purpose |
|--------|---------|
| GET | Proxy thumbnail image - see PART 14 |

**Frontend Pages:**

| URL | Description |
|-----|-------------|
| `/` | Home page with search |
| `/search?q={query}` | Search results page |
| `/preferences` | User preferences |
| `/age-verify` | Age verification gate |
| `/content-restricted` | Geographic content restriction acknowledgment |
| `/server/about` | About page |
| `/server/privacy` | Privacy policy |
| `/server/contact` | Contact page |
| `/server/help` | Help page |

**Server Administration:**
- All configuration via `server.yml` (file-only) — no admin web routes (AI.md PART 5).
- Operator restarts the server after editing `server.yml`.

### Data sources

- 42 video sites via HTML parsing.
- Engine definitions embedded at build time (`src/server/engine/engines.go`).
- Search results fetched in real-time.
- No local storage of search results (stateless).
- Thumbnails proxied through server on-demand (proxy may cache temporarily to reduce repeated fetches).
- Engine definitions updated at build time.

### Engine registry (reference detail)

**All Registered Engines (42 total)**

**Tier 1 - Major Sites (5 engines):**

| Engine | Bang | Capabilities |
|--------|------|--------------|
| pornhub | !ph | Preview (data-mediabook), Duration, Views, Rating, Quality |
| xvideos | !xv | Preview (data-preview), Duration, Views, Quality |
| xnxx | !xx | Duration, Views |
| redtube | !rt | Preview (data-mediabook), Duration, Views, Rating, Quality |
| xhamster | !xh | Duration, Views, Rating |

**Tier 2 - Popular Sites (3 engines):**

| Engine | Bang | Capabilities |
|--------|------|--------------|
| eporner | !ep | Duration, Views, Rating |
| youporn | !yp | Preview (data-mediabook), Duration, Views, Rating, Quality |
| pornmd | !pmd | Duration, Views |

**Tier 3-6 - Additional Sites (34 engines):**
4tube, fux, porntube, youjizz, sunporno, txxx, nuvid, tnaflix, drtuber, empflix, hellporno, alphaporno, pornflip, gotporn, xxxymovies, lovehomeporn, pornerbros, nonktube, nubilesporn, pornbox, porntop, pornotube, pornhd, xbabe, pornone, pornhat, porntrex, hqporner, vjav, flyflv, tube8, anyporn, tubegalore, 3movs.

**Engine configuration:**
- Default: All 42 engines enabled.
- SSE streaming behavior: results stream as each engine responds; Tier 1 first; Tier 2-6 fill in below; invalid thumbnails auto-discarded.

**Preview URL sources (generic extraction across engines):**
- `data-mediabook`, `data-preview`, `data-video-preview`, `data-rollover`.
- `data-preview-url`, `data-gif`, `data-webm`, `data-mp4`.
- `data-thumb-url`, `data-trailer`, `data-teaser`, `data-preview-custom`.

**Engine-specific sources:**

| Engine | Attribute / Method | Content Type |
|--------|-------------------|--------------|
| PornHub | `data-mediabook` | Video preview |
| XVideos | `data-preview` | Video preview |
| RedTube | `data-mediabook` | Video preview |
| YouPorn | `data-mediabook` | Video preview |
| TNAFlix | `data-preview-url` | Video preview |
| PornHat | `data-preview-custom` on `<a>` | MP4 video preview |
| PornHD | ttcache.com CDN constructed from `data-public-id` | MP4 video preview |
| TubeGalore | ttcache.com CDN constructed from `data-public-id` | MP4 video preview |

### Database configuration (reference detail)

**Supported Backends:**
- SQLite (default) — embedded, zero-config (per PART 10).
- libsql/Turso — remote-only, for cloud or edge deployments (per PART 10).

**Default:** SQLite with WAL mode, 5s busy timeout.

**Single-instance only** — there is no cluster mode, no horizontal scaling, no node election (AI.md line 2055).
