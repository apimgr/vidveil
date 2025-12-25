# Vidveil

**Privacy-respecting adult video meta search engine**

Vidveil is a meta search engine that aggregates results from multiple adult video sites without tracking users.

## Features

- **Meta Search**: Aggregates results from 50+ adult video sites
- **Privacy First**: No tracking, no cookies, no personal data storage
- **Age Verification**: Client-side age gate before accessing content
- **Tor Support**: Built-in Tor hidden service support
- **Multiple Formats**: HTML, JSON API, GraphQL, plain text
- **Bang Syntax**: Direct site searches with `!site query`

## Quick Start

### Docker (Recommended)

```bash
docker run -d -p 8888:80 ghcr.io/apimgr/vidveil:latest
```

### Binary

```bash
# Download the latest release
curl -LO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64

# Make executable
chmod +x vidveil-linux-amd64

# Run
./vidveil-linux-amd64
```

Access the web interface at `http://localhost:8888`

## Configuration

Vidveil uses a YAML configuration file located at:

- Linux: `~/.config/vidveil/server.yml`
- macOS: `~/Library/Application Support/vidveil/server.yml`
- Windows: `%APPDATA%\vidveil\server.yml`

See [Configuration](getting-started/configuration.md) for details.

## API

Vidveil provides multiple API endpoints:

- **REST API**: `/api/v1/search`
- **GraphQL**: `/graphql`
- **OpenAPI**: `/openapi`

See [API Reference](api/rest.md) for full documentation.

## License

MIT License - see [LICENSE](https://github.com/apimgr/vidveil/blob/main/LICENSE.md)
