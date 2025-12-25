# Vidveil TODO - AI.md Compliance Implementation

## Overview
This file tracks the implementation of all AI.md (formerly TEMPLATE.md) requirements for the vidveil project.

**AI.md Created**: 2025-12-25
**Total PARTs**: 35 (32 mandatory + 3 optional)
**Project**: vidveil (Privacy-respecting meta search engine for adult video content)

---

## Status Legend
- ✅ **Complete**: Fully implemented and verified
- 🔄 **In Progress**: Currently being worked on
- ⚠️ **Partial**: Partially implemented, needs completion
- ❌ **Missing**: Not implemented, needs work
- ⏭️ **Skip**: Not applicable for this project

---

## PART 0: AI Assistant Rules (NON-NEGOTIABLE)
- [x] AI.md created from TEMPLATE.md
- [x] Variables replaced ({projectname} → vidveil, etc.)
- [x] PART 32 customized with vidveil-specific details
- [x] TODO.AI.md tracking implementation
- [ ] .git/COMMIT_MESS workflow implemented

**Status**: ✅ Mostly Complete (commit message workflow pending)

---

## PART 1: Critical Rules (NON-NEGOTIABLE)

### File & Directory Naming
- [ ] Audit all files for lowercase naming
- [ ] Audit all files for snake_case multi-word naming
- [ ] Verify singular directory names (handler/ not handlers/)
- [ ] Ensure package names match directory names

### Build Rules
- [ ] Verify CGO_ENABLED=0 in all builds
- [ ] Verify single static binary output
- [ ] Verify 8 platform builds (linux/darwin/windows/freebsd × amd64/arm64)
- [ ] Verify binary naming: vidveil-{os}-{arch}[.exe]

### Docker-Only Development
- [ ] Verify no direct `go` commands (all use Docker)
- [ ] Update Makefile to use Docker containers
- [ ] Verify `make dev` uses temp dir
- [ ] Verify `make build` uses Docker

### Security
- [ ] Input validation on all endpoints
- [ ] Parameterized SQL queries (no string concat)
- [ ] XSS prevention (HTML escaping in templates)
- [ ] CSRF protection on all forms
- [ ] Rate limiting implemented
- [ ] Security headers configured

**Status**: ⚠️ Partial (needs audit and verification)

---

## PART 2: Project Structure (NON-NEGOTIABLE)

### Directory Structure
- [x] src/ directory exists
- [x] docker/ directory exists
- [ ] docs/ directory for MkDocs
- [ ] binaries/ in .gitignore
- [ ] releases/ in .gitignore
- [x] AI.md in root
- [x] README.md in root
- [ ] LICENSE.md in root (verify MIT)
- [x] Makefile in root
- [x] go.mod/go.sum in root
- [ ] release.txt in root (version source of truth)

### Source Structure
- [x] src/main.go exists
- [x] src/config/ package exists
- [ ] src/config/bool.go exists (ParseBool function)
- [x] src/mode/ package exists
- [ ] src/paths/ package exists
- [ ] src/ssl/ package exists
- [ ] src/scheduler/ package exists
- [x] src/server/ package exists
- [ ] src/service/ package exists (service manager)
- [x] src/client/ package exists (CLI client)

**Status**: ⚠️ Partial (missing several packages)

---

## PART 3: OS-Specific Paths (NON-NEGOTIABLE)
- [ ] Implement path resolution for Linux (root/user)
- [ ] Implement path resolution for macOS
- [ ] Implement path resolution for Windows
- [ ] Implement path resolution for BSD
- [ ] Docker paths: /config, /data, /data/logs
- [ ] Create src/paths/paths.go package

**Status**: ❌ Missing (needs implementation)

---

## PART 4: Configuration (NON-NEGOTIABLE)

### Configuration File
- [ ] server.yml format (NOT .yaml)
- [ ] Single instance mode (yml is source of truth)
- [ ] Cluster mode support (database is source of truth)
- [ ] Maintenance mode (self-healing)

### Boolean Handling
- [ ] src/config/bool.go with ParseBool() function
- [ ] Support 50+ truthy/falsy values
- [ ] NEVER use strconv.ParseBool()

### Database Schema
- [ ] Server config table: srv_config
- [ ] Admin sessions: srv_admin_sessions
- [ ] Audit log: srv_audit_log
- [ ] Users database: users.db
- [ ] Admin table: usr_admins
- [ ] User table: usr_users
- [ ] API keys: usr_api_keys

**Status**: ⚠️ Partial (config exists, boolean handling missing)

---

## PART 5: Application Modes (NON-NEGOTIABLE)
- [x] src/mode/mode.go exists
- [ ] Production mode (strict validation)
- [ ] Development mode (relaxed validation)
- [ ] Debug mode (DEBUG=true, /debug/pprof)
- [ ] Mode detection from ENV/flag

**Status**: ⚠️ Partial (mode.go exists, needs verification)

---

## PART 6: Server Binary CLI (NON-NEGOTIABLE)

### Required Flags (CANNOT CHANGE)
- [ ] --help (-h)
- [ ] --version (-v)
- [ ] --mode {production|development}
- [ ] --config {configdir}
- [ ] --data {datadir}
- [ ] --log {logdir}
- [ ] --pid {pidfile}
- [ ] --address {listen}
- [ ] --port {port}
- [ ] --debug
- [ ] --status
- [ ] --daemon (Unix only)
- [ ] --service {start,restart,stop,reload,--install,--uninstall}
- [ ] --maintenance {backup,restore,update,mode,setup}
- [ ] --update [check|yes|branch {stable|beta|daily}]

### Features
- [ ] PID file with stale detection
- [ ] Daemonization (Unix only)
- [ ] Service manager auto-detection
- [ ] Signal handling (SIGTERM, SIGINT, SIGQUIT, etc.)
- [ ] Graceful shutdown (30s timeout)

**Status**: ❌ Missing (needs full CLI implementation)

---

## PART 7: Update Command (NON-NEGOTIABLE)
- [ ] --update check (check for updates)
- [ ] --update yes (auto-update)
- [ ] --update branch {stable|beta|daily}
- [ ] GitHub API integration
- [ ] Binary replacement logic
- [ ] Version comparison

**Status**: ❌ Missing (needs implementation)

---

## PART 8: Privilege Escalation & Service (NON-NEGOTIABLE)
- [ ] Service installation requires privilege escalation
- [ ] Auto-detect sudo/doas/runas
- [ ] Re-exec with elevated privileges
- [ ] Service manager detection

**Status**: ❌ Missing (needs implementation)

---

## PART 9: Service Support (NON-NEGOTIABLE)
- [ ] systemd (Linux)
- [ ] runit (Linux)
- [ ] launchd (macOS)
- [ ] Windows Service Manager
- [ ] BSD rc.d
- [ ] Auto-detection of service manager

**Status**: ❌ Missing (needs implementation)

---

## PART 10: Binary Requirements (NON-NEGOTIABLE)
- [ ] CGO_ENABLED=0
- [ ] Single static binary
- [ ] All assets embedded (Go embed)
- [ ] 8 platforms: linux/darwin/windows/freebsd × amd64/arm64
- [ ] Binary naming: vidveil-{os}-{arch}[.exe]

**Status**: ⚠️ Partial (needs verification)

---

## PART 11: Makefile (NON-NEGOTIABLE)

### Required Targets
- [ ] make dev (quick build to temp dir)
- [ ] make build (full 8-platform build)
- [ ] make test (run tests in Docker)
- [ ] make docker (build Docker image)
- [ ] make clean (clean artifacts)

### Build Configuration
- [ ] CGO_ENABLED=0 always
- [ ] LDFLAGS with version/commit/date
- [ ] Random temp dir (mktemp)
- [ ] Docker-based builds

**Status**: ⚠️ Partial (Makefile exists, needs audit)

---

## PART 12: Testing & Development (NON-NEGOTIABLE)
- [ ] Docker-based test execution
- [ ] make test target
- [ ] make dev target (temp dir)
- [ ] Test coverage reports
- [ ] Integration tests
- [ ] NO Go on host (all via Docker)

**Status**: ⚠️ Partial (tests exist, Docker workflow needs verification)

---

## PART 13: Docker (NON-NEGOTIABLE)

### Docker Structure
- [ ] docker/Dockerfile (multi-stage)
- [ ] docker/docker-compose.yml (production)
- [ ] docker/docker-compose.dev.yml (development)
- [ ] docker/docker-compose.test.yml (testing)
- [ ] docker/rootfs/usr/local/bin/entrypoint.sh

### Dockerfile Requirements
- [ ] Multi-stage: golang:alpine → alpine:latest
- [ ] Internal port: 80
- [ ] External port: Random 64xxx
- [ ] STOPSIGNAL: SIGRTMIN+3
- [ ] ENTRYPOINT: tini
- [ ] Required packages: curl, bash, tini, tor
- [ ] Tor auto-enabled
- [ ] ENV MODE=development

### Runtime Volumes
- [ ] ./rootfs/config:/config:z
- [ ] ./rootfs/data:/data:z

**Status**: ⚠️ Partial (Docker exists, needs compliance audit)

---

## PART 14: CI/CD Workflows (NON-NEGOTIABLE)

### Required Workflows
- [ ] .github/workflows/release.yml (stable releases)
- [ ] .github/workflows/beta.yml (beta releases)
- [ ] .github/workflows/daily.yml (nightly builds)
- [ ] .github/workflows/docker.yml (Docker images on every push)

### Docker Tags
- [ ] Any push: devel, {commit}
- [ ] Beta: adds beta
- [ ] Tag: {version}, latest, YYMM, {commit}

### Multi-Arch
- [ ] linux/amd64
- [ ] linux/arm64

**Status**: ⚠️ Partial (.github/workflows exist, needs audit)

---

## PART 15: Health & Versioning (NON-NEGOTIABLE)
- [ ] /healthz endpoint (HTML + JSON)
- [ ] /api/v1/healthz endpoint (JSON only)
- [ ] Health response format per spec
- [ ] Version info embedded (via LDFLAGS)
- [ ] Commit ID embedded
- [ ] Build date embedded

**Status**: ⚠️ Partial (endpoints exist, format needs verification)

---

## PART 16: Web Frontend (NON-NEGOTIABLE)

### Technology Requirements
- [ ] Go templates (html/template) ONLY
- [ ] Vanilla JavaScript ONLY (NO frameworks)
- [ ] CSS-first design
- [ ] NO React/Vue/Alpine/jQuery
- [ ] ONE JavaScript file: static/js/app.js
- [ ] NO inline CSS/JS (except simple onclick)
- [ ] NO JavaScript alerts (custom modals)

### Responsive Design
- [ ] Mobile-first design
- [ ] <720px: 98% width
- [ ] ≥720px: 90% width
- [ ] 44x44px touch targets

### CSS Structure
- [ ] static/css/common.css (reset, variables, utilities)
- [ ] static/css/public.css (public layout)
- [ ] static/css/admin.css (admin layout)
- [ ] static/css/components.css (shared components)

### Template Structure
- [ ] templates/layouts/public.tmpl
- [ ] templates/layouts/admin.tmpl
- [ ] templates/partials/head.tmpl
- [ ] templates/partials/scripts.tmpl
- [ ] templates/partials/public/ (header, nav, footer)
- [ ] templates/partials/admin/ (header, sidebar, footer)
- [ ] templates/pages/ (page content)

### Accessibility
- [ ] WCAG 2.1 AA compliant
- [ ] Semantic HTML5
- [ ] ARIA labels
- [ ] Keyboard navigation

### PWA Support
- [ ] manifest.json
- [ ] Service worker
- [ ] Offline support

**Status**: ⚠️ Partial (templates exist, needs compliance audit)

---

## PART 17: Server Configuration (NON-NEGOTIABLE)
- [ ] server.yml configuration file
- [ ] All standard configuration keys
- [ ] Configuration validation
- [ ] Live reload (except port/address)

**Status**: ⚠️ Partial (config exists, needs verification)

---

## PART 18: Admin Panel (NON-NEGOTIABLE)

### Admin Panel Isolation
- [ ] NO links to /admin from public pages
- [ ] Separate authentication from users
- [ ] Stored in users.db (admins table)

### Layout
- [ ] Header: Logo, Search, Status, Admin, Logout
- [ ] Sidebar with sections (icons)
- [ ] Main content area
- [ ] Footer: Version, Docs, Status

### Sidebar Sections
- [ ] Dashboard
- [ ] Server (Settings, Branding, SSL, Scheduler, Email, Logs, Backup, Updates)
- [ ] Security (Auth, Tokens, Rate Limiting, Firewall)
- [ ] Network (Tor, GeoIP, Blocklists)
- [ ] Users (if multi-user)
- [ ] Cluster (if enabled)
- [ ] Help

### Form Controls
- [ ] Toggle switches
- [ ] Checkboxes
- [ ] Dropdowns
- [ ] Text/Textarea
- [ ] Number inputs
- [ ] Password fields (with show/hide)
- [ ] Tags input
- [ ] Duration input (number + unit)

### Critical Requirements
- [ ] ALL server.yml settings configurable via UI
- [ ] Live reload for config changes
- [ ] No SSH/CLI required

**Status**: ⚠️ Partial (admin exists, needs full sidebar + all settings)

---

## PART 19: API Structure (NON-NEGOTIABLE)

### API Types (ALL REQUIRED)
- [ ] REST API (primary)
- [ ] Swagger/OpenAPI (auto-generated)
- [ ] GraphQL (auto-generated)

### Content Negotiation
- [ ] Accept header detection
- [ ] .txt extension support (ALL endpoints)
- [ ] HTML for browsers
- [ ] JSON for API clients

### Required Endpoints
- [x] / (web interface)
- [ ] /healthz (HTML/JSON)
- [ ] /openapi (Swagger UI)
- [ ] /openapi.json (OpenAPI spec - JSON ONLY)
- [ ] /graphql (GraphiQL + POST)
- [ ] /metrics (Prometheus)
- [x] /admin (admin panel)
- [ ] /api/v1/healthz
- [ ] /api/v1/admin/*

### Response Formats
- [ ] Single item: Return directly (no wrapper)
- [ ] Action: {success, message, id}
- [ ] Error: {error, code, status}
- [ ] Pagination: Default 250 items

### Swagger & GraphQL
- [ ] Auto-generated from code
- [ ] Synced with each other
- [ ] Dracula theme
- [ ] JSON only for OpenAPI (NO YAML)

**Status**: ⚠️ Partial (some endpoints exist, needs full compliance)

---

## PART 20: SSL/TLS & Let's Encrypt (NON-NEGOTIABLE)
- [ ] SSL/TLS support
- [ ] Let's Encrypt integration
- [ ] DNS-01 challenge
- [ ] TLS-ALPN-01 challenge
- [ ] HTTP-01 challenge
- [ ] Auto-renewal
- [ ] Certificate management via admin UI

**Status**: ⚠️ Partial (SSL exists, Let's Encrypt needs verification)

---

## PART 21: Security & Logging (NON-NEGOTIABLE)

### Security
- [ ] Input validation
- [ ] SQL injection prevention
- [ ] XSS prevention
- [ ] CSRF protection
- [ ] Command injection prevention
- [ ] Path traversal prevention
- [ ] Rate limiting
- [ ] Security headers

### Logging
- [ ] Access logs
- [ ] Error logs
- [ ] Audit logs
- [ ] Structured JSON logging
- [ ] Log rotation
- [ ] Log viewer in admin panel

**Status**: ⚠️ Partial (logging exists, security needs full audit)

---

## PART 22: User Management (NON-NEGOTIABLE)
- [ ] Server admins (separate from users)
- [ ] Primary admin (first setup)
- [ ] Additional admins
- [ ] OIDC/LDAP admin support (optional)
- [ ] Regular users (if multi-user)
- [ ] User registration
- [ ] User authentication
- [ ] Session management
- [ ] API keys/tokens
- [ ] Password reset
- [ ] Email verification

**Status**: ⚠️ Partial (admin auth exists, full user management needs verification)

---

## PART 23: Database & Cluster (NON-NEGOTIABLE)

### Database
- [ ] SQLite default (modernc.org/sqlite)
- [ ] server.db (server config)
- [ ] users.db (users/admins)
- [ ] Argon2id password hashing (NOT bcrypt)
- [ ] Database migrations
- [ ] PostgreSQL support (optional)

### Cluster Support
- [ ] Valkey/Redis support
- [ ] Config sync across nodes
- [ ] Leader election
- [ ] Cluster health monitoring

**Status**: ⚠️ Partial (database exists, cluster needs implementation)

---

## PART 24: Backup & Restore (NON-NEGOTIABLE)
- [ ] --maintenance backup
- [ ] --maintenance restore
- [ ] Backup via admin UI
- [ ] Restore via admin UI
- [ ] Backup config + data + database
- [ ] Automated backups (scheduler)

**Status**: ❌ Missing (needs implementation)

---

## PART 25: Email & Notifications (NON-NEGOTIABLE)
- [ ] SMTP configuration
- [ ] Email templates
- [ ] Password reset emails
- [ ] Admin notifications
- [ ] Email test endpoint
- [ ] Bell icon notifications in UI

**Status**: ⚠️ Partial (email service exists, needs verification)

---

## PART 26: Scheduler (NON-NEGOTIABLE)
- [ ] Background task scheduler
- [ ] Periodic tasks
- [ ] Cron-like scheduling
- [ ] Task management via admin UI
- [ ] GeoIP updates
- [ ] Blocklist updates
- [ ] Automated backups

**Status**: ⚠️ Partial (scheduler exists, needs full implementation)

---

## PART 27: GeoIP (NON-NEGOTIABLE)
- [ ] GeoIP database support
- [ ] Download from ip-location-db
- [ ] Scheduler updates (NOT embedded)
- [ ] Country detection
- [ ] City detection
- [ ] ASN detection

**Status**: ❌ Missing (needs implementation)

---

## PART 28: Metrics (NON-NEGOTIABLE)
- [ ] /metrics endpoint
- [ ] Prometheus format
- [ ] Request counters
- [ ] Response time histograms
- [ ] Error counters
- [ ] Custom metrics

**Status**: ⚠️ Partial (metrics endpoint exists, needs verification)

---

## PART 29: Tor Hidden Service (NON-NEGOTIABLE)
- [ ] Tor support
- [ ] Auto-enabled if tor binary installed
- [ ] Hidden service configuration
- [ ] Onion address generation
- [ ] Tor status in admin panel

**Status**: ⚠️ Partial (Tor support exists, needs verification)

---

## PART 30: Error Handling & Caching (NON-NEGOTIABLE)

### Error Handling
- [ ] Consistent error responses
- [ ] User-friendly errors (minimal)
- [ ] Admin errors (actionable)
- [ ] Console errors (full details)
- [ ] Log errors (structured JSON)

### Caching
- [ ] In-memory caching
- [ ] Redis/Valkey caching
- [ ] Cache invalidation
- [ ] Cache configuration

**Status**: ⚠️ Partial (error handling exists, caching needs verification)

---

## PART 31: I18N & A11Y (NON-NEGOTIABLE)

### Internationalization
- [ ] i18n support
- [ ] Translation files
- [ ] Language detection
- [ ] Language selector

### Accessibility
- [ ] WCAG 2.1 AA compliance
- [ ] Semantic HTML
- [ ] ARIA labels
- [ ] Keyboard navigation
- [ ] Screen reader support

**Status**: ⚠️ Partial (i18n exists, full accessibility needs verification)

---

## PART 32: Project-Specific (CUSTOMIZED FOR VIDVEIL)
- [x] Project description added to AI.md
- [x] Purpose documented in AI.md
- [x] Key features listed in AI.md
- [x] API endpoints documented
- [x] Data files documented
- [x] Architecture notes added

**Status**: ✅ Complete (AI.md customized)

---

## PART 33: CLI Client (OPTIONAL - INCLUDED FOR VIDVEIL)
- [x] src/client/ directory exists
- [ ] vidveil-cli binary
- [ ] Search command
- [ ] Config command
- [ ] TUI mode
- [ ] API client
- [ ] Separate builds for CLI

**Status**: ⚠️ Partial (client code exists, needs verification)

---

## PART 34: Custom Domains (OPTIONAL - NOT APPLICABLE)
⏭️ **Skipped**: Vidveil is a meta search engine, not a multi-tenant SaaS requiring custom domains.

**Status**: ⏭️ Skip

---

## PART 35: ReadTheDocs Documentation (NON-NEGOTIABLE)
- [ ] docs/ directory
- [ ] mkdocs.yml configuration
- [ ] .readthedocs.yaml
- [ ] Getting Started guide
- [ ] API documentation
- [ ] Admin guide
- [ ] Architecture documentation
- [ ] Deployment guide

**Status**: ⚠️ Partial (docs exist, needs full MkDocs setup)

---

## FINAL CHECKPOINT: Compliance Verification

### File Structure
- [x] AI.md created and customized
- [x] TODO.AI.md tracking implementation
- [ ] All 35 PARTs reviewed
- [ ] Applicable sections implemented
- [ ] Non-applicable sections documented

### Build & Deployment
- [ ] Docker multi-stage build working
- [ ] 8 platform builds (4 OS × 2 arch)
- [ ] CGO_ENABLED=0 verified
- [ ] Makefile targets working
- [ ] CI/CD workflows configured

### Core Features
- [ ] Admin panel complete
- [ ] Frontend responsive and accessible
- [ ] All CLI commands working
- [ ] API endpoints implemented
- [ ] Health checks working

### Security & Operations
- [ ] Input validation complete
- [ ] Security headers configured
- [ ] Rate limiting implemented
- [ ] Logging complete
- [ ] Backup/restore working

### Documentation
- [ ] README.md complete
- [ ] ReadTheDocs setup
- [ ] API documentation
- [ ] Admin guide

---

## Implementation Priority

### Phase 1: Critical Infrastructure (IMMEDIATE)
1. src/config/bool.go (ParseBool)
2. src/paths/paths.go (OS-specific paths)
3. CLI flags implementation (PART 6)
4. Makefile Docker-based builds
5. Docker compliance (multi-stage, entrypoint.sh)

### Phase 2: Core Features (HIGH)
1. Admin panel full sidebar
2. All server.yml settings in admin UI
3. Health endpoints (/healthz, /api/v1/healthz)
4. OpenAPI/Swagger auto-generation
5. GraphQL auto-generation
6. Metrics endpoint

### Phase 3: Security & Operations (HIGH)
1. CSRF protection
2. Rate limiting
3. Security headers
4. Audit logging
5. Backup/restore
6. Email notifications

### Phase 4: Advanced Features (MEDIUM)
1. Service manager support
2. Update command
3. GeoIP integration
4. Scheduler tasks
5. Cluster mode (Valkey/Redis)

### Phase 5: Documentation (MEDIUM)
1. ReadTheDocs setup
2. API documentation
3. Admin guide
4. Deployment guide

### Phase 6: Polish & Verification (LOW)
1. Accessibility audit (WCAG 2.1 AA)
2. I18N completion
3. PWA enhancements
4. Performance optimization
5. Final compliance verification

---

## Next Steps

1. ✅ AI.md created from TEMPLATE.md
2. ✅ TODO.AI.md comprehensive plan created
3. **NEXT**: Audit existing codebase against AI.md
4. **THEN**: Implement Phase 1 (Critical Infrastructure)
5. **THEN**: Implement Phase 2 (Core Features)
6. Continue through all phases

---

**Last Updated**: 2025-12-25
**AI.md Version**: 1.0 (from TEMPLATE.md 2025-12-25)
