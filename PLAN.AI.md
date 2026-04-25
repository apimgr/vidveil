# Current Implementation Plan

## Goal

Keep the exact AI.md spec-compliance audit moving while storing the current plan and task history inside the repository so work survives the dev-server migration.

## Current Status

- Bootstrap and local rule-file restoration are complete.
- The broad documentation and infrastructure compliance sweep is already in progress in the worktree.
- The client PART 33 sweep has been carried through the currently verified drift set, including:
  - shell/help flag alignment
  - config selection and persistence
  - YAML/CSV output support
  - timeout/debug/api_version/admin_path handling
  - cluster failover and autodiscover refresh
  - auth/output/logging/cache config blocks
  - startup order, cli.log init, config/token permissions, and ownership verification

## Working Rules

1. Re-read AI.md and the relevant `.claude/rules/*.md` file before each implementation task.
2. Only fix verified mismatches; do not guess or invent behavior.
3. Validate changes with `make dev` and `make test`.
4. Keep `.git/COMMIT_MESS`, `TODO.AI.md`, and user-facing docs aligned with the actual worktree.

## Next Verification Targets

1. Re-check remaining PART 33 hard requirements that are stronger than sample-only config drift.
2. Resolve the client root/administrator prohibition only after finding exact implementation wording that fits the current containerized validation flow.
3. Continue the repo-wide exact drift sweep one verified mismatch at a time.

## Reference Material

- Session checkpoint history is preserved under `.copilot/session-state/.../checkpoints/` on the current machine.
- `TODO.AI.md` now carries the migrated repo-local task log so the next server has the task history in-tree.
