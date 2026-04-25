# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Guess or assume — always read the spec or ask
- Implement without re-reading the relevant PART first
- Run `git add`, `git commit`, or `git push`
- Include AI attribution anywhere
- Create report files instead of fixing the issue
- Add inline comments — comments go above code only
- Use generic names like `Mode`, `Type`, `Status`, `Config`

## CRITICAL - ALWAYS DO

- Read AI.md PART 0 and PART 1 at session start
- Read the relevant PART before each implementation task
- Check `TODO.AI.md` and `.claude/rules/` during session init
- Write commit text to `.git/COMMIT_MESS` instead of committing
- Verify work before claiming completion
- Keep docs and implementation in sync
- Treat AI.md as the source of truth

## Session Initialization

1. Read AI.md PART 0 and PART 1 completely.
2. Check `.claude/rules/` and create or update all rule files if missing or stale.
3. Read `TODO.AI.md` if it exists.
4. Commit COMMIT / NEVER / MUST rules to memory.

## Commit Rules

- Run `git status --porcelain` before touching `.git/COMMIT_MESS`
- `.git/COMMIT_MESS` must match actual uncommitted changes
- Format: `{emoji} Title max 64 chars {emoji}` then a blank line and bullets
- Never include AI attribution or co-author lines

## Naming and Style

- Use intent-revealing names
- Use comments above code, never inline
- Prefer correctness and verification over speed
- Stop and ask when anything is unclear

For complete details, see AI.md PART 0, PART 1
