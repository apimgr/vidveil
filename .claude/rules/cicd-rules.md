# CI/CD Rules (PART 28)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: CI/CD Workflows

## CRITICAL - NEVER DO

- Use the Makefile in CI - explicit commands with all env vars
- Float third-party action versions on @main/@master/broad tags - pin to full SHA
- Use pull_request_target for untrusted code execution paths
- Expose secrets to fork PR workflows
- Fake signatures/attestations or claim signed when keys are unavailable

## CRITICAL - ALWAYS DO

- .github/workflows/{build,release,security}.yml at minimum
- Mirror gates on .gitea/workflows or Jenkinsfile when those targets exist
- Default workflow permissions read-only/least-privilege; write only on the specific release/publish job
- .github/dependabot.yml covering Go modules, Actions, Docker
- Secret scanning on push/PR - findings are blockers
- Releases publish SHA-256 checksums, SBOM, release notes, provenance/attestation when supported
- Default branch protected; PR + checks + CODEOWNERS required

---
For complete details, see AI.md PART 28
