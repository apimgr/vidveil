# VidVeil

## Project Description

Privacy-respecting meta search engine for adult video content that aggregates results from 43 video sites without tracking, logging, or analytics. Results stream in real-time via SSE as each engine responds.

**Target Users:**
- Privacy-conscious users seeking adult content without tracking
- Self-hosters wanting their own private search instance
- Developers needing a unified API across multiple adult video platforms
- Tor users requiring anonymous access to adult content

---

## Project-Specific Features

- **Privacy-First Search**: No tracking, logging, or analytics - complete privacy
- **Multi-Engine Aggregation**: 43 engines with bang shortcuts for targeted searches
- **Real-Time Streaming**: SSE streaming delivers results as each engine responds
- **Thumbnail Proxy**: All thumbnails proxied through server to prevent tracking
- **Video Preview**: Hover (desktop) and swipe (mobile) preview support
- **Client-Side Preferences**: All settings stored in localStorage, no server storage
- **Favorites & History**: Local-only bookmarks and search history with export/import
- **Tor Support**: Built-in Tor hidden service support for maximum anonymity
- **Static Binary**: Single static binary with all assets embedded

---

## Detailed Specification

### Data Models

```go
// VideoResult represents a single video search result
type VideoResult struct {
    ID           string   `json:"id"`             // Video identifier (engine-specific)
    Title        string   `json:"title"`          // Video title
    Engine       string   `json:"engine"`         // Source engine name
    URL          string   `json:"url"`            // URL to video page on source site
    DownloadURL  string   `json:"download_url,omitempty"`  // Direct download URL (if available)
    Thumbnail    string   `json:"thumbnail"`      // Proxied thumbnail URL (static image)
    PreviewURL   string   `json:"preview_url,omitempty"`   // Preview video URL for hover/swipe
    Duration     int      `json:"duration"`       // Video duration in seconds
    Views        int64    `json:"views,omitempty"`         // View count (if available)
    UploadDate   string   `json:"upload_date,omitempty"`   // Upload date (if available)
    Quality      string   `json:"quality,omitempty"`       // Video quality (HD, 4K, etc.)
}

// Engine represents a search engine
type Engine struct {
    Name        string   `json:"name"`           // Internal engine name
    DisplayName string   `json:"display_name"`   // Display name
    Bang        string   `json:"bang"`           // Bang shortcut (e.g., "ph" for !ph)
    Tier        int      `json:"tier"`           // Engine tier (1=API, 2=JSON, 3+=HTML)
    Enabled     bool     `json:"enabled"`        // Whether engine is enabled
    Method      string   `json:"method"`         // Search method (api, json, html)
    HasPreview  bool     `json:"has_preview"`    // Supports preview URLs
    HasDownload bool     `json:"has_download"`   // Supports download URLs
}

// SearchResponse represents search results
type SearchResponse struct {
    Query        string        `json:"query"`           // Original query (with bangs)
    SearchQuery  string        `json:"search_query"`    // Cleaned query (bangs removed)
    HasBang      bool          `json:"has_bang"`        // Whether query contained bang(s)
    BangEngines  []string      `json:"bang_engines,omitempty"`  // Engines targeted by bangs
    Results      []VideoResult `json:"results"`         // Search results
    EnginesUsed  []string      `json:"engines_used"`    // Engines used in search
    SearchTimeMs int64         `json:"search_time_ms"`  // Search time in milliseconds
}

// AutocompleteResponse for API endpoint
type AutocompleteResponse struct {
    Success     bool                     `json:"success"`
    Type        string                   `json:"type"`         // "bang", "search", "popular"
    Suggestions []AutocompleteSuggestion `json:"suggestions"`
}

// AutocompleteSuggestion unified structure
type AutocompleteSuggestion struct {
    Bang        string `json:"bang,omitempty"`          // "!ph" (for bangs)
    EngineName  string `json:"engine_name,omitempty"`   // "pornhub" (for bangs)
    DisplayName string `json:"display_name,omitempty"`  // "PornHub" (for bangs)
    ShortCode   string `json:"short_code,omitempty"`    // "!ph" (for bangs)
    Term        string `json:"term,omitempty"`          // "amateur" (for search terms)
    Source      string `json:"source,omitempty"`        // "static", "history", "popular"
    Frequency   int    `json:"frequency,omitempty"`     // For history sorting
}
```

### Business Rules

**Search Behavior:**
- Bang search syntax: `!xx query` where `!xx` is engine shortcut
- Multiple bangs supported: `!ph !rt query` searches both engines
- Without bang, search queries all enabled engines
- SSE streaming delivers results as each engine responds
- All thumbnails proxied through server to prevent tracking
- Autocomplete suggests bang shortcuts as user types `!`
- Results are merged from all queried engines
- Page parameter supports infinite scroll
- No user accounts - stateless, privacy-first design
- Admin panel requires authentication (server-admin only)

**Engine Tiers:**
- Tier 1: PornHub, XVideos, XNXX, RedTube, xHamster - major sites
- Tier 2: Eporner, YouPorn, PornMD - popular sites
- Tier 3-6: 35 additional engines - HTML parsing

**Validation:**
- Query must be non-empty
- Page must be >= 1 (default: 1)
- Bang shortcuts must exist in bangs list
- Engine names must be valid registered engines

**Client-Side Filtering (applied after results load):**
- Duration: Any, Under 10min, 10-30min, Over 30min
- Quality: Any, 4K (2160p), 1080p HD, 720p
- Sources: Multi-source selection with "All Sources" toggle
- Preview filter: Show only videos with preview
- Minimum duration: 0, 1, 3, 5, 10, 20, 30 minutes

**Client-Side Sorting:**
- Relevance (original order from engines)
- Duration descending (longest first)
- Duration ascending (shortest first)
- View count descending (most viewed)
- Quality score (4K=3, 1080p=2, 720p=1)

### Features

#### Pages & Layout

**Home Page (`/`):**
- Large centered search form with bang hints
- Search history display (up to 8 recent with timestamps)
- Per-item remove button and clear all button
- Timestamps: "just now", "5m ago", "1h ago", "2d ago"
- Engine statistics (43 engines, no tracking, Tor support)
- Collapsible filters panel

**Search Results Page (`/search?q={query}`):**
- Inline compact search form for refinement
- Real-time status bar (streaming status, engine count)
- Collapsible filters panel with filter count badge
- Related searches section (up to 20 suggestions)
- Video grid with infinite scroll
- Dynamic loading indicator

**Preferences Page (`/preferences`):**
- All settings stored in localStorage (`vidveil_prefs` key)
- No server-side storage
- Reset to defaults button

**Other Pages:**
- `/server/about` - About page
- `/server/privacy` - Privacy policy
- `/server/contact` - Contact information
- `/server/help` - Help documentation
- NoJS fallback versions of all pages

#### Video Cards

**Card Components:**
- Thumbnail with 16:9 aspect ratio
- Title (2-line clamp with ellipsis)
- Source badge with engine-specific styling
- Duration badge (bottom-right, formatted as MM:SS or HH:MM:SS)
- Quality badge (top-left: 4K, HD)
- View count (when available)
- Three-dot menu button (top-right)

**Card Menu (three-dot dropdown):**
- Open in new tab
- Copy link to clipboard
- Add to / Remove from favorites (dynamic text)
- Download (if available)
- Closes when clicking outside

#### Video Preview

**Desktop (hover-based):**
- Configurable delay: Instant, 200ms, 500ms, 1000ms
- Smooth opacity transition between static and preview
- Autoplay toggle in preferences (default: enabled)
- Only shows for engines that support previews

**Mobile (swipe-based):**
- Swipe-right gesture (50px threshold) to start preview
- Auto-stops after 8 seconds
- Swipe-left to stop manually
- Shows "Swipe to preview" hint on touch devices

#### Autocomplete System

**Mode Switching:**
- **Bang Mode**: Triggered when typing `!` character
  - Shows engine bang suggestions with lightning icon
  - Activates immediately after `!` with 1+ characters
- **Search Term Mode**: When no active `!` being typed
  - Shows search term suggestions with search icon
  - Requires 2+ characters minimum

**Multi-Bang Support:**
- User types: `!ph !rt lesbian`
- Each bang handled in sequence
- Space completes current bang, switches to next

**Frontend Behavior:**
- Hidden by default (does not show on focus)
- Shows when user starts typing (1+ characters)
- Keyboard navigation: Arrow Up/Down, Enter, Tab, Escape
- Click/tap to select suggestion
- 150ms debounce on input to reduce API calls
- Works on all search inputs: home, nav, results

**Suggestion Sources:**
1. User history (localStorage, priority 1)
2. Static suggestions (800+ terms, priority 2)
3. Popular searches (priority 3)

#### Search History

**Home Page Display:**
- Up to 8 recent searches visible
- Timestamps (relative time)
- Per-item remove button (×)
- Clear all button
- Deduplicated (case-insensitive)

**Storage:**
- localStorage key: `vidveil_history`
- Maximum items configurable (default: unlimited)
- Auto-clear option: Never, 1 day, 7 days, 30 days
- Export/Import as JSON

#### Favorites System

- localStorage key: `vidveil_favorites`
- Each favorite: url, title, thumbnail, source, added_at
- Add/remove from card menu
- Export/Import as JSON
- Clear all with confirmation
- Count display on preferences page

#### Related Searches

- Fetched via API after search completes
- Displays up to 20 suggestions
- First 8 visible, "Show more" button for rest
- Collapsible accordion pattern with animation
- Each tag includes search icon
- Animated staggered appearance (0.03s delay per item)

#### Infinite Scroll

- Intersection Observer API for performance
- Loads next page 200px before sentinel
- Prevents duplicate results by URL comparison
- Load more indicator during pagination
- Stops when no more results available
- Works alongside SSE streaming

#### Filter Panel

- Collapsible design with toggle button
- Filter count badge shows active filters
- Multi-source selection (checkbox per engine)
- "All Sources" toggle
- Filters persist during session

### User Preferences

All preferences stored in localStorage (`vidveil_prefs` key). No server-side storage.

**Appearance:**
- Theme: Auto (system), Dark (Dracula), Light - default: Auto
- Grid density: Comfortable (340px), Default, Compact (220px) - default: Default
- Thumbnail size: Small, Medium, Large - default: Medium

**Video Preview:**
- Auto-play preview on hover (toggle) - default: Yes
- Preview delay: Instant, 200ms, 500ms, 1000ms - default: Instant

**Search Settings:**
- Results per page: Infinite scroll, 20, 50, 100 - default: Infinite scroll
- Open links in new tab (toggle) - default: Yes

**Default Filters (auto-applied to new searches):**
- Show only videos with preview (toggle) - default: Yes
- Default duration filter: Any, Under 10min, 10-30min, Over 30min - default: Any
- Default quality filter: Any, 4K, 1080p HD, 720p - default: Any
- Default sort: Relevance, Longest, Shortest, Most Viewed, Best Quality - default: Relevance
- Minimum video duration: 0, 1, 3, 5, 10, 20, 30 minutes - default: 10 minutes

**History:**
- Maximum history items: Unlimited, 10, 25, 50, 100 - default: Unlimited
- Auto-clear history: Never, 1 day, 7 days, 30 days - default: Never
- Export/Import/Clear buttons

**Favorites:**
- Export/Import/Clear buttons
- Count display

**Privacy:**
- Use Tor for all searches (toggle) - default: No
- Proxy thumbnails through server (toggle) - default: Yes

**Search Engines:**
- Checkbox list of all 43 engines
- Select All / Select None buttons

### UI Components

**Toggle Switches (per AI.md PART 16):**
- CSS-only using hidden checkbox pattern
- Used for all boolean preferences
- Keyboard accessible (space/enter)
- Visual feedback: slider moves, color changes

**Notifications/Toasts:**
- Success (green), Error (red), Warning (orange), Info (blue)
- Auto-dismiss after 3 seconds
- Top-right corner position

**Loading States:**
- Large spinner during initial search
- Compact spinner for pagination
- "Connecting to engines..." → "Searching engines..." text updates
- Disabled button with spinner during submission

**Status Bar:**
- Fixed at bottom of page
- Shows connection status and engine count
- Real-time updates during search
- Final result count after completion

### Keyboard Shortcuts

- `/` = Focus search input
- `Escape` = Blur search input or close dropdowns
- Arrow Up/Down = Navigate autocomplete
- Enter/Tab = Select autocomplete item

### Responsive Design

- **Desktop (>1024px)**: Full layout, 4+ column grid
- **Tablet (≤768px)**: 2-column video grid
- **Mobile (≤600px)**: 1-column centered grid, stacked search
- **Extra Small (≤380px)**: Adjusted padding and font sizes
- Touch-friendly button sizes (44px minimum)
- Hamburger nav for mobile menu

### Accessibility (A11Y)

- WCAG 2.1 AA compliance target
- ARIA labels on all interactive elements
- Role attributes (listbox, list, menu, menuitem, etc.)
- aria-expanded for toggleable panels
- aria-live polite for status updates
- Skip to main content link
- Semantic HTML5 structure
- Keyboard navigation support
- Focus management

### SSE Streaming

**Primary Method:**
- Real-time result display from 43 engines in parallel
- Accept header: `text/event-stream`
- Tier 1 APIs appear first (fastest)
- Status updates show engine count
- Elapsed time display in milliseconds

**Fallback:**
- Automatic fallback to JSON API if SSE fails
- Accept header: `application/json`
- Batch loading of all results

### Error Handling

- Image fallback to placeholder.svg if missing
- JSON parse error handling with user feedback
- SSE fallback to JSON API on failure
- Network error handling with retry option
- Missing preference defaults applied
- Graceful UI state management

### localStorage Keys

- `vidveil-theme`: Current theme preference
- `vidveil_prefs`: Complete preferences object
- `vidveil_history`: Search history array
- `vidveil_favorites`: Favorites array

### Endpoints

**Search Endpoints:**
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

**Engine Endpoints:**
| Method | Purpose |
|--------|---------|
| GET | List all engines - see PART 14 |
| GET | Get engine details - see PART 14 |
| GET | List bang shortcuts - see PART 14 |

**Proxy Endpoints:**
| Method | Purpose |
|--------|---------|
| GET | Proxy thumbnail image - see PART 14 |

**Frontend Pages:**
| URL | Description |
|-----|-------------|
| `/` | Home page with search |
| `/search?q={query}` | Search results page |
| `/preferences` | User preferences |
| `/server/about` | About page |
| `/server/privacy` | Privacy policy |
| `/server/contact` | Contact page |
| `/server/help` | Help page |

**Admin Routes:**
- Admin panel follows PART 17: ADMIN PANEL hierarchy
- VidVeil admin manages: engines, rate limiting, Tor settings, GeoIP, backups
- No user management (VidVeil is stateless)

### Data Sources

**External Sources:**
- 43 video sites via HTML parsing
- Engine definitions embedded at build time
- Search results fetched in real-time

**Update Strategy:**
- No local storage of search results (stateless)
- Thumbnails proxied through server on-demand
- Engine definitions updated at build time

**Data Location:**
- `src/server/engine/engines.go`: Engine definitions and bang mappings

---

## Engine Registry

### All Registered Engines (43 total)

**Tier 1 - Major Sites (5 engines):**
| Engine | Bang | Capabilities |
|--------|------|--------------|
| pornhub | !ph | Preview (data-mediabook), Duration, Views, Rating, Quality |
| xvideos | !xv | Duration, Views, Quality |
| xnxx | !xx | Duration, Views |
| redtube | !rt | Preview (data-mediabook), Duration, Views, Rating, Quality |
| xhamster | !xh | Duration, Views, Rating |

**Tier 2 - Popular Sites (3 engines):**
| Engine | Bang | Capabilities |
|--------|------|--------------|
| eporner | !ep | Duration, Views, Rating |
| youporn | !yp | Preview (data-mediabook), Duration, Views, Rating, Quality |
| pornmd | !pm | Duration, Views |

**Tier 3-6 - Additional Sites (35 engines):**
4tube, fux, porntube, youjizz, sunporno, txxx, nuvid, tnaflix, drtuber, empflix, hellporno, alphaporno, pornflip, gotporn, xxxymovies, lovehomeporn, pornerbros, nonktube, nubilesporn, pornbox, porntop, pornotube, pornhd, xbabe, pornone, pornhat, porntrex, hqporner, vjav, flyflv, tube8, anyporn, tubegalore, motherless, 3movs

### Engine Configuration

**Default:** All 43 engines enabled

**SSE Streaming Behavior:**
- Results stream as each engine responds
- Tier 1 engines (faster APIs) appear first
- Tier 2-6 engines (HTML parsing) fill in below
- Invalid thumbnails auto-discarded

### Preview URL Sources

| Engine | Attribute | Content Type |
|--------|-----------|--------------|
| PornHub | `data-mediabook` | Video preview |
| XVideos | `data-preview` | Video preview |
| RedTube | `data-mediabook` | Video preview |
| YouPorn | `data-mediabook` | Video preview |

---

## Notes

- VidVeil has NO user accounts - privacy-first, stateless design
- All searches are real-time - no caching of search results
- Thumbnail proxy caches images temporarily to reduce repeated fetches
- Engine availability may vary - some sites may block or rate limit
- Tor support allows operation as hidden service for maximum privacy
- Admin panel is for server configuration only (per PART 17)
