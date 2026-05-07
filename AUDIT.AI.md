# Project Audit

Started: 2026-05-07
AI.md md5: d180bc9dd77aac60b310d787a7796d23 (60,825 lines)

## Issues Found

### Repository Hygiene

- [ ] A. `binaries/` tracked in git (26 files) — PART 3 lists `binaries/` as gitignored. Need to `git rm` and add to `.gitignore`.
- [ ] B. Inconsistent client binary naming. Repo has BOTH `vidveil-cli-{os}-{arch}` AND `vidveil-{os}-{arch}-cli` patterns. Per PART 7/33 only the prior is correct. Resolved by removing tracked binaries (Finding A).
- [ ] C. `.gitignore` missing `binaries/`, `releases/`, `volumes/` entries.
- [ ] D. Stale `IDEA.md.preMigration.bak` in repo root — not in PART 3 "Allowed Root Files". Migration completed 2026-05-06 and confirmed; safe to remove.

### CI/CD (PART 28)

- [ ] E. `.github/workflows/build.yml` missing — REQUIRED for public repos (build/test/coverage/repo validation).
- [ ] F. `.github/workflows/security.yml` missing — REQUIRED for public repos (secret scanning, dep checks, workflow policy).
- [ ] G. Third-party actions not pinned to full SHA — `actions/checkout@v6`, `actions/setup-go@v5`, `actions/upload-artifact@v5`, `actions/download-artifact@v5`, `softprops/action-gh-release@v2`, `docker/setup-qemu-action@v3`, `docker/setup-buildx-action@v3`, `docker/login-action@v3`, `docker/build-push-action@v6` across `.github/workflows/*.yml` and `.gitea/workflows/*.yml`.
- [ ] H. No workflow-level default permissions in `.github/workflows/*.yml` — all 4 files only set permissions at job level. Needs `permissions: { contents: read }` at workflow level for least privilege.

### .github/ Community Files (PART 28)

- [ ] I. Missing `.github/CONTRIBUTING.md`
- [ ] J. Missing `.github/CODE_OF_CONDUCT.md`
- [ ] K. Missing `.github/SECURITY.md`
- [ ] L. Missing `.github/CODEOWNERS`
- [ ] M. Missing `.github/dependabot.yml`
- [ ] N. Missing `.github/ISSUE_TEMPLATE/bug_report.md`
- [ ] O. Missing `.github/ISSUE_TEMPLATE/feature_request.md`
- [ ] P. Missing `.github/ISSUE_TEMPLATE/config.yml`
- [ ] Q. Missing `.github/PULL_REQUEST_TEMPLATE.md`

### Documentation (PART 3, 30)

- [ ] R. `docs/security.md` missing at top level (only `docs/admin-guide/security.md` exists). PART 3 directory tree lists root-level `docs/security.md` for "Security, public endpoints, and reporting".
- [ ] S. `docs/integrations.md` missing. PART 3 lists it as required for "External identity and protocol integrations". VidVeil is stateless with no auth/users; verify whether this docs file is meaningfully required or if a brief stub explaining "no integrations" suffices.
- [ ] T. README build badge points to `release.yml` instead of `build.yml`. After Finding E fix, switch badge to `build.yml`.

### Rule Cheatsheets

- [ ] U. `.claude/rules/makefile-rules.md` ALWAYS-DO line lists "make dev / local / build / test / clean" — wrong. PART 26 mandates exactly six targets: `dev`, `local`, `build`, `test`, `release`, `docker` (and explicitly says "DO NOT ADD MORE"). No `clean` target.

## Fixed

(empty — to be filled as fixes land)
