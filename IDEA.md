## Project Business Purpose

Purpose: Privacy-respecting meta search engine for adult video content that aggregates results from 50 video sites without tracking, logging, or analytics.

Target Users:
- Privacy-conscious users seeking adult content without tracking
- Self-hosters wanting their own private search instance
- Developers needing a unified API across multiple adult video platforms
- Tor users requiring anonymous access to adult content

Unique Value:
- No tracking, logging, or analytics - complete privacy
- 50 engines with bang shortcuts for targeted searches
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

### Admin Routes

**Admin panel follows PART 17: ADMIN PANEL hierarchy exactly.**

| Route Type | Pattern | Reference |
|------------|---------|-----------|
| **Admin Web** | `/{admin_path}/*` | PART 17 |
| **Admin API** | `/api/v1/{admin_path}/*` | PART 17 |
| **Server Management** | `/{admin_path}/server/*` | PART 17 (all server config) |

VidVeil admin manages: engines (enable/disable), rate limiting, Tor settings, GeoIP, backups.
No user management (VidVeil is stateless, no user accounts).

### Health & System Endpoints

| Method | URL | Description |
|--------|-----|-------------|
| GET | `/healthz` | Health check (plain text) |
| GET | `/api/v1/healthz` | Health check (JSON) |
| GET | `/api/v1/version` | Version info |
| GET | `/api/v1/stats` | Search statistics |

### VidVeil Endpoints

**Search Endpoints:**
| Method | URL | Description |
|--------|-----|-------------|
| GET | search endpoint | Search videos (content negotiation) - see PART 14 |
| GET | autocomplete endpoint | Unified autocomplete (bangs + search terms) - see PART 14 |

**Search Content Negotiation:**
| Accept Header | Response |
|---------------|----------|
| `application/json` (default) | JSON with caching |
| `text/event-stream` | SSE streaming results as engines respond |
| `text/plain` | Plain text format |

**Engine Endpoints:**
| Method | URL | Description |
|--------|-----|-------------|
| GET | engines list endpoint | List all available engines - see PART 14 |
| GET | engine details endpoint | Get specific engine info - see PART 14 |
| GET | bangs list endpoint | List all bang shortcuts - see PART 14 |

**Proxy Endpoints:**
| Method | URL | Description |
|--------|-----|-------------|
| GET | thumbnail proxy endpoint | Proxy thumbnail image - see PART 14 |

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
- SSE streaming via `Accept: text/event-stream` delivers results as each engine responds
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

**Status:** Implemented - video page URL used as download URL (works with yt-dlp and download managers)

### Autocomplete System

**Overview:**
Full autocomplete system for both bang shortcuts and adult search terms with privacy-first design.

**Mode Switching Behavior:**
- **Bang Mode**: Triggered when user types `!` character
  - Shows engine bang suggestions (e.g., `!ph`, `!xv`, `!rt`)
  - Activates immediately after `!` with 1+ characters
  - Example: `!p` → shows PornHub, PornMD, PornTube, etc.
- **Search Term Mode**: Triggered when no active `!` being typed
  - Shows adult search term suggestions
  - Requires 2+ characters minimum
  - Example: `ama` → shows "amateur", "amateur couple", "amazing", etc.

**Multi-Bang Support:**
User types: `!ph !rt lesbian`
1. `!ph` → Bang mode, suggests pornhub engines
2. Space → Complete, switch to next item
3. `!rt` → Bang mode, suggests redtube engines  
4. Space → Complete, switch to next item
5. `les` → Search term mode, suggests "lesbian", "lesbian massage", etc.

**Autocomplete handles all bangs in sequence as they're typed.**

**Dropdown Display:**
- Hidden by default (does not show on focus)
- Shows when user starts typing (1+ characters)
- Shows when user clicks "Popular" button/link
- Mode switches dynamically based on context:
  - Typing `!` = bang suggestions
  - No `!` = search term suggestions
- Keyboard navigation: Arrow keys, Enter, Tab, Escape
- Click/tap to select
- Hides when no matches found (accepts user's custom search term)

**Search Term Sources (Combined):**
1. **User history** - Client-side localStorage (opt-in, never sent to server) - **priority 1**
2. **Static suggestions** - Hardcoded adult search terms (always available) - **priority 2**
3. **Popular searches** - Shown via "Popular" button or when requested - **priority 3**
4. **Admin-customizable** - Admin can add/remove/edit suggestion terms via admin panel

**Result Ordering:**
- History matches shown first (most relevant to user)
- Static suggestions shown second
- Popular searches shown third
- Within each category: sorted by relevance score (prefix match > contains match > frequency)
- Duplicates removed (if term exists in multiple sources, show once from highest priority source)
- No maximum limit - show all matching results

**Data Model:**

```go
// AutocompleteResponse for API endpoint
type AutocompleteResponse struct {
    Success     bool                    `json:"success"`
    Type        string                  `json:"type"`        // "bang", "search", "popular"
    Suggestions []AutocompleteSuggestion `json:"suggestions"`
}

// AutocompleteSuggestion unified structure
type AutocompleteSuggestion struct {
    // For bangs
    Bang        string `json:"bang,omitempty"`         // "!ph"
    EngineName  string `json:"engine_name,omitempty"`  // "pornhub"
    DisplayName string `json:"display_name,omitempty"` // "PornHub"
    ShortCode   string `json:"short_code,omitempty"`   // "!ph"
    
    // For search terms
    Term        string `json:"term,omitempty"`         // "amateur"
    Source      string `json:"source,omitempty"`       // "static", "history", "popular"
    Frequency   int    `json:"frequency,omitempty"`    // For history sorting
}
```

**Privacy Features:**
- **No server-side tracking** - Suggestions don't reveal user searches
- **Static suggestions** - Embedded at build time, no external calls
- **Optional localStorage** - User can enable/disable personal history
- **History management** - Clear history button in preferences
- **No sync** - History stays local, never sent to server

**User Preferences:**
- Enable/disable autocomplete entirely
- Enable/disable search history (localStorage)
- Max history items (default: 50)
- Clear history button
- Autocomplete delay (default: 150ms debounce)

**Admin Configuration:**
```yaml
search:
  autocomplete:
    enabled: true
    # Static suggestions always available
    custom_terms:
      - "popular custom term 1"
      - "popular custom term 2"
    # Allow admin to extend default list
```

**API Endpoint:**
- Unified autocomplete endpoint - see PART 14 for implementation
  - Returns bangs when query contains `!`
  - Returns search terms otherwise
  - Returns popular searches when query is empty

**Frontend Behavior:**
1. User focuses search input → No dropdown shown (hidden by default)
2. User clicks "Popular" button → Show popular searches in dropdown
3. User types 1+ character:
   - `!p` → Bang mode, show matching engine suggestions
   - `a` → Search term mode, show matching suggestions (if any exist)
4. User completes `!ph ` (space) → Switch to search term mode for next word
5. User types more → Continue showing matches or hide if no matches
6. User presses Enter/Tab/Click → Insert selected suggestion
7. User presses Escape → Hide dropdown
8. No matches found → Hide dropdown (accept user's custom search term)

**Implementation Requirements:**
- Minimum query length: 1 character (show matches if any exist)
- Debounced input (150ms default) to reduce API calls
- Client-side caching of suggestions
- Intelligent scoring: prefix match > contains match > frequency
- Deduplication: same term in multiple sources shown once
- No maximum results limit (show all matches)
- "Popular" button/link to display popular searches on demand
- Keyboard accessible - see PART 16 for accessibility requirements
- Mobile-optimized - see PART 16 for responsive design requirements

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

## Engine Registry

### All Registered Engines (50 total)

Complete list of engines with source URLs, API types, and capabilities.

**Tier 1 - Major Sites** (5 engines - enabled by default)
| Engine | Source URL | API Type | Capabilities |
|--------|------------|----------|--------------|
| pornhub | https://www.pornhub.com | html | Preview (data-mediabook), Download, Duration, Views, Rating, Quality |
| xvideos | https://www.xvideos.com | html | Duration, Views, Quality |
| xnxx | https://www.xnxx.com | html | Duration, Views |
| redtube | https://www.redtube.com | html | Preview (data-mediabook), Duration, Views, Rating, Quality |
| xhamster | https://xhamster.com | html | Duration, Views, Rating |

**Tier 2 - Popular Sites** (3 engines - enabled by default)
| Engine | Source URL | API Type | Capabilities |
|--------|------------|----------|--------------|
| eporner | https://www.eporner.com | html | Duration, Views, Rating |
| youporn | https://www.youporn.com | html | Preview (data-mediabook), Duration, Views, Rating, Quality |
| pornmd | https://www.pornmd.com | html | Duration, Views |

**Tier 3 - Additional Sites** (15 engines - enable via config)
| Engine | Source URL | API Type | Capabilities |
|--------|------------|----------|--------------|
| 4tube | https://www.4tube.com | html | Duration, Views |
| fux | https://www.fux.com | html | Duration, Views |
| porntube | https://www.porntube.com | html | Duration, Views |
| youjizz | https://www.youjizz.com | html | Duration, Views |
| sunporno | https://www.sunporno.com | html | Duration, Views |
| txxx | https://www.txxx.com | html | Duration, Views |
| nuvid | https://www.nuvid.com | html | Duration, Views |
| tnaflix | https://www.tnaflix.com | html | Duration, Views |
| drtuber | https://www.drtuber.com | html | Duration, Views |
| empflix | https://www.empflix.com | html | Duration, Views |
| hellporno | https://www.hellporno.com | html | Duration, Views |
| alphaporno | https://www.alphaporno.com | html | Duration, Views |
| pornflip | https://www.pornflip.com | html | Duration, Views |
| zenporn | https://www.zenporn.com | html | Duration, Views |
| gotporn | https://www.gotporn.com | html | Duration, Views |
| xxxymovies | https://www.xxxymovies.com | html | Duration, Views |
| lovehomeporn | https://www.lovehomeporn.com | html | Duration, Views |

**Tier 4 - Additional Sites** (17 engines)
| Engine | Source URL | API Type | Capabilities |
|--------|------------|----------|--------------|
| pornerbros | https://www.pornerbros.com | html | Duration, Views |
| nonktube | https://www.nonktube.com | html | Duration, Views |
| nubilesporn | https://www.nubilesporn.com | html | Duration, Views |
| pornbox | https://www.pornbox.com | html | Duration, Views |
| porntop | https://www.porntop.com | html | Duration, Views |
| pornotube | https://www.pornotube.com | html | Duration, Views |
| pornhd | https://www.pornhd.com | html | Duration, Views |
| xbabe | https://www.xbabe.com | html | Duration, Views |
| pornone | https://www.pornone.com | html | Duration, Views |
| pornhat | https://www.pornhat.com | html | Duration, Views |
| porntrex | https://www.porntrex.com | html | Duration, Views |
| hqporner | https://www.hqporner.com | html | Duration, Views |
| vjav | https://www.vjav.com | html | Duration, Views |
| flyflv | https://www.flyflv.com | html | Duration, Views |
| tube8 | https://www.tube8.com | html | Duration, Views |
| xtube | https://www.xtube.com | html | Duration, Views |

**Tier 5 - New Engines** (4 engines)
| Engine | Source URL | API Type | Capabilities |
|--------|------------|----------|--------------|
| anyporn | https://www.anyporn.com | html | Duration, Views |
| superporn | https://www.superporn.com | html | Duration, Views |
| tubegalore | https://www.tubegalore.com | html | Duration, Views |
| motherless | https://www.motherless.com | html | Duration, Views |

**Tier 6 - Additional Engines** (5 engines)
| Engine | Source URL | API Type | Capabilities |
|--------|------------|----------|--------------|
| keezmovies | https://www.keezmovies.com | html | Duration, Views |
| spankwire | https://www.spankwire.com | html | Duration, Views |
| extremetube | https://www.extremetube.com | html | Duration, Views |
| 3movs | https://www.3movs.com | html | Duration, Views |
| sleazyneasy | https://www.sleazyneasy.com | html | Duration, Views |

### Engine Configuration

**Default Enabled:**
- Tier 1: All 5 engines (pornhub, xvideos, xnxx, redtube, xhamster)
- Tier 2: All 3 engines (eporner, youporn, pornmd)
- Total: 8 engines enabled by default

**Enable Additional Engines:**
```yaml
search:
  default_engines:
    - pornhub
    - xvideos
    - xnxx
    - redtube
    - xhamster
    - eporner
    - youporn
    - pornmd
    # Add more engines as needed
    - 4tube
    - txxx
    # etc...
```

### Debug Tooling

**Problem:** Code shows what we attempt to extract, not what sites actually return.

**Debug Endpoint:** Debug endpoint - see PART 14 for implementation
- Returns raw response (HTML/JSON) from source site
- Shows parsed results alongside raw data
- Lists available but unused attributes/fields
- Shows extraction failures/misses
- Enables live investigation of engine responses without custom tooling

## Engine Data Standardization

### Investigation Required

**Problem:** Code analysis only shows what we *attempt* to extract - not what sites *actually* return. Live debugging required.

### Debug Tooling Needed

**Debug Endpoint** - see PART 14 for implementation
- Returns raw response (HTML/JSON) from source site
- Shows parsed results alongside raw data
- Lists available but unused attributes/fields
- Shows extraction failures/misses
- Enables live investigation of engine responses

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
| Static Thumbnails | All (50) | Implemented |
| Preview URLs | 3 (PornHub, XVideos, RedTube) | Implemented (code only) |
| Download URLs | 6 (Tier 1-2 engines) | Implemented (video page URL) |
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
