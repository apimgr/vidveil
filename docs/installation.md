# Installation

## Quick Start

### Docker (Recommended)

```bash
docker run -d \
  --name vidveil \
  -p 64580:80 \
  -v ./rootfs/config:/config:z \
  -v ./rootfs/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

Access at `http://localhost:64580`

### Docker Compose

```bash
curl -O https://raw.githubusercontent.com/apimgr/vidveil/main/docker/docker-compose.yml
docker compose up -d
```

### Binary Download

Download from [Releases](https://github.com/apimgr/vidveil/releases)

```bash
# Linux AMD64
curl -LO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

## System Requirements

- 64-bit processor (AMD64 or ARM64)
- 512MB RAM minimum (1GB+ recommended)
- 100MB disk space

## Supported Platforms

- Linux (AMD64, ARM64)
- macOS (AMD64, ARM64)
- BSD (AMD64, ARM64)
- Windows (AMD64, ARM64)

## Service Installation

### Linux (systemd)

```bash
# Install binary
sudo cp vidveil-linux-amd64 /usr/local/bin/vidveil

# Install service
sudo vidveil --service --install

# Start service
sudo systemctl enable --now vidveil
```

### macOS (launchd)

```bash
# Install binary
sudo cp vidveil-darwin-amd64 /usr/local/bin/vidveil

# Install service
sudo vidveil --service --install

# Start service
sudo launchctl load /Library/LaunchDaemons/com.apimgr.vidveil.plist
```

### BSD (rc.d)

```bash
# Install binary
sudo cp vidveil-freebsd-amd64 /usr/local/bin/vidveil

# Install service
sudo vidveil --service --install

# Enable at boot
echo 'vidveil_enable="YES"' | sudo tee -a /etc/rc.conf

# Start service
sudo service vidveil start
```

## First Run

On first run, VidVeil will:

1. Create default configuration in `/etc/apimgr/vidveil/server.yml`
2. Initialize SQLite database
3. Generate admin setup token
4. Display setup URL

Visit the setup URL to complete installation.
