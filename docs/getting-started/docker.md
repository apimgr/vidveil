# Docker

## Quick Start

```bash
docker run -d \
  --name vidveil \
  -p 64580:80 \
  -v ./rootfs/config:/config:z \
  -v ./rootfs/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

## Docker Compose

```bash
curl -q -LSsfO https://raw.githubusercontent.com/apimgr/vidveil/main/docker/docker-compose.yml
docker compose up -d
```

## Runtime Volumes

Vidveil uses two runtime volume roots in compose-based deployments:

| Host Path | Container Path | Purpose |
|---|---|---|
| `./rootfs/config` | `/config` | Configuration root |
| `./rootfs/data` | `/data` | Data and logs root |

The Vidveil config file inside the container is `/config/vidveil/server.yml`.

## Development and Test Compose Files

- `docker/docker-compose.yml` - production-oriented compose
- `docker/docker-compose.dev.yml` - development workflow
- `docker/docker-compose.test.yml` - automated testing workflow
