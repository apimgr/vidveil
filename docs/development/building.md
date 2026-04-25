# Building

## Requirements

- Docker
- Make
- Incus or Docker for test execution

VidVeil uses containerized Go builds. Do **not** run `go build` directly on the host machine.

## Build Commands

```bash
# Quick host-platform build to a temporary directory
make dev

# Host-platform binaries in ./binaries/
make local

# Cross-platform binaries for all supported targets
make build

# Run unit tests
make test

# Build and push the container image
make docker
```

## Test Commands

```bash
./tests/run_tests.sh
./tests/incus.sh
./tests/docker.sh
```

## Output Locations

- `make dev` writes a temporary build under your OS temp directory
- `make local` and `make build` write binaries to `./binaries/`
- Docker assets live under `docker/`
