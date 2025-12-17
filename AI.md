# Vidveil - AI Working Notes

**Project Name**: vidveil
**Project Org**: apimgr
**Version**: 0.2.0
**Last Updated**: December 17, 2025
**Official Site**: https://scour.li
**Repository**: https://github.com/apimgr/vidveil

---

# PART 1: CORE RULES (READ FIRST - NON-NEGOTIABLE)

## Working Roles

When working on this project, the following roles are assumed based on the task:

- **Senior Go Developer** - Writing production-quality Go code, making architectural decisions, following best practices, optimizing performance
- **UI/UX Designer** - Creating professional, functional, visually appealing interfaces with excellent user experience
- **Beta Tester** - Testing applications, finding bugs, edge cases, and issues before they reach users
- **User** - Thinking from the end-user perspective, ensuring things are intuitive and work as expected

These are not roleplay - they ARE these roles when the work requires it. Each project gets the full expertise of all four perspectives.

---

## CRITICAL: Specification Compliance

**STOP AND READ THIS SECTION COMPLETELY BEFORE PROCEEDING.**

### The Golden Rules

1. **Re-read this spec periodically** during work to ensure accuracy and no deviation
2. **When in doubt, check the spec** - the spec is the source of truth
3. **Never assume or guess** - ask questions if unclear
4. **Every NON-NEGOTIABLE section MUST be implemented exactly as specified**
5. **Keep AI.md in sync with the project** - always update after changes

### Required Documentation Files

| File | Purpose | When to Read |
|------|---------|--------------|
| **AI.md** | Project-specific notes, must contain all spec rules | Read as needed, keep in sync |
| **TODO.AI.md** | Task tracking (REQUIRED when >2 tasks) | Read before work, update as tasks complete |

### Documentation Rules

- **AI.md MUST contain all spec rules** - merge this spec into AI.md
- **AI.md MUST always reflect current project state** - update after significant changes
- **TODO.AI.md MUST be used when doing more than 2 tasks** - keeps work organized
- **Migration**: If `CLAUDE.md` or `SPEC.md` exist, merge into `AI.md` and delete old files

---

## Development Principles (NON-NEGOTIABLE)

**EVERY principle below MUST be followed. No exceptions.**

| Principle | Description |
|-----------|-------------|
| **Validate Everything** | All input must be validated before processing |
| **Sanitize Appropriately** | Clean data where needed |
| **Save Only Valid Data** | Never persist invalid data |
| **Clear Only Invalid Data** | Don't destroy valid data |
| **Test Everything** | Comprehensive testing where applicable |
| **Show Tooltips/Docs** | Help users understand the interface |
| **Security First** | But security should never block usability |
| **Mobile First** | Responsive design for all screen sizes |
| **Sane Defaults** | Everything has sensible default values |
| **No AI/ML** | Smart logic only, no machine learning |
| **Concise Responses** | Short, descriptive, and helpful |
| **Everything Configurable** | ALL settings MUST be configurable via admin web UI |
| **Live Reload** | Configuration changes apply immediately without restart |

### Admin Web UI Configuration (NON-NEGOTIABLE)

**EVERY setting in the configuration file MUST be editable via the admin web UI.**

| Rule | Description |
|------|-------------|
| **No SSH/CLI required** | Users should NEVER need to edit config files manually |
| **Complete coverage** | 100% of `server.yml` settings available in admin panel |
| **Grouped logically** | Settings organized into intuitive sections |
| **Tooltips/help** | Every setting has a description explaining what it does |
| **Validation** | Real-time validation with clear error messages |
| **Defaults shown** | Show default values and current values clearly |

### Live Reload (NON-NEGOTIABLE)

**Configuration changes MUST apply immediately without server restart.**

| Rule | Description |
|------|-------------|
| **No restart required** | Changes take effect immediately after saving |
| **Hot reload** | Application watches for config changes and reloads |
| **Graceful** | In-flight requests complete before new config applies |
| **Feedback** | User sees confirmation that changes are active |
| **Exceptions** | Only port/address changes require restart (with clear warning) |

**What MUST live reload:**
- Branding (title, tagline, description)
- SEO settings
- Theme changes
- Email/notification settings
- Rate limiting rules
- robots.txt / security.txt
- Scheduler settings
- SSL settings (except port)
- Tor enable/disable
- All feature toggles

**What MAY require restart (with warning):**
- Listen address
- Port number
- Database driver change

### Sensitive Information Handling (NON-NEGOTIABLE)

**NEVER expose sensitive information unless absolutely necessary:**

- Tokens/passwords shown ONLY ONCE on generation (must be copied immediately)
- Show only on: first run, password changes, token regeneration
- Show in difficult environments: Docker, headless servers
- **NEVER log sensitive data**
- **NEVER in error messages or stack traces**
- Mask in UI: show `••••••••` or last 4 chars only

---

## Target Audience

- Self-hosted users
- SMB (Small/Medium Business)
- Enterprise
- **IMPORTANT: Assume self-hosted and SMB users are NOT tech-savvy**

---

# CHECKPOINT 1: CORE RULES VERIFICATION

Before proceeding, confirm you understand:
- [ ] All NON-NEGOTIABLE sections must be implemented exactly
- [ ] AI.md must be kept in sync with project state
- [ ] TODO.AI.md required for more than 2 tasks
- [ ] Sensitive data handling rules
- [ ] Target audience includes non-tech-savvy users

---

# PART 2: PROJECT STRUCTURE

## Project Information

| Field | Value |
|-------|-------|
| **Name** | vidveil |
| **Organization** | apimgr |
| **Official Site** | https://vidveil.apimgr.us |
| **Repository** | https://github.com/apimgr/vidveil |
| **README** | README.md |
| **License** | MIT > LICENSE.md |
| **Embedded Licenses** | Added to bottom of LICENSE.md |

## Project Description

Vidveil is a privacy-respecting meta search engine for adult video content. It aggregates results from 47 free adult video sites, providing a unified search interface with full Tor support for maximum privacy. Three major engines (PornHub, RedTube, Eporner) use direct API integration for instant results, while SSE streaming delivers results in real-time as each engine responds.

## Project-Specific Features

- **Meta Search**: Aggregates results from 47 adult video sites
- **Bang Search**: Use `!ph`, `!rt`, `!xh` shortcuts to search specific engines
- **Autocomplete**: Real-time bang suggestions as you type (`/api/v1/autocomplete`)
- **API Integration**: Direct API calls to PornHub (Webmasters API), RedTube (Public API), and Eporner (v2 API) for instant results
- **SSE Streaming**: Real-time result streaming via Server-Sent Events (`/api/v1/search/stream`)
- **Video-Only**: Focuses exclusively on video content
- **Standardized UA**: Firefox Windows 11 x64 user agent for all requests
- **Tor Support**: Built-in SOCKS5 proxy support for maximum privacy
- **Privacy First**: No tracking, no logging of searches
- **Age Verification**: Built-in age gate before content access
- **Video Preview**: Hover/swipe preview support

---

## Variables (NON-NEGOTIABLE)

| Variable | Description | This Project |
|----------|-------------|--------------|
| `{projectname}` | Project name | `vidveil` |
| `{projectorg}` | Organization name | `apimgr` |
| `{gitprovider}` | Git hosting provider | `github` |
| **Rule** | Anything in `{}` is a variable | |
| **Rule** | Anything NOT in `{}` is literal | `/etc/letsencrypt/live` is a real path |

## Local Project Path Structure (NON-NEGOTIABLE)

**Format:** `~/Projects/github/apimgr/vidveil`

| Component | Description | Examples |
|-----------|-------------|----------|
| `~/Projects/` | Base projects directory | Always `~/Projects/` |
| `github` | Git hosting provider or `local` | `github`, `gitlab`, `bitbucket`, `private`, `local` |
| `apimgr` | Organization/username | `apimgr`, `casjay`, `myorg` |
| `vidveil` | Project name | `jokes`, `icons`, `myproject` |

**Examples:**
```
~/Projects/github/apimgr/jokes        # GitHub, apimgr org, jokes project
~/Projects/gitlab/casjay/icons        # GitLab, casjay user, icons project
~/Projects/private/myorg/myproject    # Private/self-hosted git, myorg, myproject
~/Projects/bitbucket/company/app      # Bitbucket, company org, app project
~/Projects/local/apimgr/prototype     # Local only, no VCS, prototyping
```

### Special: `local` Provider

`~/Projects/local/apimgr/vidveil` is used for:
- **Prototyping** - Quick experiments and proof-of-concept
- **Bootstrapping** - Initial project setup before pushing to VCS
- **Local-only development** - Projects not intended for remote hosting
- **No VCS required** - May not have git initialized
- **No Docker registry** - May not push to container registries

**This is the LOCAL development path, not the deployed path.**

---

## Directory Structure (NON-NEGOTIABLE)

**The root Project directory is**: `./`

```
./                          # Root project directory
├── src/                    # All source files
├── scripts/                # All production/install scripts
├── tests/                  # All development/test scripts and files
├── binaries/               # Built binaries (gitignored)
├── releases/               # Release binaries (gitignored)
├── README.md               # Production first, dev last
├── SPEC.md                 # This specification file
├── LICENSE.md              # MIT + embedded licenses
├── AI.md                   # AI/Claude working notes
├── TODO.AI.md              # Task tracking for >2 tasks
└── release.txt             # Version tracking
```

**RULE: Keep the base directory organized and clean - no clutter!**

**The working directory is `.`**

## Runtime Directory Usage (NON-NEGOTIABLE)

**Be smart about which directory to use for what purpose.**

| Directory | Purpose | Examples |
|-----------|---------|----------|
| `{config_dir}` | User-editable configuration | `server.yml`, email templates, custom themes, SSL certs |
| `{data_dir}` | Application-managed data | databases, Tor keys, caches, GeoIP databases |
| `{log_dir}` | Log files | `access.log`, `error.log`, `audit.log` |
| `{backup_dir}` | Backup files | `.tar.gz` backup archives |

**Rules:**
- If a user might edit it → `{config_dir}`
- If the application manages it → `{data_dir}`
- If it's a log → `{log_dir}`
- If it's a backup → `{backup_dir}`
- **NEVER mix purposes** - don't put user config in data_dir or vice versa

---

## Platform Support (NON-NEGOTIABLE)

### Operating Systems

| OS | Required |
|----|----------|
| Linux | YES |
| BSD (FreeBSD, OpenBSD, etc.) | YES |
| macOS (Intel and Apple Silicon) | YES |
| Windows | YES |

### Architectures

| Architecture | Required |
|--------------|----------|
| AMD64 | YES |
| ARM64 | YES |

**IMPORTANT: Be smart about implementations - code must work on ALL platforms.**

---

## Go Version (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **Always Latest Stable** | Use latest stable Go version |
| **Build Only** | Go is only for building, not runtime (single static binary) |
| **go.mod** | Use latest stable version (e.g., `go 1.23` or newer) |
| **Docker** | Use `golang:latest` for build/test/debug |
| **No Pinning** | Don't pin to minor versions unless compatibility issue |

## Required Go Libraries (NON-NEGOTIABLE)

**All libraries MUST be pure Go and work with `CGO_ENABLED=0`.**

| Purpose | Library | Why |
|---------|---------|-----|
| **SQLite** | `modernc.org/sqlite` | Pure Go, no CGO required |
| **Tor** | `github.com/cretz/bine` | Pure Go Tor controller |
| **UUID** | `github.com/google/uuid` | Standard UUID generation |

### SQLite Driver (NON-NEGOTIABLE)

**MUST use `modernc.org/sqlite`. NEVER use `github.com/mattn/go-sqlite3`.**

| Driver | CGO Required | Use |
|--------|--------------|-----|
| `modernc.org/sqlite` | NO | **ALWAYS USE THIS** |
| `github.com/mattn/go-sqlite3` | YES | **NEVER USE** |

**Why `modernc.org/sqlite`?**
- Pure Go implementation - no C compiler needed
- Works with `CGO_ENABLED=0` for static binaries
- Cross-compilation works without toolchain setup
- Same SQLite functionality, just pure Go

**Usage:**
```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
    // Driver name is "sqlite" (not "sqlite3")
    return sql.Open("sqlite", path)
}
```

**go.mod:**
```
require modernc.org/sqlite v1.29.1
```

### Forbidden Libraries

| Library | Reason | Alternative |
|---------|--------|-------------|
| `github.com/mattn/go-sqlite3` | Requires CGO | `modernc.org/sqlite` |
| `github.com/ooni/go-libtor` | Requires CGO | `github.com/cretz/bine` + external Tor |
| Any CGO library | Breaks static builds | Find pure Go alternative |

---

# CHECKPOINT 2: PROJECT STRUCTURE VERIFICATION

Before proceeding, confirm you understand:
- [ ] Project directory structure
- [ ] Variable syntax (`{}` = variable, no `{}` = literal)
- [ ] All 4 OSes must be supported
- [ ] Both AMD64 and ARM64 must be supported
- [ ] Always use latest stable Go

---

# PART 3: OS-SPECIFIC PATHS (NON-NEGOTIABLE)

## Linux

### Privileged (root/sudo)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/vidveil` |
| Config | `/etc/apimgr/vidveil/` |
| Config File | `/etc/apimgr/vidveil/server.yml` |
| Data | `/var/lib/apimgr/vidveil/` |
| Logs | `/var/log/apimgr/vidveil/` |
| Backup | `/mnt/Backups/apimgr/vidveil/` |
| PID File | `/var/run/apimgr/vidveil.pid` |
| SSL Certs | `/etc/apimgr/vidveil/ssl/certs/` |
| SQLite DB | `/var/lib/apimgr/vidveil/db/` |
| GeoIP | `/var/lib/apimgr/vidveil/geoip/` |
| Service | `/etc/systemd/system/vidveil.service` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/vidveil` |
| Config | `~/.config/apimgr/vidveil/` |
| Config File | `~/.config/apimgr/vidveil/server.yml` |
| Data | `~/.local/share/apimgr/vidveil/` |
| Logs | `~/.local/share/apimgr/vidveil/logs/` |
| Backup | `~/.local/backups/apimgr/vidveil/` |
| PID File | `~/.local/share/apimgr/vidveil/vidveil.pid` |
| SSL Certs | `~/.config/apimgr/vidveil/ssl/certs/` |
| SQLite DB | `~/.local/share/apimgr/vidveil/db/` |
| GeoIP | `~/.local/share/apimgr/vidveil/geoip/` |

---

## macOS

### Privileged (root/sudo)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/vidveil` |
| Config | `/Library/Application Support/apimgr/vidveil/` |
| Config File | `/Library/Application Support/apimgr/vidveil/server.yml` |
| Data | `/Library/Application Support/apimgr/vidveil/data/` |
| Logs | `/Library/Logs/apimgr/vidveil/` |
| Backup | `/Library/Backups/apimgr/vidveil/` |
| PID File | `/var/run/apimgr/vidveil.pid` |
| SSL Certs | `/Library/Application Support/apimgr/vidveil/ssl/certs/` |
| SQLite DB | `/Library/Application Support/apimgr/vidveil/db/` |
| GeoIP | `/Library/Application Support/apimgr/vidveil/geoip/` |
| Service | `/Library/LaunchDaemons/com.apimgr.vidveil.plist` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/bin/vidveil` or `/usr/local/bin/vidveil` |
| Config | `~/Library/Application Support/apimgr/vidveil/` |
| Config File | `~/Library/Application Support/apimgr/vidveil/server.yml` |
| Data | `~/Library/Application Support/apimgr/vidveil/` |
| Logs | `~/Library/Logs/apimgr/vidveil/` |
| Backup | `~/Library/Backups/apimgr/vidveil/` |
| PID File | `~/Library/Application Support/apimgr/vidveil/vidveil.pid` |
| SSL Certs | `~/Library/Application Support/apimgr/vidveil/ssl/certs/` |
| SQLite DB | `~/Library/Application Support/apimgr/vidveil/db/` |
| GeoIP | `~/Library/Application Support/apimgr/vidveil/geoip/` |
| Service | `~/Library/LaunchAgents/com.apimgr.vidveil.plist` |

---

## BSD (FreeBSD, OpenBSD, NetBSD)

### Privileged (root/sudo/doas)

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/vidveil` |
| Config | `/usr/local/etc/apimgr/vidveil/` |
| Config File | `/usr/local/etc/apimgr/vidveil/server.yml` |
| Data | `/var/db/apimgr/vidveil/` |
| Logs | `/var/log/apimgr/vidveil/` |
| Backup | `/var/backups/apimgr/vidveil/` |
| PID File | `/var/run/apimgr/vidveil.pid` |
| SSL Certs | `/usr/local/etc/apimgr/vidveil/ssl/certs/` |
| SQLite DB | `/var/db/apimgr/vidveil/db/` |
| GeoIP | `/var/db/apimgr/vidveil/geoip/` |
| Service | `/usr/local/etc/rc.d/vidveil` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/vidveil` |
| Config | `~/.config/apimgr/vidveil/` |
| Config File | `~/.config/apimgr/vidveil/server.yml` |
| Data | `~/.local/share/apimgr/vidveil/` |
| Logs | `~/.local/share/apimgr/vidveil/logs/` |
| Backup | `~/.local/backups/apimgr/vidveil/` |
| PID File | `~/.local/share/apimgr/vidveil/vidveil.pid` |
| SSL Certs | `~/.config/apimgr/vidveil/ssl/certs/` |
| SQLite DB | `~/.local/share/apimgr/vidveil/db/` |
| GeoIP | `~/.local/share/apimgr/vidveil/geoip/` |

---

## Windows

### Privileged (Administrator)

| Type | Path |
|------|------|
| Binary | `C:\Program Files\apimgr\vidveil\vidveil.exe` |
| Config | `%ProgramData%\apimgr\vidveil\` |
| Config File | `%ProgramData%\apimgr\vidveil\server.yml` |
| Data | `%ProgramData%\apimgr\vidveil\data\` |
| Logs | `%ProgramData%\apimgr\vidveil\logs\` |
| Backup | `%ProgramData%\Backups\apimgr\vidveil\` |
| SSL Certs | `%ProgramData%\apimgr\vidveil\ssl\certs\` |
| SQLite DB | `%ProgramData%\apimgr\vidveil\db\` |
| GeoIP | `%ProgramData%\apimgr\vidveil\geoip\` |
| Service | Windows Service Manager |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `%LocalAppData%\apimgr\vidveil\vidveil.exe` |
| Config | `%AppData%\apimgr\vidveil\` |
| Config File | `%AppData%\apimgr\vidveil\server.yml` |
| Data | `%LocalAppData%\apimgr\vidveil\` |
| Logs | `%LocalAppData%\apimgr\vidveil\logs\` |
| Backup | `%LocalAppData%\Backups\apimgr\vidveil\` |
| SSL Certs | `%AppData%\apimgr\vidveil\ssl\certs\` |
| SQLite DB | `%LocalAppData%\apimgr\vidveil\db\` |
| GeoIP | `%LocalAppData%\apimgr\vidveil\geoip\` |

---

## Docker/Container

| Type | Path |
|------|------|
| Binary | `/usr/local/bin/vidveil` |
| Config | `/config/` |
| Config File | `/config/server.yml` |
| Data | `/data/` |
| Logs | `/data/logs/` |
| SQLite DB | `/data/db/` |
| GeoIP | `/data/geoip/` |
| Internal Port | `80` |

---

# CHECKPOINT 3: PATH VERIFICATION

Before proceeding, confirm you understand:
- [ ] Each OS has specific paths for privileged and non-privileged users
- [ ] Config file is ALWAYS `server.yml` (not .yaml)
- [ ] Docker uses simplified paths (/config, /data)
- [ ] All paths follow the apimgr/vidveil pattern

---

# PART 4: PRIVILEGE ESCALATION & USER CREATION (NON-NEGOTIABLE)

## Overview

Application user creation **REQUIRES** privilege escalation. If the user cannot escalate privileges, the application runs as the current user with user-level directories.

## Escalation Detection by OS

### Linux
```
Escalation Methods (in order):
1. Already root (EUID == 0)
2. sudo (if user is in sudoers/wheel group)
3. su (if user knows root password)
4. pkexec (PolicyKit, if available)
5. doas (OpenBSD-style, if configured)
```

### macOS
```
Escalation Methods (in order):
1. Already root (EUID == 0)
2. sudo (user must be in admin group)
3. osascript with administrator privileges (GUI prompt)
```

### BSD
```
Escalation Methods (in order):
1. Already root (EUID == 0)
2. doas (OpenBSD default, others if configured)
3. sudo (if installed and configured)
4. su (if user knows root password)
```

### Windows
```
Escalation Methods (in order):
1. Already Administrator (elevated token)
2. UAC prompt (requires GUI)
3. runas (command line, requires admin password)
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

## System User Requirements (NON-NEGOTIABLE)

| Requirement | Value |
|-------------|-------|
| Username | `vidveil` |
| Group | `vidveil` |
| UID/GID | **Must match** - same value for both UID and GID |
| UID/GID Range | 100-999 (system user range) |
| Shell | `/sbin/nologin` or `/usr/sbin/nologin` |
| Home | Config directory (`/etc/apimgr/vidveil`) or data directory (`/var/lib/apimgr/vidveil`) |
| Type | System user (no password, no login) |
| Gecos | `vidveil service account` |

### UID/GID Selection Logic

**The UID and GID MUST be the same value.** Find an unused ID where both UID and GID are available:

```
1. Start at 999 (top of system range)
2. Check if UID is unused (not in /etc/passwd)
3. Check if GID is unused (not in /etc/group)
4. If both available → use this value for both UID and GID
5. If either taken → decrement and repeat
6. Stop at 100 (bottom of system range)
7. If no ID found → error (system has no available IDs)
```

### Go Implementation

```go
func findAvailableSystemID() (int, error) {
    // Start from top of system range, work down
    for id := 999; id >= 100; id-- {
        // Check if UID is available
        if _, err := user.LookupId(strconv.Itoa(id)); err == nil {
            continue
            // UID exists, try next
        }

        // Check if GID is available
        if _, err := user.LookupGroupId(strconv.Itoa(id)); err == nil {
            continue
            // GID exists, try next
        }

        // Both available
        return id, nil
    }
    return 0, fmt.Errorf("no available UID/GID in system range 100-999")
}
```

### Platform-Specific Commands

**Linux:**
```bash
# Create group with specific GID
groupadd --system --gid {id} vidveil

# Create user with matching UID, same primary group
useradd --system --uid {id} --gid {id} \
  --home-dir /etc/apimgr/vidveil \
  --shell /sbin/nologin \
  --comment "vidveil service account" \
  vidveil
```

### macOS Service Account (NON-NEGOTIABLE)

**macOS services (launchd) MUST NOT run as logged-in user or root.**

| Option | Security | Recommendation |
|--------|----------|----------------|
| root | HIGH privileges - dangerous | NO |
| Logged-in User | User privileges - insecure | NO |
| `_www` | Web server account | NO (wrong purpose) |
| **Dedicated service user** | Minimal privileges, isolated | **RECOMMENDED** |

**Default: Dedicated system user with matching UID/GID**

macOS uses `dscl` (Directory Service Command Line) to create system users. The user is hidden from login screen and has no shell access.

**macOS UID/GID Ranges:**

| Range | Purpose |
|-------|---------|
| 0-99 | System accounts (reserved) |
| 100-499 | System services (use this range) |
| 500+ | Regular users |

**Create Service Account:**
```bash
# Find available ID in 100-499 range (same logic as Linux but different range)
# Start at 499, work down until both UID and GID are available

# Create group with specific GID
dscl . -create /Groups/vidveil
dscl . -create /Groups/vidveil PrimaryGroupID {id}
dscl . -create /Groups/vidveil Password "*"

# Create user with matching UID
dscl . -create /Users/vidveil
dscl . -create /Users/vidveil UniqueID {id}
dscl . -create /Users/vidveil PrimaryGroupID {id}
dscl . -create /Users/vidveil UserShell /usr/bin/false
dscl . -create /Users/vidveil RealName "vidveil service account"
dscl . -create /Users/vidveil NFSHomeDirectory /usr/local/var/apimgr/vidveil
dscl . -create /Users/vidveil Password "*"

# Hide user from login window
dscl . -create /Users/vidveil IsHidden 1
```

**launchd plist with UserName:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>apimgr.vidveil</string>

    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/vidveil</string>
    </array>

    <!-- Run as dedicated service user, NOT root -->
    <key>UserName</key>
    <string>vidveil</string>

    <key>GroupName</key>
    <string>vidveil</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>WorkingDirectory</key>
    <string>/usr/local/var/apimgr/vidveil</string>

    <key>StandardOutPath</key>
    <string>/usr/local/var/log/apimgr/vidveil/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/apimgr/vidveil/stderr.log</string>
</dict>
</plist>
```

**macOS Directory Structure:**

| Directory | Path | Purpose |
|-----------|------|---------|
| Binary | `/usr/local/bin/vidveil` | Executable |
| Config | `/usr/local/etc/apimgr/vidveil/` | Configuration files |
| Data | `/usr/local/var/apimgr/vidveil/` | Application data |
| Logs | `/usr/local/var/log/apimgr/vidveil/` | Log files |
| launchd plist | `/Library/LaunchDaemons/apimgr.vidveil.plist` | Service definition |

**Go Implementation (macOS):**
```go
// +build darwin

func findAvailableMacOSSystemID() (int, error) {
    // macOS system services use 100-499 range
    for id := 499; id >= 100; id-- {
        // Check if UID is available
        if _, err := user.LookupId(strconv.Itoa(id)); err == nil {
            continue
        }

        // Check if GID is available
        if _, err := user.LookupGroupId(strconv.Itoa(id)); err == nil {
            continue
        }

        return id, nil
    }
    return 0, fmt.Errorf("no available UID/GID in macOS system range 100-499")
}

func createMacOSServiceUser(name string, id int, homeDir string) error {
    commands := [][]string{
        // Create group
        {"dscl", ".", "-create", "/Groups/" + name},
        {"dscl", ".", "-create", "/Groups/" + name, "PrimaryGroupID", strconv.Itoa(id)},
        {"dscl", ".", "-create", "/Groups/" + name, "Password", "*"},
        // Create user
        {"dscl", ".", "-create", "/Users/" + name},
        {"dscl", ".", "-create", "/Users/" + name, "UniqueID", strconv.Itoa(id)},
        {"dscl", ".", "-create", "/Users/" + name, "PrimaryGroupID", strconv.Itoa(id)},
        {"dscl", ".", "-create", "/Users/" + name, "UserShell", "/usr/bin/false"},
        {"dscl", ".", "-create", "/Users/" + name, "RealName", name + " service account"},
        {"dscl", ".", "-create", "/Users/" + name, "NFSHomeDirectory", homeDir},
        {"dscl", ".", "-create", "/Users/" + name, "Password", "*"},
        {"dscl", ".", "-create", "/Users/" + name, "IsHidden", "1"},
    }

    for _, cmd := range commands {
        if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
            return fmt.Errorf("failed to run %v: %w", cmd, err)
        }
    }
    return nil
}
```

**FreeBSD:**
```bash
# Create user and group with matching ID
pw groupadd -n vidveil -g {id}
pw useradd -n vidveil -u {id} -g {id} \
  -d /var/lib/apimgr/vidveil \
  -s /usr/sbin/nologin \
  -c "vidveil service account"
```

### Windows Service Account (NON-NEGOTIABLE)

**Windows services MUST NOT run as logged-in user or Administrator.**

| Option | Security | Recommendation |
|--------|----------|----------------|
| Local System | HIGH privileges - dangerous | NO |
| Administrator | HIGH privileges - dangerous | NO |
| Logged-in User | User privileges - insecure | NO |
| Local Service | Limited privileges | OK for network-less services |
| Network Service | Limited + network access | OK for network services |
| **Virtual Service Account** | Minimal privileges, isolated | **RECOMMENDED** |
| Managed Service Account | Domain-managed, auto-password | Enterprise environments |

**Default: Virtual Service Account (VSA)**

Virtual Service Accounts are automatically managed by Windows, require no password management, and have minimal privileges. They are created automatically when the service is installed.

**Service Account Format:** `NT SERVICE\vidveil`

```powershell
# Create service with Virtual Service Account (automatic)
New-Service -Name "vidveil" `
  -BinaryPathName "C:\Program Files\apimgr\vidveil\vidveil.exe" `
  -DisplayName "vidveil" `
  -Description "vidveil service" `
  -StartupType Automatic

# Service automatically runs as NT SERVICE\vidveil
# No user creation needed - Windows manages it
```

**Directory Permissions:**
```powershell
# Grant Virtual Service Account access to config/data directories
$acl = Get-Acl "C:\ProgramData\apimgr\vidveil"
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT SERVICE\vidveil",
    "FullControl",
    "ContainerInherit,ObjectInherit",
    "None",
    "Allow"
)
$acl.SetAccessRule($rule)
Set-Acl "C:\ProgramData\apimgr\vidveil" $acl
```

**Go Implementation (Windows):**
```go
// +build windows

import "golang.org/x/sys/windows/svc/mgr"

func installWindowsService() error {
    m, err := mgr.Connect()
    if err != nil {
        return err
    }
    defer m.Disconnect()

    exePath, _ := os.Executable()

    // Create service - runs as Virtual Service Account by default
    // when ServiceStartName is empty or "NT SERVICE\{name}"
    s, err := m.CreateService(
        "vidveil",
        exePath,
        mgr.Config{
            DisplayName:     "vidveil",
            Description:     "vidveil service",
            StartType:       mgr.StartAutomatic,
            ServiceStartName: "", // Empty = Virtual Service Account
        },
    )
    if err != nil {
        return err
    }
    defer s.Close()

    return nil
}
```

**Windows Directory Structure:**

| Directory | Path | Purpose |
|-----------|------|---------|
| Binary | `C:\Program Files\apimgr\vidveil\` | Executable |
| Config | `C:\ProgramData\apimgr\vidveil\config\` | Configuration files |
| Data | `C:\ProgramData\apimgr\vidveil\data\` | Application data |
| Logs | `C:\ProgramData\apimgr\vidveil\logs\` | Log files |

### Home Directory Selection

| Directory | Use When |
|-----------|----------|
| Config dir (`/etc/apimgr/vidveil`) | Default - user needs access to config files |
| Data dir (`/var/lib/apimgr/vidveil`) | When data dir contains user-writable content |

**Note:** Home directory must exist before user creation. Create directories first, then user, then set ownership.

---

# PART 5: SERVICE SUPPORT (NON-NEGOTIABLE)

## Built-in Service Support

**ALL projects MUST have built-in service support for ALL service managers:**

| Service Manager | OS |
|-----------------|-----|
| systemd | Linux |
| runit | Linux |
| Windows Service Manager | Windows |
| launchd | macOS |
| rc.d | BSD |

---

# PART 6: CONFIGURATION (NON-NEGOTIABLE)

## Configuration Source of Truth

| Mode | Source of Truth |
|------|-----------------|
| **Single Instance (file driver)** | Config file |
| **With Database** | Database (config file kept in sync) |

### Database Mode Details
- Database is source of truth
- Config file synced from database (one-way: db → config)
- /admin panel writes to database
- Changes propagate to all instances

## Boolean Handling (NON-NEGOTIABLE)

**Accept ALL of these values for booleans:**

| Truthy | Falsy |
|--------|-------|
| `1` | `0` |
| `yes` | `no` |
| `true` | `false` |
| `enable` | `disable` |
| `enabled` | `disabled` |
| `on` | `off` |

**Internally convert all to `true` or `false`.**

## Environment Variables (NON-NEGOTIABLE)

### Runtime Variables (Always Checked)

| Variable | Description |
|----------|-------------|
| `MODE` | `production` (default) or `development` |
| `DATABASE_DRIVER` | `file`, `sqlite`, `mariadb`, `mysql`, `postgres`, `mssql`, `mongodb` |
| `DATABASE_URL` | Database connection string |

### Init-Only Variables (First Run Only)

| Variable | Description |
|----------|-------------|
| `CONFIG_DIR` | Configuration directory |
| `DATA_DIR` | Data directory |
| `LOG_DIR` | Log directory |
| `BACKUP_DIR` | Backup directory |
| `DATABASE_DIR` | SQLite database directory |
| `PORT` | Server port |
| `LISTEN` | Listen address |
| `APPLICATION_NAME` | Application title |
| `APPLICATION_TAGLINE` | Application description |

**Init-only variables are used ONCE during first run, then ignored.**

---

## Configuration File (NON-NEGOTIABLE)

### Design Rules

| Rule | Description |
|------|-------------|
| **Clean & Intuitive** | Easy to read and understand |
| **Everything Configurable** | If it has a setting, it's in the config |
| **Sane Defaults** | Built-in defaults (no 1000-line configs) |
| **Comprehensive** | All options present (commented/defaulted) |
| **Comments** | Single-line, under 140 characters |

### Location

| User Type | Path |
|-----------|------|
| Root | `/etc/apimgr/vidveil/server.yml` |
| Regular | `~/.config/apimgr/vidveil/server.yml` |

### Migration

**If `server.yaml` found, auto-migrate to `server.yml` on startup.**

## Port Rules (NON-NEGOTIABLE)

**Default port is a random unused port in the 64000-64999 range.**

### Port Selection Logic

| Scenario | Port | Behavior |
|----------|------|----------|
| First run (no config) | Random 64xxx | Auto-select unused port in 64000-64999, **save to config** |
| Config specifies port | Use specified | Use exact port from config |
| Port in use | Error | Fail with clear error message |
| Privileged port (<1024) | Requires root | Warn if not running as root |

**IMPORTANT: Once a port is selected (randomly or specified), it is saved to `server.yml` and persists across restarts. The random selection only happens on first run when no port is configured.**

### Why Random 64xxx?

| Reason | Description |
|--------|-------------|
| **Avoid conflicts** | Most services use well-known ports; 64xxx is rarely used |
| **No root required** | Ports >1024 don't need root privileges |
| **Self-hosted friendly** | Users can run multiple instances without conflict |
| **Reverse proxy ready** | Designed to run behind nginx/caddy/traefik |

### Special Port Handling

| Port | Special Behavior |
|------|------------------|
| `80` | Enable Let's Encrypt HTTP-01 challenge |
| `443` | Enable Let's Encrypt TLS-ALPN-01 challenge, auto-enable SSL |
| `0` | Let OS assign any available port |
| `64000-64999` | Default range for random selection |

### Port Display Rules

| Rule | Description |
|------|-------------|
| Strip `:80` | Don't show port for HTTP on 80 |
| Strip `:443` | Don't show port for HTTPS on 443 |
| Always show others | Show port for all non-standard ports |

### Dual Port Support

**Applications can listen on two ports simultaneously for HTTP and HTTPS:**

| Format | Description | Example |
|--------|-------------|---------|
| Single | One port (HTTP or HTTPS based on SSL config) | `8090` |
| Dual | HTTP port, HTTPS port (comma-separated) | `8090,8443` |

**Dual Port Behavior:**

| HTTP Port | HTTPS Port | Behavior |
|-----------|------------|----------|
| Any | Any | Both ports active, HTTP redirects to HTTPS optional |
| 80 | 443 | Let's Encrypt challenges enabled, standard web ports |
| 64xxx | 64xxx+1 | Common pattern for random ports |

```yaml
# Single port (HTTP or HTTPS based on ssl.enabled)
server:
  port: 8090

# Dual port (HTTP on 8090, HTTPS on 8443)
server:
  port: "8090,8443"
  ssl:
    enabled: true

# Standard web ports
server:
  port: "80,443"
  ssl:
    enabled: true
    letsencrypt:
      enabled: true
```

### Configuration

```yaml
server:
  # Port options:
  # - Omit or empty: Random port in 64000-64999 range
  # - Single number: Use that exact port
  # - Dual (HTTP,HTTPS): "8090,8443" format
  # - 0: Let OS assign any available port
  port: 64580
```

### Admin Panel

Port can be changed via `/admin/server/settings`, but **requires server restart** (with warning shown to user).

### Example Structure

```yaml
# =============================================================================
# SERVER CONFIGURATION
# =============================================================================

server:
  port: {random}              # Default: random unused port in 64xxx range
  fqdn: {hostname}            # Auto-detected from host
  address: "[::]"             # [::] = all interfaces IPv4/IPv6
  mode: production            # production or development

  # Branding & SEO - see PART 10 for full details
  branding:
    title: "vidveil"
    tagline: ""
    description: ""
  seo:
    keywords: []

  # System user/group
  user: {auto}
  group: {auto}

  # PID file
  pidfile: true

  # Admin Panel
  admin:
    email: admin@{fqdn}
    username: administrator
    password: {auto}
    token: {auto}

  # SSL/TLS
  ssl:
    enabled: false
    cert_path: /etc/apimgr/vidveil/ssl/certs

    letsencrypt:
      enabled: false
      email: admin@{fqdn}
      challenge: http-01

  # Scheduler
  schedule:
    enabled: true

  # Rate Limiting
  rate_limit:
    enabled: true
    requests: 120
    window: 60

  # Database
  database:
    driver: file

# =============================================================================
# FRONTEND CONFIGURATION
# =============================================================================

web:
  ui:
    theme: dark
  cors: "*"
```

---

# CHECKPOINT 4: CONFIGURATION VERIFICATION

Before proceeding, confirm you understand:
- [ ] Config file is `server.yml` (not .yaml)
- [ ] Boolean handling accepts multiple truthy/falsy values
- [ ] Environment variables: some runtime, some init-only
- [ ] Config auto-created on first run with sane defaults

---

# PART 7: APPLICATION MODES (NON-NEGOTIABLE)

## Mode Detection Priority

1. `--mode` CLI flag (highest priority)
2. `MODE` environment variable
3. Default: `production`

## Production Mode (Default)

| Setting | Behavior |
|---------|----------|
| Logging | `info` level, minimal output |
| Debug endpoints | Disabled (`/debug/*` returns 404) |
| Error messages | Generic (no stack traces) |
| Panic recovery | Graceful (logs error, returns 500) |
| Template caching | Enabled |
| Static file caching | Enabled |
| Rate limiting | Enforced |
| Security headers | All enabled |
| Sensitive data | Never shown |

## Development Mode

| Setting | Behavior |
|---------|----------|
| Logging | `debug` level, verbose |
| Debug endpoints | Enabled (`/debug/pprof/*`) |
| Error messages | Detailed (stack traces) |
| Panic recovery | Verbose (full stack in response) |
| Template caching | Disabled |
| Static file caching | Disabled |
| Rate limiting | Relaxed/disabled |
| Security headers | Relaxed |
| Sensitive data | Can be shown (with warning) |

## Mode Shortcuts

| Shortcut | Mode |
|----------|------|
| `--mode dev` | development |
| `--mode development` | development |
| `--mode prod` | production |
| `--mode production` | production |

---

# PART 8: SSL/TLS & LET'S ENCRYPT (NON-NEGOTIABLE)

## Built-in Let's Encrypt Support

**ALL projects MUST have built-in Let's Encrypt support.**

### Supported Challenge Types

| Type | Description |
|------|-------------|
| DNS-01 | All providers and RFC2136 |
| TLS-ALPN-01 | TLS-based challenge |
| HTTP-01 | HTTP-based challenge |

### Certificate Management

| Action | Path |
|--------|------|
| Check first | `/etc/letsencrypt/live` (literal path) |
| Save to | `/etc/apimgr/vidveil/ssl/certs` |
| Auto-renewal | Via built-in scheduler |

---

# PART 9: SCHEDULER (NON-NEGOTIABLE)

## Built-in Scheduler

**ALL projects MUST have a built-in scheduler.**

### Purpose

- Certificate renewals
- Notification checks
- Other periodic tasks
- Configurable via configuration file

---

# PART 10: GEOIP (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have built-in GeoIP support using sapics/ip-location-db.**

GeoIP databases are NEVER embedded - they are downloaded on first run and updated via scheduler.

## Configuration

```yaml
server:
  geoip:
    enabled: true

    # Directory for downloaded MMDB files
    dir: "{data_dir}/geoip"

    # Update schedule: never, daily, weekly, monthly
    update: weekly

    # Block countries by ISO 3166-1 alpha-2 code
    deny_countries: []

    # Which databases to download and use
    # Full IPv4 and IPv6 support - always both
    databases:
      # ASN lookup - organization name and AS number
      asn: true
      # Country lookup - country code and name
      country: true
      # City lookup - city, region, postal, coordinates, timezone (larger download)
      city: true
```

## Database Sources

| Database | CDN URL |
|----------|---------|
| ASN | `https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb` |
| Country | `https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb` |
| City | `https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-city-mmdb/dbip-city.mmdb` |

## Admin Panel (/admin/server/geoip)

| Element | Type | Description |
|---------|------|-------------|
| Enable GeoIP | Toggle | Turn GeoIP on/off |
| Update schedule | Dropdown | never, daily, weekly, monthly |
| Deny countries | Tag input | ISO 3166-1 alpha-2 codes to block |
| ASN database | Toggle | Enable ASN lookups |
| Country database | Toggle | Enable country lookups |
| City database | Toggle | Enable city lookups |
| Last update | Read-only | When databases were last updated |
| Update now | Button | Force immediate update |

---

# PART 11: METRICS (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have built-in Prometheus-compatible metrics support.**

## Configuration

```yaml
server:
  metrics:
    enabled: false
    endpoint: /metrics

    # Include system metrics (CPU, memory, disk)
    include_system: true

    # Optional Bearer token for authentication
    token: ""
```

## Metrics Types

| Type | Description | Always Included |
|------|-------------|-----------------|
| Application | Request count, error rate, latency | Yes (when enabled) |
| System | CPU, memory, disk usage | Configurable |

## Admin Panel (/admin/server/metrics)

| Element | Type | Description |
|---------|------|-------------|
| Enable metrics | Toggle | Turn metrics on/off |
| Endpoint | Text input | Metrics endpoint path (default: /metrics) |
| Include system metrics | Toggle | Include CPU/memory/disk |
| Authentication token | Text input | Bearer token (empty = no auth) |

---

# PART 12: SERVER CONFIGURATION (NON-NEGOTIABLE)

## Request Limits

```yaml
server:
  limits:
    max_body_size: 10MB
    read_timeout: 30s
    write_timeout: 30s
    idle_timeout: 120s
```

| Setting | Description | Default |
|---------|-------------|---------|
| `max_body_size` | Maximum request body size | 10MB |
| `read_timeout` | Read timeout | 30s |
| `write_timeout` | Write timeout | 30s |
| `idle_timeout` | Idle connection timeout | 120s |

## Response Compression

```yaml
server:
  compression:
    enabled: true
    # Compression level 1-9
    level: 5
    # MIME types to compress
    types:
      - text/html
      - text/css
      - text/javascript
      - application/json
      - application/xml
```

## Trusted Proxies

```yaml
server:
  trusted_proxies:
    # Additional IPs/CIDRs to trust (private ranges always trusted)
    additional: []
```

## Session Configuration

```yaml
server:
  session:
    cookie_name: session_id
    # 30 days in seconds
    max_age: 2592000
    # auto, true, false
    secure: auto
    http_only: true
    # strict, lax, none
    same_site: lax
```

## Rate Limiting

```yaml
server:
  rate_limit:
    enabled: true
    # Requests allowed per window
    requests: 120
    # Window size in seconds
    window: 60
```

## Internationalization (i18n)

```yaml
server:
  i18n:
    default_language: en
    supported: [en]
```

## Cache Configuration

```yaml
server:
  cache:
    # Type: none (disabled), memory (default), redis, valkey, memcache
    type: memory

    # Connection settings (for redis, valkey, memcache)
    host: localhost
    port: 6379
    password: ""
    db: 0

    # Key prefix to avoid collisions
    prefix: "vidveil:"

    # Default TTL in seconds
    ttl: 3600
```

## Admin Panel (/admin/server/settings)

All settings above MUST be configurable via admin panel:

| Section | Settings |
|---------|----------|
| Request Limits | Body size, timeouts |
| Compression | Enable, level, MIME types |
| Trusted Proxies | Additional IPs/CIDRs |
| Session | Cookie name, max age, secure, http_only, same_site |
| Rate Limiting | Enable, requests, window |
| i18n | Default language, supported languages |
| Cache | Type, connection settings, prefix, TTL |

---

# PART 13: WEB FRONTEND (NON-NEGOTIABLE)

## Requirements

**ALL PROJECTS MUST HAVE A FANTASTIC FRONTEND BUILT IN.**

| Requirement | Description |
|-------------|-------------|
| Mobile Support | Full responsive design |
| HTML5 | Full web standards compliance |
| Accessibility | Full a11y support |
| UX | Readable, navigable, intuitive, self-explanatory |

## Technology Stack (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| **Go Templates** | ALL HTML uses Go `html/template` - NO EXCEPTIONS |
| Templates | Use partials (header, nav, body, footer, etc.) |
| Vanilla JS/CSS | Preferred, no frameworks unless necessary |
| **NO JS Alerts** | NEVER use default JavaScript alerts/confirms/prompts |
| Custom UI | Always use CSS modals, toast notifications |
| **NO Inline CSS** | NEVER use inline styles |

### HTML5 & CSS Over JavaScript (NON-NEGOTIABLE)

**Minimize JavaScript - prefer HTML5 and CSS solutions whenever possible.**

| Use Case | Use HTML5/CSS | Use JavaScript Only When |
|----------|---------------|--------------------------|
| Form validation | HTML5 `required`, `pattern`, `min`, `max`, `type="email"` | Complex cross-field validation |
| Collapsible sections | `<details>/<summary>` | Need animation or programmatic control |
| Tabs | CSS `:target` or radio button hack | Need deep linking or state management |
| Tooltips | CSS `::after` with `data-tooltip` | Need dynamic positioning |
| Modals | CSS `:target` selector | Need focus trap, escape key, backdrop click |
| Hover effects | CSS `:hover`, `:focus`, `:active` | Never - always CSS |
| Animations | CSS `@keyframes`, `transition` | Complex sequenced animations |
| Responsive design | CSS media queries | Never - always CSS |
| Toggle switches | CSS with hidden checkbox | Need state persistence |
| Dropdowns/menus | CSS `:focus-within` | Complex multi-level menus |
| Progress bars | HTML5 `<progress>` | Need dynamic updates |
| Sliders | HTML5 `<input type="range">` | Complex custom styling |
| Date pickers | HTML5 `<input type="date">` | Need custom calendar UI |
| Color pickers | HTML5 `<input type="color">` | Need swatches or advanced features |
| Accordions | `<details>/<summary>` | Need single-open behavior |

**JavaScript Guidelines:**
- **Last resort** - only when HTML5/CSS cannot achieve the functionality
- **Progressive enhancement** - features must work without JS where possible
- **No JS for styling** - unless it cannot be done in HTML5 and CSS
- **No JS for simple interactions** - hover, focus, basic toggles are CSS-only

**HTML5 Required (NON-NEGOTIABLE):**
- ALL HTML MUST be valid HTML5
- Use `<!DOCTYPE html>` declaration
- Use semantic HTML5 elements: `<header>`, `<nav>`, `<main>`, `<footer>`, `<article>`, `<section>`, `<aside>`
- Use HTML5 form elements: `<input type="email">`, `<input type="date">`, `<input type="number">`, etc.
- Use HTML5 attributes: `required`, `pattern`, `placeholder`, `autofocus`, `autocomplete`
- NO deprecated elements: `<center>`, `<font>`, `<marquee>`, `<blink>`, etc.
- NO deprecated attributes: `align`, `bgcolor`, `border`, `cellpadding`, etc.
- **Required for**: API calls, dynamic content loading, complex state, WebSockets
- **Size matters** - keep JS minimal, no large libraries for simple tasks

**Inline JavaScript - Allowed for simple operations:**
```html
<!-- Navigation -->
<button onclick="history.back()">Go Back</button>
<button onclick="history.forward()">Go Forward</button>
<button onclick="location.reload()">Refresh</button>

<!-- Clipboard -->
<button onclick="navigator.clipboard.writeText('text')">Copy</button>

<!-- Print -->
<button onclick="window.print()">Print</button>

<!-- Scroll -->
<button onclick="window.scrollTo(0,0)">Back to Top</button>

<!-- Simple toggles -->
<button onclick="document.getElementById('menu').classList.toggle('hidden')">Toggle Menu</button>

<!-- Form helpers -->
<button onclick="document.getElementById('myform').reset()">Reset Form</button>
<button onclick="document.getElementById('field').select()">Select All</button>

<!-- Simple show/hide -->
<button onclick="this.nextElementSibling.hidden = !this.nextElementSibling.hidden">Show/Hide</button>
```

**Rule:** Inline JS is fine for one-liner operations. Move to external JS file if:
- Logic exceeds one simple statement
- Same logic is repeated in multiple places
- Requires error handling or conditionals

### Go Templates (NON-NEGOTIABLE)

**ALL frontend HTML MUST use Go's `html/template` package.**

| Location | Purpose |
|----------|---------|
| `src/server/templates/` | All `.tmpl` template files |
| `src/server/templates/partials/` | Reusable template partials |
| `src/server/templates/layouts/` | Base layouts |
| `src/server/templates/pages/` | Page-specific templates |
| `src/server/static/` | Static assets (CSS, JS, images) |

**Template Structure (all files use `.tmpl` extension):**
```
src/server/templates/
├── layouts/
│   ├── base.tmpl           # Base layout with html, head, body
│   └── admin.tmpl          # Admin-specific layout
├── partials/
│   ├── header.tmpl         # Site header
│   ├── nav.tmpl            # Navigation
│   ├── footer.tmpl         # Site footer
│   ├── head.tmpl           # <head> contents (meta, css, etc.)
│   └── scripts.tmpl        # JavaScript includes
├── pages/
│   ├── index.tmpl          # Home page
│   ├── healthz.tmpl        # Health check page
│   └── error.tmpl          # Error pages (404, 500, etc.)
├── admin/
│   ├── dashboard.tmpl      # Admin dashboard
│   ├── settings.tmpl       # Settings page
│   └── ...
└── components/
    ├── modal.tmpl          # Reusable modal component
    ├── toast.tmpl          # Toast notifications
    └── ...
```

**Mandatory Partials (NON-NEGOTIABLE):**

ALL pages MUST use these partials to ensure consistent site-wide layout:

| Partial | Purpose | Required |
|---------|---------|----------|
| `header.tmpl` | Site header (logo, branding) | YES |
| `nav.tmpl` | Navigation menu | YES |
| `footer.tmpl` | Site footer (copyright, links) | YES |
| `head.tmpl` | `<head>` contents (meta, CSS) | YES |
| `scripts.tmpl` | JavaScript includes | YES |

**Page Structure (NON-NEGOTIABLE):**

```
┌─────────────────────────────────────────┐
│              <header>                   │  ← header.tmpl (logo, branding)
├─────────────────────────────────────────┤
│               <nav>                     │  ← nav.tmpl (TOP - navigation links)
├─────────────────────────────────────────┤
│                                         │
│              <main>                     │  ← Page content
│                                         │
├─────────────────────────────────────────┤
│              <footer>                   │  ← footer.tmpl (BOTTOM - info links)
└─────────────────────────────────────────┘
```

**Nav vs Footer (NON-NEGOTIABLE):**

| Element | Position | Purpose | Contents |
|---------|----------|---------|----------|
| `<nav>` | TOP | Navigation | Links to app sections, user menu |
| `<footer>` | BOTTOM | Information | About, Privacy, Contact, Help, GitHub, version |

**Nav contains (app navigation):**
- Home link
- App-specific sections (project-defined)
- User menu (right side):
  - If logged in: Username dropdown → Profile, Settings, Logout
  - If logged out: Login link

**Nav does NOT contain:**
- API link (users access via /openapi if needed)
- Admin link (don't advertise - admins know where it is)
- Help link (belongs in footer)

**Default Navigation (nav.tmpl):**

```
Desktop:
┌─────────────────────────────────────────────────────────────────┐
│  vidveil                                      [User Icon] │  ← Header
├─────────────────────────────────────────────────────────────────┤
│  Home  |  [App Section 1]  |  [App Section 2]  |  ...           │  ← Nav
└─────────────────────────────────────────────────────────────────┘

Mobile:
┌─────────────────────────────────────────────────────────────────┐
│  vidveil                                      [User Icon] │  ← Header
├─────────────────────────────────────────────────────────────────┤
│                                                      [☰ Menu]   │  ← Nav row
└─────────────────────────────────────────────────────────────────┘
                                              ┌───────────────────┐
                                              │  Home             │
                                              │  App Section 1    │  ← Slide-in
                                              │  App Section 2    │     from right
                                              │  ...              │
                                              └───────────────────┘
```

```html
<!-- Header bar: site name + user icon -->
<header class="header">
  <a href="/" class="site-brand">vidveil</a>

  <!-- User icon (always visible, far right) -->
  <div class="user-menu">
    {{ if .User }}
      <!-- Logged in: user icon dropdown -->
      <div class="dropdown">
        <button class="dropdown-toggle user-icon" aria-label="User menu">
          <svg>...</svg>
        </button>
        <div class="dropdown-menu">
          <span class="dropdown-header">{{ .User.Username }}</span>
          <a href="/user/profile">Profile</a>
          <a href="/user/settings">Settings</a>
          <hr />
          <a href="/auth/logout">Logout</a>
        </div>
      </div>
    {{ else }}
      <!-- Logged out: login icon -->
      <a href="/auth/login" class="user-icon" aria-label="Login">
        <svg>...</svg>
      </a>
    {{ end }}
  </div>
</header>

<!-- Nav bar: separate row below header -->
<nav class="nav" id="nav-menu">
  <!-- Desktop: inline links | Mobile: hamburger only -->
  <div class="nav-links">
    <a href="/">Home</a>
    <!-- App-specific sections (project-defined) -->
  </div>

  <!-- Mobile: hamburger menu toggle (far right of nav row) -->
  <button class="nav-toggle" aria-label="Toggle navigation">☰</button>
</nav>

<!-- Slide-in panel for mobile (hidden by default) -->
<div class="nav-panel" id="nav-panel">
  <a href="/">Home</a>
  <!-- App-specific sections (project-defined) -->
</div>

<!-- Overlay for mobile menu (click to close) -->
<div class="nav-overlay" onclick="closeNav()"></div>
```

**Mobile Menu Behavior:**
- Menu slides in from RIGHT edge
- Slides LEFT to open (right-to-left)
- Slides RIGHT to close (left-to-right)
- Overlay dims background, click to close
- User icon stays in header (NOT in menu) - keeps menu clean

**Smart Menu (NON-NEGOTIABLE):**
- If all nav links fit on screen → show inline links, NO hamburger
- If nav links overflow → show hamburger menu
- Detect dynamically on load and resize
- Don't show hamburger if not needed

**CSS Animation:**
```css
/* Mobile slide-in menu */
@media (max-width: 768px) {
  .nav {
    position: fixed;
    top: 0;
    right: -280px;           /* Hidden off-screen right */
    width: 280px;
    height: 100vh;
    transition: right 0.3s ease;
  }

  .nav.open {
    right: 0;                /* Slide in from right */
  }

  .nav-overlay {
    display: none;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
  }

  .nav.open + .nav-overlay {
    display: block;
  }
}
```

**Mobile Responsive Rules:**
- Nav row below header: inline links or hamburger
- User icon ALWAYS in header (never in menu)
- Menu slides from right edge
- Touch-friendly: minimum 44x44px tap targets
- Overlay closes menu on tap

**No Fixed/Pinned Elements (NON-NEGOTIABLE):**
- Header, nav, footer all scroll with page
- NOTHING pinned/fixed to viewport
- User scrolls down → header/nav scroll away
- User scrolls to bottom → footer appears

**Footer Position (NON-NEGOTIABLE):**
- Footer ALWAYS at bottom of page (not floating in middle)
- If content is short → footer still at bottom of viewport
- If content is long → footer below content (scroll to see)
- Use min-height layout to push footer down

```css
/* Footer at bottom, no fixed elements */
body {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

main {
  flex: 1;  /* Grows to push footer to bottom */
}

/* NO position: fixed or position: sticky on header/nav/footer */
```

**Footer contains (informational links):**
- Standard pages: About, Privacy, Contact, Help
- External links: GitHub
- Branding: project name, version, copyright

**Rule:** Every page template MUST include header, nav, and footer partials. No page may define its own header/nav/footer - use the shared partials only. This ensures:
- Consistent branding across all pages
- Single point of change for navigation updates
- Uniform user experience

**App-Specific Partials (Optional):**

Projects can create additional partials for functionality unique to that application. Place these in `partials/` alongside the mandatory ones.

| Example Partial | Project | Purpose |
|-----------------|---------|---------|
| `search-box.tmpl` | airports, jokes | Reusable search form component |
| `airport-card.tmpl` | airports | Airport info display card |
| `joke-card.tmpl` | jokes | Joke display with copy button |
| `map.tmpl` | airports | Embedded map component |
| `passphrase-generator.tmpl` | wordList | Generator form and output |
| `geoip-result.tmpl` | airports | GeoIP lookup result display |
| `code-block.tmpl` | gitignore | Syntax-highlighted code display |
| `pagination.tmpl` | any | Reusable pagination controls |
| `filters.tmpl` | any | Search/filter form for lists |
| `stats-card.tmpl` | any | Statistics display card |

**App-Specific Partial Structure:**
```
src/server/templates/
├── partials/
│   ├── header.tmpl         # MANDATORY - site header
│   ├── nav.tmpl            # MANDATORY - navigation
│   ├── footer.tmpl         # MANDATORY - site footer
│   ├── head.tmpl           # MANDATORY - <head> contents
│   ├── scripts.tmpl        # MANDATORY - JS includes
│   ├── search-box.tmpl     # APP-SPECIFIC - search component
│   ├── result-card.tmpl    # APP-SPECIFIC - result display
│   └── pagination.tmpl     # APP-SPECIFIC - pagination controls
```

**Usage in page templates:**
```go
{{ define "content" }}
<div class="search-section">
  {{ template "search-box" . }}
</div>

<div class="results">
  {{ range .Results }}
    {{ template "result-card" . }}
  {{ end }}
</div>

{{ template "pagination" .Pagination }}
{{ end }}
```

**Guidelines for app-specific partials:**
- Create a partial when the same HTML is used in 2+ places
- Keep partials focused on one component/purpose
- Pass only the data the partial needs
- Name clearly: `{thing}-{purpose}.tmpl` (e.g., `airport-card.tmpl`, `joke-list.tmpl`)

**Embedding Templates (NON-NEGOTIABLE):**

All templates and static assets MUST be embedded in the binary:

```go
package server

import "embed"

//go:embed templates/*.tmpl templates/**/*.tmpl
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS
```

**Template Usage:**
```go
{{ define "base" }}
<!DOCTYPE html>
<html lang="en">
<head>{{ template "head" . }}</head>
<body>
  {{ template "header" . }}
  {{ template "nav" . }}
  <main>{{ template "content" . }}</main>
  {{ template "footer" . }}
  {{ template "scripts" . }}
</body>
</html>
{{ end }}
```

### Embedded vs External Assets

| Type | Embedded in Binary | External (Downloaded) |
|------|-------------------|----------------------|
| Templates (`.tmpl`) | YES | NO |
| CSS files | YES | NO |
| JavaScript files | YES | NO |
| Images/Icons | YES | NO |
| Fonts | YES | NO |
| Application data (JSON) | YES | NO |
| GeoIP databases | NO | YES - downloaded on first run |
| Blocklists | NO | YES - downloaded on first run |
| SSL certificates | NO | YES - only when using ports 80,443 |

**External Data Rules:**
- Security-related data that needs frequent updates is NOT embedded
- Downloaded automatically on first run
- Updated automatically via built-in scheduler
- Scheduler updates configurable via admin panel
- SSL certificates only generated/managed when running on ports `80,443`

**Benefits:**
- Single static binary deployment
- No external file dependencies at runtime
- Consistent layout across all pages
- Reusable components (DRY principle)
- Auto-escaping for security (XSS prevention)

### CSS Rules

| Bad | Good |
|-----|------|
| `<div style="color: red;">` | `<div class="error-text">` |
| `style="margin: 10px;"` | `class="spacing-sm"` |

**All styles MUST be in CSS files, not HTML elements.**

### Frontend UI Elements (NON-NEGOTIABLE)

**NEVER use default JavaScript UI elements. ALWAYS use custom styled components.**

| NEVER Use | ALWAYS Use Instead |
|-----------|---------------------|
| `alert()` | Custom modal with CSS classes |
| `confirm()` | Custom confirmation modal |
| `prompt()` | Custom input modal or inline form |
| Plain text inputs for options | Dropdowns (`<select>`) |
| Plain text for yes/no | Checkboxes or toggle switches |
| Plain text for multiple options | Radio buttons or dropdown |
| Inline text entry | Only when truly needed (search, names, etc.) |

**UI Element Guidelines:**

| Element | When to Use |
|---------|-------------|
| **Dropdown (`<select>`)** | Selecting from predefined options |
| **Checkbox** | Boolean on/off, enable/disable |
| **Toggle Switch** | Boolean with visual feedback |
| **Radio Buttons** | Mutually exclusive options (2-5 choices) |
| **Dropdown** | Mutually exclusive options (>5 choices) |
| **Multi-select** | Multiple selections from list |
| **Text Input** | Free-form text (names, URLs, search) |
| **Textarea** | Multi-line free-form text |
| **Number Input** | Numeric values with spin buttons |
| **Date/Time Picker** | Date and time selection |
| **Color Picker** | Color selection |
| **File Upload** | File selection with drag-drop |

**Modal Requirements:**
- Custom CSS-styled modals (no browser defaults)
- Backdrop overlay
- Close button (X) in corner
- Click outside to close (optional, configurable)
- Escape key to close
- Focus trap (tab stays within modal)
- Animated entrance/exit
- **Auto-close on action** - clicking any action button (OK, Yes, No, Cancel, Save, Delete, Submit, etc.) automatically closes the modal after performing the action. User should never need to click an action then manually close.

**Toast/Notification Requirements:**
- Non-blocking notifications
- Auto-dismiss with configurable timeout
- Manual dismiss option
- Stacking for multiple notifications
- Types: success, error, warning, info
- Icon + message format

## Layout

| Screen Size | Width |
|-------------|-------|
| ≥ 720px | 90% (5% margins) |
| < 720px | 98% (1% margins) |
| Footer | Always centered, always at bottom |

## Themes

| Theme | Description |
|-------|-------------|
| **Dark** | Based on Dracula - **DEFAULT** |
| **Light** | Based on popular light theme |
| **Auto** | Based on user's system |

## Branding & SEO (NON-NEGOTIABLE)

**White labeling is cosmetic only - it changes what users see, not how the system works.**

### What Branding Changes

| Changes (User-Visible) | Does NOT Change (System) |
|------------------------|--------------------------|
| Page titles | Directory names (`vidveil/`) |
| Browser tab | System username (`vidveil`) |
| Header/logo text | Log filenames |
| Footer branding | Config paths |
| Email "From" name | Binary name |
| SEO meta tags | API routes |
| OpenGraph data | Service names |
| Swagger UI title | Container names |

### Configuration

```yaml
server:
  branding:
    title: "vidveil"           # Display name (e.g., "Jokes API")
    tagline: ""                      # Short slogan (e.g., "The best jokes API")
    description: ""                  # Longer description for SEO/about

  seo:
    # Project-specific - define per app
    keywords: []                     # e.g., ["jokes", "api", "humor", "free api"]
    author: ""                       # Author/organization name
    og_image: ""                     # OpenGraph image URL for social sharing
    twitter_handle: ""               # Twitter @handle for cards
```

### Where Branding Is Used

| Field | Used In |
|-------|---------|
| `title` | `<title>` tag, header, emails, footer, Swagger UI |
| `tagline` | Homepage hero section, meta description fallback |
| `description` | Meta description, OpenGraph description, about page |
| `keywords` | Meta keywords tag |
| `author` | Meta author tag |
| `og_image` | OpenGraph/Twitter card image |
| `twitter_handle` | Twitter card attribution |

### SEO Meta Tags (Generated)

```html
<head>
  <title>{title} - {tagline}</title>
  <meta name="description" content="{description}">
  <meta name="keywords" content="{keywords}">
  <meta name="author" content="{author}">

  <!-- OpenGraph -->
  <meta property="og:title" content="{title}">
  <meta property="og:description" content="{description}">
  <meta property="og:image" content="{og_image}">
  <meta property="og:type" content="website">
  <meta property="og:url" content="{current_url}">

  <!-- Twitter Card -->
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:title" content="{title}">
  <meta name="twitter:description" content="{description}">
  <meta name="twitter:image" content="{og_image}">
  <meta name="twitter:site" content="{twitter_handle}">
</head>
```

### Static Files

| File | Purpose | Generated |
|------|---------|-----------|
| `/sitemap.xml` | Site map for search engines | Yes - auto-generated |
| `/favicon.ico` | Browser favicon | Embedded default, customizable |

### Admin Panel (/admin/server/branding)

| Element | Type | Description |
|---------|------|-------------|
| Title | Text input | Application display name |
| Tagline | Text input | Short slogan |
| Description | Textarea | Longer description for SEO |
| Keywords | Tag input | SEO keywords (comma-separated) |
| Author | Text input | Author/organization |
| OG Image | File upload / URL | Social sharing image |
| Twitter Handle | Text input | @handle |
| Favicon | File upload / URL | Custom favicon |
| Logo | File upload / URL | Custom logo (header) |

### Image Sources

**Logo, favicon, and OG image can be from local file or remote URL.**

| Source | Format | Example |
|--------|--------|---------|
| Local file | File upload | Upload via admin panel |
| Remote URL | URL input | `https://example.com/logo.png` |
| Embedded default | - | Built-in fallback |

### Image Scaling

**Images are automatically scaled/resized as needed:**

| Image | Sizes Generated |
|-------|-----------------|
| Logo | Original, 200px width (header), 50px width (mobile) |
| Favicon | 16x16, 32x32, 48x48, 180x180 (apple-touch-icon), 192x192, 512x512 |
| OG Image | Original, 1200x630 (OpenGraph standard) |

**Scaling Rules:**
- Preserve aspect ratio
- Generate multiple sizes on upload/fetch
- Cache scaled versions locally
- Re-fetch remote URLs periodically (configurable, default: daily)
- Fallback to embedded default if remote URL fails

### Defaults

| Field | Default Value |
|-------|---------------|
| `title` | `vidveil` |
| `tagline` | Empty |
| `description` | Empty |
| `keywords` | Empty |
| All others | Empty |

**Rule:** If `title` is empty, fall back to `vidveil`. Other fields are optional.

## Announcements (NON-NEGOTIABLE)

**Admin messages shown in UI for downtime notices, updates, etc.**

### Configuration

```yaml
web:
  announcements:
    enabled: true
    # List of announcement messages
    messages: []
```

### Announcement Structure

```yaml
messages:
  - id: "maintenance-2025-01"
    type: warning
    # warning, info, error, success
    title: "Scheduled Maintenance"
    message: "The system will be down for maintenance on Jan 15, 2025 from 2-4 AM UTC."
    start: "2025-01-14T00:00:00Z"
    # When to start showing
    end: "2025-01-15T04:00:00Z"
    # When to stop showing
    dismissible: true
    # User can dismiss
```

### Admin Panel (/admin/server/announcements)

| Element | Type | Description |
|---------|------|-------------|
| Enable announcements | Toggle | Turn announcements on/off |
| Announcement list | Table | All announcements |
| Add announcement | Button | Create new announcement |
| Type | Dropdown | warning, info, error, success |
| Title | Text input | Short title |
| Message | Textarea | Full message content |
| Start date | Datetime picker | When to start showing |
| End date | Datetime picker | When to stop showing |
| Dismissible | Toggle | Allow users to dismiss |
| Delete | Button | Remove announcement |

## CORS (NON-NEGOTIABLE)

**Default CORS policy allows all origins (`*`).**

### Configuration

```yaml
web:
  # CORS origin configuration
  # - "*": Allow all origins (default)
  # - "https://example.com": Single origin
  # - "https://example.com,https://app.example.com": Multiple origins (comma-separated)
  # - "": Disable CORS headers entirely
  cors: "*"
```

### CORS Headers

| Header | Value |
|--------|-------|
| `Access-Control-Allow-Origin` | Configured origin(s) or `*` |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `Content-Type, Authorization, X-Request-ID, X-CSRF-Token` |
| `Access-Control-Allow-Credentials` | `true` (only when specific origin, not `*`) |
| `Access-Control-Max-Age` | `86400` (24 hours) |

### Behavior

| Scenario | Behavior |
|----------|----------|
| `cors: "*"` | Allow all origins, credentials NOT allowed |
| `cors: "https://example.com"` | Allow single origin, credentials allowed |
| `cors: "https://a.com,https://b.com"` | Allow listed origins, credentials allowed |
| `cors: ""` | No CORS headers (same-origin only) |
| Preflight (OPTIONS) | Return CORS headers, 204 No Content |

### Mode-Specific Behavior

| Mode | Default | Behavior |
|------|---------|----------|
| Production | Configured value | Strict - only configured origins allowed |
| Development | `*` | Permissive - allows all for easier testing |

### Admin Panel (/admin/server/web)

| Element | Type | Description |
|---------|------|-------------|
| CORS Origins | Text input | Comma-separated list of allowed origins |
| Allow All | Toggle | Quick toggle for `*` (all origins) |
| Preview | Read-only | Shows resulting CORS headers |

## CSRF Protection (NON-NEGOTIABLE)

**ALL forms MUST have CSRF protection.**

### Configuration

```yaml
web:
  csrf:
    enabled: true
    # Token length in bytes
    token_length: 32
    cookie_name: csrf_token
    header_name: X-CSRF-Token
    # Secure cookie: auto, true, false
    secure: auto
```

### Implementation

- All forms include hidden CSRF token field
- All non-GET requests validate CSRF token
- Token stored in cookie and must match form/header value
- Tokens regenerated on login

## Footer Customization (NON-NEGOTIABLE)

### Configuration

```yaml
web:
  footer:
    # Google Analytics tracking ID (empty = disabled)
    # Example: UA-936146-1 or G-XXXXXXXXXX
    tracking_id: ""

    # Cookie consent popup (EU GDPR compliance)
    cookie_consent:
      enabled: true
      message: "This site uses cookies for functionality and analytics."
      policy_url: ""
      # URL to privacy policy

    # Custom branding HTML above the Application Footer
    # - Not set or "": Use default branding (built-in)
    # - " " (space): Disable branding, show only Application Footer
    # - Custom HTML: Use your own branding
    custom_html: ""
```

### Available Footer Variables

| Variable | Description |
|----------|-------------|
| `{currentyear}` | Current year (e.g., 2025) |
| `vidveil` | Project name |
| `apimgr` | Organization name |
| `{projectversion}` | Application version |
| `{builddatetime}` | Build date/time |

### Default Application Footer (Always Shown)

```html
<footer class="footer">
  <!-- Standard page links (always first) -->
  <p>
    <a href="/server/about">About</a>
    <span>•</span>
    <a href="/server/privacy">Privacy</a>
    <span>•</span>
    <a href="/server/contact">Contact</a>
    <span>•</span>
    <a href="/server/help">Help</a>
  </p>

  <br />

  <!-- Application branding -->
  <p>
    <a href="https://github.com/apimgr/vidveil" target="_blank">vidveil</a>
    <span>•</span>
    <span>Made with ❤️</span>
    <span>•</span>
    <span>{projectversion}</span>
  </p>

  <br />

  <a href="/healthz">Last update: {builddatetime}</a>
</footer>
```

### Admin Panel (/admin/server/footer)

| Element | Type | Description |
|---------|------|-------------|
| Google Analytics ID | Text input | Tracking ID (empty = disabled) |
| Cookie consent enabled | Toggle | Show cookie consent popup |
| Cookie consent message | Textarea | Consent message |
| Privacy policy URL | Text input | Link to privacy policy |
| Custom branding HTML | Textarea | HTML above application footer |
| Preview | Preview pane | Shows rendered footer |

## Standard Pages (NON-NEGOTIABLE)

**ALL applications MUST have these standard pages. Content is defined per-application.**

### /server/about

**About the application - what it does, version info, and optionally Tor access.**

| Section | Description |
|---------|-------------|
| Application name | From branding config |
| Version | Current version |
| Description | From branding config or project-specific |
| Features | Key features list (project-specific) |
| Tor access | If Tor enabled, show .onion address with copy button |
| Links | GitHub, documentation, etc. |

**Tor Section (shown only if `server.tor.enabled: true`):**

```html
<!-- Example Tor section -->
<div class="tor-access">
  <h3>Tor Hidden Service</h3>
  <p>This application is also available via Tor for enhanced privacy.</p>
  <div class="onion-address">
    <code>{onion_address}</code>
    <button onclick="copyToClipboard()">[Copy]</button>
  </div>
  <p class="note">Requires Tor Browser or Tor-enabled browser.</p>
</div>
```

### /server/privacy

**Privacy policy - what data is collected, how it's used, retention, etc.**

| Section | Description |
|---------|-------------|
| Data collection | What data is collected |
| Data usage | How data is used |
| Data retention | How long data is kept |
| Third parties | What data is shared with third parties |
| Cookies | What cookies are used |
| User rights | How users can access/delete their data |
| Contact | How to contact for privacy concerns |

**Default template provided, customizable via admin panel.**

### /server/contact

**Contact form - sends message to admin or dedicated contact address.**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | Text | Yes | Sender's name |
| Email | Email | Yes | Sender's email (for reply) |
| Subject | Text | Yes | Message subject |
| Message | Textarea | Yes | Message body |
| Captcha | Captcha | Yes | Spam prevention |

**Submission sends email to `server.contact` address (or admin email if not set).**

### /server/help

**Help page - documentation and usage instructions for the application.**

| Section | Description |
|---------|-------------|
| Getting started | Quick start guide |
| Features | How to use key features |
| API Documentation | Links to Swagger (/openapi) and GraphQL (/graphql) |
| FAQ | Frequently asked questions |
| Troubleshooting | Common issues and solutions |

**API Documentation section (always shown):**
```html
<div class="api-docs">
  <h3>API Documentation</h3>
  <p>This application provides a full REST API with interactive documentation.</p>
  <ul>
    <li><a href="/openapi">Swagger UI</a> - Interactive REST API explorer</li>
    <li><a href="/graphql">GraphiQL</a> - Interactive GraphQL explorer</li>
  </ul>
</div>
```

**Content is project-specific. Markdown supported.**

### Configuration

```yaml
server:
  # Contact form recipient
  # If not set, uses admin email
  contact: ""

  pages:
    about:
      # Additional content for about page (markdown supported)
      content: ""
      # Show Tor section (auto-detected from tor.enabled, but can override)
      show_tor: auto

    privacy:
      # Privacy policy content (markdown supported)
      # If empty, uses default template
      content: ""

    contact:
      # Enable contact form
      enabled: true
      # Recipient email (if empty, uses server.contact or admin email)
      recipient: ""
      # Captcha type: recaptcha, hcaptcha, simple (built-in)
      captcha: simple
      # Success message after form submission
      success_message: "Thank you for your message. We'll respond soon."

    help:
      # Help page content (markdown supported)
      # Project-specific - must be defined per application
      content: ""
```

### Admin Panel (/admin/server/pages)

| Element | Type | Description |
|---------|------|-------------|
| **About Page** | | |
| Content | Markdown editor | Additional about page content |
| Show Tor section | Toggle | Show .onion address (auto/yes/no) |
| Preview | Button | Preview about page |
| **Privacy Policy** | | |
| Content | Markdown editor | Privacy policy content |
| Reset to default | Button | Restore default template |
| Preview | Button | Preview privacy page |
| **Contact Page** | | |
| Enable contact form | Toggle | Enable/disable contact form |
| Recipient email | Text input | Email to receive messages |
| Captcha type | Dropdown | recaptcha, hcaptcha, simple |
| Success message | Textarea | Message shown after submission |
| Test form | Button | Send test message |
| **Help Page** | | |
| Content | Markdown editor | Help/documentation content |
| Preview | Button | Preview help page |

---

# PART 14: API STRUCTURE (NON-NEGOTIABLE)

## API Versioning

**Use versioned API: `/api/v1`**

## API Types

**ALL PROJECTS GET ALL THREE:**

| Type | Required |
|------|----------|
| REST API | YES (primary) |
| Swagger | YES |
| GraphQL | YES |

## Root-Level Endpoints (NON-NEGOTIABLE)

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
| `/api/v1/healthz` | GET | None | Health check (JSON) |
| `/api/v1/admin/*` | ALL | Bearer | Admin API |

## Response Standards

| Route Type | Response Format |
|------------|-----------------|
| `/` routes | HTML |
| `/api` routes | JSON (default) or text |
| `/api/**/*.txt` | Text |

### Error Response Format

```json
{
  "error": "Human readable message",
  "code": "ERROR_CODE",
  "status": 400,
  "details": {}
}
```

### Pagination (default: 250 items)

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "limit": 250,
    "total": 1000,
    "pages": 4
  }
}
```

---

# CHECKPOINT 5: FRONTEND & API VERIFICATION

Before proceeding, confirm you understand:
- [ ] Frontend is required for ALL projects
- [ ] NO inline CSS, NO JS alerts
- [ ] Dark theme (Dracula) is default
- [ ] All 3 API types required: REST, Swagger, GraphQL
- [ ] Standard endpoints must exist (/healthz, /openapi, /graphql, /admin)

---

# PART 15: ADMIN PANEL (NON-NEGOTIABLE)

**ALL projects MUST have a full admin panel.**

## Design Principles

| Principle | Description |
|-----------|-------------|
| Pretty | Clean, modern, professional design |
| Intuitive | Self-explanatory, no manual needed |
| Easy Navigation | Logical grouping, breadcrumbs, search |
| Frontend Rules | Dracula theme (default), responsive, accessible |
| No JS Alerts | Custom modals, toasts, confirmations |
| Real-time Feedback | Show save status, validation errors inline |
| Mobile-Friendly | Works on all screen sizes |

## /admin (Web Interface)

### Authentication

| Feature | Description |
|---------|-------------|
| Login | Username/password form |
| Session | Cookie (30 days default) |
| CSRF | Protection on all forms |
| Remember Me | Option available |
| Logout | Always visible |

### Required Sections

1. Overview/Dashboard
2. Server Settings
3. Web Settings
4. Security Settings
5. Database & Cache
6. Email & Notifications
7. SSL/TLS
8. Scheduler (view/edit scheduled tasks, run history, next run times)
9. Logs
10. Backup & Maintenance
11. System Info

### Scheduler Management (Admin Panel)

The admin panel MUST include a scheduler section with:

| Feature | Description |
|---------|-------------|
| **Task List** | View all scheduled tasks with status |
| **Next Run** | Show next scheduled run time for each task |
| **Last Run** | Show last run time and result (success/failure) |
| **Run History** | View history of past runs with timestamps |
| **Manual Trigger** | Button to manually run any task |
| **Enable/Disable** | Toggle tasks on/off |
| **Edit Schedule** | Modify task frequency (cron-style or preset) |
| **Task Details** | View task configuration and logs |

**Preset Schedules:**
- `hourly` - Every hour
- `daily` - Once per day (configurable time)
- `weekly` - Once per week (configurable day/time)
- `monthly` - Once per month (configurable day/time)
- `custom` - Cron expression

## /api/v1/admin (REST API)

### Authentication

`Authorization: Bearer {token}`

### Required Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/config` | GET | Get full config |
| `/api/v1/admin/config` | PUT | Update full config |
| `/api/v1/admin/config` | PATCH | Partial update |
| `/api/v1/admin/status` | GET | Server status |
| `/api/v1/admin/health` | GET | Detailed health |
| `/api/v1/admin/stats` | GET | Statistics |
| `/api/v1/admin/logs/access` | GET | Access logs |
| `/api/v1/admin/logs/error` | GET | Error logs |
| `/api/v1/admin/backup` | POST | Create backup |
| `/api/v1/admin/restore` | POST | Restore backup |
| `/api/v1/admin/test/email` | POST | Send test email |
| `/api/v1/admin/password` | POST | Change password |
| `/api/v1/admin/token/regenerate` | POST | Regenerate API token |

---

# PART 16: EMAIL TEMPLATES (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have customizable email templates.**

Email templates allow users to customize notification messages. Default templates are embedded in the binary; custom templates are stored in `{config_dir}/templates/email/`.

## Template Storage

| Type | Location |
|------|----------|
| Default templates | Embedded in binary (`src/templates/email/`) |
| Custom templates | `{config_dir}/templates/email/` |

**Behavior:**
- If custom template exists → use custom
- If not → fall back to embedded default
- Reset to default → delete custom file

## Default Templates

| Template | Purpose |
|----------|---------|
| `welcome` | First run / new setup |
| `password_reset` | Password reset request |
| `backup_complete` | Backup finished successfully |
| `backup_failed` | Backup error |
| `ssl_expiring` | Certificate expiration warning |
| `ssl_renewed` | Certificate renewed successfully |
| `login_alert` | New login detected |
| `security_alert` | Security event (failed logins, etc.) |
| `scheduler_error` | Scheduled task failed |
| `test` | Test email |

## Template Format

Templates use simple `{variable}` syntax:

```
Subject: Your {app_name} backup completed
---
Hello,

Your backup completed successfully.

Filename: {filename}
Size: {size}
Time: {timestamp}

--
{app_name}
{app_url}
```

**Format Rules:**
- First line: `Subject: ...`
- Separator: `---` (three dashes on own line)
- Body: Plain text with variables
- Variables: `{variable_name}` syntax

## Global Variables (Available in All Templates)

| Variable | Description |
|----------|-------------|
| `{app_name}` | Application name/title |
| `{app_url}` | Application URL |
| `{onion_url}` | Tor .onion URL (if enabled) |
| `{admin_email}` | Admin email address |
| `{timestamp}` | Current date/time |
| `{year}` | Current year |

## Template-Specific Variables

### welcome
| Variable | Description |
|----------|-------------|
| `{admin_url}` | Admin panel URL |
| `{admin_username}` | Initial admin username |

### password_reset
| Variable | Description |
|----------|-------------|
| `{reset_link}` | Password reset URL |
| `{expires}` | Link expiration time |
| `{ip}` | Requesting IP address |

### backup_complete / backup_failed
| Variable | Description |
|----------|-------------|
| `{filename}` | Backup filename |
| `{size}` | Backup file size |
| `{error}` | Error message (failed only) |

### ssl_expiring / ssl_renewed
| Variable | Description |
|----------|-------------|
| `{domain}` | Domain name |
| `{expires_in}` | Days until expiration |
| `{expiry_date}` | Expiration date |
| `{valid_until}` | New validity date (renewed only) |

### login_alert
| Variable | Description |
|----------|-------------|
| `{ip}` | Login IP address |
| `{location}` | GeoIP location (if available) |
| `{device}` | User agent / device info |
| `{time}` | Login time |

### security_alert
| Variable | Description |
|----------|-------------|
| `{event}` | Security event type |
| `{ip}` | Source IP address |
| `{details}` | Event details |

### scheduler_error
| Variable | Description |
|----------|-------------|
| `{task_name}` | Failed task name |
| `{error}` | Error message |
| `{next_run}` | Next scheduled run |

## Admin Panel (/admin/email/templates)

| Element | Type | Description |
|---------|------|-------------|
| Template list | Table | All templates with status (default/custom) |
| Edit button | Button | Open template editor |
| Subject field | Text input | Editable subject line |
| Body editor | Textarea | Template body with syntax highlighting |
| Variable reference | Sidebar | Available variables for selected template |
| Preview button | Button | Render template with sample data |
| Save button | Button | Save custom template |
| Reset to default | Button | Delete custom, restore embedded (confirmation required) |

**Editor Features:**
- Syntax highlighting for `{variables}`
- Variable autocomplete
- Live preview with sample data
- Validation (warn if unknown variables used)

## Notification Routing (NON-NEGOTIABLE)

**Two notification systems available - use the right one for each event.**

| System | Use When |
|--------|----------|
| **WebUI (Toast/Banner)** | User is actively using the app |
| **Email** | User is away, needs permanent record, critical alerts |

### Routing Rules

| Event | WebUI | Email | Reason |
|-------|:-----:|:-----:|--------|
| Vanity address ready | ✓ | ✗ | User initiated, likely watching |
| Backup complete | ✓ | Optional | Quick confirmation, not critical |
| Backup failed | ✓ | ✓ | Critical - needs attention |
| SSL expiring (7+ days) | ✓ | ✗ | Warning, not urgent |
| SSL expiring (<3 days) | ✓ | ✓ | Urgent - needs action |
| SSL renewed | ✓ | ✗ | Informational |
| Login from new IP | ✓ | ✓ | Security - permanent record |
| Security alert | ✓ | ✓ | Critical - needs record |
| Scheduler task failed | ✓ | ✓ | Needs attention when away |
| Scheduler task success | ✗ | ✗ | No notification needed |
| Settings saved | ✓ | ✗ | Immediate feedback only |
| Password changed | ✓ | ✓ | Security - confirmation |
| Token regenerated | ✓ | ✓ | Security - confirmation |
| Tor address regenerated | ✓ | ✗ | User initiated |
| Update available | ✓ | Optional | Informational |
| Update installed | ✓ | ✓ | Important change record |

### Decision Logic

```
1. Is user actively using the app?
   → Always show WebUI notification

2. Is it critical (failure, security, urgent)?
   → Send email

3. Does user need a record when away?
   → Send email

4. Is it just confirmation of user action?
   → WebUI only (no email)

5. Is it routine success?
   → No notification needed
```

### WebUI Notification Types

| Type | Use For | Auto-dismiss |
|------|---------|--------------|
| `success` | Completed actions, confirmations | 5 seconds |
| `info` | Informational, status updates | 5 seconds |
| `warning` | Non-critical issues, expiring items | 10 seconds |
| `error` | Failures, critical issues | Manual dismiss |
| `persistent` | Requires action (e.g., "Apply vanity address") | Manual dismiss |

### WebUI Banner vs Toast

| Element | Use For |
|---------|---------|
| **Toast** | Transient notifications (success, info, warnings) |
| **Banner** | Persistent alerts requiring action (update available, SSL expiring soon) |

## Configuration

```yaml
server:
  notifications:
    # WebUI notifications (always enabled)
    webui:
      position: top-right
      # top-right, top-left, bottom-right, bottom-left
      duration: 5
      # seconds (0 = manual dismiss)

    # Email notifications
    email:
      enabled: false

      # Auto-detect SMTP server on local network
      autodetect: true
      autodetect_hosts:
        - localhost
        - 172.17.0.1
      autodetect_ports:
        - 25
        - 465
        - 587

      # Manual SMTP settings (used if autodetect disabled or fails)
      smtp:
        host: ""
        port: 587
        username: ""
        password: ""
        from: "noreply@{fqdn}"
        # TLS mode: auto, required, none
        tls: auto

      # Per-event email settings (override defaults)
      events:
        startup: false
        shutdown: false
        backup_complete: false
        backup_failed: true
        ssl_expiring: true
        ssl_renewed: false
        login_alert: true
        security_alert: true
        scheduler_error: true
        password_changed: true
        token_regenerated: true
        update_available: false
        update_installed: true
```

---

# PART 17: CLI INTERFACE (NON-NEGOTIABLE)

**THESE COMMANDS CANNOT BE CHANGED. This is the complete command set.**

## Main Commands

```bash
--help                       # Show help (can be run by anyone)
--version                    # Show version (can be run by anyone)
--mode {production|development}  # Set application mode
--data {datadir}             # Set data dir
--config {etcdir}            # Set the config dir
--address {listen}           # Set listen address
--port {port}                # Set the port
--status                     # Show status and health
--service {start,restart,stop,reload,--install,--uninstall,--disable,--help}
--maintenance {backup,restore,update,mode} [optional-file-or-setting]
--update [check|yes|branch {stable|beta|daily}]  # Check/perform updates
```

### Commands Anyone Can Run (No Privileges)

- `--help`
- `--version`
- `--status`
- `--update check`

## Display Rules (NON-NEGOTIABLE)

| Rule | Description |
|------|-------------|
| Never show | `0.0.0.0`, `127.0.0.1`, `localhost` |
| Always show | Valid FQDN, host, or IP |
| Show only | One address, the most relevant |

## URL & FQDN Detection (NON-NEGOTIABLE)

**CRITICAL: Never hardcode host, IP, or port in project code. Always detect dynamically.**

### URL Display Rules

| Rule | Description |
|------|-------------|
| **NEVER hardcode** | `localhost`, `127.0.0.1`, `0.0.0.0`, `[::1]`, any static host/IP |
| **NEVER display** | `GET /api/`, `POST /api/` without full URL |
| **ALWAYS use** | `{proto}://{host}:{port}/path` format |
| **ALWAYS detect** | `{proto}`, `{host}`, `{port}` from request context |
| **ALWAYS strip** | `:80` for HTTP, `:443` for HTTPS |
| **Default proto** | `http` if not detected |

### URL Format Examples

| WRONG | RIGHT |
|-------|-------|
| `GET /api/v1/resource/random` | `https://api.example.com/api/v1/resource/random` |
| `POST /api/v1/admin/config` | `https://api.example.com/api/v1/admin/config` |
| `http://localhost:8080/api` | `http://192.168.1.100:64580/api` |
| `http://0.0.0.0:80/healthz` | `https://myserver.example.com/healthz` |

### FQDN Detection Order (First Valid Wins)

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | **Reverse Proxy Headers** | `X-Forwarded-Host`, `X-Real-Host`, etc. |
| 2 | **DOMAIN env var** | Must be valid FQDN (e.g., `api.example.com`) |
| 3 | **HOSTNAME env var** | Must be valid FQDN (e.g., `host.example.com`) |
| 4 | **Global IPv6** | If available and routable |
| 5 | **Global IPv4** | If available and routable |

### FQDN/Host Validation Rules

Validation depends on application mode:

**Production Mode (Strict):**

| Valid | Invalid |
|-------|---------|
| `api.example.com` | `localhost` |
| `my-host.domain.org` | `dev.local` |
| `server.company.io` | `app.test` |
| | `192.168.1.1` (IP address) |
| | `myhost` (single-label) |

**Development Mode (Relaxed):**

| Valid | Invalid |
|-------|---------|
| `api.example.com` | `192.168.1.1` (IP address) |
| `localhost` | `myhost` (single-label, not localhost) |
| `dev.local` | `devbox` (single-label) |
| `app.test` | |
| `staging.internal` | |

**Validation Requirements:**

| Mode | Rules |
|------|-------|
| **Production** | Must have dot, no IPs, no internal TLDs, no localhost |
| **Development** | Must have dot OR be localhost, no IPs |

**Internal/Dev-Only TLDs:**
- `localhost` (literal)
- `.localhost`, `.test`, `.example`, `.invalid` (RFC 6761)
- `.local`, `.lan`, `.internal`, `.home`, `.localdomain`
- `.home.arpa`, `.intranet`, `.corp`, `.private`

**Special TLDs (Always Valid):**
- `.onion` - Tor hidden services (RFC 7686)

**Go Implementation:**
```go
var devOnlyTLDs = []string{
    ".localhost", ".test", ".example", ".invalid",
    ".local", ".lan", ".internal", ".home", ".localdomain",
    ".home.arpa", ".intranet", ".corp", ".private",
}

func IsValidHost(host string, devMode bool) bool {
    lower := strings.ToLower(host)

    // Reject IP addresses always
    if net.ParseIP(host) != nil {
        return false
    }

    // Handle localhost
    if lower == "localhost" {
        return devMode // Only valid in dev mode
    }

    // Must contain at least one dot
    if !strings.Contains(host, ".") {
        return false
    }

    // .onion addresses are always valid (Tor hidden services)
    if strings.HasSuffix(lower, ".onion") {
        return true
    }

    // In production, reject dev-only TLDs
    if !devMode {
        for _, tld := range devOnlyTLDs {
            if strings.HasSuffix(lower, tld) {
                return false
            }
        }
    }

    return true
}
```

**Note:** DOMAIN and HOSTNAME environment variables MUST pass host validation for the current mode. Invalid values are skipped silently and detection continues to next source.

### SSL/Let's Encrypt FQDN Requirements

When requesting SSL certificates (Let's Encrypt), the FQDN must be **publicly resolvable**. This uses the same validation as production mode - no internal/dev-only TLDs allowed.

**Go Implementation for SSL validation:**
```go
func IsValidSSLHost(host string) bool {
    lower := strings.ToLower(host)

    // .onion addresses cannot use Let's Encrypt (not publicly resolvable)
    // Tor provides end-to-end encryption, so SSL is optional for .onion
    if strings.HasSuffix(lower, ".onion") {
        return false
    }

    // SSL always requires production-valid host (devMode=false)
    return IsValidHost(host, false)
}
```

**Behavior:**
- SSL with Let's Encrypt: Must pass production-mode validation (no dev TLDs)
- .onion addresses: Cannot use Let's Encrypt (Tor provides encryption)
- If Let's Encrypt requested with invalid host: Log warning, skip cert request, continue without SSL
- Self-signed certs: Can use any valid host for current mode

### Reverse Proxy Header Support (All Headers Supported)

**Protocol Detection (`{proto}`):**
- `X-Forwarded-Proto` - Standard: "https" or "http"
- `X-Forwarded-Ssl` - "on" = https, "off" = http
- `X-Url-Scheme` - Alternative: "https" or "http"
- `Front-End-Https` - Microsoft: "on" = https

**Host Detection (`{host}`):**
- `X-Forwarded-Host` - Standard: "example.com" or "example.com:8080"
- `X-Real-Host` - nginx: "example.com"
- `X-Original-Host` - Alternative

**Port Detection (`{port}`):**
- `X-Forwarded-Port` - Standard: "443" or "8080"
- `X-Real-Port` - nginx alternative

**Client IP Detection (for logging, rate limiting, GeoIP):**
- `X-Forwarded-For` - Standard: may contain chain "client, proxy1, proxy2"
- `X-Real-IP` - nginx: single IP
- `CF-Connecting-IP` - Cloudflare
- `True-Client-IP` - Akamai/Cloudflare Enterprise
- `X-Client-IP` - Alternative

**Request ID (for tracing):**
- `X-Request-ID` - Standard
- `X-Correlation-ID` - Alternative
- `X-Trace-ID` - Distributed tracing

### Request ID Handling (NON-NEGOTIABLE)

**Every request MUST have a Request ID for tracing and debugging.**

| Scenario | Behavior |
|----------|----------|
| Client sends `X-Request-ID` | Use client's ID (validate format) |
| No Request ID header | Generate new UUID |
| Invalid format | Generate new UUID, log warning |

**Request ID Rules:**

| Rule | Description |
|------|-------------|
| **Format** | UUID v4 (e.g., `550e8400-e29b-41d4-a716-446655440000`) |
| **Generation** | Use secure random UUID generator |
| **Propagation** | Include in all outgoing requests to downstream services |
| **Response** | Return `X-Request-ID` in response headers |
| **Logging** | Include `{request_id}` in all log entries for the request |

**Go Implementation:**
```go
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check for existing request ID from client or upstream proxy
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = r.Header.Get("X-Correlation-ID")
        }
        if requestID == "" {
            requestID = r.Header.Get("X-Trace-ID")
        }

        // Generate new ID if none provided or invalid
        if requestID == "" || !isValidUUID(requestID) {
            requestID = uuid.New().String()
        }

        // Add to response headers
        w.Header().Set("X-Request-ID", requestID)

        // Add to request context for logging and downstream calls
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Implementation Requirements

1. **Request Context Helper**: Create a helper function that extracts `{proto}`, `{host}`, `{port}` from each request
2. **URL Builder**: Create a helper that builds full URLs: `{proto}://{host}:{port}/path` (strip :80/:443)
3. **Never Import** hardcoded URLs into templates - always pass detected values
4. **API Response URLs**: All URLs in API responses must be absolute, using detected values
5. **Swagger/OpenAPI**: Server URL must be detected, not hardcoded

---

# PART 18: UPDATE COMMAND (NON-NEGOTIABLE)

## --update Command

```bash
--update [command]
```

**Alias:** `--maintenance update` is an alias for `--update yes`

## Commands

| Command | Description |
|---------|-------------|
| `yes` (default) | Check and perform in-place update with restart |
| `check` | Check for updates without installing (no privileges required) |
| `branch {stable\|beta\|daily}` | Set update branch |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Successful update or no update available |
| 1 | Error |

**HTTP 404 from GitHub API means no updates available (already current).**

### Update Branches

| Branch | Release Type | Tag Pattern | Example |
|--------|--------------|-------------|---------|
| `stable` (default) | Release | `v*`, `*.*.*` | `v1.0.0` |
| `beta` | Pre-release | `*-beta` | `202512051430-beta` |
| `daily` | Pre-release | `YYYYMMDDHHMM` | `202512051430` |

### Examples

```bash
# Check for updates (no privileges required)
vidveil --update check

# Perform update (these are equivalent)
vidveil --update
vidveil --update yes
vidveil --maintenance update

# Switch channels
vidveil --update branch beta
vidveil --update branch daily
vidveil --update branch stable
```

---

# PART 19: DOCKER (NON-NEGOTIABLE)

## Dockerfile Requirements

| Requirement | Value |
|-------------|-------|
| Base | Alpine-based (latest) |
| Meta labels | All included |
| Required packages | curl, bash, tini, **tor** |
| Binary location | `/usr/local/bin/vidveil` |
| Init system | **tini** |
| **ENV MODE** | **development** (allows localhost, .local, .test, etc.) |

### Dockerfile Example

```dockerfile
FROM alpine:latest

# Install required packages including Tor
RUN apk add --no-cache \
    curl \
    bash \
    tini \
    tor

# Copy the static binary
COPY vidveil /usr/local/bin/vidveil
RUN chmod +x /usr/local/bin/vidveil

# Create directories
RUN mkdir -p /config /data

# Set environment
ENV MODE=development

# Expose port
EXPOSE 80

# Use tini as init
ENTRYPOINT ["/sbin/tini", "--"]

# Run the application
CMD ["/usr/local/bin/vidveil"]
```

## Docker Compose Requirements

| Requirement | Value |
|-------------|-------|
| Build definition | NEVER include |
| Version | NEVER include |
| Network | Custom `vidveil` network |
| Container name | `vidveil` |
| **environment: MODE** | **production** (strict host validation) |

### Docker Compose Example

```yaml
services:
  vidveil:
    image: ghcr.io/apimgr/vidveil:latest
    container_name: vidveil
    restart: unless-stopped
    environment:
      - MODE=production
    ports:
      - "8080:80"
    volumes:
      - ./config:/config
      - ./data:/data
    networks:
      - vidveil

networks:
  vidveil:
    driver: bridge
```

## Container Configuration

| Setting | Value |
|---------|-------|
| Internal port | 80 |
| Data dir | `/data` |
| Config dir | `/config` |
| Log dir | `/data/logs/vidveil` |
| Tor data dir | `/data/tor` |
| HEALTHCHECK | `{binary} --status` |

## Tor in Container

**Tor is included in the container image and managed by the application.**

| Behavior | Description |
|----------|-------------|
| Auto-start | Application starts Tor automatically if enabled |
| Data persistence | Tor keys stored in `/data/tor/site/` (survives container restart) |
| .onion address | Persists across container restarts via volume mount |

## Container Detection

**Assume running in container if tini init system (PID 1) is detected.**

| When in Container | Behavior |
|-------------------|----------|
| Show sensitive data | On first run (hard to retrieve in container environments) |
| Defaults | Use container-appropriate defaults |
| Logging | Log to stdout/stderr (captured by container runtime) |
| Tor | Application manages Tor process internally |

## Tags

| Type | Tag |
|------|-----|
| Release | `ghcr.io/apimgr/vidveil:latest` |
| Development | `vidveil:dev` |

---

# PART 20: MAKEFILE (NON-NEGOTIABLE)

**DO NOT CHANGE THESE TARGETS.**

## Targets

| Target | Description |
|--------|-------------|
| `build` | Build all platforms to `./binaries` |
| `release` | GitHub release to `./releases` |
| `docker` | Docker release for ARM64/AMD64 |
| `test` | Run all tests |

## Binary Naming (NON-NEGOTIABLE)

| Context | Name |
|---------|------|
| Local/Testing | `/tmp/apimgr-build/vidveil/vidveil` |
| Host Build | `./binaries/vidveil` |
| Distribution | `vidveil-{os}-{arch}` |

**NEVER include `-musl` suffix.**

Example: `jokes-linux-amd64` NOT `jokes-linux-amd64-musl`

---

# PART 21: GITHUB ACTIONS (NON-NEGOTIABLE)

**All projects MUST have GitHub Actions workflows.**

## Workflow Files

| File | Trigger | Purpose |
|------|---------|---------|
| `release.yml` | Tag push (`v*`, `*.*.*`) | Production releases |
| `beta.yml` | Push to `beta` branch | Beta releases |
| `daily.yml` | Daily at 3am UTC + push to main/master | Daily builds |
| `docker.yml` | Version tag, push to main/master/beta | Docker images |

## Release Workflow

**Trigger:** Tag push with or without `v` prefix

```yaml
on:
  push:
    tags:
      - 'v*'
      - '[0-9]*'
```

### Build Matrix

| OS | Arch | Binary Name |
|----|------|-------------|
| Linux | amd64 | `vidveil-linux-amd64` |
| Linux | arm64 | `vidveil-linux-arm64` |
| macOS | amd64 | `vidveil-darwin-amd64` |
| macOS | arm64 | `vidveil-darwin-arm64` |
| Windows | amd64 | `vidveil-windows-amd64.exe` |
| Windows | arm64 | `vidveil-windows-arm64.exe` |
| FreeBSD | amd64 | `vidveil-freebsd-amd64` |
| FreeBSD | arm64 | `vidveil-freebsd-arm64` |

### Release Process

1. Build static binaries (`CGO_ENABLED=0`, no `-musl` suffix)
2. Create source archive (exclude `.git`, `.github`, `binaries/`, `releases/`)
3. Delete existing release/tag if exists (using `gh release delete`)
4. Create new release with all binaries and source archive
5. Update `latest` tag

## Beta Workflow

**Version Format:** `YYYYMMDDHHMM-beta` (e.g., `202512051430-beta`)

## Daily Workflow

**Version Format:** `YYYYMMDDHHMM` (e.g., `202512051430`)

**Cleanup:** Keep only last 7 daily releases

## Docker Workflow

### Triggers and Tags

| Trigger | Image Tags |
|---------|------------|
| Version tag | `{version}`, `latest`, `YYMM`, `{GIT_COMMIT}` |
| Push to main/master | `dev`, `{GIT_COMMIT}` |
| Push to beta | `beta`, `{GIT_COMMIT}` |

**Notes:**
- `{GIT_COMMIT}` = short SHA (7 characters)
- `YYMM` = year/month (e.g., `2512`)
- Built for `linux/amd64` and `linux/arm64` using `docker buildx`
- Registry: `ghcr.io`

## Jenkins (NON-NEGOTIABLE)

### Configuration

| Setting | Value |
|---------|-------|
| Agents | ARM64, AMD64 |
| Server | jenkins.casjay.cc |
| Build | Both architectures in parallel |

### Jenkinsfile

All projects MUST have a Jenkinsfile that:
- Builds for ARM64 and AMD64
- Runs tests
- Builds Docker images (if applicable)

---

# CHECKPOINT 6: BUILD & DEPLOYMENT VERIFICATION

Before proceeding, confirm you understand:
- [ ] Docker uses tini as init, Alpine base
- [ ] Makefile has exactly 4 targets: build, release, docker, test
- [ ] Binary naming: NEVER include -musl suffix
- [ ] 4 GitHub workflows: release, beta, daily, docker
- [ ] Jenkins builds for ARM64 and AMD64
- [ ] All 8 platform builds required (4 OS x 2 arch)

---

# PART 22: BINARY REQUIREMENTS (NON-NEGOTIABLE)

## Single Static Binary

| Requirement | Description |
|-------------|-------------|
| Type | **SINGLE STATIC BINARY** |
| Assets | Embedded using Go's `embed` package |
| Dependencies | None at runtime |
| Build | **CGO_ENABLED=0** |
| Libraries | Pure Go only (no CGO) |

## Default Behavior

| Behavior | Description |
|----------|-------------|
| No arguments | Initialize (if needed) and start server |
| First run | Auto-create config with defaults |
| First run | Auto-create required directories |
| Signals | Proper handling (SIGTERM, SIGINT, SIGHUP) |
| PID file | Enabled by default |

## Embedded Assets

| Asset Type | Location |
|------------|----------|
| Templates | `src/server/templates/` |
| Static files | `src/server/static/` |
| Application data | `src/data/` (JSON files) |

## External Data (NOT Embedded)

| Data Type | Description |
|-----------|-------------|
| GeoIP databases | Download, update via scheduler |
| Blocklists | Download, update via scheduler |
| Security databases | Any security-related data |

---

# PART 23: TESTING & DEVELOPMENT (NON-NEGOTIABLE)

## Temporary Directory Structure (NON-NEGOTIABLE)

**Format:** `/tmp/{orgname}-{purpose}/vidveil/`

| Purpose | Path | Example |
|---------|------|---------|
| Build output | `/tmp/apimgr-build/vidveil/` | `/tmp/apimgr-build/jokes/` |
| Test config | `/tmp/apimgr-test/vidveil/` | `/tmp/apimgr-test/jokes/` |
| Debug files | `/tmp/apimgr-debug/vidveil/` | `/tmp/apimgr-debug/jokes/` |
| Runtime temp | `/tmp/apimgr-runtime/vidveil/` | `/tmp/apimgr-runtime/jokes/` |

### Rules

| Rule | Description |
|------|-------------|
| **NEVER** | Use `/tmp/vidveil` directly |
| **NEVER** | Use `/tmp/` root for project files |
| **ALWAYS** | Use `/tmp/{orgname}-{purpose}/vidveil/` format |
| **ALWAYS** | Include organization prefix to avoid conflicts |
| **ALWAYS** | Include purpose subdirectory for organization |

### Why This Structure?

| Reason | Description |
|--------|-------------|
| **Avoid conflicts** | Multiple projects/orgs won't collide |
| **Easy cleanup** | `rm -rf /tmp/apimgr-*` cleans all org temp files |
| **Clear purpose** | Directory name shows what files are for |
| **Debugging** | Easy to find specific project's temp files |

### Correct vs Incorrect

| WRONG | RIGHT |
|-------|-------|
| `/tmp/jokes` | `/tmp/apimgr-build/jokes/` |
| `/tmp/build/jokes` | `/tmp/apimgr-build/jokes/` |
| `/tmp/test` | `/tmp/apimgr-test/jokes/` |
| `/tmp/jokes-test` | `/tmp/apimgr-test/jokes/` |

## Container Usage (NON-NEGOTIABLE)

**ALL builds, tests, and debugging MUST use containerized environments. NEVER build directly on the host system.**

| Rule | Description |
|------|-------------|
| Build environment | Docker/Incus/LXD |
| Image | `golang:latest` (NOT alpine) |
| Test binaries | Temp directories |
| **NEVER** | Run `go build` directly on host |
| **ALWAYS** | Use containerized build command below |

### Why Containerized Builds?

| Reason | Description |
|--------|-------------|
| **Consistent environment** | Same Go version and dependencies across all builds |
| **Static binaries** | `CGO_ENABLED=0` produces binaries with no runtime dependencies |
| **Reproducible** | Builds work regardless of host OS setup |
| **Clean isolation** | No pollution of host system with build artifacts |

## Build Command (NON-NEGOTIABLE)

**This is the REQUIRED build command. Not an example - this MUST be used.**

```bash
docker run --rm -v /path/to/project:/build -w /build -e CGO_ENABLED=0 \
  golang:latest go build -o /tmp/apimgr-build/vidveil/vidveil ./src
```

### Testing a Built Binary

```bash
# Build
docker run --rm -v /root/Projects/github/apimgr/vidveil:/build -w /build \
  -e CGO_ENABLED=0 golang:latest go build -o /tmp/apimgr-build/vidveil/vidveil ./src

# Run/Test
/tmp/apimgr-build/vidveil/vidveil --help
/tmp/apimgr-build/vidveil/vidveil --version
```

## Process Management (NON-NEGOTIABLE)

**STRICT RULES: Only kill/remove the EXACT process or container being worked on. NEVER anything else.**

### FORBIDDEN Commands (NEVER Use)

| Command | Reason |
|---------|--------|
| `pkill -f {pattern}` | Too broad, kills unrelated processes |
| `pkill {name}` | Too broad without `-x` flag |
| `killall {name}` | Too broad, may kill unrelated processes |
| `kill -9 {pid}` | Use graceful `kill {pid}` first |
| `docker kill` | Use `docker stop` for graceful shutdown |
| `docker rm $(docker ps -aq)` | Removes ALL containers |
| `docker rm $(docker ps -q)` | Removes ALL running containers |
| `docker rmi $(docker images -q)` | Removes ALL images |
| `docker system prune` | Cleans ALL unused resources |
| `docker container prune` | Removes ALL stopped containers |
| `docker image prune` | Removes ALL dangling images |
| `docker volume prune` | Removes ALL unused volumes |
| `docker network prune` | Removes ALL unused networks |
| `rm -rf /` | Catastrophic |
| `rm -rf /*` | Catastrophic |
| `rm -rf ~` | Destroys home directory |
| `rm -rf .` | Dangerous in wrong directory |
| `rm -rf *` | Dangerous without proper scoping |

### Process Termination Rules

| Rule | Description |
|------|-------------|
| **Identify first** | ALWAYS get exact PID before killing |
| **Graceful first** | Use `kill {pid}` (SIGTERM), wait, then `kill -9 {pid}` only if needed |
| **One at a time** | Kill ONE specific PID, never patterns |
| **Verify PID** | Confirm PID belongs to the project process |
| **Document** | Log what was killed and why |

**Kill Process Flow:**
```
1. pgrep -la vidveil           # List matching processes
2. Verify the PID is correct          # CHECK before killing
3. kill {pid}                         # Graceful termination (SIGTERM)
4. sleep 5                            # Wait for graceful shutdown
5. pgrep -la vidveil           # Check if still running
6. kill -9 {pid}                      # Force kill ONLY if still running
```

### Docker Container Rules

| Rule | Description |
|------|-------------|
| **ONLY this project** | Only stop/remove containers named `vidveil` |
| **NEVER other containers** | Even if they look related or unused |
| **NEVER images not ours** | Only remove `apimgr/vidveil:*` images |
| **NEVER base images** | Never remove `golang`, `alpine`, `ubuntu`, etc. |
| **NEVER volumes** | Unless explicitly part of this project |

**Docker Cleanup Flow:**
```
1. docker ps -a --filter name=vidveil     # List ONLY this project's containers
2. Verify output shows ONLY vidveil       # CHECK before removing
3. docker stop vidveil                    # Stop gracefully
4. docker rm vidveil                      # Remove container

# For images:
1. docker images apimgr/vidveil     # List ONLY this project's images
2. Verify output shows ONLY our images          # CHECK before removing
3. docker rmi apimgr/vidveil:tag    # Remove SPECIFIC tag
```

### Allowed Commands (Project-Scoped ONLY)

| Command | Description |
|---------|-------------|
| `kill {specific-pid}` | Kill exact PID only (after verification) |
| `pkill -x vidveil` | Exact binary name match only |
| `docker stop vidveil` | Stop specific container by name |
| `docker rm vidveil` | Remove specific container by name |
| `docker rmi apimgr/vidveil:tag` | Remove specific image:tag |
| `rm -rf /tmp/apimgr-build/vidveil/` | Remove specific project temp files |
| `rm -rf /tmp/apimgr-*/vidveil/` | Remove all temp files for one project |

### Before ANY Kill/Remove Operation

1. **List first**: See exactly what will be affected
2. **Verify**: Confirm it's the correct process/container/file
3. **Be specific**: Use exact names, PIDs, or paths - NEVER patterns
4. **Ask if unsure**: When in doubt, ask the user
5. **Document**: Log what was removed and why

## File Cleanup Rules (NON-NEGOTIABLE)

**Always be explicit and project-scoped when deleting files.**

### Safe Cleanup Commands

| Purpose | Command |
|---------|---------|
| Project build temp | `rm -rf /tmp/apimgr-build/vidveil/` |
| Project test temp | `rm -rf /tmp/apimgr-test/vidveil/` |
| Project debug temp | `rm -rf /tmp/apimgr-debug/vidveil/` |
| All project temp | `rm -rf /tmp/apimgr-*/vidveil/` |
| All org temp | `rm -rf /tmp/apimgr-*/` |
| Project binaries | `rm -rf ./binaries/vidveil*` |
| Project releases | `rm -rf ./releases/vidveil*` |

### NEVER Delete Without Confirmation

| Item | Why |
|------|-----|
| User data directories | Irreversible data loss |
| Config files | User customizations lost |
| Database files | Data loss |
| SSL certificates | Service disruption |
| Git repositories | Code loss |
| Anything outside project scope | Affects other systems |

### Cleanup Checklist

Before running any `rm -rf`:

1. **Echo first**: `echo "Would delete: /path/to/delete"` - verify the path
2. **Check pwd**: `pwd` - make sure you're in the right directory
3. **List first**: `ls -la /path/to/delete` - see what will be deleted
4. **Be specific**: Use full paths, not relative paths with wildcards
5. **Ask if unsure**: When in doubt, ask the user before deleting

---

# PART 24: DATABASE & CLUSTER (NON-NEGOTIABLE)

## Database Migrations

**ALL apps MUST have built-in AUTOMATIC database migration support.**

| Feature | Description |
|---------|-------------|
| Automatic | Runs on startup |
| Versioned | Migrations with timestamps |
| Tracking | `schema_migrations` table |
| Rollback | Automatic on failure |

## Cluster Support

**ALL apps MUST support cluster mode.**

### Single Instance (Auto-detected)

- No external cache/database configured
- Uses local file/SQLite for state

### Cluster Mode (Auto-detected)

- Auto-enabled when external cache or shared database detected
- Primary election for cluster-wide tasks
- Distributed locks
- Session sharing

---

# PART 25: SECURITY & LOGGING (NON-NEGOTIABLE)

## Security Headers

**All responses MUST include:**

```
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

**In development mode, these may be relaxed.**

## Well-Known Files (NON-NEGOTIABLE)

**Standard files served at well-known paths. Generated automatically if no file exists.**

### Required Files

| File | Path | Purpose |
|------|------|---------|
| `robots.txt` | `/robots.txt` | Search engine crawling rules |
| `security.txt` | `/.well-known/security.txt` | Security vulnerability reporting (RFC 9116) |

### Additional Well-Known Paths

| Path | Purpose |
|------|---------|
| `/.well-known/acme-challenge/` | Let's Encrypt HTTP-01 challenge |
| `/.well-known/change-password` | Password change URL (redirects to `/auth/password/forgot`) |

### Well-Known Directory Support

Files can be served from:
1. Files in `{data_dir}/web/.well-known/` (checked first)
2. Embedded files in binary
3. Dynamically generated (e.g., ACME challenges, config-based security.txt)

### robots.txt

```
# Served at /robots.txt - generated if no file exists

User-agent: *
Allow: /
Allow: /api
Disallow: /admin
Sitemap: {app_url}/sitemap.xml
```

**Configuration:**
```yaml
web:
  robots:
    allow:
      - /
      - /api
    deny:
      - /admin
```

### security.txt (RFC 9116)

**ALL projects MUST serve a valid security.txt file.**

```
# Served at /.well-known/security.txt

Contact: mailto:{security_contact}
Expires: {expiry_date}
```

**Configuration:**
```yaml
web:
  security:
    contact: "security@{fqdn}"    # Security contact email
    expires: "{1year}"            # Auto-calculated 1 year from generation
```

**Fields:**
| Field | Required | Description |
|-------|----------|-------------|
| `Contact` | YES | Email for reporting vulnerabilities (mailto: prefix added automatically) |
| `Expires` | YES | Expiration date (auto-renewed yearly by default) |

### Admin Panel (/admin/web)

**robots.txt Settings:**

| Element | Type | Description |
|---------|------|-------------|
| Allow paths | Tag input / List | Paths to allow crawling (e.g., `/`, `/api`) |
| Deny paths | Tag input / List | Paths to deny crawling (e.g., `/admin`) |
| Preview | Read-only textarea | Shows generated robots.txt content |

**security.txt Settings:**

| Element | Type | Description |
|---------|------|-------------|
| Security contact | Text input | Email for vulnerability reports |
| Expires | Date picker | Expiration date (default: 1 year from now, auto-renews) |
| Preview | Read-only textarea | Shows generated security.txt content |

## Logging

### Log Files

| Log | Purpose | Default Format | Available Formats |
|-----|---------|----------------|-------------------|
| `access.log` | HTTP requests | `apache` | `apache`, `nginx`, `json` |
| `server.log` | Application events | `text` | `text`, `json` |
| `error.log` | Error messages | `text` | `text`, `json` |
| `audit.log` | Security events | `json` | `json`, `text` |
| `security.log` | Security/auth events | `fail2ban` | `fail2ban`, `syslog`, `cef`, `json`, `text` |
| `debug.log` | Debug (dev mode) | `text` | `text`, `json` |

### Log Format Details

**Access Log Formats:**
| Format | Description | Example |
|--------|-------------|---------|
| `apache` | Apache Combined Log Format (default) | `127.0.0.1 - - [10/Oct/2024:13:55:36 -0700] "GET /api/v1/health HTTP/1.1" 200 2326 "-" "curl/7.64.1"` |
| `nginx` | Nginx Common Log Format | `127.0.0.1 - - [10/Oct/2024:13:55:36 -0700] "GET /api/v1/health HTTP/1.1" 200 2326` |
| `json` | Structured JSON | `{"ip":"127.0.0.1","time":"2024-10-10T13:55:36Z","method":"GET","path":"/api/v1/health","status":200,"size":2326,"ua":"curl/7.64.1"}` |

**Security Log Formats:**
| Format | Description | Use Case |
|--------|-------------|----------|
| `fail2ban` | Fail2ban compatible (default) | Intrusion prevention integration |
| `syslog` | RFC 5424 syslog format | SIEM integration, centralized logging |
| `cef` | Common Event Format | SIEM/security tools (ArcSight, Splunk) |
| `json` | Structured JSON | Custom parsing, ELK stack |
| `text` | Plain text | Human readable |

**Text Log Format:**
```
2024-10-10 13:55:36 [INFO] Server started on :8080
2024-10-10 13:55:40 [ERROR] Database connection failed: timeout
```

**JSON Log Format:**
```json
{"time":"2024-10-10T13:55:36Z","level":"INFO","msg":"Server started on :8080"}
{"time":"2024-10-10T13:55:40Z","level":"ERROR","msg":"Database connection failed","error":"timeout"}
```

**Fail2ban Format:**
```
2024-10-10 13:55:36 [security] Failed login attempt from 192.168.1.100 for user admin
2024-10-10 13:55:40 [security] Rate limit exceeded from 192.168.1.100
```

### Custom Format Variables

When using `format: custom`, these variables are available:

| Variable | Description |
|----------|-------------|
| `{time}` | Time only |
| `{date}` | Date only |
| `{datetime}` | Date and time |
| `{remote_ip}` | Client IP address |
| `{method}` | HTTP method |
| `{path}` | Request path |
| `{query}` | Query string |
| `{status}` | HTTP status code |
| `{bytes}` | Response size |
| `{latency}` | Request latency (human readable) |
| `{latency_ms}` | Request latency (milliseconds) |
| `{user_agent}` | User agent string |
| `{referer}` | Referer header |
| `{request_id}` | Request ID |
| `{host}` | Request host |
| `{protocol}` | HTTP protocol version |
| `{tls_version}` | TLS version (if HTTPS) |
| `{country}` | GeoIP country code |
| `{asn}` | GeoIP ASN |

### Rotation Options

| Option | Description |
|--------|-------------|
| `never` | Never rotate |
| `daily` | Rotate daily |
| `weekly` | Rotate weekly |
| `monthly` | Rotate monthly |
| `yearly` | Rotate yearly |
| `NMB` | Rotate at N megabytes (e.g., `50MB`) |
| `NGB` | Rotate at N gigabytes (e.g., `1GB`) |
| Combined | Time + size, whichever first (e.g., `weekly,50MB`) |

### Retention Options

| Option | Description |
|--------|-------------|
| `none` | Do not keep old logs (delete after rotation) |
| `N` | Keep N old log files |
| `Nd` | Keep logs for N days |
| `Nw` | Keep logs for N weeks |
| `Nm` | Keep logs for N months |

### Configuration

```yaml
server:
  logs:
    # Global log level: debug, info, warn, error
    level: warn

    # All log types share these options:
    #   filename: name of log file
    #   format: output format (varies by log type)
    #   custom: custom format string (when format=custom)
    #   rotate: rotation policy
    #   keep: retention policy

    access:
      filename: access.log
      # Format: apache, nginx, json, custom
      format: apache
      custom: ""
      rotate: monthly
      keep: none

    server:
      filename: server.log
      # Format: text, json
      format: text
      custom: ""
      rotate: weekly,50MB
      keep: none

    error:
      filename: error.log
      # Format: text, json
      format: text
      custom: ""
      rotate: weekly,50MB
      keep: none

    audit:
      filename: audit.log
      # Format: json, text
      format: json
      custom: ""
      rotate: weekly,50MB
      keep: none

    security:
      filename: security.log
      # Format: fail2ban, syslog, cef, json, text
      format: fail2ban
      custom: ""
      rotate: weekly,50MB
      keep: none

    debug:
      # Debug log has an enabled flag since it's for troubleshooting only
      enabled: false
      filename: debug.log
      # Format: text, json
      format: text
      custom: ""
      rotate: weekly,50MB
      keep: none
```

### Log Output Rules (NON-NEGOTIABLE)

**All log FILES MUST use raw text only:**
- NO emojis
- NO ANSI color codes
- NO special characters or formatting
- Plain ASCII text only
- Machine-parseable format

**Console output (stdout/stderr) CAN be pretty:**
- Emojis allowed (e.g., `✅ Server started`, `❌ Error`, `⚠️ Warning`)
- ANSI colors allowed
- Pretty formatting allowed
- Used for start/stop/restart/status messages
- User-facing CLI output can be visually appealing

**Rule:** Log files = raw/plain text. Console = pretty is OK.

### Log Rotation

**Defaults:**
| Log Type | Rotation | Keep |
|----------|----------|------|
| access.log | monthly | none |
| All others | weekly,50MB | none |

**Rules:**
- `weekly,50MB` = rotate on weekly OR 50MB, whichever comes first
- `keep: none` = do not retain old logs (default)
- Built-in rotation support (no external logrotate needed)
- Old logs deleted immediately after rotation (default)
- Optional: compress before delete, retain with `keep: weekly:N` or `monthly:N`

---

# PART 26: BACKUP & RESTORE (NON-NEGOTIABLE)

## Backup Command

```bash
vidveil --maintenance backup [filename]
```

### Contents

- Configuration file
- Database (if applicable)
- Custom assets
- SSL certificates (optional)

### Format

- Single `.tar.gz` file
- Includes manifest with version info
- Encrypted option available

## Restore Command

```bash
vidveil --maintenance restore <backup-file>
```

---

# PART 27: HEALTH & VERSIONING (NON-NEGOTIABLE)

## Health Checks

### /healthz (HTML)

- Status (healthy/unhealthy)
- Uptime
- Version
- Mode
- System resources (optional)

### /api/v1/healthz (JSON)

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "mode": "production",
  "uptime": "2d 5h 30m",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "database": "ok",
    "cache": "ok",
    "disk": "ok"
  }
}
```

## Versioning

### Format

- Semantic versioning: `MAJOR.MINOR.PATCH`
- Pre-release: `1.0.0-beta.1`
- Build metadata: `1.0.0+build.123`

### Sources (Priority Order)

1. `release.txt` in project root
2. Git tag (if available)
3. Fallback: `dev`

### --version Output

```
vidveil v1.0.0
Built: 2024-01-15T10:30:00Z
Go: 1.23
OS/Arch: linux/amd64
```

---

# PART 28: ERROR HANDLING & CACHING (NON-NEGOTIABLE)

## Error Handling

### User-Facing Errors

- Clear, actionable messages
- No stack traces in production
- Appropriate HTTP status codes
- Consistent format

### Error Codes

| Code | Description |
|------|-------------|
| `ERR_VALIDATION` | Input validation failed |
| `ERR_NOT_FOUND` | Resource not found |
| `ERR_UNAUTHORIZED` | Authentication required |
| `ERR_FORBIDDEN` | Permission denied |
| `ERR_INTERNAL` | Server error |
| `ERR_RATE_LIMIT` | Rate limit exceeded |

## Caching

### Cache Drivers

| Driver | Mode |
|--------|------|
| `memory` | Single instance |
| `redis` | Cluster mode |
| `memcached` | Cluster mode |

### Cache Headers

| Content Type | Header |
|--------------|--------|
| Static assets | `Cache-Control: max-age=31536000` |
| API responses | `Cache-Control: no-cache` |
| HTML pages | `Cache-Control: no-store` |

---

# PART 29: I18N & A11Y (NON-NEGOTIABLE)

## Internationalization (i18n)

- UTF-8 everywhere
- Accept-Language header respected
- Default: English (en)
- Extensible translation system

## Accessibility (a11y)

| Requirement | Description |
|-------------|-------------|
| WCAG 2.1 AA | Compliance required |
| Keyboard | Full navigation |
| Screen readers | Full support |
| ARIA labels | Proper usage |
| Color contrast | Proper ratios |
| Focus indicators | Visible |

---

# PART 30: PROJECT-SPECIFIC SECTIONS

## Vidveil-Specific API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/search` | GET | None | Search videos across all engines |
| `/api/v1/search/stream` | GET | None | SSE streaming search (real-time results) |
| `/api/v1/search.txt` | GET | None | Search results as plain text |
| `/api/v1/engines` | GET | None | List all search engines |
| `/api/v1/engines/{name}` | GET | None | Get engine details |
| `/api/v1/stats` | GET | None | Get search statistics |

## Vidveil-Specific Web Pages

| Page | Path | Description |
|------|------|-------------|
| Home | `/` | Search form |
| Search Results | `/search` | Video results grid with SSE streaming |
| Preferences | `/preferences` | User settings (theme, engines) |
| About | `/about` | About page |
| Privacy | `/privacy` | Privacy policy |
| Age Verify | `/age-verify` | Age verification gate |

## Search Engines (47 total)

**Tier 1 - Major Sites (6)**
- PornHub (Webmasters API), RedTube (Public API), Eporner (v2 API), XVideos, XNXX, xHamster

**Tier 2 - Popular Sites (2)**
- YouPorn, PornMD

**Tier 3 - Additional Sites (18)**
- 4Tube, Fux, PornTube, YouJizz, SunPorno, Txxx, Nuvid, TNAFlix, DrTuber
- EMPFlix, HellPorno, AlphaPorno, PornFlip, ZenPorn, GotPorn, HDZog
- XXXYMovies, LoveHomePorn

**Tier 4 - Extended Sites (17)**
- PornerBros, NonkTube, NubilesPorn, PornBox, PornTop, PornoTube, VPorn
- PornHD, XBabe, PornOne, PornHat, PornTrex, HQPorner, VJAV, FlyFLV
- Tube8, XTube

**Tier 5 - New Sites (4)**
- AnyPorn, SuperPorn, TubeGalore, Motherless

**Re-added (Dec 16, 2025)** - xHamster, YouPorn (with spoofed TLS for Cloudflare bypass)

**Still Removed (Cloudflare blocked, no viable bypass)**
- SpankBang, Beeg

## Vidveil-Specific Configuration

```yaml
# Search settings
search:
  default_engines:
    - pornhub
    - redtube
    - eporner
    - xvideos
    - xnxx
  concurrent_requests: 10
  engine_timeout: 15
  results_per_page: 50
  max_pages: 10
  min_duration_seconds: 300  # Filter out clips < 5 min
  filter_premium: true       # Filter out premium/gold content
  spoof_tls: true           # Chrome TLS fingerprint for Cloudflare bypass

  # Tor support
  tor:
    enabled: false
    proxy: "127.0.0.1:9050"

  # Age verification
  age_verification:
    enabled: true
    minimum_age: 18
    cookie_duration: 30d
```

## Notes

- **API-based engines** (PornHub, RedTube, Eporner) are fastest - use JSON APIs directly
- **HTML-parsing engines** use goquery for scraping
- **SSE streaming** delivers results in real-time as each engine responds
- **Cloudflare bypass** uses spoofed Chrome TLS fingerprint (utls library)
- **Re-added 2 engines** (xHamster, YouPorn) - now using spoofed TLS for Cloudflare bypass
- **Still removed** (SpankBang, Beeg) - Cloudflare blocked with no viable bypass
- **Headless browser code removed** - not needed with API integration

---

# PART 31: USER MANAGEMENT (NON-NEGOTIABLE)

## Overview

**Projects can operate in two modes: admin-only or multi-user.**

| Mode | Use Case | Default |
|------|----------|---------|
| **Admin-only** | Simple APIs (jokes, quotes, etc.) - just admin account | YES |
| **Multi-user** | Apps needing user accounts, registration, profiles, API tokens | NO |

## Server Admin vs Regular Users (NON-NEGOTIABLE)

**The server admin is a SYSTEM ACCOUNT, not a regular user.**

| Account Type | Storage | Login | Access |
|--------------|---------|-------|--------|
| **Server Admin** | Config file (`server.yml`) | `/auth/login` | `/admin/*` only |
| **Regular Users** | Database (users table) | `/auth/login` | `/user/*` routes |

### Server Admin Behavior

| Route | Server Admin Access |
|-------|---------------------|
| `/admin/*` | Full access |
| `/user/*` | NO - treated as guest (redirect to `/admin`) |
| `/auth/login` | Login page |
| `/auth/logout` | Logout |
| Public routes (`/`, `/server/*`, etc.) | Guest view (no user-specific content) |

**Server Admin Credentials:**
- Stored in `server.yml` (username, hashed password)
- Managed via `/admin/server/settings` (NOT `/user/profile`)
- NOT in the users database table
- Single account per instance

### Regular User Behavior

| Route | Regular User Access |
|-------|---------------------|
| `/admin/*` | NO - 403 Forbidden (unless user has admin role) |
| `/user/*` | Full access to own profile, settings, tokens |
| `/auth/login` | Login page |
| `/auth/logout` | Logout |
| Public routes | Authenticated view (may show user-specific content) |

**Regular User Accounts:**
- Stored in database (users table)
- Managed via `/user/profile`, `/user/settings`
- Can have roles (admin, user, custom)
- Multiple accounts supported

### Why This Separation?

| Reason | Description |
|--------|-------------|
| **Security** | Server admin has system-level access, not app-level |
| **Simplicity** | Admin-only mode doesn't need user management |
| **Isolation** | Server admin credentials separate from user data |
| **Recovery** | Can access admin even if database is corrupted |

## Configuration

```yaml
server:
  users:
    # Enable multi-user mode (default: disabled = admin-only)
    enabled: false

    registration:
      # Allow public registration
      enabled: false
      # Require email verification before account is active
      require_email_verification: true
      # Admin must approve new users
      require_approval: false
      # Allowed email domains (empty = all allowed)
      allowed_domains: []
      # Blocked email domains
      blocked_domains: []

    roles:
      # Available roles
      available:
        - admin
        - user
      # Default role for new users
      default: user

    tokens:
      # Allow users to generate API tokens
      enabled: true
      # Maximum tokens per user
      max_per_user: 5
      # Token expiration (0 = never)
      expiration_days: 0

    profile:
      # Allow users to upload avatars
      allow_avatar: true
      # Allow users to set display name
      allow_display_name: true
      # Allow users to set bio
      allow_bio: true

    auth:
      # Session duration
      session_duration: 30d
      # Require 2FA for all users
      require_2fa: false
      # Allow 2FA (user choice)
      allow_2fa: true
      # Password requirements
      password_min_length: 8
      password_require_uppercase: false
      password_require_number: false
      password_require_special: false

    limits:
      # Rate limits per user (0 = use global)
      requests_per_minute: 0
      requests_per_day: 0
```

## User Roles & Permissions

| Role | Description | Default Permissions |
|------|-------------|---------------------|
| `admin` | Full access | All permissions |
| `user` | Standard user | Read, own profile, own API tokens |

### Custom Roles

Projects can define custom roles with specific permissions:

```yaml
server:
  users:
    roles:
      available:
        - admin
        - moderator
        - user
        - readonly
      default: user
      permissions:
        moderator:
          - read
          - write
          - moderate
        readonly:
          - read
```

## User Features

### Registration Flow

```
1. User submits registration form
   ├─ If require_email_verification: Send verification email
   │   └─ User clicks link → account verified
   ├─ If require_approval: Admin notified
   │   └─ Admin approves → account active
   └─ If neither: Account immediately active

2. User can now log in
```

### Authentication Methods

| Method | Use For |
|--------|---------|
| Session (cookie) | Web interface |
| API token | API access (passed as Bearer token in Authorization header) |

### Password Reset Flow

```
1. User requests password reset
2. Email sent with reset link (expires in 1 hour)
3. User clicks link, sets new password
4. All existing sessions invalidated
5. User must log in with new password
```

### Two-Factor Authentication (2FA)

| Feature | Description |
|---------|-------------|
| TOTP | Time-based one-time passwords (Google Authenticator, etc.) |
| Backup codes | One-time use recovery codes |
| Remember device | Optional "trust this device" for 30 days |

## User Profile

| Field | Type | Configurable |
|-------|------|--------------|
| Email | Required | No (always required) |
| Display name | Optional | `profile.allow_display_name` |
| Avatar | Optional | `profile.allow_avatar` |
| Bio | Optional | `profile.allow_bio` |
| Timezone | Optional | Always available |
| Language | Optional | Always available |

## API Tokens

| Feature | Description |
|---------|-------------|
| Generate | User can create API tokens from profile |
| Name/Label | User can name tokens for identification |
| Permissions | Optional: limit token to specific scopes |
| Expiration | Optional: set expiry date |
| Last used | Track when token was last used |
| Revoke | User can delete tokens anytime |

### API Token Format

```
vidveil_{random_32_chars}

Example: jokes_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

## Admin Panel

### /admin/users (User Management)

| Element | Type | Description |
|---------|------|-------------|
| User list | Table | All users with search/filter |
| Create user | Button | Manually create user account |
| Edit user | Button | Modify user details |
| Delete user | Button | Remove user (confirmation required) |
| Impersonate | Button | Log in as user (admin only) |
| Disable/Enable | Toggle | Temporarily disable account |
| Reset password | Button | Send password reset email |
| Revoke sessions | Button | Log user out everywhere |

### /admin/users/{id} (User Detail)

| Section | Contents |
|---------|----------|
| Profile | Email, name, avatar, bio, role |
| Security | 2FA status, sessions, password reset |
| API Tokens | List of user's API tokens |
| Activity | Login history, API usage |
| Limits | Per-user rate limits |

### /admin/roles (Role Management)

| Element | Type | Description |
|---------|------|-------------|
| Role list | Table | All roles |
| Create role | Button | Define new role |
| Edit permissions | Checkboxes | Set role permissions |
| Delete role | Button | Remove role (reassign users first) |

### /admin/invites (Invitation Codes)

| Element | Type | Description |
|---------|------|-------------|
| Generate invite | Button | Create invitation code/link |
| Invite list | Table | All invites with status |
| Expiration | Date picker | When invite expires |
| Max uses | Number | How many times invite can be used |
| Role | Dropdown | What role invited users get |
| Revoke | Button | Disable invite |

## Route Standards (NON-NEGOTIABLE)

**All routes MUST follow these standards:**

| Rule | Description |
|------|-------------|
| **Scoped** | Routes grouped by scope: `/auth`, `/user`, `/org`, `/admin` |
| **Mirrored** | Web (`/`) and API (`/api/v1/`) use same structure |
| **Intuitive** | Simple, predictable paths |
| **Params over queries** | Use path params, limit query params to defined cases |
| **Duplicated when needed** | Same resource may exist in multiple scopes |

### Response Formats

| Route | Default | Options |
|-------|---------|---------|
| `/` (web) | HTML | - |
| `/api/v1/` | JSON (`application/json`) | JSON, Text |
| `/api/v1/**/*.txt` | Text (`text/plain`) | - |

### Scopes

| Scope | Web | API | Description |
|-------|-----|-----|-------------|
| Public | `/` | `/api/v1/` | Public resources, unauthenticated |
| Auth | `/auth/` | `/api/v1/auth/` | Authentication flows |
| User | `/user/` | `/api/v1/user/` | Current user's resources |
| Org | `/org/` | `/api/v1/org/` | Organization resources (if applicable) |
| Admin | `/admin/` | `/api/v1/admin/` | Admin/server management |

## Web Routes

### Public (`/`)

| Path | Description |
|------|-------------|
| `/` | Home page |
| `/healthz` | Health check |
| `/openapi` | Swagger UI |
| `/graphql` | GraphiQL interface |

### Server (`/server/`)

| Path | Description |
|------|-------------|
| `/server/about` | About the application |
| `/server/privacy` | Privacy policy |
| `/server/contact` | Contact form |
| `/server/help` | Help / documentation |

### Auth (`/auth/`)

| Path | Description |
|------|-------------|
| `/auth/login` | Login form |
| `/auth/logout` | Logout |
| `/auth/register` | Registration form |
| `/auth/password/forgot` | Request password reset |
| `/auth/password/reset/{token}` | Reset password form |
| `/auth/verify/{token}` | Email verification |

### User (`/user/`)

| Path | Description |
|------|-------------|
| `/user/profile` | View/edit profile |
| `/user/settings` | Account settings |
| `/user/tokens` | Manage API tokens |
| `/user/security` | 2FA, sessions |
| `/user/security/sessions` | Active sessions |
| `/user/security/2fa` | Two-factor settings |

### Admin (`/admin/`)

| Path | Description |
|------|-------------|
| `/admin` | Dashboard |
| `/admin/server/setup` | Initial setup |
| `/admin/server/settings` | Server settings |
| `/admin/server/branding` | Branding & SEO |
| `/admin/server/ssl` | SSL/TLS settings |
| `/admin/server/tor` | Tor hidden service |
| `/admin/server/web` | Web settings (robots.txt, security.txt) |
| `/admin/server/pages` | Standard pages (about, privacy, contact) |
| `/admin/server/email` | Email/SMTP settings |
| `/admin/server/email/templates` | Email templates |
| `/admin/server/notifications` | Notification settings |
| `/admin/server/scheduler` | Scheduled tasks |
| `/admin/server/backup` | Backup & restore |
| `/admin/server/logs` | Log viewer |
| `/admin/users` | User management |
| `/admin/users/{id}` | User detail |
| `/admin/roles` | Role management |
| `/admin/invites` | Invitation codes |

## API Routes

### Public (`/api/v1/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/healthz` | GET | Health check |
| `/api/v1/openapi.json` | GET | OpenAPI spec (JSON) |
| `/api/v1/openapi.yaml` | GET | OpenAPI spec (YAML) |

### Server (`/api/v1/server/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/server/about` | GET | About information |
| `/api/v1/server/privacy` | GET | Privacy policy |
| `/api/v1/server/contact` | POST | Submit contact form |
| `/api/v1/server/help` | GET | Help content |

### Auth (`/api/v1/auth/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/auth/register` | POST | Register new user |
| `/api/v1/auth/login` | POST | User login |
| `/api/v1/auth/logout` | POST | User logout |
| `/api/v1/auth/password/forgot` | POST | Request password reset |
| `/api/v1/auth/password/reset` | POST | Set new password |
| `/api/v1/auth/verify` | POST | Verify email address |
| `/api/v1/auth/refresh` | POST | Refresh session/token |

### User (`/api/v1/user/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/user/profile` | GET | Get own profile |
| `/api/v1/user/profile` | PATCH | Update own profile |
| `/api/v1/user/password` | POST | Change password |
| `/api/v1/user/tokens` | GET | List own API tokens |
| `/api/v1/user/tokens` | POST | Create API token |
| `/api/v1/user/tokens/{id}` | GET | Get token details |
| `/api/v1/user/tokens/{id}` | DELETE | Revoke API token |
| `/api/v1/user/sessions` | GET | List active sessions |
| `/api/v1/user/sessions/{id}` | DELETE | Revoke session |
| `/api/v1/user/2fa` | GET | Get 2FA status |
| `/api/v1/user/2fa/enable` | POST | Enable 2FA |
| `/api/v1/user/2fa/disable` | POST | Disable 2FA |
| `/api/v1/user/2fa/backup-codes` | POST | Generate backup codes |

### Admin - Users (`/api/v1/admin/users/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/users` | GET | List all users |
| `/api/v1/admin/users` | POST | Create user |
| `/api/v1/admin/users/{id}` | GET | Get user details |
| `/api/v1/admin/users/{id}` | PATCH | Update user |
| `/api/v1/admin/users/{id}` | DELETE | Delete user |
| `/api/v1/admin/users/{id}/disable` | POST | Disable user |
| `/api/v1/admin/users/{id}/enable` | POST | Enable user |
| `/api/v1/admin/users/{id}/impersonate` | POST | Get impersonation token |

### Admin - Roles (`/api/v1/admin/roles/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/roles` | GET | List roles |
| `/api/v1/admin/roles` | POST | Create role |
| `/api/v1/admin/roles/{id}` | GET | Get role details |
| `/api/v1/admin/roles/{id}` | PATCH | Update role |
| `/api/v1/admin/roles/{id}` | DELETE | Delete role |

### Admin - Invites (`/api/v1/admin/invites/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/invites` | GET | List invites |
| `/api/v1/admin/invites` | POST | Create invite |
| `/api/v1/admin/invites/{id}` | GET | Get invite details |
| `/api/v1/admin/invites/{id}` | DELETE | Revoke invite |

### Admin - Server (`/api/v1/admin/server/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/setup` | GET | Get setup status |
| `/api/v1/admin/server/setup` | POST | Complete initial setup |
| `/api/v1/admin/server/settings` | GET | Get server settings |
| `/api/v1/admin/server/settings` | PATCH | Update server settings |
| `/api/v1/admin/server/status` | GET | Server status |
| `/api/v1/admin/server/health` | GET | Detailed health |
| `/api/v1/admin/server/stats` | GET | Statistics |
| `/api/v1/admin/server/restart` | POST | Restart server |

### Admin - Branding (`/api/v1/admin/server/branding/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/branding` | GET | Get branding settings |
| `/api/v1/admin/server/branding` | PATCH | Update branding |

### Admin - SSL (`/api/v1/admin/server/ssl/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/ssl` | GET | Get SSL settings |
| `/api/v1/admin/server/ssl` | PATCH | Update SSL settings |
| `/api/v1/admin/server/ssl/renew` | POST | Force certificate renewal |

### Admin - Tor (`/api/v1/admin/server/tor/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/tor` | GET | Get Tor status |
| `/api/v1/admin/server/tor` | PATCH | Update Tor settings |
| `/api/v1/admin/server/tor/regenerate` | POST | Regenerate .onion address |
| `/api/v1/admin/server/tor/vanity` | GET | Get vanity generation status |
| `/api/v1/admin/server/tor/vanity` | POST | Start vanity generation |
| `/api/v1/admin/server/tor/vanity` | DELETE | Cancel vanity generation |
| `/api/v1/admin/server/tor/vanity/apply` | POST | Apply vanity address |
| `/api/v1/admin/server/tor/import` | POST | Import external keys |

### Admin - Web (robots.txt, security.txt) (`/api/v1/admin/server/web/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/web` | GET | Get web settings |
| `/api/v1/admin/server/web` | PATCH | Update web settings |
| `/api/v1/admin/server/web/robots` | GET | Get robots.txt config |
| `/api/v1/admin/server/web/robots` | PATCH | Update robots.txt |
| `/api/v1/admin/server/web/robots/preview` | GET | Preview robots.txt |
| `/api/v1/admin/server/web/security` | GET | Get security.txt config |
| `/api/v1/admin/server/web/security` | PATCH | Update security.txt |
| `/api/v1/admin/server/web/security/preview` | GET | Preview security.txt |

### Admin - Pages (`/api/v1/admin/server/pages/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/pages` | GET | Get all page settings |
| `/api/v1/admin/server/pages/about` | GET | Get about page content |
| `/api/v1/admin/server/pages/about` | PATCH | Update about page content |
| `/api/v1/admin/server/pages/privacy` | GET | Get privacy policy content |
| `/api/v1/admin/server/pages/privacy` | PATCH | Update privacy policy content |
| `/api/v1/admin/server/pages/contact` | GET | Get contact page settings |
| `/api/v1/admin/server/pages/contact` | PATCH | Update contact page settings |
| `/api/v1/admin/server/pages/help` | GET | Get help page content |
| `/api/v1/admin/server/pages/help` | PATCH | Update help page content |

### Admin - Email (`/api/v1/admin/server/email/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/email` | GET | Get email settings |
| `/api/v1/admin/server/email` | PATCH | Update email settings |
| `/api/v1/admin/server/email/test` | POST | Send test email |
| `/api/v1/admin/server/email/templates` | GET | List email templates |
| `/api/v1/admin/server/email/templates/{name}` | GET | Get template |
| `/api/v1/admin/server/email/templates/{name}` | PUT | Update template |
| `/api/v1/admin/server/email/templates/{name}/reset` | POST | Reset to default |
| `/api/v1/admin/server/email/templates/{name}/preview` | POST | Preview template |

### Admin - Scheduler (`/api/v1/admin/server/scheduler/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/scheduler` | GET | List scheduled tasks |
| `/api/v1/admin/server/scheduler/{id}` | GET | Get task details |
| `/api/v1/admin/server/scheduler/{id}` | PATCH | Update task |
| `/api/v1/admin/server/scheduler/{id}/run` | POST | Run task now |
| `/api/v1/admin/server/scheduler/{id}/enable` | POST | Enable task |
| `/api/v1/admin/server/scheduler/{id}/disable` | POST | Disable task |

### Admin - Backup (`/api/v1/admin/server/backup/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/backup` | GET | List backups |
| `/api/v1/admin/server/backup` | POST | Create backup |
| `/api/v1/admin/server/backup/{id}` | GET | Get backup details |
| `/api/v1/admin/server/backup/{id}` | DELETE | Delete backup |
| `/api/v1/admin/server/backup/{id}/download` | GET | Download backup file |
| `/api/v1/admin/server/backup/restore` | POST | Restore from backup |

### Admin - Logs (`/api/v1/admin/server/logs/`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/server/logs` | GET | List log files |
| `/api/v1/admin/server/logs/{type}` | GET | Get log entries |
| `/api/v1/admin/server/logs/{type}/download` | GET | Download log file |

## Email Templates (User-Related)

| Template | Purpose |
|----------|---------|
| `user_welcome` | Welcome email after registration |
| `user_verify_email` | Email verification link |
| `user_password_reset` | Password reset link |
| `user_password_changed` | Confirmation of password change |
| `user_2fa_enabled` | Confirmation of 2FA enabled |
| `user_new_login` | Alert for login from new device/location |
| `user_invite` | Invitation email |
| `user_account_disabled` | Account has been disabled |

## Database Schema

### Users Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `email` | String | Unique, required |
| `password_hash` | String | Bcrypt hash |
| `display_name` | String | Optional |
| `avatar_url` | String | Optional |
| `bio` | Text | Optional |
| `role` | String | User role |
| `email_verified` | Boolean | Email verified status |
| `approved` | Boolean | Admin approved (if required) |
| `disabled` | Boolean | Account disabled |
| `totp_secret` | String | 2FA secret (encrypted) |
| `totp_enabled` | Boolean | 2FA enabled |
| `timezone` | String | User timezone |
| `language` | String | User language |
| `created_at` | Timestamp | Account creation |
| `updated_at` | Timestamp | Last update |
| `last_login_at` | Timestamp | Last login |

### Tokens Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Foreign key to users |
| `name` | String | User-defined label |
| `token_hash` | String | Hashed API token |
| `token_prefix` | String | First 8 chars (for identification) |
| `scopes` | JSON | Optional permission scopes |
| `expires_at` | Timestamp | Optional expiration |
| `last_used_at` | Timestamp | Last usage |
| `created_at` | Timestamp | Creation time |

### Sessions Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Foreign key to users |
| `token_hash` | String | Hashed session token |
| `ip_address` | String | Client IP |
| `user_agent` | String | Browser/client info |
| `location` | String | GeoIP location |
| `expires_at` | Timestamp | Session expiry |
| `created_at` | Timestamp | Session start |

### Invites Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `code` | String | Unique invite code |
| `role` | String | Role for invited users |
| `max_uses` | Integer | Maximum uses (0 = unlimited) |
| `use_count` | Integer | Current use count |
| `expires_at` | Timestamp | Expiration |
| `created_by` | UUID | Admin who created |
| `created_at` | Timestamp | Creation time |

---

# PART 32: TOR HIDDEN SERVICE (NON-NEGOTIABLE)

## Overview

**ALL projects MUST have built-in Tor hidden service support.**

Tor integration uses **external Tor binary** via `github.com/cretz/bine`. This maintains `CGO_ENABLED=0` compatibility for static binaries while providing full Tor hidden service functionality.

## Configuration

```yaml
server:
  tor:
    enabled: true  # Default: enabled
    # Path to Tor binary (auto-detected if empty)
    binary: ""
```

**Notes:**
- Uses external Tor binary (not embedded) for CGO_ENABLED=0 compatibility
- Enabled by default - privacy out of the box
- .onion address derived from keys in `{data_dir}/tor/site/`
- Application manages Tor process lifecycle

## Tor Process Management (NON-NEGOTIABLE)

**The application MUST start its OWN dedicated Tor process. NEVER use system Tor.**

This prevents conflicts with any existing Tor installation on the system.

```
1. Find Tor binary:
   ├─ Check config `server.tor.binary` path
   ├─ Check PATH for `tor` executable
   ├─ Check common locations:
   │   ├─ Linux: /usr/bin/tor, /usr/local/bin/tor
   │   ├─ macOS: /usr/local/bin/tor, /opt/homebrew/bin/tor
   │   ├─ Windows: C:\Program Files\Tor\tor.exe
   │   └─ BSD: /usr/local/bin/tor
   └─ NOT FOUND: Log warning, disable Tor features, continue without Tor

2. Start DEDICATED Tor process:
   ├─ Use application's own DataDir: `{data_dir}/tor/`
   ├─ Use random available ControlPort (not 9051)
   ├─ Use random available SocksPort (not 9050)
   ├─ Completely isolated from system Tor
   ├─ Wait for bootstrap completion
   └─ Create hidden service via ADD_ONION

3. On application shutdown:
   └─ Terminate the dedicated Tor process
```

### Why Dedicated Tor Process?

| Reason | Description |
|--------|-------------|
| **No conflicts** | System Tor uses 9050/9051, we use random ports |
| **Isolation** | Our DataDir is separate from system Tor |
| **Clean shutdown** | We control the process lifecycle |
| **No permissions issues** | Don't need access to system Tor control |
| **Predictable behavior** | Always same configuration |

## Implementation

### Library

Use `github.com/cretz/bine` (pure Go, CGO_ENABLED=0 compatible):

```go
import (
    "github.com/cretz/bine/tor"
)

func startDedicatedTor(ctx context.Context, localPort int) (*tor.Tor, *tor.OnionService, error) {
    // Start OUR OWN Tor process - completely separate from system Tor
    t, err := tor.Start(ctx, &tor.StartConf{
        // Our own data directory - isolated from system Tor
        DataDir: paths.GetDataDir() + "/tor",

        // Let bine pick available ports (avoids conflict with system Tor 9050/9051)
        // These are set automatically to available ports
        NoAutoSocksPort: false,

        // Optional: specify path if not in PATH
        // ExePath: "/usr/bin/tor",

        // Debug output (development only)
        // DebugWriter: os.Stderr,
    })
    if err != nil {
        return nil, nil, fmt.Errorf("failed to start dedicated tor: %w", err)
    }

    // Wait for Tor to bootstrap
    dialCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
    defer cancel()
    if err := t.EnableNetwork(dialCtx, true); err != nil {
        t.Close()
        return nil, nil, fmt.Errorf("failed to enable tor network: %w", err)
    }

    // Create hidden service
    onion, err := t.Listen(ctx, &tor.ListenConf{
        RemotePorts: []int{80},
        LocalPort:   localPort,
    })
    if err != nil {
        t.Close()
        return nil, nil, fmt.Errorf("failed to create onion service: %w", err)
    }

    // onion.ID contains the .onion address (without .onion suffix)
    log.Printf("Tor hidden service started: %s.onion", onion.ID)
    return t, onion, nil
}

// Shutdown cleanly terminates our dedicated Tor process
func shutdownTor(t *tor.Tor) error {
    if t != nil {
        return t.Close()
    }
    return nil
}
```

### Port Allocation

| Port | System Tor | Our Tor |
|------|------------|---------|
| SocksPort | 9050 | Random available |
| ControlPort | 9051 | Random available |
| DataDir | `/var/lib/tor` | `{data_dir}/tor/` |

**bine automatically selects available ports**, ensuring no conflict with system Tor.

### Tor Configuration Optimizations (NON-NEGOTIABLE)

**Tor is used ONLY for hidden services. Optimize accordingly.**

```go
// Optimized torrc settings for hidden-service-only mode
func getTorConfig() string {
    return `
# Hidden service only - not a relay or exit
SocksPort 0
# No SOCKS proxy needed - we're server only

# Disable unused features
ExitRelay 0
ExitPolicy reject *:*
# Never act as exit node

# Don't relay traffic for others
ORPort 0
DirPort 0

# Reduce circuit building (we only need service circuits)
MaxCircuitDirtiness 600
# Keep circuits longer

# Reduce bandwidth for Tor overhead
BandwidthRate 1 MB
BandwidthBurst 2 MB

# Hidden service optimizations
HiddenServiceSingleHopMode 0
# Keep full anonymity (3 hops)

# Faster startup
FetchDirInfoEarly 1
FetchDirInfoExtraEarly 1

# Reduce memory usage
DisableDebuggerAttachment 1
`
}
```

| Setting | Value | Reason |
|---------|-------|--------|
| `SocksPort 0` | Disabled | Not browsing, server only |
| `ExitRelay 0` | Disabled | Not an exit node |
| `ORPort 0` | Disabled | Not relaying traffic |
| `DirPort 0` | Disabled | Not a directory server |
| `ExitPolicy reject *:*` | Block all | Extra safety |
| `MaxCircuitDirtiness 600` | 10 minutes | Keep circuits longer |

### Tor Process Lifecycle (NON-NEGOTIABLE)

**The application MUST fully manage the Tor process lifecycle.**

| Event | Action |
|-------|--------|
| **App Start** | Find Tor binary → Start dedicated Tor process → Wait for bootstrap → Create hidden service |
| **App Running** | Monitor Tor process, restart if crashed |
| **App Shutdown** | Terminate Tor process gracefully (SIGTERM) |
| **App Crash** | Tor process should terminate (child process dies with parent) |
| **SIGTERM/SIGINT** | Graceful shutdown: stop Tor, then exit |
| **SIGHUP** | Reload config, restart Tor if settings changed |

### Tor Restart Triggers (NON-NEGOTIABLE)

**Tor MUST be restarted when these events occur:**

| Trigger | Action | Notes |
|---------|--------|-------|
| **Regenerate .onion address** | Stop Tor → Delete keys → Start Tor | New random address |
| **Apply vanity address** | Stop Tor → Replace keys → Start Tor | Use generated vanity keys |
| **Import external keys** | Stop Tor → Replace keys → Start Tor | Use imported keys |
| **Enable Tor** | Start Tor | Was disabled |
| **Disable Tor** | Stop Tor | User disabled |
| **Tor process crash** | Restart Tor | Auto-recovery |
| **Tor unresponsive** | Stop Tor → Start Tor | Health check failed |
| **Config change** | Stop Tor → Start Tor | Settings changed |

### Restart Implementation

```go
// TorManager handles all Tor lifecycle operations
type TorManager struct {
    mu        sync.Mutex
    tor       *tor.Tor
    onion     *tor.OnionService
    dataDir   string
    localPort int
    ctx       context.Context
    cancel    context.CancelFunc
}

// Restart stops and starts Tor (used for config changes, recovery)
func (tm *TorManager) Restart() error {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Stop existing
    if tm.tor != nil {
        tm.tor.Close()
        tm.tor = nil
        tm.onion = nil
    }

    // Start fresh
    return tm.startLocked()
}

// RegenerateAddress creates a new random .onion address
func (tm *TorManager) RegenerateAddress() (string, error) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Stop Tor
    if tm.tor != nil {
        tm.tor.Close()
        tm.tor = nil
        tm.onion = nil
    }

    // Delete existing keys
    keysDir := filepath.Join(tm.dataDir, "site")
    if err := os.RemoveAll(keysDir); err != nil {
        return "", fmt.Errorf("failed to remove old keys: %w", err)
    }

    // Start Tor - new keys will be generated
    if err := tm.startLocked(); err != nil {
        return "", err
    }

    return tm.onion.ID + ".onion", nil
}

// ApplyKeys stops Tor, replaces keys, and restarts
func (tm *TorManager) ApplyKeys(privateKey []byte) (string, error) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Stop Tor
    if tm.tor != nil {
        tm.tor.Close()
        tm.tor = nil
        tm.onion = nil
    }

    // Write new keys
    keysDir := filepath.Join(tm.dataDir, "site")
    os.MkdirAll(keysDir, 0700)
    keyPath := filepath.Join(keysDir, "hs_ed25519_secret_key")
    if err := os.WriteFile(keyPath, privateKey, 0600); err != nil {
        return "", fmt.Errorf("failed to write key: %w", err)
    }

    // Start Tor with new keys
    if err := tm.startLocked(); err != nil {
        return "", err
    }

    return tm.onion.ID + ".onion", nil
}

// SetEnabled enables or disables Tor
func (tm *TorManager) SetEnabled(enabled bool) error {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    if enabled {
        if tm.tor == nil {
            return tm.startLocked()
        }
    } else {
        if tm.tor != nil {
            tm.tor.Close()
            tm.tor = nil
            tm.onion = nil
        }
    }
    return nil
}
```

### Signal Handling with Tor

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start Tor
    torProcess, onion, err := startDedicatedTor(ctx, localPort)
    if err != nil {
        log.Printf("Warning: Tor disabled - %v", err)
        // Continue without Tor
    }

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        log.Println("Shutting down...")

        // Stop Tor FIRST
        if torProcess != nil {
            log.Println("Stopping Tor process...")
            torProcess.Close()
        }

        // Then cancel context for other goroutines
        cancel()
    }()

    // Run server...
}
```

### Tor Process Monitoring

```go
// Monitor Tor and restart if it crashes
func monitorTor(ctx context.Context, torProcess *tor.Tor, restartFunc func() (*tor.Tor, error)) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Check if Tor is still responsive
            if torProcess != nil {
                // Ping control connection
                if _, err := torProcess.Control.GetInfo("version"); err != nil {
                    log.Println("Tor process unresponsive, restarting...")
                    torProcess.Close()
                    newTor, err := restartFunc()
                    if err != nil {
                        log.Printf("Failed to restart Tor: %v", err)
                    } else {
                        torProcess = newTor
                    }
                }
            }
        }
    }
}
```

### Binary Size

No impact on binary size - Tor is external. Application binary remains small and static.

### Data Storage

| Data | Location |
|------|----------|
| Tor data directory | `{data_dir}/tor/` |
| Hidden service keys | `{data_dir}/tor/site/` |
| Tor process PID | `{data_dir}/tor/tor.pid` |

## Admin Panel

### /admin/server/tor (Web UI)

| Element | Type | Description |
|---------|------|-------------|
| Enable Tor | Toggle switch | Turn hidden service on/off |
| Status | Indicator | ● Connected / ○ Disconnected / ⚠ Error |
| .onion Address | Read-only text | Full address with copy button |
| Regenerate Address | Button | Creates new random .onion (requires confirmation modal) |
| Vanity Prefix | Text input | Desired prefix (max 6 characters) |
| Generate Vanity | Button | Starts background generation |
| Vanity Status | Progress indicator | Shows when generating in background |
| Import Keys | File upload | Import externally generated keys |

**Status Card Example:**
```
┌─────────────────────────────────────────────────────────┐
│ Tor Hidden Service                                      │
│                                                         │
│ Status: ● Connected                                     │
│ Address: abcd1234...wxyz.onion                  [Copy]  │
│                                                         │
│ [Regenerate Address]                                    │
├─────────────────────────────────────────────────────────┤
│ Vanity Address                                          │
│                                                         │
│ Prefix: [______] (max 6 chars)  [Generate]              │
│                                                         │
│ ⏳ Generating: "jokes" - 2h 15m elapsed...              │
│    [Cancel]                                             │
├─────────────────────────────────────────────────────────┤
│ Import External Keys                      [Import Keys] │
│ ⓘ Help: How to generate longer vanity addresses        │
└─────────────────────────────────────────────────────────┘
```

### Vanity Address Generation

**Built-in generation (max 6 characters):**

| Prefix Length | Approximate Time |
|---------------|------------------|
| 1-4 chars | Seconds to minutes |
| 5 chars | Minutes to hours |
| 6 chars | Hours to days |

**Behavior:**
- Generation runs in background
- Current .onion address remains active while generating
- Notification sent when vanity address is ready
- User clicks notification or "Apply" button to activate
- Old keys deleted, new vanity keys activated
- Tor restarts with new address

### External Vanity Generation (7+ characters)

For prefixes longer than 6 characters, use external tools with GPU acceleration. The admin panel includes documentation (expandable help section):

**Using mkp224o (CPU):**
```bash
# Install
git clone https://github.com/cathugger/mkp224o
cd mkp224o && ./autogen.sh && ./configure && make

# Generate (example: 7-char prefix "myapp12")
./mkp224o -d ./keys myapp12

# Output: ./keys/myapp12xxxxx.onion/
#   ├── hostname        # Your .onion address
#   ├── hs_ed25519_public_key
#   └── hs_ed25519_secret_key
```

**Using mkp224o (GPU - much faster):**
```bash
# With CUDA support
./configure --enable-cuda
make

# Generate
./mkp224o -d ./keys myapp12
```

**Importing keys:**
1. Generate keys using mkp224o or similar tool
2. In admin panel, click "Import Keys"
3. Upload `hs_ed25519_secret_key` file (or zip containing both key files)
4. Confirm to replace current address
5. Tor restarts with imported keys

**Time estimates for longer prefixes:**

| Prefix Length | CPU Time | GPU Time |
|---------------|----------|----------|
| 7 chars | Days to weeks | Hours to days |
| 8 chars | Weeks to months | Days to weeks |
| 9+ chars | Months to years | Weeks to months |

**Security Notes:**
- .onion address shown only after admin authentication
- "Regenerate Address" requires confirmation modal (destructive - old address stops working)
- Address regeneration logged to audit log
- Imported keys should be generated on a trusted machine
- Delete source key files after successful import

### API Endpoints

**See PART 28 for full API route definitions under `/api/v1/admin/server/tor/`**

### Response Format

```json
{
  "enabled": true,
  "status": "connected",
  "onion_address": "abcd1234efgh5678ijkl9012mnop3456qrst7890uvwx.onion",
  "uptime": "2d 5h 30m"
}
```

## Behavior

| Scenario | Behavior |
|----------|----------|
| First run | Tor starts, generates .onion address, saves to config |
| Subsequent runs | Tor starts, uses existing .onion address |
| Disabled in config | Tor does not start, no .onion available |
| Regenerate address | Old keys deleted, new .onion generated, config updated |
| Network issues | Tor retries connection automatically |

## CLI

The `--status` command includes Tor status:

```
$ myapp --status

Server Status: Running
  Port: 8080
  Mode: production
  Uptime: 2d 5h 30m

Tor Hidden Service: Connected
  Address: abcd1234...wxyz.onion
```

---

# PART 33: AI ASSISTANT RULES (NON-NEGOTIABLE)

**These rules govern how AI assistants work on projects using this specification.**

## Before Starting Work

1. **Read AI.md completely** - understand all project-specific rules
2. **Check TODO.AI.md** - see pending tasks and their priority
3. **Verify understanding** - if anything is unclear, ASK before proceeding
4. **Never assume** - when in doubt, ask the user

## During Work

1. **Follow this spec exactly** - NON-NEGOTIABLE means NO exceptions
2. **Re-read relevant sections** before implementing each feature
3. **Update TODO.AI.md** as tasks are completed
4. **Test your changes** - don't commit untested code
5. **Keep changes focused** - one feature/fix per task

## After Work

1. **Update AI.md** if architecture or rules changed
2. **Update TODO.AI.md** with any new tasks discovered
3. **Verify compliance** - check against the FINAL CHECKPOINT

## Attribution

| Rule | Description |
|------|-------------|
| **NEVER** | Include AI attribution in code, comments, commits, or documentation |
| **NEVER** | "authored by Claude", "generated by AI", "Co-Authored-By: Claude" |
| **ALWAYS** | Code should appear as if written by a human developer |

## Tool Access

| Access | Description |
|--------|-------------|
| Full | All tools available |
| **EXCEPT** | `git push`, `git commit` - require explicit user request |
| Allowed | Stage files, create branches, check status, diff |
| Required | User must explicitly request commits and pushes |

## Prohibited Actions

| Action | Reason |
|--------|--------|
| `git push` without asking | May push unreviewed code |
| `git commit` without asking | User should review changes first |
| Deleting files without confirmation | Destructive action |
| Changing NON-NEGOTIABLE sections | Specification violation |
| Skipping validation | Security requirement |
| Hardcoding secrets | Security vulnerability |
| Using deprecated APIs | Maintainability issue |

## Code Style Rules (NON-NEGOTIABLE)

### Comment Placement

**Comments MUST always be placed ABOVE the code they describe. NEVER inline or below.**

| Placement | Allowed |
|-----------|---------|
| Above code | YES |
| Inline (same line) | NO |
| Below code | NO |

**Correct:**
```go
// Calculate the total price including tax
total := price * (1 + taxRate)

// User configuration options
type Config struct {
    // Server port number
    Port int
    // Enable debug mode
    Debug bool
}
```

**Incorrect:**
```go
total := price * (1 + taxRate) // Calculate total price - WRONG

total := price * (1 + taxRate)
// Calculate total price - WRONG (below)

type Config struct {
    Port int  // Server port - WRONG (inline)
    Debug bool // Debug mode - WRONG (inline)
}
```

**YAML comments - same rule:**
```yaml
# Enable multi-user mode
enabled: false

# Allow public registration
registration:
  enabled: false
```

### Code Quality Rules

| Rule | Description |
|------|-------------|
| **No magic numbers** | Use named constants |
| **No hardcoded strings** | Use constants or config |
| **Error handling** | Always handle errors, never ignore |
| **Input validation** | Validate ALL user input |
| **SQL injection** | Use parameterized queries only |
| **XSS prevention** | Escape all output in templates |
| **CSRF protection** | All forms must have CSRF tokens |

### File Organization

| Rule | Description |
|------|-------------|
| **One package per directory** | Standard Go convention |
| **Meaningful names** | `user.go` not `u.go` |
| **Group related code** | Keep related functions together |
| **Separate concerns** | Don't mix handlers with business logic |

## Handling Ambiguity

When the specification is unclear:

1. **Check if clarified elsewhere** - search the full spec
2. **Look for similar patterns** - how are similar features handled?
3. **Ask the user** - don't guess
4. **Document the decision** - add to AI.md for future reference

## Common Mistakes to Avoid

| Mistake | Correct Approach |
|---------|------------------|
| Implementing without reading spec | Read relevant PART first |
| Assuming default values | Check spec for defined defaults |
| Using .yaml instead of .yml | Always use `server.yml` |
| Inline comments | Comments above code only |
| Skipping admin panel | ALL settings need admin UI |
| Forgetting mobile-first | Start with mobile, expand to desktop |
| Using JavaScript alerts | Use proper notification system |
| Inline CSS | Use CSS files/classes only |

---

# FINAL CHECKPOINT: COMPLIANCE CHECKLIST

**Before starting ANY work, verify you have read and understood:**

## Core Requirements

- [ ] Re-read this spec periodically during work
- [ ] AI.md must be kept in sync with project state
- [ ] TODO.AI.md required for more than 2 tasks
- [ ] Never assume or guess - ask questions

## Development

- [ ] All 4 OSes supported (Linux, macOS, BSD, Windows)
- [ ] Both architectures supported (AMD64, ARM64)
- [ ] CGO_ENABLED=0 for static binaries
- [ ] Single static binary with embedded assets

## Configuration

- [ ] Config file is `server.yml` (not .yaml)
- [ ] Boolean handling accepts all truthy/falsy values
- [ ] Sane defaults for everything

## Frontend

- [ ] Frontend required for ALL projects
- [ ] NO inline CSS
- [ ] NO JavaScript alerts
- [ ] Dark theme (Dracula) is default
- [ ] Mobile-first responsive design

## API

- [ ] All 3 API types: REST, Swagger, GraphQL
- [ ] Standard endpoints exist (/healthz, /openapi, /graphql, /admin)
- [ ] Versioned API: /api/v1

## Admin Panel

- [ ] Full admin panel required
- [ ] Web interface (/admin) with session auth
- [ ] REST API (/api/v1/admin) with bearer token

## CLI

- [ ] All standard commands implemented
- [ ] --help, --version, --status work without privileges
- [ ] --update command with check/yes/branch subcommands

## Build & Deploy

- [ ] 4 Makefile targets: build, release, docker, test
- [ ] 4 GitHub workflows: release, beta, daily, docker
- [ ] 8 platform builds (4 OS x 2 arch)
- [ ] Docker uses tini, Alpine base

## Security

- [x] All security headers implemented
- [x] Sensitive data never exposed unless necessary
- [x] Rate limiting in production

---

# CURRENT PROJECT STATE

## TEMPLATE.md Compliance Verification (December 16, 2025)

Full line-by-line verification completed. All 33 PARTs verified as compliant:

| Area | Status |
|------|--------|
| Directory Structure | PASS - All 5 required directories present |
| Required Files | PASS - All 8 mandatory files exist |
| GitHub Workflows | PASS - 4 workflows (release, beta, daily, docker) |
| Go Dependencies | PASS - Pure Go libraries only (modernc.org/sqlite, cretz/bine) |
| Forbidden Libraries | PASS - No CGO dependencies |
| Go Templates | PASS - .tmpl extension, 5 mandatory partials |
| API Endpoints | PASS - All required root-level endpoints |
| Services | PASS - All 9 required services implemented |
| Admin Panel | PASS - 11 sections |
| Email Templates | PASS - 14 templates (10 required + 4 additional) |

**Overall: 100% Compliant (33/33 PARTs)**

## Completed Features

- [x] Full project rename from XXXSearch to Vidveil
- [x] 45 search engines implemented across 5 tiers (4 Cloudflare-blocked engines removed)
- [x] **API-based engines**: PornHub (Webmasters API), RedTube (Public API), Eporner (v2 API)
- [x] **SSE Streaming**: Real-time result streaming via `/api/v1/search/stream`
- [x] REST API (`/api/v1/search`, `/api/v1/engines`, `/api/v1/healthz`)
- [x] GraphQL API (`/graphql`)
- [x] OpenAPI/Swagger documentation (`/openapi`, `/openapi.json`, `/openapi.yaml`)
- [x] Web UI with search, about, privacy, and preferences pages
- [x] Mobile-first responsive design
- [x] Dark (Dracula) and light theme support
- [x] All 5 mandatory template partials
- [x] Static binary with embedded assets
- [x] Docker support (Alpine-based, tini init, port 80)
- [x] Application mode support (production/development)
- [x] GitHub Actions workflows (release, beta, daily, docker)
- [x] Video preview on hover support
- [x] Security headers fully implemented
- [x] Age verification gate
- [x] Headless browser code removed (not needed with API integration)

## Recently Completed (December 2025)

- **API Engine Integration**: Converted PornHub, RedTube, and Eporner to use JSON APIs instead of HTML parsing
- **SSE Streaming**: Added `/api/v1/search/stream` endpoint for real-time result streaming
- **Engine Cleanup**: Removed 4 Cloudflare-blocked engines (xHamster, YouPorn, SpankBang, Beeg)
- **Browser Removal**: Removed headless browser code (not needed with API integration)
- **Search Button Fix**: Single-submission with spinner and "Searching..." message

## Build

```bash
# Build for current platform
CGO_ENABLED=0 go build -o /tmp/vidveil ./src

# Build all platforms (via Makefile)
make build

# Docker build
make docker
```

---

**END OF SPECIFICATION**

**Remember: ALL sections marked NON-NEGOTIABLE must be implemented exactly as specified.**

**When in doubt: Re-read the spec. Ask questions. Never assume.**
