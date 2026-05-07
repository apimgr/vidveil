# Binary Rules (PART 7, 8, 33)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Binary Requirements, Server Binary CLI, Client & Agent

## CRITICAL - NEVER DO

- Run go build/test/run directly on host - use `make dev`/`make local`/`make build`/`make test`
- Use -musl suffix in binary names - Alpine builds are not musl-specific
- Skip platforms - build all 8: linux/darwin/windows/freebsd x amd64/arm64
- Hardcode dev paths in CLI flag defaults
- Add short flags for any flag besides -h and -v

## CRITICAL - ALWAYS DO

- Single static binary with all assets embedded
- Match exact CLI flags from PART 8 (--help, --version, --mode, --config, --data, --log, --pid, --address, --port, --baseurl, --debug, --status, --service, --daemon, --maintenance, --update)
- Provide both server and client binaries (vidveil + vidveil-cli)
- Detect TERM=dumb / NO_COLOR and disable ANSI accordingly
- Run vidveil-cli as a normal user only - never as root/admin

---
For complete details, see AI.md PART 7, 8, 33
