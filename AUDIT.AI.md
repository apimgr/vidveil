# Project Audit

Started: 2026-04-28
Spec: AI.md refreshed from `~/Documents/templates/go/AI.md` on 2026-04-28 (~60k lines)
Approach: section-by-section, in spec order. Pre-PART-0 first, then PART 0, then PART 1, etc.

## Audit Plan

| Pass | Source lines | Status |
|------|--------------|--------|
| Pre-PART-0 (front matter + critical rules + structure rules) | 1–1997 | Findings logged below |
| PART 0 (AI Assistant Rules) | 1998–3955 | Pending |
| PART 1 (Critical Rules) | 3956–5270 | Pending |
| PART 2 (License & Attribution) | 5271–5604 | Pending |
| PART 3 (Project Structure) | 5605–6574 | Pending |
| PART 4 (OS-Specific Paths) | 6575–6768 | Pending |
| PART 5 (Configuration) | 6769–8689 | Pending |
| PART 6 (Application Modes) | 8690–9327 | Pending |
| PART 7 (Binary Requirements) | 9328–9981 | Pending |
| PART 8 (Server Binary CLI) | 9982–13293 | Pending |
| PART 9 (Error Handling & Caching) | 13294–13677 | Pending |
| PART 10 (Database & Cluster) | 13678–14278 | Pending |
| PART 11 (Security & Logging) | 14279–16889 | Pending |
| PART 12 (Server Configuration) | 16890–18198 | Pending |
| PART 13 (Health & Versioning) | 18199–18962 | Pending |
| PART 14 (API Structure) | 18963–20682 | Pending |
| PART 15 (SSL/TLS & Let's Encrypt) | 20683–21654 | Pending |
| PART 16 (Web Frontend) | 21655–27926 | Pending |
| PART 17 (Admin Panel) | 27927–30347 | Pending |
| PART 18 (Email & Notifications) | 30348–31675 | Pending |
| PART 19 (Scheduler) | 31676–32160 | Pending |
| PART 20 (GeoIP) | 32161–32257 | Pending |
| PART 21 (Metrics) | 32258–33702 | Pending |
| PART 22 (Backup & Restore) | 33703–34447 | Pending |
| PART 23 (Update Command) | 34448–34926 | Pending |
| PART 24 (Privilege Escalation & Service) | 34927–35835 | Pending |
| PART 25 (Service Support) | 35836–36128 | Pending |
| PART 26 (Makefile) | 36129–36905 | Pending |
| PART 27 (Docker) | 36906–38413 | Pending |
| PART 28 (CI/CD Workflows) | 38414–41347 | Pending |
| PART 29 (Testing & Development) | 41348–43186 | Pending |
| PART 30 (ReadTheDocs Documentation) | 43187–43922 | Pending |
| PART 31 (I18N & A11Y) | 43923–45883 | Pending |
| PART 32 (Tor Hidden Service) | 45884–47635 | Pending |
| PART 33 (Client & Agent) | 47636–52262 | Pending |
| PART 34 (Multi-User) | 52263–56063 | OPTIONAL — currently not implemented (verify clean absence) |
| PART 35 (Organizations) | 56064–56704 | OPTIONAL — verify clean absence |
| PART 36 (Custom Domains) | 56705–57727 | OPTIONAL — verify clean absence |
| PART 37 (IDEA.md Reference) | 57728–57981 | Reference-only |
| FINAL (Compliance Checklist) | 57982+ | Pending |

---

# Pre-PART-0 Findings

Source: AI.md lines 1–1997 (First-Time Setup, Critical Rules, Build/Binary, Container-Only Development, Runtime Detection, Performance, JSON, Docker, CI/CD, Database, CLI Quick Reference, Directory Structure, File & Directory Naming, Files & Directories Master Rules, AI Tool Configuration, Terminology, Monitoring Endpoints, How to Read).

## Issues Found

### Decision-required
- [ ] **placeholder-instantiation** — AI.md still contains 1037 `{project_name}`, 376 `{project_org}`, 314 `{PROJECT_NAME}`, 74 `{PROJECT_ORG}` occurrences. Pre-PART-0 "First-Time AI.md Setup" describes substituting these for the project; PART 0 also says PARTS 0-37 are "NEVER modify". Many occurrences are intentional documentation about the variable system itself (variable tables, examples, derivation rules) — a blind global replace would corrupt the spec. **Needs user direction**: leave AI.md as a generic template (and rely on CLAUDE.md plus the rule files for project-specific values), or do a careful selective substitution outside the variable-documentation tables.

### Code drift (real bugs)
- [ ] **cli-missing-baseurl** — `src/main.go` flag parser supports `--config`, `--data`, `--cache`, `--log`, `--backup`, `--pid`, `--address`, `--port`, `--mode`, `--debug`, `--color`, `--service`, `--update`, `--maintenance`, `--status`, `--daemon`, `--shell`, `--help`, `--version` but not `--baseurl`. Spec requires `--baseurl {path}` (pre-PART-0 line 501; PART 8 lines 10164, 10205; reverse-proxy precedence lines 7781, 16946). Add the flag, plumb it through `server.baseurl`, and ensure the `X-Forwarded-Prefix` → `X-Forwarded-Path` → `X-Script-Name` → `server.baseurl` → `/` precedence is honored.

- [ ] **i18n-package-location** — Spec (lines 608–625) puts translations at `src/common/i18n/` with subdir `locales/` and files `{en,es,fr,de,zh,ar,ja}.json`. Current implementation lives at `src/server/service/i18n/translations/{...}.json`. Move package to `src/common/i18n/`, rename subdir `translations/` → `locales/`, update embed path and all importers.

- [ ] **gitignore-missing-claude-and-ai** — `.gitignore` lists `.cursor/`, `.aider/`, `.windsurf/` but not `.claude/` or `.ai/`. Spec (lines 829, 1533) requires ALL AI config dirs gitignored. Currently 14 files in `.claude/rules/` are tracked by git. Resolve by: (a) adding `.claude/` and `.ai/` to `.gitignore`, (b) `git rm -r --cached .claude/` to untrack the rule files, and (c) treating rule files as locally regenerated from AI.md going forward. Note: `CLAUDE.md` at the project root remains tracked (allowed root file per spec line 1550).

### Possibly-needed (verify before fixing)
- [ ] **internal-name-vs-project-name** — New spec introduces `{internal_name}` (lines 1601–1638) as a locked filesystem identity that survives binary renames. Filesystem paths, systemd unit, log/cache/backup/db dirs, PID file MUST use `{internal_name}`; user-visible strings (banner, --version, User-Agent) MUST use `{project_name}`. Current code does not implement this distinction (every reference uses literal `vidveil`). Practically harmless while the binary is never renamed, but the abstraction needs to be added so paths persist across rename.

- [ ] **plist-label-derivation** — New spec adds `{plist_label}` (line 1607) as a reverse-DNS identifier used for launchd `<Label>`, plist filename, and Windows service name. Derivation rule: reverse of `{official_site}` first (`https://x.scour.li` → `li.scour.x`), then reverse of repo host (`github.com/apimgr/vidveil` → `io.github.apimgr.vidveil`), then `local.{project_org}.{internal_name}`. CLAUDE.md currently documents `com.apimgr.vidveil` for macOS; per the new derivation rule with `{official_site}=https://x.scour.li`, the spec-derived label would be `li.scour.x`. Verify what the launchd plist currently uses and decide whether to follow the new derivation rule or persist the current value (spec allows persistence at first install).

### Already in TODO.AI.md
- [ ] **client-elevated-user-rule** (existing TODO entry) — PART 33 says `vidveil-cli` always runs as a normal user and never as root/administrator. Still pending.

## Verified compliant in pre-PART-0 (no fix needed)

Build & binary:
- License is MIT with `LICENSE.md` in project root.
- `CGO_ENABLED=0` in Dockerfile and Makefile.
- All 8 platforms in Makefile `PLATFORMS`: linux/darwin/windows/freebsd × amd64/arm64.
- Binary built from `./src` into `binaries/`.

Docker:
- `docker/Dockerfile` (not in repo root). Multi-stage build: `golang:alpine` → `alpine:latest`.
- `STOPSIGNAL SIGRTMIN+3`, `EXPOSE 80`, `ENTRYPOINT ["tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh"]`.
- Required runtime packages installed: `git`, `curl`, `bash`, `tini`, `tor`.
- `docker/file_system/usr/local/bin/entrypoint.sh` overlay present.
- Compose file uses `64893:80` (random 64xxx convention).
- TZ defaults to `America/New_York`.

CI/CD:
- `.github/workflows/{release,beta,daily,docker}.yml` and `.gitea/workflows/` exist; no workflow shells out to `make` (explicit commands only).

CLI surface (pre-PART-0 quick reference):
- All required flags present except `--baseurl` (flagged above): `--help/-h`, `--version/-v`, `--mode`, `--config`, `--data`, `--log`, `--pid`, `--address`, `--port`, `--debug`, `--status`, `--service`, `--daemon`, `--maintenance`, `--update`. Extras `--cache`, `--backup`, `--color`, `--shell` all defined in later PARTs (8, 33).
- Short flags limited to `-h`, `-v` only.

Directory structure:
- `src/`, `docker/`, `docs/`, `scripts/`, `tests/`, `.github/`, `.gitea/` all present.
- `binaries/`, `releases/`, `rootfs/` gitignored.
- No forbidden root files (no `SUMMARY.md`, `CHANGELOG.md`, `AUDIT.md`, `Dockerfile`, `docker-compose.yml`, `.env*`, `server.yml`, `cli.yml`, `*.example.*`).
- No forbidden root directories (no `config/`, `data/`, `logs/`, `tmp/`, `vendor/`, `node_modules/`, `utils/`, `common/`, `lib/`).
- `src/common/` is allowed (sub-package, distinct from forbidden root-level `common/`).
- `src/admin/admin.go` is a thin re-export of `src/server/service/admin` — acceptable.
- `.dockerignore` excludes `.claude/`, `.cursor/`, `.aider/`, `.ai/`, `tests/`, `docs/`, `Makefile`, `*.md`.
- `release.txt` = `1.0.0`, `site.txt` = `https://x.scour.li`.

AI tool configuration:
- `CLAUDE.md` at project root with the required structure (per spec lines 1090–1187).
- All 14 `.claude/rules/*.md` cheatsheets present and structurally aligned (header, NEVER/ALWAYS, reference back to AI.md PARTs).
- IDEA.md, TODO.AI.md, PLAN.AI.md, README.md present.

Documentation conventions:
- Translation file set matches spec languages (en/es/fr/de/zh/ar/ja) — only the package location is wrong (see issue above).

## Sync Required

- [ ] README.md — verify pre-PART-0 changes don't drift docs (defer until after PART 1 audit).
- [ ] docs/cli.md — must reflect `--baseurl` once added.
- [ ] mkdocs.yml — verify after PART 30 audit.

## Completed

(Empty — fixes pending user triage.)
