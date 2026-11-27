# Vidveil

**Privacy-respecting adult video meta search engine**

Vidveil aggregates results from adult video sites without user accounts or server-side per-visitor search history. The search-history list is stored in the visitor's browser. Privacy-filtered query text and timestamps may also appear in public recent-search feeds. Configured access logs may include request paths, IP addresses, and other request metadata; custom formats can include query strings. Optional analytics require operator configuration and visitor consent.

## Quick Start

### Docker

```bash
docker run -d \
  --name vidveil \
  -p 172.17.0.1:64580:80 \
  -v ./volumes/config:/config:z \
  -v ./volumes/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

Open `http://172.17.0.1:64580` from the host. If that address is not available
in your environment, publish the container on an address reachable from your
browser.

### Binary

```bash
curl -q -LSsfO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

The server and CLI releases support Linux, macOS, Windows, and FreeBSD on amd64 and arm64. See the [complete artifact list](installation.md#release-artifacts).

## Configuration

Default config file locations:

- Linux (root): `/etc/apimgr/vidveil/server.yml`
- Linux (user): `~/.config/apimgr/vidveil/server.yml`
- macOS (user): `~/Library/Application Support/apimgr/vidveil/server.yml`
- Windows: `%AppData%\apimgr\vidveil\server.yml`
- Docker: `/config/vidveil/server.yml`

See [Configuration](configuration.md) for details.

## Documentation

- [Installation](installation.md)
- [Docker](getting-started/docker.md)
- [Search Guide](user-guide/search.md)
- [Preferences](user-guide/preferences.md)
- [Translating](translating.md)
- [REST API](api/rest.md)
- [API overview](api.md)
- [GraphQL](api/graphql.md)
- [API authentication](api/authentication.md)
- [CLI Reference](cli.md)
- [Configuration](configuration.md)
- [Security](security.md)
- [Integrations](integrations.md)
- [Administration](admin.md)
- [Development](development.md)

## Project Links

- [Source repository](https://github.com/apimgr/vidveil)
- [Official site](https://x.scour.li)
- [OpenAPI documentation](https://x.scour.li/server/docs/swagger)
- [GraphiQL explorer](https://x.scour.li/server/docs/graphql)

## License

MIT License - see [LICENSE.md](https://github.com/apimgr/vidveil/blob/main/LICENSE.md)
