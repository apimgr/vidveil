# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️
<!-- Rule summary checked against AI.md on 2026-10-17; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never guess or assume a requirement — STOP and ASK when unsure
- Never modify AI.md (PARTS 0-33) — it is READ-ONLY. Project changes go in IDEA.md / SPEC.md
- Never rely on memory — read the spec sections relevant to the current task before implementing
- Never "improve" or "optimize" the spec, invent patterns, or add unrequested features
- Never skip validation, sections, or "minor details" — every non-optional PART is mandatory
- Never jump between unfinished features — complete one fully before starting the next
- Never say "done" without verification (read → search → test → verify → confident)
- Never create report/analysis files (AUDIT.md, COMPLIANCE.md, SUMMARY.md) — fix issues directly; temporary `AUDIT.AI.md` only for explicit audits with >5 issues, deleted when resolved
- Never truncate/empty `TODO.AI.md` in one shot — remove items individually as each is resolved and committed; never delete or empty human-owned `TODO.md`
- Never install Go locally or run `go` commands on the host — ALL builds/tests use Docker (`casjaysdev/go:latest`) or Incus
- Never use `SELECT *` in application code — name columns explicitly
- Never concatenate SQL, shell out with user input, or trust input — parameterize, validate, allowlist
- Never expose sensitive data (paths, stack traces, connection strings, internal IPs) to users/logs/API responses
- Never use inline or below-line comments — comments always go ABOVE code (all languages; JSON never allows comments)
- Never use bare relative paths in code/emails/templates — use `{fqdn}/path` via `BuildURL(r, ...)`; router registration is the only exception
- Never include AI attribution anywhere (code, comments, commits, PRs, docs)
- Never use plain `git commit`/`git push` — use `gitcommit` wrapper only; subagents never write COMMIT_MESS or call gitcommit
- Never let a subagent run builds, tests, lint, formatting, type checks, benchmarks, or commit gates — the primary agent validates once after integrating delegated edits
- Never weaken security (authn/authz, TLS, CSRF/CSP/CORS, rate limiting, input validation) to improve usability
- Never reveal operator tokens or secrets after generation; owner and linked API tokens are the explicit exception and may be revealed on demand from `/server/preferences`, masked by default and never logged
- Never assume self-hosted/SMB users are tech-savvy — write UI text, docs, and error messages for non-technical users

## CRITICAL - ALWAYS DO

- Read the AI.md PART(s) relevant to the current task before implementing (not the whole file speculatively)
- Ask numbered/lettered questions when the spec is ambiguous or multiple approaches are valid
- Verify every claim of completion: read files, test, verify output — self-validation loop (see table below)
- Treat servers as internet-facing and hostile-traffic-exposed by default; make the secure path easier, never weaker
- Use parameterized queries, HTML-escape output, CSRF tokens on state-changing forms, `filepath.Clean()` + reject `..`
- Keep README.md and all documentation in sync with actual code after every change
- Use intent-revealing names (`AppMode` not `Mode`, `IsAppModeDev()` not `IsDevelopment()`)
- Use `gofmt`/2-space (or ecosystem default) formatting; every file ends with exactly one trailing newline
- Use `curl -q -LSsf` (+ `-I`/`-o`/`-H`/`-X`/`-d` as needed) for ALL curl examples/scripts
- Create/update `.claude/rules/*.md` (13 files) at session start if missing or if AI.md is newer
- Read non-empty `.claude/memory/MEMORY.md` and its relevant references; preserve project-specific discoveries there instead of duplicating the AI.md spec
- Write `.git/COMMIT_MESS` and re-read it before `gitcommit --dir {dir} all` (parent instance only)

## Key Rules Summary

**AI.md structure:** PARTS 0-33 = HOW (implementation, read-only). IDEA.md = WHAT (business logic, editable).

**Session Init (first read):** check `.claude/rules/`, create/update rule files, create IDEA.md if missing, check TODO.AI.md/TODO.md, commit COMMIT/NEVER/MUST rules to memory.

**Rule file → PART mapping:**
| File | PARTs |
|------|-------|
| ai-rules.md | 0, 1 |
| project-rules.md | 2, 3, 4 |
| config-rules.md | 5, 6, 12 |
| binary-rules.md | 7, 8, 32 |
| backend-rules.md | 9, 10, 11, 31 |
| api-rules.md | 13, 14, 15 |
| frontend-rules.md | 16 |
| features-rules.md | 17-22 |
| service-rules.md | 23, 24 |
| makefile-rules.md | 25 |
| docker-rules.md | 26 |
| cicd-rules.md | 27 |
| testing-rules.md | 28, 29, 30 |

**Self-validation table (pick per change type):** backend/API → `make test` + curl; CLI → `make build` + run + check `--help`/`--version`; frontend → dev server + browser; Docker → build + run + smoke test; CI/CD → run workflow/`act`; security → test success AND bypass paths.

**Full web app architecture:** every feature works via Browser (HTML), PWA, API (JSON), and CLI client. Pattern: `/path` (HTML) ↔ `/api/{api_version}/path` (JSON), always both.

**Rate limiting defaults:** failed auth 5/15min; API configurable/1min; uploads 10/hour. Response: 429 + `Retry-After` header.

**Error message rule:** user sees minimal/generic message; log/console gets full detail + stack + request_id. Never reveal resource existence, DB structure, internal IPs, or specific auth-failure reason.

**Naming conventions:** files `lowercase_snake.go`; packages `lowercase`; public `PascalCase`; private `camelCase`; interfaces `PascalCase`+`-er`. Exceptions: ID, URL, API, HTTP, HTML, JSON, SQL, CSS, JS stay uppercase.

**File-only configuration:** `server.yml` is the only config source, no admin web UI/runtime mutation API, hot-reload on save (except listen address/port/DB driver which need restart + warning).

**Container-only development:** `make dev` (quick build to tempdir), `make local` (binaries/ with version), `make build` (full cross-platform), `make test` (unit tests via Docker); `./tests/incus.sh` for full OS integration tests.

**Commit workflow:** `gitcommit --dir {dir} all` only; message from `.git/COMMIT_MESS`; ✅ emoji title reserved for TODO.AI.md full-completion commits: `✅ all todo items have been completed ✅`.

**Audit (explicit "audit" trigger only):** verify code↔spec, file sync (README/Swagger/GraphQL/docs), infrastructure accuracy, all 13 rule files present/correct, FIX issues directly (don't just report), track >5 issues in `AUDIT.AI.md` (delete when resolved).

**Large image handling:** check dimensions before reading; if >1000×1000, downscale to `${TMPDIR:-/tmp}` copy first (identify/magick/gm/vips/sips/ffmpeg fallback chain).

**README section order:** Title/Badges → About → Official Site → Features → Production → Client → Configuration → API → Other → Development (last) → Disclaimer → License. Every badge is a linked badge `[![alt](img)](link)`.

**URL rules:** docs/README use `{official_site}/path` (full URL); embedded Go/JS/template code uses `{fqdn}/path` via `BuildURL(r, ...)` (proxy-header-aware) — never hardcode `https://`, even with `{fqdn}` interpolated directly.

**Target audience:** self-hosted, SMB, and enterprise — assume self-hosted/SMB users are NOT tech-savvy; keep UI copy, docs, and error messages plain and non-technical.

**Sensitive information:** never expose credentials, connection strings, internal infra, or config internals in healthz/API responses/error messages/logs/HTML. Owner and linked API tokens are encrypted at rest and revealable only by their owner on demand; operator tokens and other secrets are never retrievable from the server.

For complete details, see AI.md PART 0, 1
