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
  -v ./rootfs/config:/config:z \
  -v ./rootfs/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

### Binary Download

```bash
curl -q -LSsfO https://github.com/apimgr/vidveil/releases/latest/download/vidveil-linux-amd64
chmod +x vidveil-linux-amd64
./vidveil-linux-amd64
```

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
sudo systemctl enable --now vidveil
```

### macOS (launchd)

```bash
sudo cp vidveil-darwin-amd64 /usr/local/bin/vidveil
sudo vidveil --service --install
sudo launchctl load /Library/LaunchDaemons/apimgr.vidveil.plist
```

### BSD (rc.d)

```bash
sudo cp vidveil-freebsd-amd64 /usr/local/bin/vidveil
sudo vidveil --service --install
echo 'vidveil_enable="YES"' | sudo tee -a /etc/rc.conf
sudo service vidveil start
```

## First Run

On first run Vidveil creates its config/data paths, initializes the database, and prints the setup token or setup URL needed to complete admin setup.
