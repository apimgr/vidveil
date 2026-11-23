# Building

## Requirements

- Docker
- Make
- Docker for builds and unit tests
- Docker for the basic integration test tier
- Incus (optional) for systemd and OpenRC integration tests

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

# Build the container image locally
make docker
```

## Test Commands

```bash
./tests/run_tests.sh
./tests/incus.sh
./tests/docker.sh
./tests/e2e.sh
```

`run_tests.sh` is the Docker-only basic tier and never selects Incus.
`docker.sh` validates the compiled binary in Alpine. Run `incus.sh` explicitly
for the full systemd and OpenRC service integration suite.

`e2e.sh` is the headless-browser suite. Run it on demand after major frontend
work, before a release, or for beta testing. It is not part of `make test` or
the commit gate. It covers three tiers: server-rendered HTML, a browser with
JavaScript disabled, and a full browser with JavaScript enabled.

## Output Locations

- `make dev` writes a temporary build under your OS temp directory
- `make local` and `make build` write binaries to `./binaries/`
- Docker assets live under `docker/`
