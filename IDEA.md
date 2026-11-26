## Project description

VidVeil is a privacy-respecting meta search engine for adult video content. It searches across multiple video sites and presents results from the selected sources in one place. It is for privacy-conscious visitors, self-hosters, developers integrating video search, and people who need access through Tor.

VidVeil minimizes retained search data while being clear that service access logs can contain request metadata. It does not have conventional user accounts. Anonymous owner tokens let visitors keep private resources such as favorites without creating named profiles.

## Project variables

project_name:      vidveil
project_org:       apimgr
internal_name:     vidveil
internal_org:      apimgr
app_name:          vidveil
official_site:     https://x.scour.li
coverage_minimum:  60
owner_token:       vidveil_owner_token_ab5213f6

## Business logic

### Product scope & non-goals

**Target users:**
- Privacy-conscious visitors searching adult video content.
- Self-hosters who want to operate a private search service.
- Developers who need a unified video search service.
- Visitors who access the service through Tor or I2P.

**Features:**
- **Video search:** Search across 27 registered video engines, target available engines with bang shortcuts, and receive results as engines respond. Bang shortcuts reflect current engine availability; disabled or unavailable engines have no usable bang. A failing or rate-limited source must not prevent results from other sources. Search responses use canonical engine names and display names; blank engine selection means all available engines. The `hellporno`, `nonktube`, `pornbox`, `porntube`, `txxx`, and `vjav` sources are unregistered because they currently return access-blocked, cookie-gated, or unrendered pages. Retest these sources before registering them again.
- **Result discovery:** Refine searches with autocomplete, related searches, source selection, duration and quality filters, sorting, and pagination. Results must match the query's explicit semantic intent; female-only searches such as `lesbian` exclude results identified as including male participants. The server applies preferences, filtering, sorting, and pagination, and reports the aggregate count collected from responding sources rather than only the displayed page. Upstream sites do not consistently expose all-pages hit counts, so this count covers the results returned for the requested upstream page. When a playable video preview is unavailable, use the result thumbnail as the visual preview. The visitor's search-history list is stored in browser `localStorage`, not as a server-side per-visitor history; privacy-filtered recent queries may separately appear in public Atom/RSS feeds. Recent-search entries differing only by case or whitespace are deduplicated.
- **Privacy:** Proxy thumbnails through VidVeil, provide optional analytics only with visitor consent, and disclose that access logs may contain request metadata. Public Atom and RSS feeds may include only privacy-filtered query text and timestamps; entries are bounded and are not retained across server restarts.
- **Private favorites:** Visitors can save, organize, export, import, and clear favorites. REST imports accept the exported array or an object containing a `favorites` array. Favorites belong to an anonymous owner, are unique by result URL for that owner, and remain private to that owner. A visitor can manage their own favorites without creating a conventional account.
- **Owner token management:** Owners can restore access to their resources and manage their own primary and linked tokens. A linked token can be used for automation without sharing the primary token.
- **Preferences:** Visitors can choose appearance, language, search behavior, and result preferences. Preferences are visitor-scoped and do not require an account. The interface supports English, Arabic, German, Spanish, French, Italian, Japanese, and Chinese.
- **Content safeguards:** Operators can configure geographic content restrictions. AI-generated content is filtered by default, with a visitor preference to show it.
- **Service information:** Visitors can read public information, privacy, help, terms, and security reporting content. Operators can extend the built-in About and Privacy pages, replace Terms and footer branding, and add a localized header fragment; this content is sanitized before display. Search engine metadata and canonical links follow operator SEO settings, and private instances can disable indexing. The report form accepts security reports, terms violations, DMCA notices, and privacy requests; it is not a general support or feedback channel. The service provides REST, GraphQL, Swagger/OpenAPI, and MCP interfaces for supported search and discovery functions.
- **Contact and security reports:** Contact messages use server-side shape, signed form-time, CAPTCHA, rate, content, and duplicate checks before delivery. Security reports use the shared shape, form-time, CAPTCHA, and rate checks before report creation. Tor and I2P requests use the built-in challenge and do not share a tunnel-wide per-IP submission limit.
- **Operator control:** Operators configure the service through node settings, area-specific configuration files, a separate secrets file, and command-line tools. Built-in scheduled tasks retain run state across restarts and can be configured without a browser-based administration product. Database and low-disk alerts are deduplicated, recover automatically, and use email only when SMTP is available. Backups use the configured secret encryption password, scheduled daily incrementals are cumulative from a full archive, and the maintenance CLI can verify an archive before restore. There is no browser-based operator administration product.
- **Privacy network access:** Tor hidden-service access starts automatically when Tor is installed; outbound Tor routing follows the operator’s server-wide `server.tor.use_network` setting. I2P eepsite hosting is enabled by default in container images and remains opt-in on bare-metal and VM installs. Visitors cannot override either operator setting.

**Guest preferences:** These preferences are cookie-only, read per request, and are never stored server-side. Preference export/import includes every valid `vidveil_pref_*` cookie listed here.

| Cookie | Allowed values | Default | Export/import |
|--------|----------------|---------|---------------|
| `vidveil_pref_grid_density` | `comfortable`, `default`, `compact` | `default` | Included |
| `vidveil_pref_thumbnail_size` | `small`, `medium`, `large` | `medium` | Included |
| `vidveil_pref_autoplay_preview` | `0`, `1` | `1` | Included |
| `vidveil_pref_preview_delay` | `0`, `200`, `500`, `1000` milliseconds | `0` | Included |
| `vidveil_pref_results_per_page` | `0`, `20`, `50`, `100`; `0` means infinite scroll | `0` | Included |
| `vidveil_pref_open_new_tab` | `0`, `1` | `1` | Included |
| `vidveil_pref_preview_first` | `0`, `1` | `1` | Included |
| `vidveil_pref_show_ai_content` | `0`, `1` | `0` | Included |
| `vidveil_pref_default_duration` | `any`, `short`, `medium`, `long` | `any` | Included |
| `vidveil_pref_min_quality` | `0`, `240`, `360`, `480`, `720`, `1080`, `2160` | `360` | Included |
| `vidveil_pref_default_sort` | `any`, `duration-desc`, `duration-asc`, `views`, `quality` | `any` | Included |
| `vidveil_pref_min_duration` | `0`, `60`, `180`, `300`, `600`, `1200`, `1800` seconds | `600` | Included |
| `vidveil_pref_max_history` | `0`, `10`, `25`, `50`, `100`; `0` means unlimited | `0` | Included |
| `vidveil_pref_auto_clear_history` | `0`, `1`, `7`, `30` days; `0` means never | `0` | Included |
| `vidveil_pref_engines_selected` | `1` means an explicit selection is active | Unset; all enabled engines | Included |
| `vidveil_pref_engines` | `none` or a compact `b`-prefixed hexadecimal registry fingerprint and engine-selection bitset (at most 64 characters) | Unset; all enabled engines | Included |

Older per-engine cookies named `vidveil_pref_engine_{name}` accept `0` or `1` and remain readable/importable for compatibility. Older dot-separated engine lists remain readable when they fit the 64-character preference-value limit. Current preference writes use the compact bitset in the engine-selection cookies above; engine names are ordered alphabetically when encoding and decoding it, and a registry fingerprint prevents a changed engine set from silently changing saved selections.

**Non-goals:**
- Named user accounts, social profiles, organizations, or multi-tenant workspaces.
- Server-side per-visitor search history or persistent storage of search results. The search-history convenience is stored in the visitor's browser. Short-lived result caching and privacy-filtered public recent-search feeds are separate operational behaviors defined by AI.md.
- Paid plans, premium features, or advertising-based targeting.
- A guarantee that any third-party video site remains available or returns complete results.

### Roles & permissions

| Role | Product permissions |
|------|---------------------|
| Anonymous visitor | Search and browse public results; manage only favorites and linked tokens belonging to their anonymous owner; change their own preferences. |
| Operator | Configure and administer the service through operating-system access and the supported command-line interface. |

There are no conventional user accounts. Possession of an owner's valid token identifies that anonymous owner for private resources. Public search does not require an owner token.

### Data model & sensitivity

- **Video result:** A result has an engine-specific identity, title, source link, thumbnail, source name, and available descriptive metadata such as duration, views, quality, publication date, tags, performer, preview, or download information.
- **Engine:** A searchable source has a name, display name, availability, and supported search or media capabilities. The engines API reports `has_preview`, `has_download`, `has_duration`, and `has_views` capability flags. VidVeil registers 27 engines. Individual sources can become unavailable.
- **Search:** A visitor's search-history list is stored in that visitor's browser, not on the server as per-visitor history. Search terms that pass the required privacy filters may appear as query text and timestamps in bounded public Atom and RSS feeds. Queries and results are otherwise transient service data.
- **Favorite:** A visitor-owned saved result can include its source link, title, thumbnail, source, and creation time. Favorites are private to their anonymous owner and persist until that owner removes them or the service data is removed.
- **Owner token:** A credential grants access to one anonymous owner's private resources. Primary and linked tokens have distinct management roles; tokens are sensitive and must not be exposed in public output.
- **System ownership:** VidVeil currently has no system-owned product resources: search results are transient and favorites belong to anonymous visitor owners. The database reserves the `system` owner for future server-managed resources; it has no owner token and is excluded from idle-owner deletion.
- **Visitor preference:** Appearance, language, and search preferences belong to the visitor's browser and are not tied to a named account.
- **Operational and compliance information:** Service configuration, security events, backups, certificates, and network identity data may be sensitive. Their handling and retention follow AI.md.

### Trust boundaries & external services

| Boundary | Product assumption | Expected failure behavior |
|----------|--------------------|---------------------------|
| Video search engines | Third-party sites and their responses are untrusted and may change or be unavailable. | Report unavailable sources while continuing with usable results from other engines. |
| Thumbnail and preview sources | External media may be unsafe, malformed, or privacy-invasive. | Serve media through the service's privacy protections; omit unusable media. |
| Tor | Tor may be available on the local machine or deployment, but is not guaranteed. | Preserve ordinary service access when Tor is unavailable. |
| GeoIP and public blocklist data | GeoIP uses the iptoasn ASN/country and DB-IP city datasets distributed by sapics/ip-location-db; data can be stale or temporarily unreachable. | Verify downloaded database checksums, retain the last usable data, and fail open on lookup errors without treating location data as authoritative identity. |
| Incoming visitors and clients | Requests may be malformed or malicious. | Validate requests and return safe errors without exposing private data or internals. |

### Browser permissions

VidVeil does not use browser camera, microphone, geolocation, sensors, USB, MIDI, display capture, or other locked browser capabilities. Server-side GeoIP lookup does not request browser geolocation.

```yaml
uses_accelerometer: false
uses_ambient_light_sensor: false
uses_battery: false
uses_camera: false
uses_display_capture: false
uses_geolocation: false
uses_gyroscope: false
uses_hid: false
uses_idle_detection: false
uses_magnetometer: false
uses_microphone: false
uses_midi: false
uses_screen_wake_lock: false
uses_serial: false
uses_usb: false
uses_xr_spatial_tracking: false
```

### Threat model & abuse cases

**Primary assets:** visitor privacy; availability of the search service; integrity of operator configuration; and the confidentiality of favorites and owner tokens.

**Untrusted inputs:** search queries, filters, client-provided preferences, external engine responses, external media, and incoming HTTP requests.

The service must limit the risk of correlating visitors with searches, leaking visitor IP addresses to thumbnail providers, injecting hostile engine content into pages, using media handling as a route to internal services, abusing search volume to exhaust resources, bypassing configured geographic restrictions, and taking over private resources through stolen or mishandled tokens. Access logs may contain request metadata, including query strings depending on operator configuration, and this must be disclosed accurately.

### Security decisions & exceptions

| Decision | Product reason |
|----------|-----------------|
| No conventional user accounts or public profiles | Named accounts would create an unnecessary identity layer for a privacy-focused search service. |
| Anonymous owner tokens protect private favorites and token management | Visitors need continuity for private resources without registering a named account. |
| Search history is stored in the visitor's browser, not as server-side per-visitor history | Persistent per-visitor history would increase the amount of sensitive visitor data held by the service. |
| Recent-search feeds include only privacy-filtered queries and are bounded | A public feed can support discovery without exposing queries that fail the required privacy checks or retaining the feed across restarts. |
| Thumbnails are served through VidVeil | Direct requests to third-party thumbnail hosts would expose visitor network information. |
| AI-generated content is filtered by default | Visitors should not be shown such results unless they choose to enable them. |
| Tor visitors are not geographically restricted by default | Their location cannot be reliably determined from the service's view; operators may configure a different policy where required. |
| Search engines are configured for the service as a whole | Visitors may choose among the available engines, while operators control which sources the service contacts. |

### API routes

- `GET /api/{api_version}/search`: No authentication; searches the available video engines using the query, source, filter, sort, and pagination parameters; returns matching results, pagination data, and source status.
- `POST /api/{api_version}/search/batch`: No authentication; accepts a batch of search requests and returns the corresponding search results and statuses.
- `GET /api/{api_version}/bangs`: No authentication; returns available bang shortcuts.
- `GET /api/{api_version}/bangs/autocomplete`: No authentication; accepts a partial bang query and returns matching shortcuts.
- `GET /api/{api_version}/engines`: No authentication; returns registered engines and their search/media capability information.
- `GET /api/{api_version}/engines/health`: No authentication; returns engine availability and health information.
- `GET /api/{api_version}/engines/{name}`: No authentication; returns information and capabilities for the named engine.
- `GET /api/{api_version}/stats`: No authentication; returns public aggregate service/search statistics.
- `GET /api/{api_version}/version`: No authentication; returns the running service version information.
- `GET /api/{api_version}/proxy/thumbnails`: No authentication; accepts a validated upstream thumbnail URL and returns the proxied image bytes.
- `GET /api/{api_version}/proxy/videos`: No authentication; accepts a validated supported preview URL and returns proxied media when available.
- `GET /api/{api_version}/favorites`: Bearer owner token required; returns the authenticated owner's private favorites.
- `POST /api/{api_version}/favorites`: Bearer owner token when an owner already exists; accepts a favorite and returns the saved favorite, issuing a primary owner token once when creating a new anonymous owner.
- `DELETE /api/{api_version}/favorites/{id}`: Bearer owner token required; removes the identified favorite belonging to that owner.
- `DELETE /api/{api_version}/favorites`: Bearer owner token required; clears the authenticated owner's favorites.
- `POST /api/{api_version}/favorites/import`: Bearer owner token required; accepts an exported favorites array or an object containing a `favorites` array and imports valid entries for that owner.
- `GET /api/{api_version}/status`: Operator Bearer token required; returns maintenance status, operator diagnostics, and recovery instructions.
- `POST /api/{api_version}/server/reports/default`: No authentication; accepts browser default-error reports with per-IP rate limiting.
- `POST /api/{api_version}/server/reports/nel`: No authentication; accepts Network Error Logging reports with per-IP rate limiting.
- `POST /api/{api_version}/server/reports/csp`: No authentication; accepts Content Security Policy violation reports with per-IP rate limiting.
