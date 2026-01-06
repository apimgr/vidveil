# VidVeil TODO

## AI.md Refreshed from TEMPLATE.md (2026-01-06)

- [x] AI.md copied from ~/Projects/github/apimgr/TEMPLATE.md
- [x] Variables replaced: {projectname}=vidveil, {PROJECTNAME}=VIDVEIL, {projectorg}=apimgr, {gitprovider}=github
- [x] Project description updated (lines 7-24)
- [x] HOW TO USE section deleted
- [x] PART index line numbers updated
- [x] .claude/rules/ created with rule files
- [x] IDEA.md verified - contains complete project specification
- [x] PART 37 updated with VidVeil-specific content

**Specification Files:**
- `AI.md` - HOW: Implementation patterns (PARTS 0-36)
- `IDEA.md` - WHAT: Project idea, data models, endpoints, business logic
- `TODO.AI.md` - Task tracking

---

## Current Status

**AI.md is now the source of truth. PART 37 references IDEA.md for WHAT. PARTS 0-36 define HOW.**

### VidVeil-Specific Notes
- NO user accounts (stateless, privacy-first design)
- PARTS 33-35 (Multi-User, Organizations, Custom Domains) do NOT apply
- PART 36 (CLI Client) - VidVeil has a CLI client in src/client/
- Admin panel uses PART 17 for server-admin authentication only
- 54+ search engines with bang shortcuts

---

## PART Compliance Verification (2026-01-06)

All non-negotiable PARTs verified against AI.md specification.

### Completed Verification
- [x] PART 0: AI Assistant Rules - AI.md setup, HOW TO USE removed, variables replaced
- [x] PART 1: Critical Rules - Full web app architecture, security-first design
- [x] PART 2: License & Attribution - MIT license, LICENSE.md with third-party licenses
- [x] PART 3: Project Structure - All directories present, .gitignore, .dockerignore
- [x] PART 4: OS-Specific Paths - paths.go handles Linux/macOS/Windows/BSD
- [x] PART 5: Configuration - config.go with all required sections
- [x] PART 6: Application Modes - production/development modes in mode/
- [x] PART 7: Binary Requirements - CGO_ENABLED=0, 8 platforms in Makefile
- [x] PART 8: Server Binary CLI - All CLI flags in main.go, --help format
- [x] PART 9: Error Handling & Caching - Error patterns implemented
- [x] PART 10: Database & Cluster - SQLite, clustering support
- [x] PART 11: Security & Logging - Security headers, rate limiting, audit logs
- [x] PART 12: Server Configuration - Server settings structure
- [x] PART 13: Health & Versioning - /healthz, /api/v1/healthz endpoints
- [x] PART 14: API Structure - REST routes, content negotiation, .txt/.json extensions
- [x] PART 15: SSL/TLS & Let's Encrypt - SSL config, security headers
- [x] PART 16: Web Frontend - HTML templates, themes, CSS
- [x] PART 17: Admin Panel - Complete admin routes, setup wizard, auth
- [x] PART 18: Email & Notifications - SMTP configuration
- [x] PART 19: Scheduler - Built-in scheduler in scheduler/
- [x] PART 20: GeoIP - GeoIP service in geoip/
- [x] PART 21: Metrics - Prometheus metrics endpoint
- [x] PART 22: Backup & Restore - Backup service and API
- [x] PART 23: Update Command - --update CLI flag
- [x] PART 24: Privilege Escalation & Service - Service management
- [x] PART 25: Service Support - Systemd integration
- [x] PART 26: Makefile - build, host, release, docker, test, dev, clean targets
- [x] PART 27: Docker - Multi-stage Dockerfile, non-root user, tini
- [x] PART 28: CI/CD Workflows - release.yml, beta.yml, daily.yml, docker.yml
- [x] PART 29: Testing & Development - run_tests.sh, docker.sh, incus.sh
- [x] PART 30: ReadTheDocs Documentation - docs/ with MkDocs structure
- [x] PART 31: I18N & A11Y - i18n service in server/service/i18n/
- [x] PART 32: Tor Hidden Service - Tor service in server/service/tor/
- [x] PART 36: CLI Client - CLI with TUI in src/client/
- [x] PART 37: Project-Specific - Updated with VidVeil endpoints and business logic

### Not Applicable
- [ ] PART 33: Multi-User - VidVeil is stateless, no user accounts
- [ ] PART 34: Organizations - VidVeil is stateless, no organizations
- [ ] PART 35: Custom Domains - VidVeil is stateless, no custom domains

---

## Key Implementation Files

| Component | Location | Notes |
|-----------|----------|-------|
| CLI Entry | `src/main.go` | CLI flags, entry point |
| Paths | `src/paths/paths.go` | OS-specific paths |
| Config | `src/config/config.go` | Configuration management |
| Server | `src/server/server.go` | API routes, middleware |
| Engines | `src/server/service/parser/` | 54+ video search engines |
| CLI Client | `src/client/` | CLI client with bubbletea TUI |
| Docker | `docker/Dockerfile` | Container build |
| CI/CD | `.github/workflows/` | GitHub Actions pipelines |
| Docs | `docs/` | ReadTheDocs documentation |

---

## Implementation Queue

No pending tasks - all PARTs verified complete.
