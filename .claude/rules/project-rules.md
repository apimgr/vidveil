# Project Rules (PART 2, 3, 4)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: License & Attribution, Project Structure, OS-Specific Paths

## CRITICAL - NEVER DO

- Use GPL/AGPL/LGPL dependencies - MIT/Apache/BSD/ISC only
- Forget to update LICENSE.md when dependencies change
- Create plural directory names - singular only (handler/, model/)
- Create root files outside the AI.md "Allowed Root Files" list
- Create root directories outside the "Allowed Root Directories" list
- Use uppercase or camelCase in Go file names
- Hardcode {project_name}/{project_org} - infer from path or git remote

## CRITICAL - ALWAYS DO

- Use latest stable Go version - never pin specific versions
- CGO_ENABLED=0 - all libraries pure Go
- Use modernc.org/sqlite (NEVER mattn/go-sqlite3)
- Use Argon2id for new passwords; SHA-256 for tokens
- Source code in src/, Docker in docker/, docs in docs/, tests in tests/
- Match all paths to OS-specific patterns from PART 4
- `server.yml` is the canonical config name (auto-migrate from server.yaml)
- Embed all assets via Go `embed` package - single static binary

---
For complete details, see AI.md PART 2, 3, 4
