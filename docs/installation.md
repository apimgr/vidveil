# Installation

## Recommended Paths

### Docker Compose

```bash
curl -q -LSsfO https://raw.githubusercontent.com/apimgr/vidveil/main/docker/docker-compose.yml
docker compose up -d
```

### Docker Run

```bash
docker run -d \
  --name vidveil \
  -p 64580:80 \
  -v ./volumes/config:/config:z \
  -v ./volumes/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

### Binary Download

```bash
curl -q -LSsfO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

#### Release Artifacts

Both the `vidveil` server and `vidveil-cli` client are published for all supported platforms:

| Platform | Server artifact | CLI artifact |
|---|---|---|
| Linux amd64 | `vidveil-linux-amd64` | `vidveil-cli-linux-amd64` |
| Linux arm64 | `vidveil-linux-arm64` | `vidveil-cli-linux-arm64` |
| macOS amd64 | `vidveil-darwin-amd64` | `vidveil-cli-darwin-amd64` |
| macOS arm64 | `vidveil-darwin-arm64` | `vidveil-cli-darwin-arm64` |
| Windows amd64 | `vidveil-windows-amd64.exe` | `vidveil-cli-windows-amd64.exe` |
| Windows arm64 | `vidveil-windows-arm64.exe` | `vidveil-cli-windows-arm64.exe` |
| FreeBSD amd64 | `vidveil-freebsd-amd64` | `vidveil-cli-freebsd-amd64` |
| FreeBSD arm64 | `vidveil-freebsd-arm64` | `vidveil-cli-freebsd-arm64` |

Download the matching artifact from the [latest release](https://github.com/apimgr/vidveil/releases/latest).

## Build From Source

Local builds use the repository Makefile and Docker-backed Go toolchain:

```bash
git clone https://github.com/apimgr/vidveil.git
cd vidveil
make local
```

Built binaries are written to `binaries/`.

## Service Installation

### Linux (systemd)

```bash
sudo cp vidveil-linux-amd64 /usr/local/bin/vidveil
sudo vidveil --service --install
```

### macOS (launchd)

```bash
sudo cp vidveil-darwin-amd64 /usr/local/bin/vidveil
sudo vidveil --service --install
```

### BSD (rc.d)

```bash
sudo cp vidveil-freebsd-amd64 /usr/local/bin/vidveil
sudo vidveil --service --install
```

## First Run

On first run VidVeil creates its config and data paths, initializes the database,
and prints the configuration file location. Operators configure the service in
`server.yml`; there is no web-based admin setup flow.
