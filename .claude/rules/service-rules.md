# Service Rules (PART 24, 25)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Privilege Escalation & Service, Service Support

## CRITICAL - NEVER DO

- Run server as permanent root unless documented in IDEA.md (vidveil does NOT have this exception)
- Skip privilege drop on Unix - bind privileged ports as root, then drop
- Modify /etc/systemd/system/* or /Library/LaunchDaemons/* on the host - container/VM only

## CRITICAL - ALWAYS DO

- Create dedicated `vidveil` system user/group on first root-mode startup
- Drop to vidveil after binding privileged ports
- Provide systemd, launchd, rc.d, runit unit templates per PART 25
- --maintenance setup/restore/mode require authorization (first-run / root / admin creds / setup token)

---
For complete details, see AI.md PART 24, 25
