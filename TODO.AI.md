# VidVeil TODO and Compliance Status

## Status Summary

**Last Updated:** 2026-01-13
**AI.md Version:** Fresh from TEMPLATE.md (47,665 lines)
**Template Variables:** All replaced (vidveil, apimgr, github)

## Session Changes (2026-01-13)

### Completed This Session

#### 1. AI.md Refresh
- [x] Copied fresh TEMPLATE.md to AI.md (47,665 lines)
- [x] Read PART 0 completely (AI Assistant Rules, lines 1010-2567)
- [x] Replaced all template variables:
  - `{projectname}` -> `vidveil`
  - `{PROJECTNAME}` -> `VIDVEIL`
  - `{projectorg}` -> `apimgr`
  - `{PROJECTORG}` -> `APIMGR`
  - `{gitprovider}` -> `github`

### Previous Session Work (Uncommitted)

#### Search Endpoint Merge (Content Negotiation)
- [x] Merged `/api/v1/search` and `/api/v1/search/stream` into single endpoint
- [x] Added content negotiation via Accept headers:
  - `application/json` -> JSON response
  - `text/event-stream` -> SSE streaming
  - `text/plain` -> Plain text
- [x] Updated `detectResponseFormat()` with SSE detection
- [x] Created `handleSearchSSE()` helper function
- [x] Removed `APISearchStream` and `APISearchText` functions
- [x] Updated routes in `server.go`

**Files Changed:**
| File | Change |
|------|--------|
| `src/server/handler/handlers.go` | Content negotiation, SSE helper |
| `src/server/server.go` | Removed /search/stream route |
| `src/server/handler/handlers_test.go` | Updated tests |
| `IDEA.md` | Updated search endpoint docs |

#### PART 25 Service Compliance
- [x] Updated systemd template (no User/Group - binary drops privileges)
- [x] Updated runit template (simplified, no chpst)
- [x] Updated launchd template (apimgr.vidveil.plist path)
- [x] Updated rc.d template (simplified)
- [x] Added `ReadWritePaths` for all 4 required directories
- [x] Fixed directory creation in `createLinuxUser()`

**Files Changed:**
| File | Change |
|------|--------|
| `src/server/service/system/service.go` | All service templates updated |

#### Privilege Dropping Implementation
- [x] Created `privilege_unix.go` - Unix privilege dropping via syscall
- [x] Created `privilege_windows.go` - No-op (VSA handles isolation)
- [x] Created `windows_service.go` - Windows svc integration
- [x] Created `windows_service_other.go` - Stubs for non-Windows

**New Files:**
| File | Purpose |
|------|---------|
| `src/server/service/system/privilege_unix.go` | Setuid/Setgid after port binding |
| `src/server/service/system/privilege_windows.go` | No-op for Windows |
| `src/server/service/system/windows_service.go` | golang.org/x/sys/windows/svc |
| `src/server/service/system/windows_service_other.go` | Build stubs |

#### Integration Tests
- [x] Fixed systemd namespace failure (ReadWritePaths requires dirs to exist)
- [x] All 13 integration tests passing

#### Port Configuration Fix
- [x] Changed default port from `"80"` to random `64xxx`
- [x] Updated `tests/incus.sh` with dynamic port detection

## Current State

| Component | Status |
|-----------|--------|
| Build (`make host`) | Working |
| Unit tests (`make test`) | Working |
| Integration tests (`./tests/incus.sh`) | 13/13 Passing |
| Systemd service | Working (privilege drop, port 64xxx) |
| Search endpoint | Merged with content negotiation |

## Pending Tasks

- [ ] All changes are uncommitted (per SPEC: AI cannot commit)

## Key PART 0 Rules

1. **AI.md is source of truth** - ALWAYS read relevant PART before implementing
2. **Re-read before every task** - Combat spec drift
3. **No report files** - Fix issues directly (no AUDIT.md, COMPLIANCE.md, etc.)
4. **IDEA.md = WHAT** - Business logic, data models, features
5. **AI.md (PARTS 0-37) = HOW** - Implementation patterns, standards
6. **Cannot commit** - Write to `.git/COMMIT_MESS` instead
7. **Ask when uncertain** - Asking is 50x cheaper than guessing wrong

---

**Remember:** AI.md (HOW) + IDEA.md (WHAT) = Complete specification
