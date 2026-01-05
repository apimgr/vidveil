# Vidveil

[![Build Status](https://github.com/apimgr/vidveil/actions/workflows/release.yml/badge.svg)](https://github.com/apimgr/vidveil/actions)
[![GitHub release](https://img.shields.io/github/v/release/apimgr/vidveil)](https://github.com/apimgr/vidveil/releases)
[![License](https://img.shields.io/github/license/apimgr/vidveil)](LICENSE.md)
[![Documentation](https://readthedocs.org/projects/apimgr-vidveil/badge/?version=latest)](https://apimgr-vidveil.readthedocs.io/en/latest/)

Privacy-respecting meta search for adult video content.

## Official Site

https://x.scour.li

**Repository**: https://github.com/apimgr/vidveil

---

## Features

| Feature | Description |
|---------|-------------|
| **Privacy First** | No tracking, no logging, no analytics |
| **54+ Engines** | Aggregates results from 54+ adult video sites |
| **Bang Search** | Use `!ph`, `!xh`, `!rt` to search specific sites |
| **Fast APIs** | Direct JSON API integration with PornHub, RedTube, Eporner |
| **SSE Streaming** | Real-time result streaming as engines respond |
| **Thumbnail Proxy** | All thumbnails proxied to prevent engine tracking |
| **Autocomplete** | Bang shortcuts autocomplete as you type |
| **Tor Support** | Built-in Tor hidden service support |
| **Admin Panel** | Full web-based administration |
| **Single Binary** | No external dependencies, embedded assets |
| **Docker Ready** | Alpine-based container with tini |

---

## Quick Start

### Docker (Recommended)

```bash
docker run -d \
  --name vidveil \
  -p 64580:80 \
  -v ./rootfs/config:/config:z \
  -v ./rootfs/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

### Docker Compose

```bash
curl -O https://raw.githubusercontent.com/apimgr/vidveil/main/docker/docker-compose.yml
docker compose up -d
```

### Binary Download

Download the latest binary for your platform from [Releases](https://github.com/apimgr/vidveil/releases).

```bash
# Linux (amd64)
curl -LO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

---

## Bang Search

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

**Full list**: See `/api/v1/bangs` for all 54+ engine shortcuts.

---

## Configuration

Configuration file location (created on first run):

| Platform | Path |
|----------|------|
| Linux (root) | `/etc/apimgr/vidveil/server.yml` |
| Linux (user) | `~/.config/apimgr/vidveil/server.yml` |
| macOS | `~/Library/Application Support/apimgr/vidveil/server.yml` |
| Windows | `%AppData%\apimgr\vidveil\server.yml` |
| Docker | `/config/server.yml` |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIDVEIL_MODE` | `production` or `development` | `production` |
| `VIDVEIL_PORT` | Listen port | `80` |
| `VIDVEIL_CONFIG` | Config directory | Platform-specific |
| `VIDVEIL_DATA` | Data directory | Platform-specific |

---

## CLI Commands

```bash
vidveil [options]

Options:
  --help                    Show help
  --version                 Show version
  --status                  Show server status
  --mode <mode>             Set mode (production/development)
  --port <port>             Set listen port
  --address <addr>          Set listen address
  --config <dir>            Set config directory
  --data <dir>              Set data directory
  --service <cmd>           Service management (start/stop/restart/install/uninstall)
  --maintenance <cmd>       Maintenance (backup/restore)
  --update [check|yes]      Check for or perform updates
```

---

## API Reference

### Search

```
GET /api/v1/search?q={query}&page={page}&engines={engines}
```

**Parameters:**
- `q` - Search query (supports bangs like `!ph amateur`)
- `page` - Page number (default: 1)
- `engines` - Comma-separated engine names (optional)

**Response:**
```json
{
  "success": true,
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
GET /api/v1/search/stream?q={query}
```

Returns Server-Sent Events with results as each engine responds.

### Bangs

```
GET /api/v1/bangs
```

Returns all available bang shortcuts.

### Autocomplete

```
GET /api/v1/bangs/autocomplete?q={partial}
```

Returns bang suggestions for partial input (e.g., `!po` suggests PornHub, PornMD, etc.).

### Engines

```
GET /api/v1/engines
GET /api/v1/engines/{name}
```

### Health

```
GET /healthz
GET /api/v1/healthz
```

### Documentation

| Endpoint | Description |
|----------|-------------|
| `/openapi` | Swagger UI |
| `/openapi.json` | OpenAPI 3.0 spec (JSON) |
| `/graphql` | GraphQL endpoint |
| `/graphiql` | GraphQL playground |
| `/graphql/schema` | GraphQL schema definition |

---

## Supported Engines

### Tier 1 - Major Sites (API-based)

| Engine | Bang | Method |
|--------|------|--------|
| PornHub | `!ph` | Webmasters API |
| RedTube | `!rt` | Public API |
| Eporner | `!ep` | v2 JSON API |
| XVideos | `!xv` | HTML parsing |
| XNXX | `!xn` | HTML parsing |
| xHamster | `!xh` | JSON extraction |

### Tier 2 - Popular Sites

| Engine | Bang |
|--------|------|
| YouPorn | `!yp` |
| PornMD | `!pmd` |
| PornHat | `!phat` |

### Tier 3+ - Additional Sites (38 engines)

4Tube, Fux, PornTube, YouJizz, SunPorno, TXXX, Nuvid, TNAFlix, DrTuber, EMPFlix, HellPorno, AlphaPorno, PornFlip, ZenPorn, GotPorn, HDZog, XXXYMovies, LoveHomePorn, PornerBros, NonkTube, NubilesPorn, PornBox, PornTop, Pornotube, VPorn, PornHD, XBabe, PornOne, PornTrex, HQPorner, VJAV, FlyFLV, Tube8, XTube, AnyPorn, SuperPorn, TubeGalore, Motherless

---

## Admin Panel

Access at `/admin` (setup token shown on first run).

| Route | Section | Description |
|-------|---------|-------------|
| `/admin` | Dashboard | Overview and statistics |
| `/admin/server/settings` | Settings | Server configuration |
| `/admin/server/branding` | Branding | Logo, title, themes |
| `/admin/server/ssl` | SSL/TLS | Let's Encrypt, certificates |
| `/admin/server/email` | Email | SMTP configuration |
| `/admin/server/scheduler` | Scheduler | Scheduled tasks |
| `/admin/server/logs` | Logs | Access and error logs |
| `/admin/server/database` | Database | SQLite management |
| `/admin/server/security/*` | Security | Auth, tokens, rate limiting, firewall |
| `/admin/server/network/*` | Network | Tor, GeoIP, blocklists |
| `/admin/server/system/*` | System | Backup, maintenance, updates |
| `/admin/server/users/*` | Users | Admin management |
| `/admin/server/engines` | Engines | Search engine configuration |

---

## CLI Client

A companion CLI client (`vidveil-cli`) is available for interacting with the server API from the terminal.

### Install

```bash
# Download latest release
curl -LO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64-cli
chmod +x vidveil-linux-amd64-cli
sudo mv vidveil-linux-amd64-cli /usr/local/bin/vidveil-cli
```

### Configure

```bash
# Connect to server (creates ~/.config/apimgr/vidveil/cli.yml)
vidveil-cli --server https://x.scour.li --token YOUR_API_TOKEN
```

### Usage

```bash
# Show help
vidveil-cli --help

# Search (launches interactive TUI)
vidveil-cli search "query"

# List engines
vidveil-cli engines

# View bangs
vidveil-cli bangs
```

The CLI automatically launches an interactive TUI (Terminal User Interface) with keyboard navigation, dark theme, and real-time updates.

---

## Development

### Requirements

- Go 1.23+
- Make
- Docker (optional)

### Build

```bash
make build          # Build binary
make test           # Run tests
make docker         # Build Docker image
make release        # Build all platforms
```

### Platforms

| OS | Architecture |
|----|--------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

---

## License

MIT License - see [LICENSE.md](LICENSE.md)

---

## Disclaimer

This software is provided for legal use only. Users are responsible for ensuring compliance with applicable laws and regulations in their jurisdiction.
