# Build Rules

@AI.md PART 26: MAKEFILE

## Local Development (NOT CI/CD)
| Command | Purpose | Output |
|---------|---------|--------|
| `make dev` | Development & debugging | temp dir |
| `make host` | Production testing | binaries/ (with version) |
| `make build` | Full release | binaries/ (all 8 platforms) |
| `make test` | Unit tests | coverage report |

## NEVER on Host
- `go build` → use `make dev` or `make host`
- `go test` → use `make test`
- `go run` → use `make dev` then run in Docker

## Caching
- GODIR: `~/.local/share/go`
- GOCACHE: `~/.local/share/go/build`

## Testing
- Incus preferred (full OS, systemd)
- Docker fallback (quick tests)
- Required tools: `curl bash file jq`
