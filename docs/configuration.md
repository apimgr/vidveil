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
`server.token` through the documented token headers or `?token=` query value.

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
