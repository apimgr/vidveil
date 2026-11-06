# Configuration

Vidveil uses `server.yml` for runtime configuration.

## Config File Locations

| Platform | Path |
|---|---|
| Linux (root) | `/etc/apimgr/vidveil/server.yml` |
| Linux (user) | `~/.config/apimgr/vidveil/server.yml` |
| macOS (user) | `~/Library/Application Support/apimgr/vidveil/server.yml` |
| Windows | `%AppData%\apimgr\vidveil\server.yml` |
| Docker | `/config/vidveil/server.yml` |

Override the detected config root with `--config`.

## MCP and agent discovery

MCP and the generated `llms.txt` document are enabled by default. MCP routes
use the existing Bearer authentication rules; disabling the feature returns
404 from all MCP aliases. The discovery document includes endpoint metadata
from OpenAPI and omits metrics and operator-only actions.

```yaml
server:
  mcp:
    enabled: true
  pages:
    help:
      mcp: >-
        VidVeil searches across 33 video engines and streams results as they respond. Use searchVideos for a phrase, optionally with a bang such as !ph to target Pornhub; use listEngines to inspect engine availability, listBangs to discover shortcuts, and autocompleteBangs to find matching shortcuts or suggestions. For example: search for ocean sunsets with !ph, then compare the returned titles and source engines. Favorites are private owner resources; use a linked token for automation and never share a primary or operator token.
  tokens:
    max_linked: 25
    cookie_max_age: 1y
    idle_purge_days: 0
web:
  api_contact: api@example.org
  llms:
    enabled: true
    include_endpoints: true
    include_schemas: false
    custom_sections: []
```

MCP is available at `/api/{api_version}/server/mcp`, `/api/mcp`, and `/mcp`.
The discovery file is available at `/.well-known/llms.txt` and `/llms.txt`.
`web.api_contact` and `web.security.contact` populate the contact section.
`server.tokens` controls the active linked-token limit, sliding owner-cookie
lifetime, and optional idle-owner purge. When `idle_purge_days` is greater
than zero, the built-in daily scheduler removes tokens for owners with no
favorites and no token use during that period. Owners with favorites are
retained. The default value `0` disables purging.
`server.pages.help.mcp` is VidVeil-specific Markdown appended to the base guide
at `/server/help/mcp`; it defaults to the project guidance above and can be
edited in `server.yml`.

## Minimal Example

If `server.port` is omitted, Vidveil selects a random unused port in the `64xxx` range on first run and saves it to `server.yml`.

```yaml
server:
  address: "0.0.0.0"
  port: "64893"
  cors:
    allowed_origins:
      - "*"
    allow_credentials: true
    max_age: 86400
```

`server.cors.allowed_origins` accepts `"*"` (the default), one or more exact
HTTP(S) origins, or the single empty string (`[""]`) to disable CORS headers.
When the list is empty, origins from `DOMAIN` and trusted reverse-proxy hosts
are used; if none are available, the server falls back to `*`. Credentials are
enabled only for a specific origin, never with `*`. CORS preflight allows the
documented API methods and authentication headers, and defaults to a 24-hour
preflight cache.

Restore uploads use independent compressed and expanded archive limits. The
server validates advertised archive metadata before writing each entry and
enforces the same limits while streaming. The defaults follow AI.md PART 21:

```yaml
server:
  restore:
    max_archive_size: "1G"
    max_expanded_size: "4G"
    max_files: 10000
    max_file_size: "512M"
    max_ratio: 100
```

`max_archive_size`, `max_expanded_size`, and `max_file_size` accept bytes or
binary `K`, `KB`, `M`, `MB`, `G`, `GB`, `T`, or `TB` values. `max_files` and
`max_ratio` must be positive integers. Archives exceeding a limit are rejected
and partial extracted files are removed.

Set `server.update.cli_min_version` to require a minimum companion CLI version.
The value is returned by `/api/autodiscover`; an empty value disables the
minimum-version check.

The companion CLI updater checks `/api/autodiscover` and downloads platform
binaries through the server. By default downloads are public; private
deployments can require the configured operator token:

```yaml
cli:
  binary_download:
    require_auth: false
```

When `require_auth` is `true`, the download endpoint accepts the configured
`server.token` only as `Authorization: Bearer {token}`. Tokens in query
parameters or alternate headers are ignored.

## Branding and SEO Images

Branding images accept local file paths or remote HTTPS URLs. The server stores
generated variants in its data directory and refreshes configured sources on
startup and daily by default. Set `server.branding.refresh_interval` to a Go
duration such as `12h` to change the internal scheduler interval.

```yaml
server:
  branding:
    logo: /etc/apimgr/vidveil/logo.png
    favicon: https://assets.example.org/favicon.png
    refresh_interval: 24h
  seo:
    og_image: https://assets.example.org/social-card.jpg
    sitemap:
      enabled: true
      max_urls: 50000
      include_images: false
```

Remote sources must use HTTPS, resolve only to public IP addresses, and return
PNG, JPEG, GIF, WebP, or icon content under 10 MiB. Generated PNG variants
include 200px and 50px logos, favicons at 16/32/48/180/192/512px, and a
1200×630 OpenGraph image. A failed refresh retains the last cached image;
without a cached custom image, templates use the embedded default asset.

The generated `/sitemap.xml` includes public pages and the Swagger/GraphQL
documentation. It omits visitor-specific preferences and favorites pages.
`server.seo.sitemap.max_urls` is limited to the sitemap protocol maximum of
50,000 URLs, and `include_images` adds the configured OpenGraph image (or the
embedded default icon) to each listed page.

## Runtime Data Paths

- Docker config root: `/config/`
- Docker data root: `/data/`
- Docker logs: `/data/log/vidveil/server.log`
- Vidveil config file in Docker: `/config/vidveil/server.yml`
- Vidveil data root in Docker: `/data/vidveil/`

## Tor

Vidveil auto-enables the built-in Tor hidden service when a compatible `tor` binary is available. The server manages its own Tor data under the Vidveil data directory.

## I2P

The I2P eepsite is opt-in and stays disabled unless `server.i2p.enabled`,
`I2P_ENABLED`, or `--i2p` enables it. The server prefers a dedicated `i2pd`
process and falls back to a reachable SAM bridge. Its destination key is stored
under the data directory and the backend listener is loopback-only.

```yaml
server:
  i2p:
    enabled: false
    binary: ""
    sam_address: "127.0.0.1:7656"
    virtual_port: 80
    inbound_length: 3
    outbound_length: 3
    inbound_quantity: 5
    outbound_quantity: 5
    signature_type: 7
    bootstrap_timeout: 5m
```

## Environment Variables

| Variable | Description |
|---|---|
| `MODE` | Application mode |
| `CONFIG_DIR` | Override config root |
| `DATA_DIR` | Override data root |
| `LOG_DIR` | Override log root |
| `LISTEN` | Override listen address (wins over `ADDRESS`) |
| `ADDRESS` | Fallback listen address, used only when `LISTEN` is unset |
| `PORT` | Initial listen port (default: random `64xxx`, `80` in containers) |
| `CACHE_DIR` | Override cache directory (default: `/var/cache/apimgr/vidveil` when root, else `~/.cache/apimgr/vidveil`) |
| `BACKUP_DIR` | Override backup directory (default: `/mnt/Backups/apimgr/vidveil` when root, else `~/.local/share/Backups/apimgr/vidveil`) |
| `DATABASE_DIR` | Override SQLite database directory (default: `<data-dir>/db`); always overrides `server.database.sqlite.dir` in the config file |
| `SSL_DIR` | Override TLS certificate directory (default: `/etc/apimgr/vidveil/ssl` when root, else `~/.config/apimgr/vidveil/ssl`) |
| `SECURITY_DIR` | Override secret/key material directory (default: `/var/lib/apimgr/vidveil/security` when root, else `~/.local/share/apimgr/vidveil/security`) |
| `PID_FILE` | Override the pid file location (default: `/var/run/apimgr/vidveil.pid` when root, else `~/.local/share/apimgr/vidveil/vidveil.pid`) |
| `VIDVEIL_PORT` | Listen port; wins over `PORT` and the config file (port chain: `--port` > `VIDVEIL_PORT` > `PORT` > config > random) |
| `BASEURL` | Serve under a URL path prefix (e.g. `/app`); ignored when unset or `/` |
| `DOMAIN` | Force the server FQDN used to build absolute URLs (highest priority) |
| `HOSTNAME` | Fallback FQDN when `DOMAIN` is unset; also the node id for distributed cache locks |
| `TZ` | Time zone for scheduler timestamps (any IANA zone, e.g. `America/New_York`) |
| `CACHE_URL` | External cache connection string; sets `server.cache.url` and promotes the cache type from `memory` to `valkey` so it is actually used. Ignored when `VIDVEIL_SERVER_CACHE_URL` or `VIDVEIL_CACHE_URL` is set |
| `I2P_ENABLED` | Opt in to the I2P eepsite; project-prefixed `VIDVEIL_SERVER_I2P_ENABLED` and `VIDVEIL_I2P_ENABLED` take precedence |

### CLI environment variables

Read by the CLI rather than the server configuration tree:

| Variable | Description |
|---|---|
| `VIDVEIL_LANG` | Language tag sent as the `Accept-Language` request header (set by the `--lang` flag). Invalid or empty values send no header, letting the server use its default locale |

### Config-file overrides

Any key in `server.yml` can be overridden with an environment variable named
`VIDVEIL_{SECTION}_{KEY}` (uppercase, `_`-joined path). Keys under the `server`
section also accept the shorter `VIDVEIL_{KEY}` form. Examples:

| Variable | Overrides |
|---|---|
| `VIDVEIL_DATABASE_TYPE` | `database.type` |
| `VIDVEIL_SESSION_SECRET` | `session.sessionSecret` |
| `VIDVEIL_SMTP_HOST` | `smtp.host` |
| `VIDVEIL_SERVER_CACHE_TYPE` | `server.cache.type` |
| `VIDVEIL_SERVER_CACHE_URL` | `server.cache.url` |

Environment overrides win over the config file but lose to explicit CLI flags.

## Complete `server.yml` defaults

The configuration tree below lists every serialized setting and its current
default. First-run generated values are shown as placeholders: the server
creates credentials and detects host-specific values at runtime. Leave
`server.token` and `server.security.encryption_key` as generated in the live
configuration; never copy a placeholder value into a deployed server.

```yaml
server:
  port: <random unused 64xxx>
  fqdn: <detected at runtime>
  address: '[::]'
  baseurl: /
  api_version: v1
  mode: production
  mcp:
    enabled: true
  pages:
    help:
      mcp: 'VidVeil searches across 33 video engines and streams results as they respond. Use `searchVideos` for a phrase, optionally with a bang such as `!ph` to target Pornhub; use `listEngines` to inspect engine availability, `listBangs` to discover shortcuts, and `autocompleteBangs` to find matching shortcuts or suggestions. For example: search for ocean sunsets with `!ph`, then compare the returned titles and source engines. Favorites are private owner resources; use a linked token for automation and never share a primary or operator token.'
  tokens:
    max_linked: 25
    cookie_max_age: 1y
    idle_purge_days: 0
  token: <generated on first run>
  branding:
    title: Vidveil
    tagline: Privacy-first video search
    description: Privacy-respecting adult video search
    logo: ""
    favicon: ""
    refresh_interval: 24h
  user: ""
  group: ""
  pidfile: true
  contact:
    admin:
      email: admin@<detected host>
    security:
      email: security@<detected host>
    abuse:
      email: ""
    general:
      email: ""
  notifications:
    email:
      smtp:
        host: ""
        port: 587
        username: ""
        password: ""
        tls: auto
      from:
        name: ""
        email: no-reply@<detected host>
  schedule:
    timezone: America/New_York
    catch_up_window: 1h
    tasks:
      backup_daily:
        schedule: 0 2 * * *
        enabled: true
        verify: true
        retention:
          max_backups: 1
          keep_weekly: 0
          keep_monthly: 0
          keep_yearly: 0
          max_total_size: 10%
      backup_hourly:
        schedule: '@hourly'
        enabled: false
      blocklist_update:
        schedule: 0 4 * * *
        enabled: true
        retry_on_fail: true
        retry_delay: 1h
      cve_update:
        schedule: 0 5 * * *
        enabled: true
        retry_on_fail: true
        retry_delay: 1h
      geoip_update:
        schedule: 0 3 * * 0
        enabled: true
        retry_on_fail: true
        retry_delay: 1h
      healthcheck_self:
        schedule: '@every 5m'
        enabled: true
      log_rotation:
        schedule: 0 0 * * *
        enabled: true
      ssl_renewal:
        schedule: 0 3 * * *
        enabled: true
      token_cleanup:
        schedule: '@every 15m'
        enabled: true
      tor_health:
        schedule: '@every 10m'
        enabled: true
        restart_on_fail: true
      update_check:
        schedule: 0 6 * * *
        enabled: true
  ssl:
    enabled: false
    cert_path: ""
    letsencrypt:
      enabled: false
      domain: ""
      email: ""
      challenge: http-01
      dns_provider_type: ""
      dns_provider_key: ""
  metrics:
    enabled: false
    root:
      enabled: true
    auth:
      allow_unauthenticated: false
      tokens:
        prometheus: ""
        grafana: ""
        loki: ""
    include_system: true
    include_runtime: true
    loki:
      max_entries: 1000
      max_age: 1h
    duration_buckets:
      - 0.001
      - 0.005
      - 0.01
      - 0.025
      - 0.05
      - 0.1
      - 0.25
      - 0.5
      - 1
      - 2.5
      - 5
      - 10
    size_buckets:
      - 100
      - 1000
      - 10000
      - 100000
      - 1e+06
      - 1e+07
  logs:
    level: info
    debug:
      enabled: false
      filename: debug.log
      format: text
      keep: none
      rotate: monthly
    access:
      enabled: false
      filename: access.log
      format: apache
      keep: none
      rotate: monthly
    server:
      enabled: false
      filename: server.log
      format: text
      keep: none
      rotate: weekly,50MB
    error:
      enabled: false
      filename: error.log
      format: text
      keep: none
      rotate: weekly,50MB
    audit:
      enabled: true
      filename: audit.log
      format: json
      keep: none
      rotate: daily
      compress: false
      events:
        configuration: true
        security: true
        backup: true
        server: true
      include_user_agent: true
    security:
      enabled: true
      filename: security.log
      format: fail2ban
      keep: none
      rotate: weekly,50MB
    auth:
      enabled: true
      filename: auth.log
      format: syslog
      keep: none
      rotate: weekly,50MB
    app:
      enabled: true
      filename: vidveil.log
      format: logfmt
      keep: none
      rotate: weekly,50MB
  rate_limit:
    enabled: true
    requests: 500
    window: 60
  limits:
    max_body_size: 10MB
    read_timeout: 30s
    write_timeout: 30s
    idle_timeout: 120s
  compression:
    enabled: true
    level: 5
    types:
      - text/html
      - text/css
      - text/javascript
      - application/json
      - application/xml
  trusted_proxies:
    additional: []
  security_headers:
    enabled: true
    hsts: true
    hsts_max_age: 31536000
    x_frame_options: SAMEORIGIN
    x_content_type_options: nosniff
    x_xss_protection: 1; mode=block
    referrer_policy: strict-origin-when-cross-origin
  cors:
    allowed_origins:
      - '*'
    allow_credentials: true
    max_age: 86400
  session:
    cookie_name: session_id
    max_age: 2592000
    secure: auto
    http_only: true
    same_site: strict
  database:
    driver: file
    sqlite:
      dir: ""
      server_db: server.db
      journal_mode: WAL
      busy_timeout: 5000
    url: ""
    token: ""
  cache:
    type: memory
    url: ""
    host: localhost
    port: 6379
    username: ""
    password: ""
    db: 0
    tls: false
    tls_skip_verify: false
    pool_size: 10
    min_idle: 2
    timeout: 5
    prefix: 'vidveil:'
    ttl: 30
  geoip:
    enabled: true
    dir: ""
    update: weekly
    country_mode: none
    deny_countries: []
    allow_countries: []
    presets: {}
    databases:
      asn: true
      country: true
      city: true
      whois: true
    content_restriction:
      mode: warn
      restricted_countries: []
      restricted_regions:
        - US:Texas
        - US:Utah
        - US:Louisiana
        - US:Arkansas
        - US:Montana
        - US:Mississippi
        - US:Virginia
        - US:North Carolina
      bypass_tor: true
      warning_message: Adult content may be restricted or require age verification in your region.
  security:
    dir: ""
    allowlist: []
    blocklists:
      enabled: false
      sources: []
    cve:
      enabled: false
      source: ""
      filter_by_cpe: false
    encryption_key: <generated on first run>
  backup:
    retention:
      max_backups: 1
      keep_weekly: 0
      keep_monthly: 0
      keep_yearly: 0
      max_total_size: 10%
    encryption:
      enabled: false
  restore:
    max_archive_size: 1G
    max_expanded_size: 4G
    max_files: 10000
    max_file_size: 512M
    max_ratio: 100
  compliance:
    gdpr: false
    ccpa: false
    hipaa: false
    soc2: false
    pci_dss: false
    iso27001: false
    fedramp: false
    lgpd: false
    pipeda: false
    appi: false
    pdpa: false
  tor:
    binary: ""
    use_network: false
    allow_user_preference: true
    allow_user_ip_forward: true
    max_circuits: 32
    circuit_timeout: 60
    bootstrap_timeout: 180
    safe_logging: true
    max_streams_per_circuit: 100
    close_circuit_on_stream_limit: true
    bandwidth_rate: 1 MB
    bandwidth_burst: 2 MB
    max_monthly_bandwidth: 100 GB
    num_intro_points: 3
    virtual_port: 80
    onion_address: ""
    contact_email: ""
  i2p:
    enabled: false
    binary: ""
    sam_address: 127.0.0.1:7656
    virtual_port: 80
    inbound_length: 3
    outbound_length: 3
    inbound_quantity: 5
    outbound_quantity: 5
    signature_type: 7
    bootstrap_timeout: 5m
  healthz:
    root:
      enabled: false
  seo:
    keywords: []
    author: ""
    og_image: ""
    twitter_handle: ""
    verification:
      google: ""
      bing: ""
      yandex: ""
      baidu: ""
      pinterest: ""
      facebook: ""
      custom: []
    sitemap:
      enabled: true
      max_urls: 50000
      include_images: false
  update:
    branch: stable
    auto_install: false
    defer_days: 0
    cli_min_version: ""
  privacy:
    consent:
      message: This site uses cookies for age verification and to improve your experience.
      message_if_sold: ""
      policy:
        url: /server/privacy
        text: Privacy Policy
      buttons:
        decline: Decline
        accept: Accept
      preferences_text: Cookie Preferences
    data:
      sold: false
      stored_on_server: true
      sharing: []
    retention:
      period: Not retained — preferences live in your browser only
      export_available: false
      deletion_available: true
    cookies:
      essential:
        enabled: true
        description: 'Required for the site to function: age acknowledgement, cookie consent state and CSRF protection.'
      preferences:
        enabled: true
        description: Remember your theme, language and search settings between visits.
      analytics:
        enabled: false
        description: Measure aggregate usage of the site.
        description_suffix_not_sold: This data is anonymized and never sold.
        description_suffix_sold: This data may be shared with third parties.
    third_party:
      services: []
    content:
      data_collection: ""
      data_usage: ""
      data_usage_if_sold: ""
      data_security: ""
cli:
  binary_download:
    require_auth: false
web:
  ui:
    theme: dark
  announcements:
    enabled: true
    messages: []
  robots:
    allow:
      - /
    deny:
      - /search
      - /api/
    ai_bots:
      default: allow
      bots: {}
  security:
    contact: security@1b3383a9067e
    expires: ""
    pgp_key_url: ""
    keyservers: []
    publish_pgp_key: false
    report_url: ""
    disclosure_window_days: 90
    policy_text: ""
  csp:
    enabled: true
    mode: enforce
    script_src_extra: ""
    style_src_extra: ""
    img_src_extra: ""
    font_src_extra: ""
    connect_src_extra: ""
    frame_src_extra: ""
    form_action_extra: ""
    script_src_override: ""
    style_src_override: ""
    reports_enabled: true
    reports_sample_rate: 1
  csrf:
    enabled: true
    token_length: 32
    cookie_name: csrf_token
    header_name: X-CSRF-Token
    secure: auto
    exempt_paths:
      - /api/v1/webhooks/*
  footer:
    custom_html: ""
  honor_dnt: false
  llms:
    enabled: true
    include_endpoints: true
    include_schemas: false
    custom_sections: []
  api_contact: api@1b3383a9067e
search:
  default_engines: []
  concurrent_requests: 10
  engine_timeout: 15
  results_per_page: 50
  max_pages: 10
  min_duration_seconds: 600
  min_relevance_score: 10
  filter_premium: true
  spoof_tls: false
  age_verification:
    enabled: true
    cookie_days: 30
  custom_terms: []
  ai_filter:
    enabled: true
    allow_user_override: true
    keywords:
      - ai generated
      - ai-generated
      - ai porn
      - ai-porn
      - deepfake
      - deep fake
      - deep-fake
      - ai model
      - ai celebrity
      - ai fake
      - generated porn
      - synthetic
      - artificially generated
      - neural network
      - machine learning porn
      - fake celebrity
      - celebrity deepfake
  engine_timeouts: {}
  engine_request_interval: 0
  engine_request_intervals: {}
  thumbnail_cache_ttl: 1440
  detail_enrichment:
    enabled: false
    max_results: 5
    timeout: 2
engines:
  useragent:
    os: windows
    version: "11"
    browser: chrome
    browser_version: "131"
```
