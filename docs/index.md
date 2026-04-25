# Vidveil

**Privacy-respecting adult video meta search engine**

Vidveil aggregates results from adult video sites without requiring user accounts or adding tracking, analytics, or server-side history.

## Quick Start

### Docker

```bash
docker run -d \
  --name vidveil \
  -p 64580:80 \
  -v ./rootfs/config:/config:z \
  -v ./rootfs/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

Open `http://localhost:64580`.

### Binary

```bash
curl -q -LSsfO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

## Configuration

Default config file locations:

- Linux (root): `/etc/apimgr/vidveil/server.yml`
- Linux (user): `~/.config/apimgr/vidveil/server.yml`
- macOS (user): `~/Library/Application Support/apimgr/vidveil/server.yml`
- Windows: `%AppData%\apimgr\vidveil\server.yml`
- Docker: `/config/vidveil/server.yml`

See [Getting Started / Configuration](getting-started/configuration.md) for details.

## Documentation

- [Installation](getting-started/installation.md)
- [Docker](getting-started/docker.md)
- [Search Guide](user-guide/search.md)
- [Preferences](user-guide/preferences.md)
- [REST API](api/rest.md)
- [GraphQL](api/graphql.md)
- [CLI Reference](cli.md)

## License

MIT License - see [LICENSE.md](../LICENSE.md)
