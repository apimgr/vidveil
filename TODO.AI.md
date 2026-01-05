# VidVeil TODO

## AI.md Refreshed from TEMPLATE.md (2026-01-03)

- [x] AI.md replaced from ~/Projects/github/apimgr/TEMPLATE.md
- [x] Variables replaced: {projectname}=vidveil, {projectorg}=apimgr, {gitprovider}=github
- [x] Project description updated (lines 1-25)
- [x] PART 37 references IDEA.md
- [x] HOW TO USE section deleted (lines 787-1085)
- [x] PART Index line numbers updated
- [x] IDEA.md verified - contains complete project specification

**Specification Files:**
- `AI.md` - HOW: Implementation patterns (PARTS 0-36)
- `IDEA.md` - WHAT: Project idea, data models, endpoints, business logic
- `TODO.AI.md` - Task tracking

---

## Previous Implementation Status (Verified Complete)

All 34 PARTs verified compliant prior to AI.md refresh:
- PART 0-32: Core spec implementation complete
- PART 36: CLI Client & TUI implemented (src/client/)
- PART 37: Project-specific business logic (54+ search engines)
- PARTS 33-35: N/A (VidVeil has no user accounts)

### Key Implementation Files
- `src/main.go` - CLI flags and entry point
- `src/paths/paths.go` - OS-specific paths
- `src/config/config.go` - Configuration management
- `src/server/server.go` - API routes
- `src/server/service/engine/` - 54+ video search engines
- `src/client/` - CLI client with bubbletea TUI
- `docker/Dockerfile` - Container build
- `.github/workflows/` - CI/CD pipelines
- `docs/` - ReadTheDocs documentation

### Test Results
- All tests pass (make test)
- Coverage: config 38.5%, handler 7.4%, engine 8.2%, i18n 68.9%, ratelimit 78.9%, retry 90.1%, validation 94.4%

---

## IDEA.md Spec Compliance (2026-01-04)

- [x] Added Admin Routes section referencing PART 17 hierarchy
- [x] Added Health & System Endpoints section
- [x] Verified all routes follow PART 14 patterns (plural nouns, versioned API)
- [x] Verified admin routes under `/{admin_path}/server/*` per PART 17
- [x] Notes section already references PART 17 for admin panel

## AI.md PART 37 Updated (2026-01-04)

- [x] Replaced template placeholders with VidVeil-specific content
- [x] All sections now reference IDEA.md for full details
- [x] Project Business Purpose → references IDEA.md
- [x] Business Logic & Rules → references IDEA.md
- [x] Data Models → references IDEA.md
- [x] Data Sources → references IDEA.md
- [x] Endpoints Summary → VidVeil-specific with IDEA.md reference
- [x] Extended Node Functions → N/A (stateless)
- [x] High Availability → N/A (stateless)
- [x] Notes → references IDEA.md

**AI.md is source of truth. PART 37 references IDEA.md for WHAT. PARTS 0-36 define HOW.**

## IDEA.md Full Compliance Verified (2026-01-04)

Per PARTS 0, 1, 2 rules - IDEA.md is WHAT, must comply with AI.md:
- [x] IDEA.md describes WHAT (business logic, features, data models)
- [x] IDEA.md uses spec-compliant terminology from AI.md
- [x] Routes follow PART 14: versioned `/api/v1/*`, plural nouns, lowercase
- [x] Admin routes reference PART 17: `/{admin_path}/server/*` hierarchy
- [x] Health endpoints: `/healthz` + `/api/v1/healthz`
- [x] Server scope: `/server/about`, `/server/privacy`
- [x] References PARTs for implementation (14, 16, 17)
- [x] No legacy endpoints
- [x] Data models properly defined
- [x] Business rules clear and specific

## Current Status

Project is implementation-complete. AI.md and IDEA.md are spec-compliant.

No pending tasks.
