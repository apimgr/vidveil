# Testing Rules (PART 28, 29, 30)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Put test data, output, config, or runtime state in the project tree or directly under `/tmp`.
- Run Go tests, builds, or binaries on the host.
- Treat a local test or workflow parse as a substitute for real provider CI.
- Add documentation outside the MkDocs/ReadTheDocs structure or leave user-facing behavior undocumented.
- Ship untranslated user-visible strings or inaccessible/keyboard-inoperable UI.

## CRITICAL - ALWAYS DO
- Put temporary data under `/tmp/{project_org}/{internal_name}-XXXXXX`, use named/limited containers, and clean only task-created artifacts.
- Cover unit behavior, content negotiation, rendered pages with and without JavaScript, integration paths, and required edge/failure cases.
- Enforce the 60% coverage target and follow the exact Docker/Incus tier split in PART 28.
- Keep MkDocs, ReadTheDocs, API/config/security/integration docs, and required theme files current.
- Use the prescribed translation keys/locales, build-time i18n validation, RTL behavior, and WCAG accessibility checks.

## KEY RULES
- `tests/run_tests.sh` is the basic Docker tier; the full application integration script is a separate explicit tier.
- ReadTheDocs content and project metadata must use the paths and templates in PART 29.

For complete details, see AI.md PART 28, 29, 30.
