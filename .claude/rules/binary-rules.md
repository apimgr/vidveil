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
- Run vidveil-cli without privilege escalation; even when the invoking user IS root/Administrator, use user-scope paths (~/.config, %APPDATA%, ...) - never /etc, /var/lib, %ProgramData% - and never call sudo / UAC (PART 33: "Runs in the invoking user's context", "Uses user/home/profile directories exclusively, even if the invoking user happens to be root/Administrator")

---
For complete details, see AI.md PART 7, 8, 33
