# Vidveil

Privacy-respecting meta search engine for adult video content.

**Official Site**: https://scour.li

**Repository**: https://github.com/apimgr/vidveil

---

## Features

| Feature | Description |
|---------|-------------|
| **Privacy First** | No tracking, no logging, no analytics |
| **47 Engines** | Aggregates results from 47+ adult video sites |
| **Bang Search** | Use `!ph`, `!xh`, `!rt` to search specific sites |
| **Fast APIs** | Direct JSON API integration with PornHub, RedTube, Eporner |
| **SSE Streaming** | Real-time result streaming as engines respond |
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
  -p 8080:80 \
  ghcr.io/apimgr/vidveil:latest
```

### Docker Compose

```bash
curl -O https://raw.githubusercontent.com/apimgr/vidveil/main/docker-compose.yml
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

**Full list**: See `/api/v1/bangs` for all 47 engine shortcuts.

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
GET /api/v1/autocomplete?q={partial}
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
| `/openapi.yaml` | OpenAPI 3.0 spec (YAML) |
| `/graphql` | GraphQL endpoint |
| `/graphiql` | GraphQL playground |

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

Access at `/admin` (credentials shown on first run).

| Section | Description |
|---------|-------------|
| Dashboard | Overview and statistics |
| Server | Server settings |
| Web | Branding, SEO, themes |
| Security | Rate limiting, headers |
| Database | SQLite management |
| Email | SMTP configuration |
| SSL | Let's Encrypt, certificates |
| Scheduler | Scheduled tasks |
| Engines | Enable/disable search engines |
| Logs | Access and error logs |
| Backup | Backup and restore |

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
