# Repo-local Task History

This file is a working task list, not the spec. AI.md is the canonical spec and source of truth.

## Current Status

- AI.md was refreshed on 2026-05-15 (commit `d7adcbd5de5a`). Current md5 (no substitution) is `26af2a455a159f2912ccabb93e002a81` (60,829 lines). The template treats `{project_name}` / `{project_org}` / `{internal_name}` / `{plist_name}` as REFERENCE TOKENS that resolve from `IDEA.md ## Project variables` at read time - AI.md stays read-only with placeholders intact. AI.md is TRACKED in git per PART 0 "Allowed Root Files".
- IDEA.md is in the AI.md PART 0 three-section format: `## Project description`, `## Project variables`, `## Business logic` (with all six required subsections plus reference detail). official_site is sourced from `site.txt` (https://x.scour.li).
- Repo-root AI loader (CLAUDE.md) reflects the 2026-05-07 md5/linecount stamp; not yet refreshed to the 2026-05-15 snapshot but the loader itself is non-authoritative.
- 14 per-tool rule cheatsheet files in `.claude/rules/` are tracked in repo and verified against the current template.
- AUDIT.AI.md does not exist; audit on 2026-05-15 found only the two TODO.AI.md / PLAN.AI.md hygiene issues fixed in this same commit (under the >5 threshold that would require an AUDIT.AI.md file).
- PLAN.AI.md deleted on 2026-05-15 - all work it described is fully committed; per global CLAUDE.md hygiene the file must be deleted once that condition is met.

## Active / Pending

- [ ] historical-copilot-attribution - Multiple existing commit bodies contain `Co-authored-by: Copilot <...>`. AI.md PART 0 forbids AI attribution in commits. Cleaning historical commits requires a force-push to main (a destructive op) and is not actionable without explicit user authorization.
