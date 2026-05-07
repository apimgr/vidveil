# Project SPEC

Project: VIDVEIL
Role: Efficient loader for AI.md

WARNING: THIS FILE IS AUTO-LOADED EVERY CONVERSATION. FOLLOW IT EXACTLY.

Purpose:
- This file is a short loader for the most important rules
- `AI.md` is the full source of truth (~60k lines)
- For complete details, read the referenced PARTs in `AI.md`

## FIRST TURN - MANDATORY

On EVERY new conversation or after "context compacted" message:
1. **READ** `AI.md` PART 0 and PART 1 before doing ANYTHING
2. **READ** the relevant `.claude/rules/*.md` for your current task
3. **NEVER** assume or guess - verify against AI.md before implementing

If you have not read AI.md this session -> STOP -> Read it NOW.

## Asking Questions

- **Default to continuing work** - do not stop just to ask whether you should continue; if the next step is implied by the spec, the current task, or the current findings, continue
- **Never guess** - if the answer cannot be determined from `AI.md`, `IDEA.md`, the codebase, or repo state AND the missing info materially changes behavior/scope/safety, ASK
- **Do NOT ask for permission to keep going** - continue until the current task is complete, blocked by a real decision, or the user explicitly asks to pause
- **Question mark = question** - when user ends with `?`, answer/clarify, do not execute
- **Use AskUserQuestion wizard** - one question at a time, options + "Other" custom input

## Before ANY Code Change

1. Have I read the relevant PART in AI.md? (If no -> read it)
2. Does this follow the spec EXACTLY? (If unsure -> check spec)
3. Am I guessing or do I KNOW from the spec? (If guessing -> read spec)
4. Would this pass the compliance checklist? (AI.md FINAL section)

WHEN IN DOUBT: READ THE SPEC. DO NOT GUESS.

## Binary Terminology
- **server** = `vidveil` (main binary, runs as service)
- **client** = `vidveil-cli` (REQUIRED companion, CLI/TUI/GUI)
- **agent** = `vidveil-agent` (NOT IMPLEMENTED for vidveil)

## Key Placeholders
- `{project_name}` = vidveil
- `{project_org}` = apimgr
- `{internal_name}` = vidveil (FROZEN)
- `{plist_name}` = io.github.apimgr.vidveil
- `{admin_path}` = admin (configurable, default)
- `{official_site}` = https://x.scour.li (from site.txt)

## Account Types (CRITICAL)
- **Server Admin** = manages the app (NOT a privileged OS user)
- **Primary Admin** = first admin, cannot be deleted
- **Regular User** = NOT IMPLEMENTED (PART 34 not adopted by vidveil)
- VidVeil is stateless / no user accounts (see IDEA.md `## Business logic`)

## NEVER Do (Top rules) - VIOLATIONS ARE BUGS
1. Use bcrypt -> Use Argon2id
2. Put Dockerfile in root -> `docker/Dockerfile`
3. Use CGO -> CGO_ENABLED=0 always
4. Hardcode dev values -> Detect at runtime
5. Use external cron -> Internal scheduler (PART 19)
6. Store passwords/tokens plaintext -> Argon2id / SHA-256
7. Premium tiers / paywalls / license keys
8. Use Makefile in CI/CD -> Explicit commands only
9. Guess values a command can produce -> Run the command
10. Skip platforms -> Build all 8 (linux/darwin/windows/freebsd x amd64/arm64)
11. Client-side rendering / SPA -> Server-side Go templates
12. Require JS for core features -> Progressive enhancement
13. Modify AI.md PART 0-33 content -> READ-ONLY SPEC; project changes go in IDEA.md
14. Plain `git commit`/`git push` -> Use `gitcommit <command>` (signs + pushes)
15. `gitcommit` without verifying `.git/COMMIT_MESS` -> message goes straight to remote

## ALWAYS Do - NON-NEGOTIABLE
1. Read AI.md before implementing ANY feature
2. Server-side processing (server does the work, client displays)
3. Mobile-first responsive CSS
4. All features work without JavaScript
5. Tor hidden service support (auto-enabled if Tor found)
6. Built-in scheduler, GeoIP, metrics, email, backup, update
7. Full admin panel with ALL settings
8. Client binary for ALL projects
9. Commit often via `gitcommit <command>` - small, focused commits with accurate `.git/COMMIT_MESS`

## File Locations
- Config: `{config_dir}/server.yml`
- Data: `{data_dir}/`
- Logs: `{log_dir}/`
- Source: `src/`
- Docker: `docker/`

## Where to Find Details
- AI behavior: `.claude/rules/ai-rules.md` (PART 0, 1)
- Project structure: `.claude/rules/project-rules.md` (PART 2, 3, 4)
- Frontend/WebUI: `.claude/rules/frontend-rules.md` (PART 16, 17)
- Full spec: `AI.md` <- **SOURCE OF TRUTH**

## Current Project State
- Last AI.md refresh: 2026-05-06 (md5 ab2546e6b41c6e844c3206c2cdbeb600 after substitution)
- IDEA.md migrated to three-section format on 2026-05-06 (backup at IDEA.md.preMigration.bak)
- AUDIT.AI.md: deleted as stale
- TODO.AI.md: refreshed against new template
- Open work: see TODO.AI.md `## Active / Pending`
