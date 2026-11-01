# Docker Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-17; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never place `Dockerfile` or `docker-compose.yml` in project root — always under `docker/`
- Never pre-build the binary outside Docker — always multi-stage build (builder + runtime)
- Never modify `ENTRYPOINT` or `CMD` — all customization goes in `entrypoint.sh`
- Never add `LABEL` blocks to the Dockerfile — all image metadata applied by CI as OCI annotations at build time
- Never include `build:` or `version:` keys in any docker-compose file
- Never use `.env` files for compose environment variables — hardcode with sane defaults, YAML map style (`KEY: value`), never list style (`- KEY=value`)
- Never create directories, set permissions, or create user/group in entrypoint.sh — the binary does all of this
- Never run the runtime container as root unless binding a privileged port (<1024) or managing system services (document exception in IDEA.md) — prefer `setcap cap_net_bind_service` first
- Never bake `MODE` or `DEBUG` into the image or into `docker-compose.yml` (production) — only `docker-compose.dev.yml`/`docker-compose.test.yml` set `DEBUG: 1` and `MODE: dev`
- Never configure Tor in the Dockerfile — Tor binary is installed but the server binary owns all Tor setup (PART 31)
- Never scatter SQLite databases — always `/data/db/sqlite/server.db` (name always `server.db`)
- Never omit `exec` at the end of entrypoint.sh — without it, signals go to bash instead of the app and graceful shutdown breaks

## CRITICAL - ALWAYS DO

- Always use multi-stage Dockerfile: builder stage `casjaysdev/go:latest`, runtime stage `alpine:latest`
- Always build with `CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH}`, `-ldflags "-s -w -X ..."`
- Always use `tini -p SIGTERM` as ENTRYPOINT wrapper around `entrypoint.sh`
- Always set `STOPSIGNAL SIGRTMIN+3` (systemd-compatible graceful shutdown)
- Always expose internal port `80` only
- Always include the three compose files: `docker-compose.yml` (prod), `docker-compose.dev.yml` (dev), `docker-compose.test.yml` (automated testing only)
- Always mount exactly two volumes: `./volumes/config:/config:z` and `./volumes/data:/data:z`
- Always include `x-logging: &default-logging` anchor (max-size 5m, max-file 1, json-file driver) and reference it via `logging: *default-logging` on every service
- Never create a user or set `USER` in the Dockerfile; the server binary creates and drops to its configured service account.

## Key Rules Summary

**Directory layout:**
```
docker/
├── Dockerfile              # production
├── Dockerfile.dev          # devel image, tagged :devel
├── docker-compose.yml      # production — human use only
├── docker-compose.dev.yml  # development — human use only
├── docker-compose.test.yml # testing — AI/automated only
└── rootfs/usr/local/bin/entrypoint.sh
```
Build context is project root; Dockerfile referenced with `-f docker/Dockerfile`; `docker/rootfs/` copied into image as build-time overlay (`COPY docker/rootfs/ /`).

**Container paths:**
| Path | Purpose |
|------|---------|
| `/config/{project_name}/` | server.yml, ssl/, tor/torrc |
| `/data/{project_name}/` | uploads, cache, tor/ (hostname, hs_ed25519_*) |
| `/data/db/sqlite/server.db` | main app database |
| `/data/db/valkey/` | Valkey/Redis persistence |
| `/data/log/{project_name}/` | access.log, error.log, tor.log |
| `/data/backups/{project_name}/` | backup archives |
| `/usr/local/bin/{project_name}` | binary |

**Host volumes:** `./volumes/config:/config:z`, `./volumes/data:/data:z` — binary auto-creates subdirs on first run.

**Dockerfile requirements:**
| Item | Value |
|------|-------|
| Builder image | `casjaysdev/go:latest` |
| Runtime image | `alpine:latest` |
| Required packages | git, curl, bash, tini, tor |
| Binary path | `/usr/local/bin/{project_name}` |
| Init system | tini |
| Internal port | 80 |
| ENV MODE | not set in image — binary defaults production |
| HEALTHCHECK | start-period 10m, interval 5m, timeout 15s, retries 3, `CMD {binary} --status` |
| STOPSIGNAL | `SIGRTMIN+3` |
| ENTRYPOINT | `["tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh"]` |

**OCI annotations (applied by CI, not Dockerfile LABEL):** `org.opencontainers.image.{vendor,authors,title,base.name,description,licenses,created,version,schema-version,revision,url,source,documentation,vcs-type}`, plus `maintainer`, `com.github.containers.toolbox=false`. Applied via `docker/metadata-action` + `docker/build-push-action` (`annotations:` output, `labels: ""`, `provenance: false`), third-party actions pinned to full commit SHA.

**entrypoint.sh** — minimal only: set env defaults (`TZ`, `CONFIG_DIR`, `DATA_DIR`), build CLI flags from env (`ADDRESS`, `PORT`, `DEBUG`), trap `SIGTERM SIGINT SIGQUIT` for graceful shutdown, `exec $APP_BIN $FLAGS "$@"`. Everything else (dirs, perms, user/group, Tor) is the binary's job.

**Entrypoint env vars:**
| Var | Default | Notes |
|-----|---------|-------|
| `TZ` | `America/New_York` | |
| `MODE` | `development` | `production` (strict) / `development` (relaxed) |
| `DEBUG` | `false` | enables `/debug/*` regardless of MODE |
| `ADDRESS` | `0.0.0.0` | |
| `PORT` | `80` | must match compose `ports:` mapping |

Boolean env vars accept any truthy/falsy value (`yes`, `enable`, `1`, `oui`, etc.). Tor auto-enabled if `tor` binary present — no `ENABLE_TOR` flag.

**Compose field reference:**
| Field | Value |
|-------|-------|
| `name:` | `{project_name}` |
| `container_name:` | `{project_name}-app`, `{project_name}-cache` |
| `hostname:` | `{project_name}` |
| `restart:` | `always` |
| `pull_policy:` | `always` |
| `networks:` | custom `{project_name}`, `external: false` |

**Production compose** (`docker-compose.yml`): app + Valkey cache service (`CACHE_URL: valkey://{project_name}-cache:6379`), no `DEBUG`/`MODE` set, port bound to `172.17.0.1:64580:80`, healthcheck `start_period: 90s`.

**Dev compose** (`docker-compose.dev.yml`): single service, `:devel` image tag, `DEBUG: 1`, `MODE: dev`, port `64580:80` (all interfaces, no 172.17.0.1 bind).

For complete details, see AI.md PART 26
