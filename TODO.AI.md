# TODO

## `.claude/rules/` completeness (PART 0)
`.claude/rules/` did not exist at all before this pass. Created and populated
`ai-rules.md` (PART 0, 1), `project-rules.md` (PART 2, 3, 4), and
`config-rules.md` (PART 5, 6, 12) from AI.md content. The remaining 10 required
files still need to be generated from their PARTs, following the same template
(`CRITICAL - NEVER DO` / `CRITICAL - ALWAYS DO` / `KEY DECISIONS` / `TERMINOLOGY`
/ `QUICK REFERENCE`):
- `binary-rules.md` - PART 7, 8, 32 (Binary Requirements, Server Binary CLI, Client)
- `backend-rules.md` - PART 9, 10, 11, 31 (Error Handling & Caching, Database, Security & Logging, Tor Hidden Service)
- `api-rules.md` - PART 13, 14, 15 (Health & Versioning, API Structure, SSL/TLS & Let's Encrypt)
- `frontend-rules.md` - PART 16 (Web Frontend) — referenced by root CLAUDE.md's "Where to Find Details" but missing on disk
- `features-rules.md` - PART 17-22 (Email & Notifications, Scheduler, GeoIP, Metrics, Backup & Restore, Update Command)
- `service-rules.md` - PART 23, 24 (Privilege Escalation & Service, Service Support)
- `makefile-rules.md` - PART 25 (Makefile, local dev only)
- `docker-rules.md` - PART 26 (Docker)
- `cicd-rules.md` - PART 27 (CI/CD Workflows)
- `testing-rules.md` - PART 28, 29, 30 (Testing & Development, ReadTheDocs Documentation, I18N & A11Y)

## IDEA.md `## Project variables` - `internal_org` not listed
PART 3 (Placeholder Reference) documents `{internal_org}` as an IDEA.md
`## Project variables` entry set once at first-run and frozen thereafter.
IDEA.md currently lists `project_name`, `project_org`, `internal_name`,
`app_name`, `official_site`, `coverage_minimum` but no explicit `internal_org`
line. In practice all OS paths in code correctly resolve to `apimgr` (matches
`project_org`, which is the documented first-run default for `internal_org`
when not otherwise set), so there is no functional drift - but the variable
should probably be added explicitly to IDEA.md for clarity. Do not edit
`## Project variables` without asking the user first (per project rules).
