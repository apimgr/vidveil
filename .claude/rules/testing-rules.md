# Testing Rules (PART 29, 30, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Run Go directly on the host machine
- Use `docker-compose.yml` as an AI testing shortcut when the spec forbids it
- Store test temp data in permanent project paths
- Ignore i18n and accessibility requirements in user-facing work

## CRITICAL - ALWAYS DO

- Use containerized testing workflows
- Prefer Incus for fuller OS testing when available
- Use temp directories for disposable test state
- Keep docs, i18n, and accessibility aligned with implemented behavior

## Testing Rules

- Use Makefile and repo test scripts for supported local workflows
- Respect container-only development rules
- Keep test state out of committed runtime directories

## Documentation and I18N

- ReadTheDocs and MkDocs config must match actual docs structure
- Keep UTF-8, language handling, and fallback behavior aligned with the spec
- Treat accessibility and localization as required quality work, not polish

For complete details, see AI.md PART 29, PART 30, PART 31
