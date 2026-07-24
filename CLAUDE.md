# Project SPEC

Project: VIDVEIL
Role: Efficient loader for AI.md

⚠️ **THIS FILE IS AUTO-LOADED EVERY CONVERSATION. FOLLOW IT EXACTLY.** ⚠️

Purpose:
- This file is a short loader for the most important rules
- `AI.md` is the full source of truth (~43k lines)
- For complete details, read the referenced PARTs in `AI.md`

## Asking Questions

- **Default to continuing work** - do not stop just to ask whether you should continue
- **Never guess** - if the answer cannot be determined from `AI.md`, `IDEA.md`, the codebase, or repo state and the missing information materially changes behavior, scope, or safety, ASK the user
- **Do NOT ask for permission to keep going** - continue until the current task is complete
- **Question mark = question** - when user ends with `?`, answer/clarify, don't execute

**Ask only when at least one of these is true:**
1. A required business/product decision is missing
2. Two or more reasonable implementations would produce materially different behavior
3. The action is destructive, irreversible, or impacts production/user data
4. The spec explicitly says to ask or confirm

## Before ANY Code Change

1. Have I read the relevant PART in AI.md? (If no → read it)
2. Does this follow the spec EXACTLY? (If unsure → check spec)
3. Am I guessing or do I KNOW from the spec? (If guessing → read spec)
4. Would this pass the compliance checklist? (AI.md FINAL section)

**WHEN IN DOUBT: READ THE SPEC. DO NOT GUESS.**

## Binary Terminology
- **server** = `vidveil` (main binary, runs as service)
- **client** = `vidveil-cli` (REQUIRED companion, CLI/TUI/GUI)

## Key Placeholders
- `{project_name}` = vidveil
- `{project_org}` = apimgr

## NEVER Do (Top 19) - VIOLATIONS ARE BUGS
1. Use bcrypt for config/backup passwords → Use Argon2id
2. Put Dockerfile in root → `docker/Dockerfile`
3. Use CGO → CGO_ENABLED=0 always
4. Hardcode dev values → Detect at runtime
5. Use external cron → Internal scheduler (PART 18)
6. Store config/backup passwords plaintext → Argon2id (API tokens use SHA-256)
7. Create premium tiers → All features free, no paywalls
8. Use Makefile in CI/CD → Explicit commands only
9. Guess or assume values that a command can produce
10. Skip platforms → Build all 8 (linux/darwin/windows/freebsd × amd64/arm64)
11. Client-side rendering (React/Vue) → Server-side Go templates
12. Require JavaScript for core features → Progressive enhancement only
13. Let long strings break mobile → Use word-break CSS
14. Skip validation → Server validates EVERYTHING
15. Implement without reading spec → Read relevant PART first
16. Modify AI.md content → READ-ONLY SPEC
17. Edit `## Project variables` in IDEA.md without confirming with the user
18. Read an image larger than 1000×1000 directly into context
19. Use a non-conforming IDEA.md without migration

## ALWAYS Do - NON-NEGOTIABLE
1. Read AI.md before implementing ANY feature
2. Server-side processing (server does the work, client displays)
3. Mobile-first responsive CSS
4. All features work without JavaScript
5. Tor hidden service support (auto-enabled if Tor found)
6. Built-in scheduler, GeoIP, metrics, email, backup, update
7. All settings configurable via API and config file
8. Client binary for ALL projects
9. Commit often via `gitcommit --dir {dir} all` — small, focused commits

## File Locations
- Config: `/etc/apimgr/vidveil/server.yml`
- Data: `/var/lib/apimgr/vidveil/`
- Logs: `/var/log/apimgr/vidveil/`
- Source: `src/`
- Docker: `docker/`

## Where to Find Details
- AI behavior: `.claude/rules/ai-rules.md` (PART 0, 1)
- Project structure: `.claude/rules/project-rules.md` (PART 2, 3, 4)
- Frontend/WebUI: `.claude/rules/frontend-rules.md` (PART 16)
- Full spec: `AI.md` (~43k lines) ← **SOURCE OF TRUTH**

## Current Project State
[AI updates this section as work progresses]
- Last read AI.md: 2026-07-19 (full compliance audit PART 0–33 complete via `audit` agent)
- Current task: PART 21 (BACKUP & RESTORE) full-implementation verification complete — 9 fixes
  (SSL path in backup+restore, manifest checksum, two-phase validate-then-extract restore,
  disk-space precheck, retention-before-backup ordering, max_total_size enforcement,
  BackupDailyFull scheduler wiring, config.go retention validation, --password flag removal
  in favor of interactive prompts) landed across 4 commits: 22d17ac903bc (config retention
  validation), f47c1019f60c (backup/restore engine hardening + diskspace files + tests),
  569b206f1a1d (BackupDaily scheduler wiring), b84c008fa37e (--password removal). CI on
  b84c008fa37e failed on vuln-check (GO-2026-5970, golang.org/x/text v0.37.0, reachable via
  ssl.go's requestTLSALPN01 -> autocert.HostWhitelist); fixed by bumping x/text to v0.39.0
  (plus cascading go mod tidy upgrades) in commit cf0e5a5ab081. CI on cf0e5a5ab081 verified
  green via `gh run view --json status,conclusion,jobs` (all 9 jobs success, including
  vuln-check). PART 21 verification is fully complete and pushed.
- Tests: full `go build` + `go vet ./...` + `go test -cover ./...` pass, zero FAIL, all packages ok
- Completed: PART 27–33 validated; Jenkinsfile ${WORKSPACE} fix (30bdb61); i18n context fix (30bdb61); engine debug logging now strips raw HTML (logfmt only, verified live via Docker debug build + search); footer/CSS restructured to PART 16 fixed-row order; change-password and well-known endpoints removed (out of spec); full PART 0–33 audit found and fixed missing PART 5 env var overrides (DATABASE_DRIVER, DATABASE_URL, APPLICATION_NAME, APPLICATION_TAGLINE were documented but not implemented in config.go) — verified fix with own build+vet+test pass before committing, not just agent's self-report
- PART 32 GUI: implemented with -tags gui build tag; native GTK4/Cocoa/Win32 (not fyne); no user decision needed
- `gitcommit --dir {dir} all` always stages ALL modified files (git add -A behavior) — pre-staging a subset via `git add` does NOT limit the commit to that subset. This is documented in AI.md ("there is no local staging window to catch mistakes" — Tool Access section, ~line 3250); per Commit Cadence (~line 1573), splitting into small focused commits when multiple logical groups are dirty simultaneously requires `git stash`/`git stash pop` between groups, not `git add` staging
