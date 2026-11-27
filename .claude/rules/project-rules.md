# Project Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Add incompatible or copyleft dependencies without an explicit project exception; do not omit required third-party license attribution.
- Hardcode project names, organizations, host-specific values, or runtime paths.
- Put runtime data, generated binaries, Docker files, or compose files in forbidden locations.
- Use `github.com/mattn/go-sqlite3`; the Go SQLite driver is `modernc.org/sqlite`.

## CRITICAL - ALWAYS DO
- Keep project files and source under the AI.md-defined project root and prescribed `src/`, `docker/`, and runtime paths.
- Resolve project variables from IDEA.md first, then the documented remote/path fallbacks.
- Use lowercase, snake_case Go filenames and singular package directories, honoring documented exceptions.
- Support Linux, macOS, Windows, and FreeBSD on amd64 and arm64.
- Keep LICENSE.md dependency and embedded-data provenance current.

## KEY RULES
- `AI.md` describes implementation; `IDEA.md` describes product intent; preserve this boundary.
- Root runtime `config/`, `data/`, `logs/`, temp/output directories, `.env` files, and a root Dockerfile/compose file are forbidden.
- Platform paths and privilege behavior must follow PART 4 for the target OS.

For complete details, see AI.md PART 2, 3, 4.
