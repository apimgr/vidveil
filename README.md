# Vidveil

Privacy-respecting meta search engine for adult video content.

**Official Site**: https://vidveil.apimgr.us

## Features

- **Privacy First**: No tracking, no logging, no analytics
- **Multi-Source**: Aggregates results from 10+ adult video sites
- **Tor Support**: Built-in SOCKS5 proxy support for Tor routing
- **Self-Hosted**: Run your own instance
- **Single Binary**: No external dependencies

## Installation

### Quick Install (Linux/macOS)

```bash
curl -sSL https://vidveil.apimgr.us/install.sh | bash
```

### Docker

```bash
docker run -d \
  --name vidveil \
  -p 8888:80 \
  ghcr.io/apimgr/vidveil:latest
```

### Docker Compose

```bash
curl -O https://raw.githubusercontent.com/apimgr/vidveil/main/docker-compose.yml
docker-compose up -d
```

### Manual Download

Download the latest binary for your platform from [Releases](https://github.com/apimgr/vidveil/releases).

## Configuration

Configuration file is created on first run at:
- **Linux (root)**: `/etc/apimgr/vidveil/server.yml`
- **Linux (user)**: `~/.config/apimgr/vidveil/server.yml`
- **macOS**: `~/Library/Application Support/apimgr/vidveil/server.yml`
- **Windows**: `%AppData%\apimgr\vidveil\server.yml`

### Tor Support

To enable Tor support, edit `server.yml`:

```yaml
search:
  tor:
    enabled: true
    proxy: "127.0.0.1:9050"
```

Or use the Docker Compose with Tor:

```bash
docker-compose --profile tor up -d
```

## Usage

Start the server:

```bash
vidveil
```

Options:

```
--help              Show help
--version           Show version
--status            Check server status
--config <dir>      Set configuration directory
--data <dir>        Set data directory
--address <addr>    Set listen address
--port <port>       Set port
```

## API

### Search

```
GET /api/v1/search?q={query}&page={page}&engines={engines}
```

### Engines

```
GET /api/v1/engines
GET /api/v1/engines/{name}
```

### Stats

```
GET /api/v1/stats
```

## Supported Sites

**Tier 1 (Major)**
- PornHub
- xHamster
- XVideos
- XNXX
- YouPorn
- RedTube
- SpankBang

**Tier 2 (Popular)**
- Eporner
- Beeg
- PornMD

## Development

### Requirements

- Go 1.23+
- Make

### Build

```bash
make build
```

### Test

```bash
make test
```

### Release

```bash
make release
```

### Docker Build

```bash
make docker
```

## License

MIT License - see [LICENSE.md](LICENSE.md)

## Disclaimer

This software is provided for educational and research purposes. Users are responsible for ensuring compliance with local laws and regulations regarding adult content.
