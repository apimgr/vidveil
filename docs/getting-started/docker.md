# Docker

## Quick Start

```bash
docker run -d -p 8888:80 ghcr.io/apimgr/vidveil:latest
```

## Docker Compose

```yaml
version: '3.8'

services:
  vidveil:
    image: ghcr.io/apimgr/vidveil:latest
    container_name: vidveil
    restart: unless-stopped
    ports:
      - "8888:80"
    volumes:
      - vidveil-data:/data
      - vidveil-config:/config
    environment:
      - MODE=production

volumes:
  vidveil-data:
  vidveil-config:
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | production | Application mode |
| `PORT` | 80 | Internal port (don't change) |

## Volumes

| Path | Description |
|------|-------------|
| `/config` | Configuration files |
| `/data` | Database and data files |
| `/logs` | Log files |

## With Tor

The Docker image includes Tor. To enable:

```yaml
environment:
  - TOR_ENABLED=true
```
