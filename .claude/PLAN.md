## Project Business Purpose

Purpose: Privacy-respecting meta search engine for adult video content that aggregates results from 54+ video sites without tracking, logging, or analytics.

Target Users:
- Privacy-conscious users seeking adult content without tracking
- Self-hosters wanting their own private search instance
- Developers needing a unified API across multiple adult video platforms
- Tor users requiring anonymous access to adult content

Unique Value:
- No tracking, logging, or analytics - complete privacy
- 54+ engines with bang shortcuts for targeted searches
- SSE streaming for real-time results as engines respond
- Thumbnail proxy prevents engine tracking of users
- Single static binary with all assets embedded
- Built-in Tor hidden service support
- Full admin panel for configuration

## Business Logic & Rules

Business Rules:
- Bang search syntax: `!xx query` where `!xx` is engine shortcut
- Multiple bangs supported: `!ph !rt query` searches both engines
- Without bang, search queries all enabled engines
- SSE streaming delivers results as each engine responds
- All thumbnails proxied through server to prevent tracking
- Autocomplete suggests bang shortcuts as user types
- Results are merged from all queried engines
- Page parameter supports infinite scroll
- No user accounts - stateless, privacy-first design
- Admin panel requires authentication (server-admin only)

Engine Tiers:
- Tier 1 (API-based): PornHub, RedTube, Eporner - direct JSON APIs
- Tier 2 (JSON extraction): xHamster, YouPorn - extract JSON from pages
- Tier 3+ (HTML parsing): XVideos, XNXX, and 38+ others - HTML scraping

Validation:
- Query must be non-empty
- Page must be >= 1 (default: 1)
- Bang shortcuts must exist in bangs list
- Engine names must be valid registered engines

## Data Models

```go
// VideoResult represents a single video search result
type VideoResult struct {
    // Video identifier (engine-specific)
    ID           string   `json:"id"`
    // Video title
    Title        string   `json:"title"`
    // Source engine name
    Engine       string   `json:"engine"`
    // URL to video page on source site
    URL          string   `json:"url"`
    // Direct download URL (if available, not proxied)
    DownloadURL  string   `json:"download_url,omitempty"`
    // Proxied thumbnail URL (static image)
    Thumbnail    string   `json:"thumbnail"`
    // Preview video URL for hover/swipe (if available)
    PreviewURL   string   `json:"preview_url,omitempty"`
    // Video duration in seconds
    Duration     int      `json:"duration"`
    // View count (if available)
    Views        int64    `json:"views,omitempty"`
    // Upload date (if available)
    UploadDate   string   `json:"upload_date,omitempty"`
    // Video quality (HD, 4K, etc.)
    Quality      string   `json:"quality,omitempty"`
}

// Engine represents a search engine
type Engine struct {
    // Internal engine name
    Name        string   `json:"name"`
    // Display name
    DisplayName string   `json:"display_name"`
    // Bang shortcut (e.g., "ph" for !ph)
    Bang        string   `json:"bang"`
    // Engine tier (1=API, 2=JSON, 3+=HTML)
    Tier        int      `json:"tier"`
    // Whether engine is enabled
    Enabled     bool     `json:"enabled"`
    // Search method (api, json, html)
    Method      string   `json:"method"`
    // Feature flags
    HasPreview  bool     `json:"has_preview"`  // Supports preview URLs
    HasDownload bool     `json:"has_download"` // Supports download URLs
}

// SearchResponse represents search results
type SearchResponse struct {
    // Original query (with bangs)
    Query        string        `json:"query"`
    // Cleaned query (bangs removed)
    SearchQuery  string        `json:"search_query"`
    // Whether query contained bang(s)
    HasBang      bool          `json:"has_bang"`
    // Engines targeted by bangs
    BangEngines  []string      `json:"bang_engines,omitempty"`
    // Search results
    Results      []VideoResult `json:"results"`
    // Engines used in search
    EnginesUsed  []string      `json:"engines_used"`
    // Search time in milliseconds
    SearchTimeMs int64         `json:"search_time_ms"`
}
```

## Data Sources

Data Sources:
- External APIs: PornHub Webmasters API, RedTube Public API, Eporner v2 JSON API
- HTML Parsing: XVideos, XNXX, xHamster, YouPorn, and 38+ other sites
- Engine definitions embedded in binary (bangs, URLs, parsing rules)

Update Strategy:
- Engine definitions embedded at build time
- Search results fetched in real-time from external sites
- Thumbnails proxied through server on-demand
- No local storage of search results (stateless)

Data Location:
- src/server/engine/engines.go: Engine definitions and bang mappings
- No persistent data storage - all searches are real-time

## Project-Specific Endpoints Summary

**Endpoint Implementation Rules:**

| Endpoint Type | Implementation Rules | Notes |
|---------------|---------------------|-------|
| **Standard API** | Follow PART 14: API STRUCTURE | `/api/v1/*` patterns, response formats |
| **Frontend** | Follow PART 16: WEB FRONTEND | HTML templates, themes, accessibility |
| **Compatibility** | Follow PART 14: External API Compatibility | External services ONLY - match their exact format |
| **Legacy** | **NEVER KEEP** | Old/changed/removed endpoints - DELETE them |

**PART 37 describes WHAT endpoints do (business purpose). PARTs 14/16 define HOW to implement them.**

### VidVeil Endpoints

**Search Endpoints:**
| Method | URL | Description |
|--------|-----|-------------|
| GET | `/api/v1/search?q={query}&page={page}&engines={engines}` | Search videos across engines |
| GET | `/api/v1/search/stream?q={query}&page={page}` | SSE stream of search results |
| GET | `/api/v1/bangs/autocomplete?q={partial}` | Bang shortcut suggestions |

**Engine Endpoints:**
| Method | URL | Description |
|--------|-----|-------------|
| GET | `/api/v1/engines` | List all available engines |
| GET | `/api/v1/engines/{name}` | Get specific engine info |
| GET | `/api/v1/bangs` | List all bang shortcuts |

**Proxy Endpoints:**
| Method | URL | Description |
|--------|-----|-------------|
| GET | `/api/v1/proxy/thumbnails?url={encoded_url}` | Proxy thumbnail image |

**Frontend Pages:**
| URL | Description |
|-----|-------------|
| `/` | Home page with search |
| `/search?q={query}` | Search results page |
| `/preferences` | User preferences (theme, engines) |
| `/server/about` | About page |
| `/server/privacy` | Privacy policy |

**Business Behavior:**
- Search supports bang shortcuts (`!ph amateur` searches PornHub only)
- Multiple bangs combine: `!ph !rt query` searches both engines
- SSE stream delivers results as each engine responds
- Autocomplete suggests bangs as user types `!` prefix
- Thumbnails proxied to prevent engine tracking
- Infinite scroll supported via page parameter
- No cookies or session storage (stateless)

## Extended Node Functions (If Applicable)

**Not applicable for VidVeil.** VidVeil is a stateless search aggregator with no node-specific functions beyond standard clustering (config sync).

## High Availability Requirements (If Applicable)

**Not applicable for VidVeil.** VidVeil is stateless - any instance can handle any request. Standard load balancing provides sufficient availability.

## Planned Features

### Video Previews (Hover/Swipe)

**Behavior:**
- Desktop: Hover on thumbnail plays preview video
- Mobile: Swipe on thumbnail plays preview video
- Preview data included in SSE VideoResult stream (always, no separate fetch)
- Only displayed for engines that support previews
- Engines without preview support show static thumbnail on hover/swipe

**Data:**
- `PreviewURL` field in VideoResult (separate from Thumbnail)
- All thumbnails are static images (jpg/png)
- Preview URLs point to video/animation content
- Preview sourced from engine-specific attributes (data-preview, data-mediabook)

### Download Button

**Behavior:**
- Shows only when valid `DownloadURL` present in result
- Direct download (NOT proxied) - user connects directly to source site
- One-time privacy warning: "Downloads connect directly to source site, exposing your IP"
- Warning dismissable and stored in localStorage (shown once per browser)

**Status:** Download URLs not currently populated by any engine - requires implementation

### Additional Enhancements

**Privacy Options:**
- Optional preview proxy (bandwidth-heavy but consistent privacy)
- Download link copy button - copy URL to clipboard for user's own download tools (VPN, Tor browser)

**User Preferences (`/preferences`):**
- Disable previews entirely (data saver mode)
- Hover delay before preview starts (prevent accidental triggers)
- Preview quality selection (if engines offer options)

**UI Indicators:**
- Engine capability badges on results showing:
  - Preview available icon
  - Download available icon
- Helps users understand why some videos have features others don't

**Local Favorites:**
- localStorage-only bookmarks (no server storage, fits stateless design)
- Save videos for later without any tracking
- Export/import favorites as JSON

**Mobile Gestures:**
- Long-press menu for quick actions: download, copy link, open source site

**Performance:**
- Optional preview prefetch for visible results (faster hover response, more bandwidth)

## Engine Data Standardization

### Investigation Required

**Problem:** Code analysis only shows what we *attempt* to extract - not what sites *actually* return. Live debugging required.

### Debug Tooling Needed

1. **Debug Endpoint:** `GET /api/v1/debug/engine/{name}?q={query}`
   - Returns raw response (HTML/JSON) from source site
   - Shows parsed results alongside raw data
   - Lists available but unused attributes/fields
   - Shows extraction failures/misses

2. **Engine Probe CLI:** `vidveil probe --all` or `vidveil probe --engine=xvideos`
   - Tests each engine with sample query
   - Captures actual response data
   - Generates capability report
   - Identifies preview/download opportunities

3. **Verbose Logging Mode:** `--debug-engines` flag
   - Logs raw responses during normal searches
   - Captures attribute discovery data
   - Helps identify site changes/breakages

### Data Standardization Plan

**Required Fields (all engines must provide):**
- `ID` - unique identifier
- `Title` - video title
- `URL` - video page URL
- `Thumbnail` - static image URL
- `Source` - engine identifier
- `SourceDisplay` - human-readable engine name

**Optional Fields (engine capability flags indicate support):**
- `PreviewURL` - animated preview (requires `HasPreview: true`)
- `DownloadURL` - direct download link (requires `HasDownload: true`)
- `Duration` / `DurationSeconds` - video length
- `Views` / `ViewsCount` - view count
- `Rating` - rating score
- `Quality` - HD/4K badge
- `UploadDate` - publication date
- `Description` - keywords/tags

**Engine Capability Declaration:**
```go
type EngineCapabilities struct {
    HasPreview    bool   // Can provide PreviewURL
    HasDownload   bool   // Can provide DownloadURL
    HasDuration   bool   // Can provide duration
    HasViews      bool   // Can provide view count
    HasRating     bool   // Can provide rating
    HasQuality    bool   // Can provide quality badge
    HasUploadDate bool   // Can provide upload date
    PreviewSource string // e.g., "data-preview", "data-mediabook"
}
```

**Validation:**
- Engines must declare capabilities accurately
- Results validated against declared capabilities
- Missing required fields = engine error
- Missing optional fields = only if capability declared

### Per-Engine Custom Code

Preview and download extraction requires custom code per engine because:
- Each site uses different HTML attributes (data-preview, data-mediabook, etc.)
- Some sites embed video URLs in JavaScript, not HTML
- API-based sites may have undocumented endpoints
- Sites change their structure frequently

**Investigation checklist per engine:**
1. Capture raw HTML/JSON response
2. Search for preview-related attributes (data-preview, data-mediabook, data-video, etc.)
3. Search for video URLs in scripts, JSON blobs, meta tags
4. Document what's available vs what we extract
5. Implement custom extraction if source exists
6. Update capability flags

## Engine Feature Matrix

### Current Engine Capabilities (Code Analysis - Needs Live Verification)

| Engine | Tier | API Type | Thumbnail | Preview | Download | Notes |
|--------|------|----------|-----------|---------|----------|-------|
| **PornHub** | 1 | Webmasters API | Static | Yes (`data-mediabook`) | No | Pagination, Sorting, Rating |
| **XVideos** | 1 | HTML Parser | Static | Yes (`data-preview`) | No | Pagination, Sorting, Quality |
| **RedTube** | 1 | Public API | Static | Yes (`data-mediabook`) | No | Pagination, Sorting, Rating |
| **xHamster** | 1 | JSON Extraction | Static | No | No | Pagination |
| **XNXX** | 1 | HTML Parser | Static | No | No | Pagination |
| **Eporner** | 2 | JSON API v2 | Static | No | No | Pagination, Sorting |
| **YouPorn** | 2 | HTML Parser | Static | No | No | Pagination |
| **PornMD** | 2 | HTML Parser | Static | No | No | Pagination (Meta-search) |
| **Others** | 3+ | Generic Parser | Static | No | No | Basic pagination |

### Feature Summary (Needs Live Verification)

| Feature | Engines Supporting | Implementation Status |
|---------|-------------------|----------------------|
| Static Thumbnails | All (54+) | Implemented |
| Preview URLs | 3 (PornHub, XVideos, RedTube) | Implemented (code only) |
| Download URLs | 0 | Not implemented |
| Pagination | All | Implemented |
| Sorting | PornHub, XVideos, Eporner | Implemented |
| Quality Detection | XVideos, XNXX | Implemented |
| Ratings | PornHub, RedTube | Implemented |

**Note:** Above data based on code analysis. Live debugging required to verify actual site responses and discover additional extraction opportunities.

### Preview URL Sources

| Engine | Attribute | Content Type |
|--------|-----------|--------------|
| PornHub | `data-mediabook` | Video preview |
| XVideos | `data-preview` | Video preview |
| RedTube | `data-mediabook` | Video preview |

## Notes

- VidVeil has NO user accounts - it's a privacy-first, stateless search aggregator
- All searches are real-time - no caching of search results
- Thumbnail proxy caches images temporarily to reduce repeated fetches
- Engine availability may vary - some external sites may block or rate limit
- Tor support allows operation as a hidden service for maximum privacy
- Admin panel is for server configuration only (per PART 17), not user management
