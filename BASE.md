# {PROJECTNAME} Specification

**This project inherits from and extends BASE.md. BASE.md rules cannot be overridden.**

## Project Information

| Field | Value |
|-------|-------|
| **Name** | {projectname} |
| **Organization** | apimgr |
| **Official Site** | https://{projectname}.apimgr.us |
| **Repository** | https://github.com/apimgr/{projectname} |
| **License** | MIT |

## Project Description

{Brief description of what this project does}

## Project-Specific Features

{List features unique to this project that extend BASE.md}

---

# Directory Structures

## Linux

### Privileged (root/sudo)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/{projectname}` |
| Config | `/etc/apimgr/{projectname}/` |
| Config File | `/etc/apimgr/{projectname}/server.yml` |
| Data | `/var/lib/apimgr/{projectname}/` |
| Logs | `/var/log/apimgr/{projectname}/` |
| Backup | `/mnt/Backups/apimgr/{projectname}/` |
| PID File | `/var/run/apimgr/{projectname}.pid` |
| SSL Certs | `/etc/apimgr/{projectname}/ssl/certs/` |
| SQLite DB | `/var/lib/apimgr/{projectname}/db/` |
| GeoIP | `/var/lib/apimgr/{projectname}/geoip/` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/{projectname}` |
| Config | `~/.config/apimgr/{projectname}/` |
| Config File | `~/.config/apimgr/{projectname}/server.yml` |
| Data | `~/.local/share/apimgr/{projectname}/` |
| Logs | `~/.local/share/apimgr/{projectname}/logs/` |
| Backup | `~/.local/backups/apimgr/{projectname}/` |
| PID File | `~/.local/share/apimgr/{projectname}/{projectname}.pid` |
| SSL Certs | `~/.config/apimgr/{projectname}/ssl/certs/` |
| SQLite DB | `~/.local/share/apimgr/{projectname}/db/` |
| GeoIP | `~/.local/share/apimgr/{projectname}/geoip/` |

---

## macOS

### Privileged (root/sudo)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/{projectname}` |
| Config | `/Library/Application Support/apimgr/{projectname}/` |
| Config File | `/Library/Application Support/apimgr/{projectname}/server.yml` |
| Data | `/Library/Application Support/apimgr/{projectname}/data/` |
| Logs | `/Library/Logs/apimgr/{projectname}/` |
| Backup | `/Library/Backups/apimgr/{projectname}/` |
| PID File | `/var/run/apimgr/{projectname}.pid` |
| SSL Certs | `/Library/Application Support/apimgr/{projectname}/ssl/certs/` |
| SQLite DB | `/Library/Application Support/apimgr/{projectname}/db/` |
| GeoIP | `/Library/Application Support/apimgr/{projectname}/geoip/` |
| LaunchDaemon | `/Library/LaunchDaemons/com.apimgr.{projectname}.plist` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/bin/{projectname}` or `/usr/local/bin/{projectname}` |
| Config | `~/Library/Application Support/apimgr/{projectname}/` |
| Config File | `~/Library/Application Support/apimgr/{projectname}/server.yml` |
| Data | `~/Library/Application Support/apimgr/{projectname}/` |
| Logs | `~/Library/Logs/apimgr/{projectname}/` |
| Backup | `~/Library/Backups/apimgr/{projectname}/` |
| PID File | `~/Library/Application Support/apimgr/{projectname}/{projectname}.pid` |
| SSL Certs | `~/Library/Application Support/apimgr/{projectname}/ssl/certs/` |
| SQLite DB | `~/Library/Application Support/apimgr/{projectname}/db/` |
| GeoIP | `~/Library/Application Support/apimgr/{projectname}/geoip/` |
| LaunchAgent | `~/Library/LaunchAgents/com.apimgr.{projectname}.plist` |

---

## BSD (FreeBSD, OpenBSD, NetBSD)

### Privileged (root/sudo/doas)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/{projectname}` |
| Config | `/usr/local/etc/apimgr/{projectname}/` |
| Config File | `/usr/local/etc/apimgr/{projectname}/server.yml` |
| Data | `/var/db/apimgr/{projectname}/` |
| Logs | `/var/log/apimgr/{projectname}/` |
| Backup | `/var/backups/apimgr/{projectname}/` |
| PID File | `/var/run/apimgr/{projectname}.pid` |
| SSL Certs | `/usr/local/etc/apimgr/{projectname}/ssl/certs/` |
| SQLite DB | `/var/db/apimgr/{projectname}/db/` |
| GeoIP | `/var/db/apimgr/{projectname}/geoip/` |
| RC Script | `/usr/local/etc/rc.d/{projectname}` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/{projectname}` |
| Config | `~/.config/apimgr/{projectname}/` |
| Config File | `~/.config/apimgr/{projectname}/server.yml` |
| Data | `~/.local/share/apimgr/{projectname}/` |
| Logs | `~/.local/share/apimgr/{projectname}/logs/` |
| Backup | `~/.local/backups/apimgr/{projectname}/` |
| PID File | `~/.local/share/apimgr/{projectname}/{projectname}.pid` |
| SSL Certs | `~/.config/apimgr/{projectname}/ssl/certs/` |
| SQLite DB | `~/.local/share/apimgr/{projectname}/db/` |
| GeoIP | `~/.local/share/apimgr/{projectname}/geoip/` |

---

## Windows

### Privileged (Administrator)

| Type | Path |
|------|------|
| Binary | `C:\Program Files\apimgr\{projectname}\{projectname}.exe` |
| Config | `%ProgramData%\apimgr\{projectname}\` |
| Config File | `%ProgramData%\apimgr\{projectname}\server.yml` |
| Data | `%ProgramData%\apimgr\{projectname}\data\` |
| Logs | `%ProgramData%\apimgr\{projectname}\logs\` |
| Backup | `%ProgramData%\Backups\apimgr\{projectname}\` |
| SSL Certs | `%ProgramData%\apimgr\{projectname}\ssl\certs\` |
| SQLite DB | `%ProgramData%\apimgr\{projectname}\db\` |
| GeoIP | `%ProgramData%\apimgr\{projectname}\geoip\` |
| Service | Windows Service Manager |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `%LocalAppData%\apimgr\{projectname}\{projectname}.exe` |
| Config | `%AppData%\apimgr\{projectname}\` |
| Config File | `%AppData%\apimgr\{projectname}\server.yml` |
| Data | `%LocalAppData%\apimgr\{projectname}\` |
| Logs | `%LocalAppData%\apimgr\{projectname}\logs\` |
| Backup | `%LocalAppData%\Backups\apimgr\{projectname}\` |
| SSL Certs | `%AppData%\apimgr\{projectname}\ssl\certs\` |
| SQLite DB | `%LocalAppData%\apimgr\{projectname}\db\` |
| GeoIP | `%LocalAppData%\apimgr\{projectname}\geoip\` |

---

## Docker/Container

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/{projectname}` |
| Config | `/config/` |
| Config File | `/config/server.yml` |
| Data | `/data/` |
| Logs | `/data/logs/` |
| SQLite DB | `/data/db/` |
| GeoIP | `/data/geoip/` |
| Internal Port | `80` |

---

# Privilege Escalation & User Creation

## Overview

Application user creation **REQUIRES** privilege escalation. If the user cannot escalate privileges, the application runs as the current user with user-level directories.

## Escalation Detection by OS

### Linux
```
Escalation Methods (in order of preference):
1. Already root (EUID == 0)
2. sudo (if user is in sudoers/wheel group)
3. su (if user knows root password)
4. pkexec (PolicyKit, if available)
5. doas (OpenBSD-style, if configured)

Detection:
- Check EUID: os.Geteuid() == 0
- Check sudo: exec.LookPath("sudo") && user in sudo/wheel group
- Check su: exec.LookPath("su")
- Check pkexec: exec.LookPath("pkexec")
- Check doas: exec.LookPath("doas") && /etc/doas.conf exists
```

### macOS
```
Escalation Methods (in order of preference):
1. Already root (EUID == 0)
2. sudo (user must be in admin group)
3. osascript with administrator privileges (GUI prompt)

Detection:
- Check EUID: os.Geteuid() == 0
- Check sudo: exec.LookPath("sudo") && user in admin group
- GUI available: os.Getenv("DISPLAY") != "" or always try osascript
```

### BSD (FreeBSD, OpenBSD, NetBSD)
```
Escalation Methods (in order of preference):
1. Already root (EUID == 0)
2. doas (OpenBSD default, others if configured)
3. sudo (if installed and configured)
4. su (if user knows root password)

Detection:
- Check EUID: os.Geteuid() == 0
- Check doas: exec.LookPath("doas") && /etc/doas.conf exists
- Check sudo: exec.LookPath("sudo")
- Check su: exec.LookPath("su")
```

### Windows
```
Escalation Methods (in order of preference):
1. Already Administrator (elevated token)
2. UAC prompt (requires GUI)
3. runas (command line, requires admin password)

Detection:
- Check Admin: windows.GetCurrentProcessToken().IsElevated()
- UAC available: GUI session detected
- runas: always available but requires password
```

## User Creation Logic

```
ON --service --install:

1. Check if can escalate privileges
   ├─ YES: Continue with privileged installation
   │   ├─ Create system user/group (UID/GID 100-999)
   │   ├─ Use system directories (/etc, /var/lib, /var/log)
   │   ├─ Install service (systemd/launchd/rc.d/Windows Service)
   │   └─ Set ownership to created user
   │
   └─ NO: Fall back to user installation
       ├─ Skip user creation (run as current user)
       ├─ Use user directories (~/.config, ~/.local/share)
       ├─ Skip system service installation
       └─ Offer alternative (cron, user systemd, launchctl user agent)
```

## System User Requirements

When creating a system user (privileged only):

| Requirement | Value |
|-------------|-------|
| Username | `{projectname}` |
| Group | `{projectname}` |
| UID/GID | Auto-detect unused in range 100-999 |
| Shell | `/sbin/nologin` or `/usr/sbin/nologin` |
| Home | Config or data directory |
| Type | System user (no password, no login) |
| Gecos | `{projectname} service account` |

### User Creation Commands by OS

**Linux:**
```bash
# Find unused UID/GID
for id in $(seq 100 999); do
  if ! getent passwd $id && ! getent group $id; then
    echo $id; break
  fi
done

# Create group and user
groupadd -r -g {UID} {projectname}
useradd -r -u {UID} -g {projectname} -s /sbin/nologin \
  -d /var/lib/apimgr/{projectname} -c "{projectname} service" {projectname}
```

**macOS:**
```bash
# Find unused UID/GID (use dscl)
dscl . -list /Users UniqueID | awk '{print $2}' | sort -n
# Pick unused ID in 100-999

# Create group and user
dscl . -create /Groups/{projectname}
dscl . -create /Groups/{projectname} PrimaryGroupID {GID}
dscl . -create /Users/{projectname}
dscl . -create /Users/{projectname} UniqueID {UID}
dscl . -create /Users/{projectname} PrimaryGroupID {GID}
dscl . -create /Users/{projectname} UserShell /usr/bin/false
dscl . -create /Users/{projectname} NFSHomeDirectory /Library/Application\ Support/apimgr/{projectname}
```

**BSD:**
```bash
# FreeBSD
pw groupadd {projectname} -g {GID}
pw useradd {projectname} -u {UID} -g {projectname} -s /sbin/nologin \
  -d /var/db/apimgr/{projectname} -c "{projectname} service"

# OpenBSD
groupadd -g {GID} {projectname}
useradd -u {UID} -g {projectname} -s /sbin/nologin \
  -d /var/db/apimgr/{projectname} -c "{projectname} service" {projectname}
```

**Windows:**
```powershell
# Windows doesn't typically create service users
# Services run as LocalSystem, LocalService, NetworkService, or a domain account
# For isolation, can create local user (requires admin):

net user {projectname} /add /active:no
# Or use a managed service account (domain environments)
```

## Privilege Check Flow

```
START
  │
  ├─ Check: Am I running as root/admin?
  │   ├─ YES → Use privileged paths, can create user
  │   └─ NO → Continue to escalation check
  │
  ├─ Check: Can I escalate privileges?
  │   │
  │   ├─ Linux:
  │   │   ├─ Can sudo? (sudo -n true 2>/dev/null)
  │   │   ├─ Can doas? (doas -n true 2>/dev/null)
  │   │   ├─ Can pkexec? (pkexec --help 2>/dev/null)
  │   │   └─ Has su access? (harder to detect without password)
  │   │
  │   ├─ macOS:
  │   │   ├─ Can sudo? (sudo -n true 2>/dev/null)
  │   │   └─ In admin group? (groups | grep -q admin)
  │   │
  │   ├─ BSD:
  │   │   ├─ Can doas? (doas -n true 2>/dev/null)
  │   │   ├─ Can sudo? (sudo -n true 2>/dev/null)
  │   │   └─ Has su access?
  │   │
  │   └─ Windows:
  │       └─ Can elevate? (check UAC settings, admin group membership)
  │
  ├─ CAN ESCALATE:
  │   ├─ Prompt: "Installation requires administrator privileges. Continue? [Y/n]"
  │   ├─ If Yes: Re-execute with escalation
  │   │   ├─ Linux: sudo/doas/pkexec {binary} --service --install
  │   │   ├─ macOS: sudo {binary} --service --install
  │   │   ├─ BSD: doas/sudo {binary} --service --install
  │   │   └─ Windows: Trigger UAC elevation
  │   └─ If No: Fall back to user installation
  │
  └─ CANNOT ESCALATE:
      ├─ Warn: "Cannot obtain administrator privileges."
      ├─ Warn: "Installing for current user only."
      ├─ Use user-level directories
      ├─ Skip system user creation
      └─ Offer user-level service alternatives:
          ├─ Linux: systemctl --user, cron @reboot
          ├─ macOS: launchctl user agent
          ├─ BSD: cron @reboot
          └─ Windows: Task Scheduler (current user)
```

## Installation Output Examples

### Privileged Installation (Success)
```
🔐 Administrator privileges detected

📦 Installing {projectname}...

Creating system user:
  ✓ Group '{projectname}' created (GID: 847)
  ✓ User '{projectname}' created (UID: 847)

Creating directories:
  ✓ /etc/apimgr/{projectname}
  ✓ /var/lib/apimgr/{projectname}
  ✓ /var/log/apimgr/{projectname}

Installing binary:
  ✓ /usr/local/bin/{projectname}

Installing service:
  ✓ /etc/systemd/system/{projectname}.service
  ✓ Service enabled

📋 Configuration file created:
   /etc/apimgr/{projectname}/server.yml

🔑 Admin credentials (SAVE THESE - shown only once):
   Username: administrator
   Password: xK9#mP2$vL5@nQ8
   API Token: apimgr_7f8a9b2c3d4e5f6a7b8c9d0e1f2a3b4c

✅ Installation complete!

To start the service:
  sudo systemctl start {projectname}

To check status:
  sudo systemctl status {projectname}
```

### User Installation (No Privileges)
```
⚠️  Cannot obtain administrator privileges
📦 Installing {projectname} for current user...

Creating directories:
  ✓ ~/.config/apimgr/{projectname}
  ✓ ~/.local/share/apimgr/{projectname}
  ✓ ~/.local/share/apimgr/{projectname}/logs

Installing binary:
  ✓ ~/.local/bin/{projectname}

📋 Configuration file created:
   ~/.config/apimgr/{projectname}/server.yml

🔑 Admin credentials (SAVE THESE - shown only once):
   Username: administrator
   Password: xK9#mP2$vL5@nQ8
   API Token: apimgr_7f8a9b2c3d4e5f6a7b8c9d0e1f2a3b4c

⚠️  System service not installed (requires administrator)

Alternative options:
  • Run manually: ~/.local/bin/{projectname}
  • Add to crontab: @reboot ~/.local/bin/{projectname}
  • User systemd: systemctl --user enable {projectname}

✅ Installation complete!
```

---

# API Endpoints

## Required Root Endpoints (from BASE.md)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/` | GET | None | Web interface (HTML) |
| `/healthz` | GET | None | Health check (HTML) |
| `/openapi` | GET | None | Swagger UI |
| `/openapi.json` | GET | None | OpenAPI spec (JSON) |
| `/openapi.yaml` | GET | None | OpenAPI spec (YAML) |
| `/graphql` | GET | None | GraphiQL interface |
| `/graphql` | POST | None | GraphQL queries |
| `/metrics` | GET | Optional | Prometheus metrics |
| `/admin` | GET | Session | Admin panel login |
| `/admin/*` | ALL | Session | Admin panel pages |

## Required API Endpoints (from BASE.md)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/healthz` | GET | None | Health check (JSON) |
| `/api/v1/admin/*` | ALL | Bearer | Admin API |

## Project-Specific Endpoints

{Define your project's unique endpoints here}

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/{resource}` | GET | None | List resources |
| `/api/v1/{resource}/{id}` | GET | None | Get single resource |

---

# Data Files

## Embedded Data (in binary)

| File | Location | Description |
|------|----------|-------------|
| `{data}.json` | `src/data/` | Main data file |

## External Data (downloaded)

| Data | Location | Update Schedule |
|------|----------|-----------------|
| GeoIP ASN | `{datadir}/geoip/asn.mmdb` | Weekly |
| GeoIP Country | `{datadir}/geoip/country.mmdb` | Weekly |
| GeoIP City | `{datadir}/geoip/city.mmdb` | Weekly |

---

# Project-Specific Configuration

{Add any configuration options unique to this project}

```yaml
# Project-specific settings (in addition to BASE.md config)
{projectname}:
  # Custom settings here
```

---

# Notes

{Any additional notes, decisions, or context for this project}

---

**Remember: This spec extends BASE.md. All BASE.md rules apply and cannot be overridden.**
