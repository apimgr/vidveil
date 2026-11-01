# Service Rules (PART 23, 24)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-10-17; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never run permanently as root/admin — bind privileged port (<1024) elevated, then drop to dedicated service user immediately after binding (Unix)
- Never skip service user creation — a system service account must exist before the service starts, with matching UID/GID across the pair
- Never pick a UID/GID from the reserved/well-known range (docker, systemd-*, polkitd, kvm, pipewire, avahi, sshd, postfix, dovecot, nobody 65534, legacy DB 170-179, etc.) when auto-selecting a system ID
- Never allow user-mode ($USER) runs to bind ports ≤1024 — only escalated/service mode may use privileged ports
- Never ship only one service manager's unit file — all four OS families (systemd/OpenRC/SysVinit/runit, rc.d, launchd, Windows) must be supported
- Never install both OpenRC and SysVinit scripts on the same host — detect and install exactly one based on what's present
- Never grant the Windows service more than the Virtual Service Account needs — no manual privilege dropping on Windows (VSA is already minimal-privilege)
- Never claim permanent root without IDEA.md explicitly requiring it and documenting why privilege drop is impossible — the only accepted justifications are ongoing kernel/device operations (firewall control, packet capture, TUN/TAP/VPN, mount/filesystem management, package/service management)
- Never perform setup/restore/mode-change/pgp/secret-rotation without checking AUTHORIZATION (token/root/first-run), not just file access — directory ownership after service install is not proof of authority

## CRITICAL - ALWAYS DO

- Always detect root/admin vs regular user at startup and branch behavior accordingly (see Run Mode table)
- Always create the system service user/group with a safe auto-selected UID/GID if one doesn't already exist
- Always drop privileges after binding privileged ports on Unix-like systems
- Always use `{internal_name}` (frozen) for service identity — unit file names, paths, user/group names — never `{project_name}` (may change on rename)
- Always implement `--service {start|stop|restart|reload|status|--install|--disable|--uninstall}` CLI subcommands internally in the binary
- Always detect the correct service manager for the host OS before installing (systemd vs OpenRC vs SysVinit vs runit vs rc.d vs launchd vs Windows SCM)
- Always use a Virtual Service Account (`NT SERVICE\{internal_name}`) on Windows — no separate privilege-drop logic needed there
- Always determine escalation need dynamically (check actual access, e.g. can write to `/etc/systemd/system/`, can bind the port, can write the binary) rather than assuming based on command name
- Always offer a graceful fallback when escalation is unavailable — user service instead of system service, random >1024 port instead of a privileged one — and clearly explain what changed

## Key Rules Summary

**Run modes:**
| Run Mode | Who Runs Binary | Port Restriction | Privilege Drop |
|----------|-----------------|-------------------|-----------------|
| Service (escalated) | root/admin | Any port | Yes (after binding) |
| User mode ($USER) | Calling user | >1024 only | No (already unprivileged) |

**Escalation detection by OS:** Linux (`sudo`/`su`/`pkexec`/`doas`), macOS (`sudo`/`osascript`), BSD (`doas`/`sudo`/`su`), Windows (UAC/`runas`). Binary self-detects effective privilege level; never assumes.

**Escalation decision symbols:** ❌ No (never needs escalation/auth) · ⚠️ Check (binary checks actual access — e.g. can write `/etc/systemd/system/`, is port <1024, is service type X installed) · 🔐 Auth (requires `server.token`, root, or first-run/empty-DB). `--service status`, normal start, and `--maintenance backup` are ❌ No. `--service {install,uninstall,disable,start,stop,restart,reload}` and `--port <1024` are ⚠️ Check. `--maintenance {restore,setup,mode,pgp <action>,secret rotate <name>}` are 🔐 Auth.

**Smart escalation flow (interactive):** already elevated → proceed; can escalate (in sudo/wheel/admin group) → prompt `[Y/n]`, re-exec elevated on yes, otherwise fall back (user service / random port) with a clear message; cannot escalate → fall back automatically with an explanatory message, never a hard failure for non-required elevation.

**Sensitive operations (🔐 Auth required, file access alone is NOT authorization):**
| Operation | Danger | Authorization Required |
|-----------|--------|----------------------|
| `--maintenance setup` | Resets server configuration to defaults | First-run only OR root |
| `--maintenance restore` | Overwrites ALL data | `server.token` OR root OR empty database |
| `--maintenance mode` | Changes server behavior (e.g. relaxes dev-mode host validation) | `server.token` OR root |
| `--maintenance pgp export private / import / delete` | Exposes, replaces, or destroys the security private key | `server.token` OR root + typed confirmation |
| `--maintenance secret rotate installation_secret / encryption_key` | Rotates a root secret or the at-rest encryption key | `server.token` OR root + typed confirmation |

Restore/setup/mode authorization flows all follow the same shape: first-run/empty-DB → always allowed; root → allowed with confirmation; service user (`{project_name}`) → requires `server.token` prompt; any other user → rejected with a clear remediation message (run as root, or provide the token).

**Privilege drop pattern (Unix):** root startup sequence — create system user → create dirs + chown → bind privileged ports while still root → spawn any managed children (e.g. Tor) with the service-user's setuid/setgid credentials → **drop privileges as the final root-phase action** → verify with a post-drop `getuid() != 0` check → init config/DB → serve. Windows never drops — VSA is already isolated/minimal-privilege.

**System user requirements:** dedicated service user/group named `{internal_name}`; shell `/usr/sbin/nologin`; home `/var/lib/{internal_org}/{internal_name}`; matching UID/GID; auto-created at install time via `findAvailableSystemID` (Go, searches top-down from 899 to 200 on Linux/BSD, 399 to 200 on macOS) if not already present; UID/GID picked from a safe range avoiding well-known reserved IDs.

**What the `{project_name}` user can/cannot do:** CAN read/write config/data/cache/logs/backups, use bound sockets inherited pre-drop, bind >1024, run scheduled tasks, manage DB/SSL certs. CANNOT bind <1024, modify service unit files, create/delete system users, write `/usr`/`/bin`, modify or self-update the binary — those remain root-owned operations.

**Reserved/well-known UIDs to avoid:** `docker`, `systemd-*`, `polkitd`, `kvm`, `pipewire`, `avahi`, `sshd`, `postfix`, `dovecot`, `nobody` (65534), legacy DB range (170-179), and other platform-reserved system accounts.

**User creation commands:**
```bash
# Linux
sudo groupadd --system {internal_name}
sudo useradd --system --no-create-home --shell /usr/sbin/nologin -g {internal_name} {internal_name}

# FreeBSD
sudo pw groupadd {internal_name}
sudo pw useradd {internal_name} -g {internal_name} -s /usr/sbin/nologin -d /nonexistent
```

**macOS service account:** created via `dscl` (phased: reserve UID → create user/group → set shell/home to nologin/nonexistent); Go helper `findAvailableMacOSSystemID`/`createMacOSServiceUser`.

**Windows service account:** Virtual Service Account (VSA), auto-managed by Windows — no manual user creation. Created via `New-Service`/Go `golang.org/x/sys/windows/svc/mgr` `CreateService`; ACLs granted to `NT SERVICE\{internal_name}`.

**Directory ownership (set by root before privilege drop):** `chown -R {internal_name}:{internal_name}` on config/data/cache/log dirs; `chmod 755` on general dirs, `chmod 700` on `security/`, `ssl/`, `tor/` subdirectories under config.

**User service vs system service:**
| Aspect | System Service | User Service |
|--------|---------------|--------------|
| Installed by | root/admin | Unprivileged OS user |
| Runs as | root → drops to `{project_name}` | Calling user |
| Ports | Any | >1024 only |
| Paths | `/etc/`, `/var/` | `~/.config/`, `~/.local/` |
| Survives logout | Yes | Depends on `lingering` |
| Auto-start | On boot | On user login |
| Service file | `/etc/systemd/system/` | `~/.config/systemd/user/` |

**Service unit files by platform:**
| Platform | Install path |
|----------|---------------|
| systemd (Linux) | `/etc/systemd/system/{internal_name}.service` |
| OpenRC (Alpine/Gentoo/Devuan) | `/etc/init.d/{internal_name}` (shell script) |
| SysVinit (legacy Linux) | `/etc/init.d/{internal_name}` (only if OpenRC/systemd absent) |
| runit (Linux) | `/etc/sv/{internal_name}/` (`run`, `log/run`, `supervise/`) |
| rc.d (FreeBSD) | `/usr/local/etc/rc.d/{internal_name}` |
| launchd (macOS) | `/Library/LaunchDaemons/{plist_name}.plist` |
| Windows | Windows Service Control Manager, VSA `NT SERVICE\{internal_name}` |

**Service manager detection order (Unix):** system service presence is checked via file existence (`/etc/systemd/system/{internal_name}.service`, `/Library/LaunchDaemons/{plist_name}.plist`, `/usr/local/etc/rc.d/{internal_name}`) — if any exists, managing it (start/stop/restart/reload/uninstall/disable) requires elevation; otherwise it's treated as a user service (no escalation).

**systemd unit requirements:** `Type=simple`, `Restart=on-failure`, `RestartSec=5`, journal logging, security hardening (`ProtectSystem=strict`, `ProtectHome=yes`, `PrivateTmp=yes`), explicit `ReadWritePaths=` for `/etc`, `/var/lib`, `/var/cache`, `/var/log` under `{internal_org}/{internal_name}`.

**OpenRC requirements:** `command_user="{internal_name}:{internal_name}"`, `pidfile`, `command_background=true`, separate `output_log`/`error_log`, `depend()` on `net`, `start_pre()` creates pid/log dirs with correct ownership via `checkpath`.

**SysVinit detection:** only installed when `/sbin/openrc-run` is absent, `systemctl` is absent, and `/etc/init.d/` + working `update-rc.d`/`chkconfig` exist. Uses `start-stop-daemon --chuid {internal_name}`.

**launchd plist requirements:** `RunAtLoad=true`, `KeepAlive=true`, `StandardOutPath`/`StandardErrorPath` under `/var/log/{internal_org}/{internal_name}/`.

**rc.d (FreeBSD) requirements:** `# PROVIDE:`, `# REQUIRE: NETWORKING`, `# KEYWORD: shutdown`, sources `/etc/rc.subr`, uses `load_rc_config`/`run_rc_command`.

**Service management CLI commands per platform:**
```bash
# systemd
sudo systemctl {start|stop|restart|status|enable|disable} {internal_name}

# OpenRC
sudo rc-update add {internal_name} default
sudo rc-service {internal_name} {start|stop|restart|status}
sudo rc-update del {internal_name} default

# SysVinit
sudo update-rc.d {internal_name} defaults        # Debian-style
sudo chkconfig --add {internal_name} && sudo chkconfig {internal_name} on  # RHEL-style
sudo /etc/init.d/{internal_name} {start|stop|restart|status}
sudo service {internal_name} start

# launchd (macOS)
sudo launchctl load /Library/LaunchDaemons/{plist_name}.plist
sudo launchctl unload /Library/LaunchDaemons/{plist_name}.plist
sudo launchctl list | grep {project_name}
```

**Windows Go integration:** `golang.org/x/sys/windows/svc` — `svc.Run("{project_name}", &windowsService{})`; `Execute()` handles `svc.Stop`/`svc.Shutdown` for clean shutdown.

**Project variables:** `project_name=vidveil`, `project_org=apimgr`, `internal_name=vidveil`, `internal_org=apimgr`, `plist_name=io.github.apimgr.vidveil`, `official_site=https://x.scour.li`.

For complete details, see AI.md PART 23, 24
