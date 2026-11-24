# Docker

## Quick Start

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

## Docker Compose

```bash
curl -q -LSsfO https://raw.githubusercontent.com/apimgr/vidveil/main/docker/docker-compose.yml
docker compose up -d
```

## Runtime Volumes

Vidveil uses two runtime volume roots in compose-based deployments:

| Local Path | Container Path | Purpose |
|---|---|---|
| `./volumes/config` | `/config` | Configuration root |
| `./volumes/data` | `/data` | Data and logs root |

The Vidveil config file inside the container is `/config/vidveil/server.yml`.

## Development and Test Compose Files

- `docker/docker-compose.yml` - production-oriented compose
- `docker/docker-compose.dev.yml` - development workflow
- `docker/docker-compose.test.yml` - automated testing workflow
