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
- Visitors who access the service through Tor.

**Features:**
- **Video search:** Search across 33 video engines, target engines with bang shortcuts, and receive results as engines respond. Engine availability is best-effort; a failing or rate-limited source must not prevent results from other sources. Search responses use canonical engine names and display names; blank engine selection means all available engines.
- **Result discovery:** Refine searches with autocomplete, related searches, source selection, duration and quality filters, sorting, and pagination. The server applies preferences, filtering, sorting, and pagination, and reports the aggregate count collected from responding sources rather than only the displayed page. Upstream sites do not consistently expose all-pages hit counts, so this count covers the results returned for the requested upstream page. When a playable video preview is unavailable, use the result thumbnail as the visual preview. Search history is a visitor-controlled convenience and is not retained as a durable server-side history; recent-search entries differing only by case or whitespace are deduplicated.
- **Privacy:** Proxy thumbnails through VidVeil, provide optional analytics only with visitor consent, and disclose that access logs may contain request metadata. Public Atom and RSS feeds may include only privacy-filtered query text and timestamps; entries are bounded and are not retained across server restarts.
- **Private favorites:** Visitors can save, organize, export, import, and clear favorites. REST imports accept the exported array or an object containing a `favorites` array. Favorites belong to an anonymous owner, are unique by result URL for that owner, and remain private to that owner. A visitor can manage their own favorites without creating a conventional account.
- **Owner token management:** Owners can restore access to their resources and manage their own primary and linked tokens. A linked token can be used for automation without sharing the primary token.
- **Preferences:** Visitors can choose appearance, language, search behavior, and result preferences. Preferences are visitor-scoped and do not require an account. The interface supports English, Arabic, German, Spanish, French, Italian, Japanese, and Chinese.
- **Content safeguards:** Operators can configure geographic content restrictions. AI-generated content is filtered by default, with a visitor preference to show it.
- **Service information:** Visitors can read public information, privacy, contact, help, and security reporting content. The service provides REST, GraphQL, Swagger/OpenAPI, and MCP interfaces for supported search and discovery functions.
- **Operator control:** Operators configure the service through its configuration file and command-line tools. There is no browser-based operator administration product.
- **Privacy network access:** The service supports Tor access. Tor use for searches can be selected by a visitor where enabled.

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
| `vidveil_pref_use_tor` | `0`, `1` | `0` | Included |
| `vidveil_pref_engines_selected` | `1` means an explicit selection is active | Unset; all enabled engines | Included |
| `vidveil_pref_engines` | `none` or a compact `b`-prefixed hexadecimal registry fingerprint and engine-selection bitset (at most 64 characters) | Unset; all enabled engines | Included |

Older per-engine cookies named `vidveil_pref_engine_{name}` accept `0` or `1` and remain readable/importable for compatibility. Older dot-separated engine lists remain readable when they fit the 64-character preference-value limit. Current preference writes use the compact bitset in the engine-selection cookies above; engine names are ordered alphabetically when encoding and decoding it, and a registry fingerprint prevents a changed engine set from silently changing saved selections.

**Non-goals:**
- Named user accounts, social profiles, organizations, or multi-tenant workspaces.
- Durable server-side search history or persistent storage of search results. Short-lived result caching is an operational behavior defined by AI.md and does not create a search-history feature.
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
- **Engine:** A searchable source has a name, display name, availability, and supported search or media capabilities. VidVeil includes 33 active engines; individual sources can become unavailable.
- **Search:** A visitor's query and its results are transient service data. Search terms that pass the required privacy filters may appear as query text and timestamps in bounded public Atom and RSS feeds. The service does not provide durable server-side search history.
- **Favorite:** A visitor-owned saved result can include its source link, title, thumbnail, source, and creation time. Favorites are private to their anonymous owner and persist until that owner removes them or the service data is removed.
- **Owner token:** A credential grants access to one anonymous owner's private resources. Primary and linked tokens have distinct management roles; tokens are sensitive and must not be exposed in public output.
- **Visitor preference:** Appearance, language, and search preferences belong to the visitor's browser and are not tied to a named account.
- **Operational and compliance information:** Service configuration, security events, backups, certificates, and network identity data may be sensitive. Their handling and retention follow AI.md.

### Trust boundaries & external services

| Boundary | Product assumption | Expected failure behavior |
|----------|--------------------|---------------------------|
| Video search engines | Third-party sites and their responses are untrusted and may change or be unavailable. | Report unavailable sources while continuing with usable results from other engines. |
| Thumbnail and preview sources | External media may be unsafe, malformed, or privacy-invasive. | Serve media through the service's privacy protections; omit unusable media. |
| Tor | Tor may be available on the host or deployment, but is not guaranteed. | Preserve ordinary service access when Tor is unavailable. |
| GeoIP and public blocklist data | Public datasets can be stale or temporarily unreachable. | Continue using the last usable data and report service status without treating location data as authoritative identity. |
| Incoming visitors and clients | Requests may be malformed or malicious. | Validate requests and return safe errors without exposing private data or internals. |

### Threat model & abuse cases

**Primary assets:** visitor privacy; availability of the search service; integrity of operator configuration; and the confidentiality of favorites and owner tokens.

**Untrusted inputs:** search queries, filters, client-provided preferences, external engine responses, external media, and incoming HTTP requests.

The service must limit the risk of correlating visitors with searches, leaking visitor IP addresses to thumbnail providers, injecting hostile engine content into pages, using media handling as a route to internal services, abusing search volume to exhaust resources, bypassing configured geographic restrictions, and taking over private resources through stolen or mishandled tokens. Access logs may contain request metadata, including query strings depending on operator configuration, and this must be disclosed accurately.

### Security decisions & exceptions

| Decision | Product reason |
|----------|-----------------|
| No conventional user accounts or public profiles | Named accounts would create an unnecessary identity layer for a privacy-focused search service. |
| Anonymous owner tokens protect private favorites and token management | Visitors need continuity for private resources without registering a named account. |
| Search history is not retained durably by the service | Persistent search history would increase the amount of sensitive visitor data held by the service. |
| Recent-search feeds include only privacy-filtered queries and are bounded | A public feed can support discovery without exposing queries that fail the required privacy checks or retaining the feed across restarts. |
| Thumbnails are served through VidVeil | Direct requests to third-party thumbnail hosts would expose visitor network information. |
| AI-generated content is filtered by default | Visitors should not be shown such results unless they choose to enable them. |
| Tor visitors are not geographically restricted by default | Their location cannot be reliably determined from the service's view; operators may configure a different policy where required. |
| Search engines are configured for the service as a whole | Visitors may choose among the available engines, while operators control which sources the service contacts. |
