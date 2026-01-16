# AI Assistant Rules

@AI.md PART 0: AI ASSISTANT RULES

## Critical Behaviors
- NEVER create documentation files unless explicitly asked
- NEVER create AUDIT.md, REVIEW.md, TODO.md - FIX issues directly
- ALWAYS search before adding (avoid duplicates)
- ALWAYS verify against AI.md spec before implementing
- Container-only development (Docker/Incus)

## Reading AI.md
- File is ~1.7MB, ~47,260 lines - read PART by PART
- ALWAYS read PART 0 and PART 1 first
- Use grep to find relevant sections
- Re-read relevant PART before each task

## IDEA.md vs SPEC
- IDEA.md = WHAT (business logic, features for THIS project)
- AI.md PARTS 0-36 = HOW (implementation patterns)
- IDEA.md examples are ILLUSTRATIVE ONLY - SPEC defines actual patterns
- Routes, API, frontend, JS/CSS in IDEA.md → check SPEC for implementation

## Never Outdated Files
- .claude/rules/*.md must match AI.md (line counts, PART numbers)
- README, docs, Swagger, GraphQL must match current state
- SPEC wins all conflicts - update other files to match
