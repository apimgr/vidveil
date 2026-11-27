# Docker Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Place Dockerfiles or compose files in the repository root.
- Override the canonical container ENTRYPOINT or CMD; startup customization belongs in `entrypoint.sh`.
- Commit local runtime volumes or use a runtime directory as the build-time rootfs overlay.
- Use unbounded/host-wide port bindings where the deployment contract requires a restricted bridge bind.

## CRITICAL - ALWAYS DO
- Use `docker/` for Dockerfiles/compose, `docker/rootfs/` for build-time overlay, and separate runtime volumes.
- Use the required multi-stage build, maintained Go builder, runtime packages, OCI labels, and multi-architecture metadata.
- Run via `tini` and the prescribed entrypoint; preserve signal forwarding and health behavior.
- Keep SQLite files under `/data/db/sqlite/` and apply the specified internal/external port, timezone, and environment defaults.
- Maintain production, development, and AI test compose variants with the exact isolation and path rules.

## KEY RULES
- Tor and I2P container detection/defaults are defined in PART 26 and PART 31.
- Build-time assets and runtime data have different lifecycles and must not be mixed.

For complete details, see AI.md PART 26.
