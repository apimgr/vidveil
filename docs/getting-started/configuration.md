# Configuration

Vidveil uses `server.yml` for runtime configuration.

## Config File Locations

| Platform | Path |
|---|---|
| Linux (root) | `/etc/apimgr/vidveil/server.yml` |
| Linux (user) | `~/.config/apimgr/vidveil/server.yml` |
| macOS (user) | `~/Library/Application Support/apimgr/vidveil/server.yml` |
| Windows | `%AppData%\apimgr\vidveil\server.yml` |
| Docker | `/config/vidveil/server.yml` |

Override the detected config root with `--config`.

## Minimal Example

If `server.port` is omitted, Vidveil selects a random unused port in the `64xxx` range on first run and saves it to `server.yml`.

```yaml
server:
  address: "0.0.0.0"
  port: "64893"
```

## Runtime Data Paths

- Docker config root: `/config/`
- Docker data root: `/data/`
- Docker logs: `/data/log/vidveil/server.log`
- Vidveil config file in Docker: `/config/vidveil/server.yml`
- Vidveil data root in Docker: `/data/vidveil/`

## Tor

Vidveil auto-enables the built-in Tor hidden service when a compatible `tor` binary is available. The server manages its own Tor data under the Vidveil data directory.

## Environment Variables

| Variable | Description |
|---|---|
| `MODE` | Application mode |
| `CONFIG_DIR` | Override config root |
| `DATA_DIR` | Override data root |
| `LOG_DIR` | Override log root |
| `LISTEN` | Override listen address |
| `PORT` | Initial listen port (default: random `64xxx`, `80` in containers) |
