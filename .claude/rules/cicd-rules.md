# CI/CD Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Use Makefile targets in workflows or weaken a provider’s tests, coverage, security, build, or release gates.
- Use floating third-party action tags, expose secrets to fork code, or grant workflow-wide write permissions.
- Publish unexpanded placeholders, unverified releases, or signatures/attestations that were not produced.
- Skip checking the real provider run after pushing.

## CRITICAL - ALWAYS DO
- Maintain equivalent GitHub, GitLab, Gitea, Forgejo, and Jenkins workflows using the prescribed provider syntax.
- Use the maintained `casjaysdev/go:latest` toolchain for Go jobs; never install tools inline in CI.
- Pin actions to full SHAs; use least-privilege permissions, secret scanning, workflow-policy checks, vulnerability scans, and image scans.
- Build the required platform matrix; keep release, beta, devel, and Docker tags/metadata aligned with PART 27.
- Verify real CI after every push and fix failed runs with a follow-up commit.

## KEY RULES
- CI version sources, LDFLAGS, triggers, and generated text expansion are provider-specific; use the exact PART 27 contract.
- Publish checksums, SBOM, release notes, and attestations where supported.

For complete details, see AI.md PART 27.
