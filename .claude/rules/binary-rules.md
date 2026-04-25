# Binary Rules (PART 7, 8, 33)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use CGO
- Build only a subset of required platforms
- Rename binaries away from the spec names
- Use `--port` when the spec requires `--address`
- Run Go directly on the host machine

## CRITICAL - ALWAYS DO

- Set `CGO_ENABLED=0`
- Build all 8 platforms: linux/darwin/windows/freebsd × amd64/arm64
- Keep binary names aligned with the spec
- Use embedded assets in the final binaries
- Follow CLI flag and help-output conventions from the spec

## Binary Names

- `vidveil` = server
- `vidveil-cli` = required client
- `vidveil-agent` = optional agent

## CLI Rules

- Follow the documented flag names and help structure exactly
- Support plain output rules such as `NO_COLOR` behavior where required
- Keep CLI behavior aligned with server/admin/API capabilities

For complete details, see AI.md PART 7, PART 8, PART 33
