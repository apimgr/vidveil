# Project Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️
<!-- Rule summary checked against AI.md on 2026-11-06; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never use GPL/AGPL/LGPL licensed dependencies — MIT-only compatibility (MIT, Apache 2.0, BSD, ISC, Public Domain are OK)
- Never use `github.com/mattn/go-sqlite3` (CGO) — use `modernc.org/sqlite` (pure Go)
- Never use `github.com/lib/pq`, `github.com/ooni/go-libtor`, `github.com/dgrijalva/jwt-go`, `github.com/gorilla/mux`, `github.com/go-redis/redis` (old path) — see forbidden table below
- Never use any library requiring CGO — all libraries must work with `CGO_ENABLED=0`
- Never hardcode `{project_name}`/`{project_org}` — always infer from `git remote get-url origin` or directory path
- Never assume current working directory is project root — use `git rev-parse --show-toplevel`, absolute paths, or explicit `cd`
- Never rename `{internal_name}` during an ordinary project rename; reset it only for an explicit user-directed org/repo rename with coordinated path migration, or at a fork's first setup
- Never mix runtime directory purposes — user-editable config never goes in `data_dir`, app-managed data never goes in `config_dir`
- Never use `.yaml` — config file is always `server.yml`
- Never hardcode a specific Go version anywhere (docs, Docker, CI) — always latest stable, `casjaysdev/go:latest` unpinned
- Never commit `binaries/`, `releases/`, `volumes/`, `docker/rootfs/` (build/runtime output) — gitignored
- Never include Makefile, docs/, tests/, or CI config in `.dockerignore` exclusions from the build itself — see required/forbidden dockerignore tables
- Never require a VCS remote or package registry for a `local`-provider project (prototyping/bootstrapping/local-only dev) — that mode is intentionally registry-free

## CRITICAL - ALWAYS DO

- Always include `LICENSE.md` (MIT) in project root, with embedded third-party licenses appended
- Always attribute third-party dependency licenses (MIT/Apache/BSD/ISC/MPL) in `LICENSE.md`, compact table format recommended for 10+ deps
- Always create `.claude/rules/*.md` (13 files, this directory) at `.claude/rules/` — required, auto-loaded
- Always keep root `AGENTS.md` as an exact real-file copy of root `CLAUDE.md`; update both together
- Always support all 4 OSes (Linux, BSD, macOS, Windows) and both architectures (AMD64, ARM64)
- Always use OS-specific paths exactly as defined below — never invent alternate paths
- Always start `.gitignore` with the two-line header: `# gitignore created on MM/DD/YY at HH:MM` then literal `ignoredirmessage`
- Always use `modernc.org/sqlite` (driver name `sqlite`, aliases `sqlite2`/`sqlite3`) for local SQLite; `github.com/tursodatabase/libsql-client-go` (driver `libsql`, alias `turso`) for remote-only libSQL/Turso

## Key Rules Summary

**License:** MIT only. `LICENSE.md` required. Docker license metadata via `--annotation "org.opencontainers.image.licenses=MIT"` at buildx time (no Dockerfile LABEL). README badge: `[![License](https://img.shields.io/github/license/{project_org}/{project_name})](LICENSE.md)`. Compatibility matrix: MIT project may depend on MIT/Apache-2.0/BSD/ISC/Public-Domain; never GPL/AGPL/LGPL.

**Variable placeholders:** `{project_name}` (lowercase, may change on rename), `{PROJECT_NAME}` (env/Makefile uppercase), `{project_org}` (lowercase), `{internal_name}` and `{internal_org}` (lowercase, both frozen after the project ships; ordinary project/org renames change only `project_name`/`project_org`), `{plist_name}` = `io.github.{internal_org}.{internal_name}`. This project: `project_name=vidveil`, `project_org=apimgr`, `internal_name=vidveil`, `internal_org=apimgr`, `official_site=https://x.scour.li`.

**Directory structure (required top-level):**
```
.github/workflows/ (or .gitea/)   CLAUDE.md   AGENTS.md   .claude/rules/*.md (13 files)
docs/ (MkDocs/ReadTheDocs)        src/        scripts/            tests/
docker/{Dockerfile,Dockerfile.dev,docker-compose*.yml,rootfs/}
binaries/ (gitignored)  releases/ (gitignored)  volumes/ (gitignored)
README.md  LICENSE.md  AI.md  TODO.AI.md  TODO.md  PLAN.AI.md  PLAN.md
Jenkinsfile  release.txt  site.txt
```

**.gitignore / .dockerignore:** base from `gitignore --dir . default` + project-specific (`binaries/`, `releases/`, `volumes/`, IDE files, `.claude/`/`.cursor/`/`.aider/`/`.ai/`/`.windsurf/`, `CLAUDE.local.md`). `.dockerignore` excludes `.git/`, CI dirs, `volumes/`, `binaries/`, `releases/`, `tests/`, `docs/`, `*.md`, `Makefile`, IDE files, AI config dirs — but KEEPS `src/`, `go.mod`/`go.sum`, `docker/` (incl. `rootfs/`).

**Go requirements:** latest stable only, build-only (static binary), `CGO_ENABLED=0` everywhere, build/test only in `casjaysdev/go:latest` Docker, never `go` directly on host.

**Required libraries:** router `go-chi/chi/v5`; YAML `gopkg.in/yaml.v3`; UUID `google/uuid`; crypto `golang.org/x/crypto`; Tor `cretz/bine`; WebSocket `gorilla/websocket`; CORS `rs/cors`; scheduler `go-co-op/gocron/v2`; rate limit `golang.org/x/time/rate`; validation `go-playground/validator/v10`; Valkey/Redis `redis/go-redis/v9`; Memcache `bradfitz/gomemcache`.

**OS paths (privileged / user), summarized:**
| OS | Config (priv) | Data (priv) | Config (user) |
|----|---------------|-------------|----------------|
| Linux | `/etc/{internal_org}/{internal_name}/` | `/var/lib/{internal_org}/{internal_name}/` | `~/.config/{internal_org}/{internal_name}/` |
| macOS | `/Library/Application Support/{internal_org}/{internal_name}/` | same + `/data/` | `~/Library/Application Support/{internal_org}/{internal_name}/` |
| BSD | `/usr/local/etc/{internal_org}/{internal_name}/` | `/var/db/{internal_org}/{internal_name}/` | `~/.config/{internal_org}/{internal_name}/` |
| Windows | `%ProgramData%\{internal_org}\{internal_name}\` | same | `%AppData%\{internal_org}\{internal_name}\` |
| Docker | `/config/{project_name}/` | `/data/{project_name}/` | n/a |

Config file is always `server.yml` (never `.yaml`). SQLite DB always under `.../db/server.db`. Docker internal port is always `80`. Linux service unit: `/etc/systemd/system/{internal_name}.service`. macOS service: `LaunchDaemons`/`LaunchAgents` plist named `{plist_name}.plist`. BSD service: `/usr/local/etc/rc.d/{internal_name}`.

For complete details, see AI.md PART 2, 3, 4
