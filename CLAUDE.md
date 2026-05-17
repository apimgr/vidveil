# Project SPEC

Project: VIDVEIL
Role: Efficient loader for AI.md

**THIS FILE IS AUTO-LOADED EVERY CONVERSATION. FOLLOW IT EXACTLY.**

Purpose:
- This file is a short loader for the most important rules
- `AI.md` is the full source of truth
- For complete details, read the referenced PARTs in `AI.md`

## FIRST TURN - MANDATORY

On EVERY new conversation or after "context compacted" message:
1. **READ** the relevant `.claude/rules/*.md` for your current task
2. **NEVER** assume or guess - verify against AI.md before implementing

## Asking Questions

- **Default to continuing work** - do not stop just to ask whether to continue
- **Never guess** - if the answer cannot be determined from `AI.md`, `IDEA.md`, the codebase, or repo state, ASK
- **Do NOT ask for permission to keep going** - continue until complete, blocked, or user pauses
- **Question mark = question** - when user ends with `?`, answer/clarify, don't execute
- Ask only when: a required business decision is missing, two implementations differ materially, the action is destructive/irreversible, or the spec says to confirm

## Before ANY Code Change

1. Have I read the relevant PART in AI.md? (If no -> read it)
2. Does this follow the spec EXACTLY? (If unsure -> check spec)
3. Am I guessing or do I KNOW from the spec? (If guessing -> read spec)
4. Would this pass the compliance checklist? (AI.md FINAL section)

**WHEN IN DOUBT: READ THE SPEC. DO NOT GUESS.**

## Binary Terminology
- **server** = `vidveil` (main binary, runs as service)
- **client** = `vidveil-cli` (REQUIRED companion, CLI/TUI/GUI)
- **agent** = `vidveil-agent` (optional, runs on remote machines)

## Key Placeholders
- `{project_name}` = vidveil
- `{project_org}` = apimgr
- `{internal_name}` = vidveil
- `{admin_path}` = admin (default)
- `{plist_name}` = io.github.apimgr.vidveil

## Account Types (CRITICAL)
- **Server Admin** = manages the app (NOT a privileged OS user)
- **Primary Admin** = first admin, cannot be deleted
- **No Regular Users** - VidVeil is stateless/privacy-first (PART 34 NOT implemented)
- **No Organizations** (PART 35 NOT implemented)
- **No Custom Domains** (PART 36 NOT implemented)

## NEVER Do (Top 19) - VIOLATIONS ARE BUGS
1. Use bcrypt -> Use Argon2id
2. Put Dockerfile in root -> `docker/Dockerfile`
3. Use CGO -> CGO_ENABLED=0 always
4. Hardcode dev values -> Detect at runtime
5. Use external cron -> Internal scheduler (PART 19)
6. Store passwords plaintext -> Argon2id (tokens use SHA-256)
7. Create premium tiers -> All features free, no paywalls
8. Use Makefile in CI/CD -> Explicit commands only
9. Guess values -> Run command or read spec or ask
10. Skip platforms -> Build all 8 (linux/darwin/windows/freebsd x amd64/arm64)
11. Client-side rendering (React/Vue) -> Server-side Go templates
12. Require JavaScript for core features -> Progressive enhancement only
13. Let long strings break mobile -> Use word-break CSS
14. Skip validation -> Server validates EVERYTHING
15. Implement without reading spec -> Read relevant PART first
16. Modify AI.md content -> READ-ONLY
17. Edit `## Project variables` in IDEA.md without confirming -> Ask first
18. Read an image >1000x1000 directly -> Resize first
19. Use a non-conforming IDEA.md -> Migrate first
20. Add user accounts, organizations, or custom domains -> PARTS 34-36 NOT implemented

## ALWAYS Do - NON-NEGOTIABLE
1. Read AI.md before implementing ANY feature
2. Server-side processing (server does the work, client displays)
3. Mobile-first responsive CSS
4. All features work without JavaScript
5. Tor hidden service support (auto-enabled if Tor found)
6. Built-in scheduler, GeoIP, metrics, email, backup, update
7. Full admin panel with ALL settings
8. Client binary for ALL projects
9. Commit often - small, focused commits

## File Locations
- Config: `/etc/apimgr/vidveil/server.yml` (Linux)
- Data: `/var/lib/apimgr/vidveil/`
- Logs: `/var/log/apimgr/vidveil/`
- Source: `src/`
- Docker: `docker/`

## Where to Find Details
- AI behavior: `.claude/rules/ai-rules.md` (PART 0, 1)
- Project structure: `.claude/rules/project-rules.md` (PART 2, 3, 4)
- Frontend/WebUI: `.claude/rules/frontend-rules.md` (PART 16, 17)
- Full spec: `AI.md` (SOURCE OF TRUTH)

## Current Project State
- Last read AI.md: 2026-05-16
- Current task: Bootstrap complete
- Relevant PARTs: 0-6
