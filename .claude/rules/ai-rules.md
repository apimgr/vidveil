# AI Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Guess requirements, values, file locations, or product behavior; search AI.md and the repository, then ask only when the spec is silent or materially ambiguous.
- Modify AI.md, skip its relevant PART, or use existing code as the authority over the spec.
- Add unspecified features, placeholder content, stubs, partial implementations, or TODO comments in production code.
- Run Go tools or project binaries directly on the host; use the project’s Docker/Makefile workflow.
- Weaken mandatory requirements because a feature is a product non-goal.

## CRITICAL - ALWAYS DO
- Read the complete relevant AI.md PART before implementation and return to the prior section after following cross-references.
- Resolve project values from IDEA.md and runtime commands; never invent them.
- Make complete changes, update IDEA.md when product features change, and keep README, docs, API specs, and implementation synchronized.
- Verify behavior with the project’s required tests, linters, and real post-push CI.
- Follow the task lifecycle and final compliance checklist in AI.md PART 0.

## KEY RULES
- AI.md is the read-only implementation source of truth; IDEA.md defines product WHAT only.
- Do one coherent task at a time; check requirements every 3–5 changes and do not report completion while required checks remain pending.
- Treat spec contradictions as template issues; do not silently “improve” the spec.

For complete details, see AI.md PART 0, 1.
