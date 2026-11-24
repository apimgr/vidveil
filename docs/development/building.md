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

# Host-platform server and CLI plus all supported cross-platform binaries
make build

# Run unit tests
make test

# Build and push the multi-architecture container image
make docker
```

## Test Commands

```bash
./tests/run_tests.sh
./tests/incus.sh [systemd|openrc|all]
./tests/docker.sh
./tests/e2e.sh
```

`run_tests.sh` is the Docker-only basic tier and never selects Incus.
`docker.sh` validates the compiled binary in Alpine. Run `incus.sh` explicitly
for the full systemd and OpenRC service integration suite.
The Docker image check waits up to the configured Tor bootstrap timeout. When
the runner has no outbound network, it reports `SKIP: no outbound network`;
set `REQUIRE_OVERLAY=1` to require Tor health instead.
The Incus selector defaults to `all`; use `systemd` or `openrc` to run one
init-system suite. Set `E2E_TEST_FILTER=TestName` before `e2e.sh` to run a
matching browser test.

`e2e.sh` is the headless-browser suite. Run it on demand after major frontend
work, before a release, or for beta testing. It is not part of `make test` or
the commit gate. It covers three tiers: server-rendered HTML, a browser with
JavaScript disabled, and a full browser with JavaScript enabled.

## Output Locations

- `make dev` writes a temporary build under your OS temp directory
- `make local` writes host-platform server and CLI binaries to `./binaries/`
- `make build` writes host-platform server and CLI binaries plus both binaries for all 8 supported cross-platform targets to `./binaries/`
- Docker assets live under `docker/`
