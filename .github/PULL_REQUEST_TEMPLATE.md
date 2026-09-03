<!--
Thanks for contributing! Please fill in every section. Empty sections will
block review. The required fields come from AI.md PART 28.
-->

## Summary

<!-- One or two sentences: what does this change do? -->

## Why

<!-- The problem this solves, the user it helps, or the spec PART it implements.
     Link to AI.md PART numbers / IDEA.md sections / GitHub issues. -->

## Test Evidence

<!-- How did you verify this change? Per AI.md PART 0 "Self-Validation Loop", a
     change is not done until verified with the right tool. List what you ran:
     `make test`, `make build`, integration tests, manual curl, browser smoke
     tests, etc. Paste relevant output snippets. -->

## Documentation Updates

<!-- Confirm one of the following per AI.md PART 30:

- [ ] No user/admin/operator/integration-facing change — docs untouched.
- [ ] Updated `docs/*.md` (ReadTheDocs) sections that this change affects.
- [ ] Updated `README.md` (features, configuration, API examples).
- [ ] Updated OpenAPI annotations / GraphQL schema for API changes.
- [ ] Updated `IDEA.md` (business logic, data model, scope).
-->

## Breaking Change

<!-- Does this change break existing config, CLI flags, API responses, or
     stored data? If yes, describe the migration. If no, write "None". -->

## Security & Privacy Impact

<!-- Anything that touches authentication, authorization, sessions, secrets,
     CSRF/CSP/CORS, validation, untrusted-content rendering, rate limiting,
     external integrations, or logged fields. AI.md PART 11 / PART 16 apply.
     If none, write "None". -->

## Checklist

- [ ] I have read the relevant PART(s) of `AI.md`.
- [ ] No `TODO`, `FIXME`, `XXX`, or stub functions left in the diff.
- [ ] No placeholder text ("Your app name", "Feature 1", `@yourname`).
- [ ] No AI / vendor attribution in code, comments, commits, or this PR.
- [ ] Tests added or updated and passing locally.
- [ ] `gofmt` and `go vet` are clean.
- [ ] Commit messages are signed via the project's `gitcommit` workflow.
