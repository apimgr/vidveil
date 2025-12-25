# Configuration

Vidveil uses a YAML configuration file for all settings.

## Configuration File Location

| Platform | Path |
|----------|------|
| Linux | `~/.config/vidveil/server.yml` |
| macOS | `~/Library/Application Support/vidveil/server.yml` |
| Windows | `%APPDATA%\vidveil\server.yml` |

Override with `--config` flag or `CONFIG_DIR` environment variable.

## Server Settings

```yaml
server:
  # Listen address and port
  address: "0.0.0.0"
  port: "8888"

  # Application mode: production or development
  mode: production

  # Fully qualified domain name (for URLs)
  fqdn: "vidveil.example.com"
```

## Search Settings

```yaml
search:
  # Default search engines
  default_engines:
    - pornhub
    - xvideos
    - xnxx
    - redtube
    - youporn
    - xhamster
    - eporner

  # Concurrent search requests
  concurrent_requests: 10

  # Timeout per engine (seconds)
  engine_timeout: 10

  # Results per page
  results_per_page: 20
```

## Tor Settings

```yaml
search:
  tor:
    enabled: false
    proxy: "socks5://127.0.0.1:9050"
```

## Rate Limiting

```yaml
server:
  ratelimit:
    enabled: true
    requests: 120
    window: 60
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MODE` | Application mode (runtime) |
| `CONFIG_DIR` | Configuration directory (init only) |
| `DATA_DIR` | Data directory (init only) |
| `LOG_DIR` | Log directory (init only) |
| `PORT` | Server port (init only) |
| `LISTEN` | Listen address (init only) |
