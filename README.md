# Vidveil

[![CI](https://github.com/apimgr/vidveil/actions/workflows/ci.yml/badge.svg)](https://github.com/apimgr/vidveil/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/apimgr/vidveil)](https://github.com/apimgr/vidveil/releases)
[![License](https://img.shields.io/github/license/apimgr/vidveil)](LICENSE.md)
[![Docs](https://readthedocs.org/projects/vidveil/badge/?version=latest)](https://vidveil.readthedocs.io/)

## About

Privacy-respecting meta search for adult video content. The search-history list is stored in your browser, not as per-visitor history on the server. Privacy-filtered query text and timestamps may appear in public recent-search feeds. Configured access logs may include request paths, IP addresses, and other request metadata; custom formats can include query strings. Optional analytics run only when configured by the operator and after visitor consent. Aggregates results from 27 registered video engines by default, with bang shortcuts for targeted searches.

## Official Site

https://x.scour.li

**Repository**: https://github.com/apimgr/vidveil

---

## Features

| Feature | Description |
|---------|-------------|
| **Privacy First** | Search history stays in the visitor's browser; the server has no per-visitor history, though filtered queries may appear in public feeds; configured logs may include request metadata and custom formats can include query strings; optional analytics require operator configuration and visitor consent; the privacy page lists applicable export, correction, and deletion rights, and Tor renders external policy links as plain text |
| **27 Engines** | Aggregates results from 27 registered adult video sites by default |
| **Bang Search** | Use `!ph`, `!xh`, `!rt` to search specific sites |
| **Fast APIs** | Direct JSON API integration with PornHub, RedTube, Eporner |
| **SSE Streaming** | Real-time result streaming as engines respond |
| **Video Previews** | Hover (desktop) and swipe (mobile) preview for 8+ engines, with thumbnail fallback when video preview is unavailable |
| **Thumbnail Proxy** | All thumbnails proxied to prevent engine tracking |
| **Private Favorites** | Favorites belong to an anonymous owner and can be restored on another browser with the primary token |
| **Preference Sync** | Visitor preferences stay in browser cookies and can be exported or imported with a shareable URL or code |
| **Autocomplete** | Bang shortcuts, performer names, and search terms autocomplete |
| **Related Searches** | Server-generated related search suggestions |
| **Semantic Search** | AND-based filtering with synonym expansion across 27 categories |
| **Overlay Access** | Tor hidden service support; I2P eepsite enabled in container images and opt-in on bare-metal and VMs |
| **8 Languages** | English, Arabic, German, Spanish, French, Italian, Japanese, and Chinese |
| **AI Integration** | OpenAPI-generated MCP tools and configurable `llms.txt` discovery |
| **Content Restriction** | Geographic warnings/blocks for regions with adult content laws |
| **Operator Management** | Configure through `server.yml`, area files, `secrets.yml`, and the CLI; create, verify, and restore encrypted or unencrypted backups with the maintenance CLI ([backup guide](docs/admin-guide/backup.md)) |
| **Single Binary** | No external dependencies, embedded assets |
| **Docker Ready** | Alpine-based container with tini |

---

## Production

### Docker (Recommended)

```bash
CONTAINER_NAME="vidveil-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)"
docker run -d \
  --rm \
  --name "$CONTAINER_NAME" \
  -p 172.17.0.1:64580:80 \
  -v ./volumes/config:/config:z \
  -v ./volumes/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

### Docker Compose

```bash
curl -q -LSsfO https://raw.githubusercontent.com/apimgr/vidveil/main/docker/docker-compose.yml
docker compose up -d
```

### Binary Download

Download the latest binary for your platform from [Releases](https://github.com/apimgr/vidveil/releases).
Release artifacts cover Linux, macOS, Windows, and FreeBSD on amd64 and arm64;
see the [complete server and CLI artifact names](docs/installation.md#release-artifacts).

```bash
# Linux (amd64)
curl -q -LSsfO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

### systemd

Install the downloaded Linux binary as a system service:

```bash
sudo install -m 0755 vidveil-linux-amd64 /usr/local/bin/vidveil
sudo /usr/local/bin/vidveil --service --install
```

The service manager starts VidVeil after installation. Its configuration and
data paths are created on first run; see the [installation guide](docs/installation.md#service-installation).

---

## Client

A companion CLI client (`vidveil-cli`) is available for interacting with the server API from the terminal.
The same eight OS/architecture combinations are published; see the [artifact table](docs/installation.md#release-artifacts).

### Install

```bash
# Download latest release
curl -q -LSsfO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-cli-linux-amd64
chmod +x vidveil-cli-linux-amd64
sudo mv vidveil-cli-linux-amd64 /usr/local/bin/vidveil-cli
```

### Configure

```bash
# Connect to server (creates the default cli.yml)
vidveil-cli --server https://x.scour.li --token YOUR_API_TOKEN
```

### Usage

```bash
# Show help
vidveil-cli --help

# Search from the command line
vidveil-cli search "query"

# List engines
vidveil-cli engines

# View bangs
vidveil-cli bangs
```

Running `vidveil-cli` without a command opens the interactive TUI in a terminal.
The optional GUI-enabled desktop client opens a native GUI when a local display
is available; the default static client uses the TUI.
On first launch, the GUI provides a native server setup and connection check.
The current GUI does not expose its widget tree through operating-system
screen-reader APIs. Use the CLI or TUI for screen-reader access.

---

## Configuration

Configuration file location (created on first run):

| Platform | Path |
|----------|------|
| Linux (root) | `/etc/apimgr/vidveil/server.yml` |
| Linux (user) | `~/.config/apimgr/vidveil/server.yml` |
| macOS | `~/Library/Application Support/apimgr/vidveil/server.yml` |
| Windows (system / user) | `%ProgramData%\apimgr\vidveil\server.yml` / `%AppData%\apimgr\vidveil\server.yml` |
| Docker | `/config/vidveil/server.yml` |

`server.yml` contains node-specific settings. Other settings live in area files
under `yaml.d/`, and credentials live in `secrets.yml`. Branding, SEO, and
operator-authored page content are configured in `yaml.d/branding.yml`.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MODE` | `production` or `development` | `production` |
| `LISTEN` | Listen address | `[::]` (dual-stack; `0.0.0.0` selects IPv4 only) |
| `PORT` | Listen port | Random `64xxx` (`80` in containers) |
| `CONFIG_DIR` | Config directory | Platform-specific |
| `DATA_DIR` | Data directory | Platform-specific |

---

## API

### Authentication and Favorites

Search and browsing are public. Favorites are private to an anonymous owner; the
first saved favorite creates an owner token. Save the primary token securely to
restore access on another browser at `/server/preferences/tokens`. Create a
separate linked token for each API or MCP client, choosing `read` or
`read-write` access and an expiry; read-only tokens cannot change favorites.
Do not use the primary or operator token for automation. API clients send owner
tokens as Bearer credentials. The token page can issue a new primary token when no primary
cookie exists, restore an existing token, reveal or rotate tokens, and create
and manage linked tokens. It works without JavaScript. See the
[authentication guide](docs/api/authentication.md) and
[token management guide](docs/user-guide/preferences.md#owner-tokens).

### Search

Search results are collected from enabled engines until they respond or the
request reaches its bounded deadline. The server applies the selected filters,
sort order, and pagination, and reports the aggregate count collected from
responding engines rather than only the displayed page. Upstream sites do not
consistently expose all-pages hit counts, so this count covers results returned
for the requested upstream page. Omitting an engine selection searches all
enabled engines.

```
GET https://x.scour.li/api/v1/search?q={query}&page={page}&engines={engines}
```

**Parameters:**
- `q` - Search query (supports bangs like `!ph amateur`)
- `page` - Page number (default: 1)
- `engines` - Comma-separated engine names, or `tier1` (top 5), `tier12` (top 8) (optional)
- `show_ai` - `1` to include AI-generated results
- `min_quality` - Minimum result quality score (integer)
- `preview_first` - `1` to prioritise results with video preview URLs
- `min_duration` - Minimum video duration in seconds

**Response:**
```json
{
  "ok": true,
  "data": {
    "query": "!ph amateur",
    "search_query": "amateur",
    "has_bang": true,
    "bang_engines": ["pornhub"],
    "results": [...],
    "engines_used": ["pornhub"],
    "search_time_ms": 450
  }
}
```

### Search Stream (SSE)

```
GET https://x.scour.li/api/v1/search?q={query}
Accept: text/event-stream
```

Returns Server-Sent Events with results as each engine responds. The same `https://x.scour.li/api/v1/search` endpoint switches to SSE mode when the `Accept: text/event-stream` header is present.

### Bangs

```
GET https://x.scour.li/api/v1/bangs
GET https://x.scour.li/api/v1/bangs/autocomplete?q={partial}
```

### Engines

```
GET https://x.scour.li/api/v1/engines
GET https://x.scour.li/api/v1/engines/{name}
```

### Health

```
GET https://x.scour.li/api/v1/server/healthz
GET https://x.scour.li/api/v1/server/healthz.txt
GET https://x.scour.li/server/healthz.txt
```

The canonical frontend health route is `https://x.scour.li/server/healthz`.
API health is JSON by default; request the `.txt` form or send
`Accept: text/plain` for text. The root `/healthz` alias is disabled by
default and has no suffix form.
During automatic maintenance, operators can retrieve detailed recovery
diagnostics from `/api/v1/status` with the configured `server.token` Bearer
credential; public health responses omit raw diagnostic details.
`server.maintenance.self_healing.max_attempts: 0` retries without a limit. A
positive limit stops retries after that many failed checks, records an exhausted
health state, and requires correcting the cause and restarting the service.

### Server Pages and API Documentation

| Endpoint | Description |
|----------|-------------|
| `https://x.scour.li/server/about` | Application description and version |
| `https://x.scour.li/server/privacy` | Configured privacy policy |
| `https://x.scour.li/server/dpo` | DPO contact and privacy-policy link |
| `https://x.scour.li/server/help` | Application help |
| `https://x.scour.li/server/terms` | Terms of service |
| `https://x.scour.li/server/security` | Security contacts and reporting guidance |
| `https://x.scour.li/server/security/policy` | Coordinated disclosure policy |
| `https://x.scour.li/server/security/thanks` | Security researcher acknowledgments |
| `https://x.scour.li/server/security/report/{tracking_id}` | One-time security report status |
| `https://x.scour.li/server/healthz` | Public health page |
| `https://x.scour.li/server/docs/swagger` | Swagger UI |
| `https://x.scour.li/api/v1/server/swagger` | OpenAPI JSON (`/api/swagger` alias) |
| `https://x.scour.li/api/v1/server/graphql` | GraphQL endpoint (POST and query-only GET; POST alias at `/api/graphql`) |
| `https://x.scour.li/server/docs/graphql` | GraphiQL playground |
| `https://x.scour.li/server/help/mcp` | MCP connection guide |
| `https://x.scour.li/server/preferences/tokens` | Restore and manage owner tokens |
| `https://x.scour.li/server/preferences/sync` | Export and import browser preferences |
| `https://x.scour.li/server/preferences` | Configure visitor preferences |
| `https://x.scour.li/server/contact` | Contact form and public contact details |
| `https://x.scour.li/feed.xml` | Atom feed of privacy-filtered recent searches |
| `https://x.scour.li/rss.xml` | RSS feed of privacy-filtered recent searches |
| `https://x.scour.li/.well-known/llms.txt` | AI-agent discovery (`/llms.txt` alias) |
| `https://x.scour.li/mcp` | MCP Streamable HTTP endpoint |

---

## Other

### Bang Search

Use bang shortcuts to search specific engines:

| Bang | Engine | Example |
|------|--------|---------|
| `!ph` | PornHub | `!ph amateur` |
| `!xh` | xHamster | `!xh milf` |
| `!rt` | RedTube | `!rt blonde` |
| `!xv` | XVideos | `!xv teen` |
| `!ep` | Eporner | `!ep hd` |
| `!yp` | YouPorn | `!yp pov` |

**Multiple bangs**: `!ph !rt amateur` searches both PornHub and RedTube.

**Full list**: See `https://x.scour.li/api/v1/bangs` for bang shortcuts for the 27 registered engines.

### Supported Engines

The 27 registered engine IDs are `3movs`, `alphaporno`, `anyporn`, `drtuber`,
`eporner`, `flyflv`, `hqporner`, `lovehomeporn`, `nuvid`, `nubilesporn`,
`pornhat`, `pornflip`, `pornhub`, `pornone`, `porntop`, `porntrex`,
`redtube`, `sunporno`, `tnaflix`, `tube8`, `xbabe`, `xhamster`, `xnxx`,
`xxxymovies`, `xvideos`, `youporn`, and `youjizz`. Current bang shortcuts and runtime
availability are returned by the [engines API](https://x.scour.li/api/v1/engines)
and [bangs API](https://x.scour.li/api/v1/bangs).

### Operator Management

VidVeil has no admin web UI. Operators manage the server through its config
files, CLI maintenance commands, and operator-token-protected API endpoints. See the
[configuration guide](docs/configuration.md), [CLI reference](docs/cli.md),
the [translation guide](docs/translating.md), and [API reference](docs/api/rest.md).

### Server CLI

```bash
vidveil [options]

Options:
  --help                    Show help
  --version                 Show version
  --status                  Show server status
  --mode <mode>             Set mode (production/development)
  --port <port>             Set listen port
  --address <addr>          Set listen address
  --i2p                     Enable the optional I2P eepsite
  --config <dir>            Set config directory
  --data <dir>              Set data directory
  --service <cmd>           Service management (start/stop/restart/install/uninstall)
  --maintenance <cmd>       Maintenance (backup/restore)
  --update [check|yes]      Check for or perform updates
```

---

## Development

### Requirements

- Docker (required for builds)
- Make
- Docker (required for unit and basic integration tests)
- Incus (optional, for full systemd/OpenRC integration tests)

### Build

```bash
make build          # Debug server and CLI binaries for all 8 OS/architecture pairs
make test           # Run lint, vulnerability, unit, coverage, and binary checks
make release        # Create local production artifacts and checksums; no tag or publish
make docker         # Build and check the host-platform image locally
make clean          # Remove binaries and release artifacts
make help           # List the supported targets
```

### Test

```bash
./tests/run_tests.sh    # Docker-only basic integration tier
./tests/docker.sh       # Quick Docker binary checks
./tests/incus.sh [systemd|openrc|all] # Full suite; defaults to both init systems
./tests/e2e.sh          # On-demand browser suite
```

`tests/docker.sh` runs the compiled binaries in Alpine and checks Tor and I2P
health in the application container. It reports `SKIP: no outbound network`
when overlay bootstrap cannot be verified; set `REQUIRE_OVERLAY=1` to require
both overlays to become healthy. The shipped image is checked after `make docker`
and in CI.

Set `E2E_TEST_FILTER=TestName` before `tests/e2e.sh` to run one matching
browser test.

### Platforms

| OS | Architecture |
|----|--------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

---

## Disclaimer

This software is provided "as is" without warranty of any kind. Use at your own risk.

- **No Warranty**: The authors are not responsible for any damages, data loss, or issues arising from use of this software
- **Legal Compliance**: This software is a search aggregator. Users are responsible for ensuring compliance with applicable laws and regulations in their jurisdiction
- **Third-Party Services**: This software connects to external adult video sites; their terms of service and content policies apply separately
- **Content**: VidVeil does not store or distribute content — it only indexes metadata from third-party sites
- **Security**: While we strive to follow security best practices, no software is guaranteed to be free of vulnerabilities
- **Production Use**: Evaluate thoroughly before deploying in production environments

By using this software, you acknowledge that you have read and understood this disclaimer.

---

## License

MIT License - see [LICENSE.md](LICENSE.md)
