# Building

## Requirements

- Go 1.21+
- Make

## Build Commands

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build Docker image
make docker

# Run tests
make test

# Development mode
make dev
```

## Build Flags

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" ./src
```
