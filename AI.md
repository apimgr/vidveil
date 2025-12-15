# Vidveil - AI Working Notes

**Project Name**: vidveil
**Project Org**: apimgr
**Version**: 0.2.0
**Last Updated**: December 9, 2025

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

### Sensitive Information Handling (NON-NEGOTIABLE)

**NEVER expose sensitive information unless absolutely necessary:**

- Tokens/passwords shown ONLY ONCE on generation (must be copied immediately)
- Show only on: first run, password changes, token regeneration
- Show in difficult environments: Docker, headless servers
- **NEVER log sensitive data**
- **NEVER in error messages or stack traces**
- Mask in UI: show `--------` or last 4 chars only

---

## Target Audience

- Self-hosted users
- SMB (Small/Medium Business)
- Enterprise
- **IMPORTANT: Assume self-hosted and SMB users are NOT tech-savvy**

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

Vidveil is a privacy-respecting meta search engine for adult video content. It aggregates results from 49 free adult video sites, providing a unified search interface with full Tor support for maximum privacy.

## Project-Specific Features

- **Meta Search**: Aggregates results from multiple adult video sites
- **Video-Only**: Focuses exclusively on video content
- **Tor Support**: Built-in SOCKS5 proxy support
- **Privacy First**: No tracking, no logging of searches
- **Age Verification**: Built-in age gate before content access
- **Video Preview**: Hover/swipe preview support

---

## Variables (NON-NEGOTIABLE)

| Variable | Description | Example |
|----------|-------------|---------|
| `{projectname}` | Project name | `vidveil` |
| `{projectorg}` | Organization name | `apimgr` |
| `{gitprovider}` | Git hosting provider | `github`, `gitlab`, `private` |
| **Rule** | Anything in `{}` is a variable | |
| **Rule** | Anything NOT in `{}` is literal | `/etc/letsencrypt/live` is a real path |

## Local Project Path Structure (NON-NEGOTIABLE)

**Format:** `~/Projects/{gitprovider}/{projectorg}/{projectname}`

| Component | Description | Examples |
|-----------|-------------|----------|
| `~/Projects/` | Base projects directory | Always `~/Projects/` |
| `{gitprovider}` | Git hosting provider or `local` | `github`, `gitlab`, `bitbucket`, `private`, `local` |
| `{projectorg}` | Organization/username | `apimgr`, `casjay`, `myorg` |
| `{projectname}` | Project name | `vidveil`, `jokes`, `myproject` |

---

## Directory Structure (NON-NEGOTIABLE)

**The root Project directory is**: `./`

```
./                          # Root project directory
├── src/                    # All source files
│   ├── main.go            # Entry point
│   ├── config/            # Configuration
│   ├── engines/           # Search engine implementations
│   ├── models/            # Data models
│   ├── services/          # Business logic
│   └── server/            # HTTP server
│       ├── handlers/      # Route handlers
│       ├── templates/     # HTML templates (.tmpl)
│       │   ├── partials/  # Reusable template partials
│       │   └── *.tmpl     # Page templates
│       └── static/        # CSS, JS, images
├── scripts/               # Install scripts
├── tests/                 # Test files
├── binaries/              # Built binaries (gitignored)
├── releases/              # Release binaries (gitignored)
├── .github/workflows/     # GitHub Actions
├── Makefile               # Build targets
├── Dockerfile             # Container build
├── README.md              # Documentation
├── LICENSE.md             # MIT license
├── AI.md                  # This file
├── TODO.AI.md             # Task tracking
└── release.txt            # Version tracking
```

**RULE: Keep the base directory organized and clean - no clutter!**

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
| Service | `/Library/LaunchDaemons/com.apimgr.vidveil.plist` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/bin/vidveil` or `/usr/local/bin/vidveil` |
| Config | `~/Library/Application Support/apimgr/vidveil/` |
| Config File | `~/Library/Application Support/apimgr/vidveil/server.yml` |
| Data | `~/Library/Application Support/apimgr/vidveil/` |
| Logs | `~/Library/Logs/apimgr/vidveil/` |
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
| Service | `/usr/local/etc/rc.d/vidveil` |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `~/.local/bin/vidveil` |
| Config | `~/.config/apimgr/vidveil/` |
| Config File | `~/.config/apimgr/vidveil/server.yml` |
| Data | `~/.local/share/apimgr/vidveil/` |
| Logs | `~/.local/share/apimgr/vidveil/logs/` |

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
| Service | Windows Service Manager |

### User (non-privileged)

| Type | Path |
|------|------|
| Binary | `%LocalAppData%\apimgr\vidveil\vidveil.exe` |
| Config | `%AppData%\apimgr\vidveil\` |
| Config File | `%AppData%\apimgr\vidveil\server.yml` |
| Data | `%LocalAppData%\apimgr\vidveil\` |
| Logs | `%LocalAppData%\apimgr\vidveil\logs\` |

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
| Internal Port | `80` |

---

# PART 4: PRIVILEGE ESCALATION & USER CREATION (NON-NEGOTIABLE)

## Overview

Application user creation **REQUIRES** privilege escalation. If the user cannot escalate privileges, the application runs as the current user with user-level directories.

## System User Requirements

| Requirement | Value |
|-------------|-------|
| Username | `vidveil` |
| Group | `vidveil` |
| UID/GID | Auto-detect unused in range 100-999 |
| Shell | `/sbin/nologin` or `/usr/sbin/nologin` |
| Home | Config or data directory |
| Type | System user (no password, no login) |
| Gecos | `vidveil service account` |

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

# PART 10: WEB FRONTEND (NON-NEGOTIABLE)

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

**JavaScript Guidelines:**
- **Last resort** - only when HTML5/CSS cannot achieve the functionality
- **Progressive enhancement** - features must work without JS where possible
- **No JS for styling** - never manipulate classes/styles for visual effects
- **No JS for simple interactions** - hover, focus, basic toggles are CSS-only
- **Required for**: API calls, dynamic content loading, complex state, WebSockets
- **Size matters** - keep JS minimal, no large libraries for simple tasks

### Go Templates (NON-NEGOTIABLE)

**ALL frontend HTML MUST use Go's `html/template` package.**

| Location | Purpose |
|----------|---------|
| `src/server/templates/` | All `.tmpl` template files |
| `src/server/templates/partials/` | Reusable template partials |
| `src/server/static/` | Static assets (CSS, JS, images) |

**Mandatory Partials (NON-NEGOTIABLE):**

ALL pages MUST use these partials to ensure consistent site-wide layout:

| Partial | Purpose | Required |
|---------|---------|----------|
| `head.tmpl` | `<head>` contents (meta, CSS) | YES |
| `header.tmpl` | Site header (logo, branding) | YES |
| `nav.tmpl` | Navigation menu | YES |
| `footer.tmpl` | Site footer (copyright, links) | YES |
| `scripts.tmpl` | JavaScript includes | YES |

**Rule:** Every page template MUST include header, nav, and footer partials. No page may define its own header/nav/footer - use the shared partials only.

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

**Modal Requirements:**
- Custom CSS-styled modals (no browser defaults)
- Backdrop overlay
- Close button (X) in corner
- Click outside to close (optional, configurable)
- Escape key to close
- Focus trap (tab stays within modal)
- Animated entrance/exit
- **Auto-close on action** - clicking any action button automatically closes the modal

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
| >= 720px | 90% (5% margins) |
| < 720px | 98% (1% margins) |
| Footer | Always centered, always at bottom |

## Themes

| Theme | Description |
|-------|-------------|
| **Dark** | Based on Dracula - **DEFAULT** |
| **Light** | Based on popular light theme |
| **Auto** | Based on user's system |

---

# PART 11: API STRUCTURE (NON-NEGOTIABLE)

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

# PART 12: ADMIN PANEL (NON-NEGOTIABLE)

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

# PART 13: CLI INTERFACE (NON-NEGOTIABLE)

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

---

# PART 14: UPDATE COMMAND (NON-NEGOTIABLE)

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

### Update Branches

| Branch | Release Type | Tag Pattern | Example |
|--------|--------------|-------------|---------|
| `stable` (default) | Release | `v*`, `*.*.*` | `v1.0.0` |
| `beta` | Pre-release | `*-beta` | `202512051430-beta` |
| `daily` | Pre-release | `YYYYMMDDHHMM` | `202512051430` |

---

# PART 15: DOCKER (NON-NEGOTIABLE)

## Dockerfile Requirements

| Requirement | Value |
|-------------|-------|
| Base | Alpine-based (latest) |
| Meta labels | All included |
| Scratch image | curl, bash, tini, binary in `/usr/local/bin` |
| Init system | **tini** |
| **ENV MODE** | **development** (allows localhost, .local, .test, etc.) |

## Docker Compose Requirements

| Requirement | Value |
|-------------|-------|
| Build definition | NEVER include |
| Version | NEVER include |
| Network | Custom `vidveil` network |
| Container name | `vidveil` |
| **environment: MODE** | **production** (strict host validation) |

## Container Configuration

| Setting | Value |
|---------|-------|
| Internal port | 80 |
| Data dir | `/data` |
| Config dir | `/config` |
| Log dir | `/data/logs/vidveil` |
| HEALTHCHECK | `vidveil --status` |

## Container Detection

**Assume running in container if tini init system (PID 1) is detected.**

## Tags

| Type | Tag |
|------|-----|
| Release | `ghcr.io/apimgr/vidveil:latest` |
| Development | `vidveil:dev` |

---

# PART 16: MAKEFILE (NON-NEGOTIABLE)

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
| Local/Testing | `/tmp/vidveil` |
| Host Build | `./binaries/vidveil` |
| Distribution | `vidveil-{os}-{arch}` |

**NEVER include `-musl` suffix.**

Example: `vidveil-linux-amd64` NOT `vidveil-linux-amd64-musl`

---

# PART 17: GITHUB ACTIONS (NON-NEGOTIABLE)

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

---

# PART 18: BINARY REQUIREMENTS (NON-NEGOTIABLE)

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

---

# PART 19: TESTING & DEVELOPMENT (NON-NEGOTIABLE)

## Temporary Directory Structure

**Format:** `/tmp/{tmpdir}/vidveil/`

| Purpose | Path |
|---------|------|
| Build output | `/tmp/apimgr-build/vidveil/` |
| Test config | `/tmp/apimgr-test/vidveil/` |
| Debug files | `/tmp/apimgr-debug/vidveil/` |

**NEVER use `/tmp/vidveil` directly - always use subdirectory structure.**

## Process Management (NON-NEGOTIABLE)

**All commands MUST be project-scoped. NEVER run global/broad commands.**

### FORBIDDEN Commands (NEVER Use)

| Command | Reason |
|---------|--------|
| `pkill -f {pattern}` | Too broad, kills unrelated processes |
| `docker rm $(docker ps -aq)` | Removes ALL containers |
| `docker rmi $(docker images -q)` | Removes ALL images |
| `docker system prune` | Cleans ALL unused resources |
| `killall {name}` | Too broad |

### Required: Project-Scoped Commands Only

| Command | Description |
|---------|-------------|
| `docker stop vidveil` | Stop specific container |
| `docker rm vidveil` | Remove specific container |
| `docker rmi apimgr/vidveil:tag` | Remove specific image |
| `kill {specific-pid}` | Kill exact PID only |
| `pkill -x vidveil` | Exact binary name match |

---

# PART 20: DATABASE & CLUSTER (NON-NEGOTIABLE)

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

# PART 21: SECURITY & LOGGING (NON-NEGOTIABLE)

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

## Logging

### Log Files

| Log | Purpose | Default Format |
|-----|---------|----------------|
| `access.log` | HTTP requests | `apache` |
| `server.log` | Application events | `text` |
| `error.log` | Error messages | `text` |
| `audit.log` | Security events | `json` |
| `security.log` | Security/auth events | `fail2ban` |
| `debug.log` | Debug (dev mode) | `text` |

### Log Output Rules (NON-NEGOTIABLE)

**All log FILES MUST use raw text only:**
- NO emojis
- NO ANSI color codes
- NO special characters or formatting
- Plain ASCII text only
- Machine-parseable format

**Console output (stdout/stderr) CAN be pretty:**
- Emojis allowed
- ANSI colors allowed
- Pretty formatting allowed

**Rule:** Log files = raw/plain text. Console = pretty is OK.

---

# PART 22: BACKUP & RESTORE (NON-NEGOTIABLE)

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

# PART 23: HEALTH & VERSIONING (NON-NEGOTIABLE)

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
  "version": "0.2.0",
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
vidveil v0.2.0
Built: 2024-01-15T10:30:00Z
Go: 1.23
OS/Arch: linux/amd64
```

---

# PART 24: ERROR HANDLING & CACHING (NON-NEGOTIABLE)

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

# PART 25: I18N & A11Y (NON-NEGOTIABLE)

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

# PART 26: PROJECT-SPECIFIC SECTIONS

## Vidveil-Specific API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/search` | GET | None | Search videos across all engines |
| `/api/v1/search.txt` | GET | None | Search results as plain text |
| `/api/v1/engines` | GET | None | List all search engines |
| `/api/v1/engines/{name}` | GET | None | Get engine details |
| `/api/v1/stats` | GET | None | Get search statistics |

## Vidveil-Specific Web Pages

| Page | Path | Description |
|------|------|-------------|
| Home | `/` | Search form |
| Search Results | `/search` | Video results grid |
| Preferences | `/preferences` | User settings |
| About | `/about` | About page |
| Privacy | `/privacy` | Privacy policy |
| Age Verify | `/age-verify` | Age verification gate |

## Search Engines (49 total)

**Tier 1 - Major Sites (7)**
- PornHub, xHamster, XVideos, XNXX, YouPorn, RedTube, SpankBang

**Tier 2 - Popular Sites (3)**
- Eporner, Beeg, PornMD

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

---

# PART 27: AI ASSISTANT RULES (NON-NEGOTIABLE)

## Attribution

| Rule | Description |
|------|-------------|
| **NEVER** | Include AI attribution in code, comments, commits, or documentation |
| **NEVER** | "authored by Claude", "generated by AI", "Co-Authored-By: Claude" |
| **ALWAYS** | Code should appear as if written by a human developer |

## Tool Access

| Access | Description |
|--------|-------------|
| Full | All tools |
| **EXCEPT** | `git push`, `git commit` - require explicit user request |
| Allowed | Stage files, create branches, check status, diff |
| Required | User must explicitly request commits and pushes |

---

# FINAL CHECKPOINT: COMPLIANCE CHECKLIST

**Before starting ANY work, verify you have read and understood:**

## Core Requirements

- [x] Re-read this spec periodically during work
- [x] AI.md must be kept in sync with project state
- [ ] TODO.AI.md required for more than 2 tasks
- [x] Never assume or guess - ask questions

## Development

- [x] All 4 OSes supported (Linux, macOS, BSD, Windows)
- [x] Both architectures supported (AMD64, ARM64)
- [x] CGO_ENABLED=0 for static binaries
- [x] Single static binary with embedded assets

## Configuration

- [x] Config file is `server.yml` (not .yaml)
- [x] Boolean handling accepts all truthy/falsy values
- [x] Sane defaults for everything

## Frontend

- [x] Frontend required for ALL projects
- [x] NO inline CSS
- [x] NO JavaScript alerts
- [x] Dark theme (Dracula) is default
- [x] Mobile-first responsive design
- [x] All 5 mandatory partials exist (head, header, nav, footer, scripts)

## API

- [x] All 3 API types: REST, Swagger, GraphQL
- [x] Standard endpoints exist (/healthz, /openapi, /openapi.json, /openapi.yaml, /graphql, /admin)
- [x] Versioned API: /api/v1

## Admin Panel

- [x] Full admin panel required
- [x] Web interface (/admin) with session auth
- [x] REST API (/api/v1/admin) with bearer token

## CLI

- [ ] All standard commands implemented
- [x] --help, --version work
- [ ] --update command with check/yes/branch subcommands

## Build & Deploy

- [x] 4 Makefile targets: build, release, docker, test
- [x] 4 GitHub workflows: release, beta, daily, docker
- [x] 8 platform builds (4 OS x 2 arch)
- [x] Docker uses tini, Alpine base

## Security

- [x] All security headers implemented
- [x] Sensitive data never exposed unless necessary
- [x] Rate limiting available

---

# CURRENT PROJECT STATE

## Completed

- Full project rename from XXXSearch to Vidveil
- 49 search engines implemented across 5 tiers
- REST API (`/api/v1/search`, `/api/v1/engines`, `/api/v1/healthz`)
- GraphQL API (`/graphql`)
- OpenAPI/Swagger documentation (`/openapi`, `/openapi.json`, `/openapi.yaml`)
- Web UI with search, about, privacy, and preferences pages
- Mobile-first responsive design
- Dark (Dracula) and light theme support
- All 5 mandatory template partials
- Footer uses CSS classes (not inline styles)
- Static binary with embedded assets
- Docker support (Alpine-based, tini init, port 80)
- Application mode support (production/development)
- Debug endpoints (/debug/pprof/*, /debug/vars) in development mode
- Mode shown in /api/v1/healthz response
- Mode shown in admin dashboard
- GitHub Actions workflows (release, beta, daily, docker)
- Templates renamed to .tmpl extension
- All inline styles removed
- Video preview on hover support
- CSS/JS for video preview hover functionality
- Security headers (PART 21) fully implemented
- Age verification gate

## Recently Completed (TEMPLATE.md Compliance Session)

- **PART 14 --update command**: Full implementation with `check`, `yes`, and `branch` subcommands
- **PART 13 URL/FQDN detection**: `IsValidHost()`, `IsValidSSLHost()`, `GetDisplayHost()` functions
- **PART 21 error.log**: Added ErrorLogConfig to LogsConfig with weekly rotation
- **PART 10 template layouts**: Embed pattern includes `templates/layouts/*.tmpl`

## Pending (per spec)

- Full admin panel (/admin) with all sections (PART 12)
- Let's Encrypt integration
- Built-in scheduler
- Cluster mode support
- Database migrations
- Comprehensive health check response

---

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

## Notes

- Directory is still named `xxxsearch` - may need manual rename to `vidveil`
- Build succeeds with `Vidveil v0.2.0` branding
- Footer uses CSS classes `.app-footer` and `.footer-timestamp`
- All inline styles removed from codebase
