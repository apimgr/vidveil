# Directory Structure Rules

@AI.md PART 3: PROJECT STRUCTURE

## Required Structure
- `src/` - ALL Go code (server, client, agent)
- `src/client/` - CLI client (if exists)
- `src/agent/` - Agent (if exists)
- `docker/` - Dockerfile, docker-compose, rootfs
- `tests/` - Test scripts (run_tests.sh, docker.sh, incus.sh)
- `binaries/` - Build output (gitignored)

## Forbidden
- NO `cmd/` directory (use `src/`)
- NO `internal/` (everything in `src/`)
- NO `pkg/` (not a library)
- NO root-level `data/`, `config/`, `logs/`
