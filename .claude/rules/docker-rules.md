# Docker Rules (PART 27)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Docker

## CRITICAL - NEVER DO

- Put Dockerfile in project root - `docker/Dockerfile` only
- Modify ENTRYPOINT/CMD - customize via docker/rootfs/usr/local/bin/entrypoint.sh
- Copy or symlink binaries into the image at build time - the multi-stage builder COPYs them
- Hardcode permanent USER directive when the binary needs privileged startup then drop

## CRITICAL - ALWAYS DO

- Multi-stage build: golang:alpine builder + alpine:latest runtime
- STOPSIGNAL SIGRTMIN+3, ENTRYPOINT [tini, -p, SIGTERM, --, /usr/local/bin/entrypoint.sh]
- Required packages: git, curl, bash, tini, tor
- Internal port 80; external port random 64xxx mapped to 80
- Mount ./volumes/config -> /config and ./volumes/data -> /data
- org.opencontainers.image.licenses="MIT" label

---
For complete details, see AI.md PART 27
