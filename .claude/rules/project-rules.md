# Project Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use any license other than MIT
- Use GPL/AGPL/LGPL dependencies
- Hardcode `vidveil` or `apimgr` in reusable template logic
- Use `.yaml` for config files — use `.yml`
- Put Docker files in the repo root

## CRITICAL - ALWAYS DO

- Keep `LICENSE.md` in the repo root
- Use `server.yml` as the config filename
- Follow the required repo structure under `src/`, `docker/`, `docs/`, `tests/`, and `binaries/`
- Respect OS-specific path rules for config, data, logs, and services
- Keep `.claude/rules/*.md` present in the repo

## Project Identity

| Field | Value |
|---|---|
| Name | vidveil |
| Organization | apimgr |
| Config file | `server.yml` |
| License | MIT via `LICENSE.md` |

## Key Paths

- Linux privileged config: `/etc/apimgr/vidveil/server.yml`
- Linux user config: `~/.config/apimgr/vidveil/server.yml`
- Docker config: `/config/vidveil/server.yml`
- Docker data: `/data/vidveil/`
- Docker logs: `/data/log/vidveil/server.log`

## Required Files

- `AI.md`
- `IDEA.md`
- `TODO.AI.md`
- `README.md`
- `LICENSE.md`
- `.claude/rules/*.md`

For complete details, see AI.md PART 2, PART 3, PART 4
