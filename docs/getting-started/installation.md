# Installation

## Requirements

- Go 1.21+ (for building from source)
- Docker (optional, for containerized deployment)

## Download

### Pre-built Binaries

Download the latest release for your platform:

| Platform | Architecture | Download |
|----------|--------------|----------|
| Linux | amd64 | [vidveil-linux-amd64](https://github.com/apimgr/vidveil/releases/latest) |
| Linux | arm64 | [vidveil-linux-arm64](https://github.com/apimgr/vidveil/releases/latest) |
| macOS | amd64 | [vidveil-darwin-amd64](https://github.com/apimgr/vidveil/releases/latest) |
| macOS | arm64 | [vidveil-darwin-arm64](https://github.com/apimgr/vidveil/releases/latest) |
| Windows | amd64 | [vidveil-windows-amd64.exe](https://github.com/apimgr/vidveil/releases/latest) |
| Windows | arm64 | [vidveil-windows-arm64.exe](https://github.com/apimgr/vidveil/releases/latest) |
| FreeBSD | amd64 | [vidveil-freebsd-amd64](https://github.com/apimgr/vidveil/releases/latest) |
| FreeBSD | arm64 | [vidveil-freebsd-arm64](https://github.com/apimgr/vidveil/releases/latest) |

### Docker

```bash
docker pull ghcr.io/apimgr/vidveil:latest
```

## Building from Source

```bash
git clone https://github.com/apimgr/vidveil.git
cd vidveil
make build
```

The binary will be created in `bin/vidveil`.

## Running

### Direct Execution

```bash
./vidveil
```

### As a Service

```bash
# Install as system service
sudo ./vidveil --service --install

# Start the service
sudo ./vidveil --service start
```

### Docker

```bash
docker run -d \
  --name vidveil \
  -p 8888:80 \
  -v vidveil-data:/data \
  -v vidveil-config:/config \
  ghcr.io/apimgr/vidveil:latest
```

## First Run

On first run, Vidveil will:

1. Create configuration directory
2. Generate a setup token (displayed in console)
3. Start the web server

Navigate to `http://localhost:8888/admin` and enter the setup token to complete the initial configuration.
