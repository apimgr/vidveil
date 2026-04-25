# CI/CD Rules (PART 28)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use Makefile targets in CI/CD
- Skip required platform builds
- Let workflows drift from the real repo structure

## CRITICAL - ALWAYS DO

- Use explicit build and test commands in workflows
- Build the full 8-platform matrix
- Keep GitHub, Gitea, and Jenkins automation aligned with the actual project

## Workflow Rules

- CI/CD should reflect the real binary names, paths, and packaging
- Release and validation steps must match the repository layout
- Avoid hidden magic; prefer explicit commands and artifacts

For complete details, see AI.md PART 28
