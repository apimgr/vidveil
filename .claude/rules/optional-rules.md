# Optional Rules (PART 34, 35, 36)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Implement optional features casually without following their full PART rules
- Leave partial optional systems in the codebase
- Treat an implemented optional feature as if it were still optional in behavior

## CRITICAL - ALWAYS DO

- Either omit optional features entirely or implement them fully per spec
- Re-read the relevant PART before touching any optional subsystem
- Remove dead or partial optional code instead of leaving drift behind

## Optional Feature Scope

- PART 34: Multi-user
- PART 35: Organizations
- PART 36: Custom domains

Once any of these are implemented, their PART becomes mandatory for that subsystem.

For complete details, see AI.md PART 34, PART 35, PART 36
