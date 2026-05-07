# Configuration Rules (PART 5, 6, 12)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Configuration, Application Modes, Server Configuration

## CRITICAL - NEVER DO

- Use strconv.ParseBool() - use config.ParseBool() everywhere
- Add inline YAML/JSON/Go comments - comments go ABOVE the line
- Add comments inside JSON files - JSON has no comment syntax
- Hardcode dev machine values (hostname, IP, cores, memory)
- Skip path validation - run SafePath()/PathSecurityMiddleware on every input
- Allow leading/trailing whitespace on passwords - reject (do not trim)
- Modify ENTRYPOINT/CMD in Docker - customize via entrypoint.sh

## CRITICAL - ALWAYS DO

- Live-reload config changes - no restart except for port/address changes
- Make EVERY setting editable in admin panel - no SSH/CLI required
- Detect resources at runtime (NumCPU(), memory, disk)
- Random port in 64000-64999 on first run; persist to server.yml
- Drop privileges after binding privileged ports (Unix); Virtual Service Account on Windows
- Maintenance mode triggers ONLY for DB connection failure or file-write failure
- Self-heal continuously in maintenance mode (retry every 30s)
- Cache cluster config to server.yml so the app survives DB outage in read-only mode

---
For complete details, see AI.md PART 5, 6, 12
