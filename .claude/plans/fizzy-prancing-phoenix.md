# Context

The audit identified open security, reliability, and AI.md compliance findings. The user requested that every actionable finding be fixed while keeping implementation aligned with the authoritative AI.md; already-resolved findings must remain documented accurately.

# Plan

1. Read the exact AI.md sections governing backup restore validation, trusted proxies/client IPs, redirects/origin construction, debug endpoints, SMTP diagnostics, anonymous favorites, logging/CEF, database timeouts/instrumentation/CSPRNG, CLI behavior, maintenance commands, environment documentation, and Makefile target rules. Reconcile any conflicting AI.md/TODO requirements before changing behavior.
2. Harden backup restore in `src/server/service/maintenance/maintenance.go`: validate every relevant tar member before staging and again before writing; reject absolute paths, traversal components, backslash traversal, unsafe symlink/special entries, and any target whose `filepath.Rel` escapes its designated Config/Data/SSL root. Add regression tests with malicious archives and verify checksum/phase-1 semantics remain intact.
3. Use the existing redirect helpers in `src/server/handler/handlers.go` for every age/content restricted return path, including protocol-relative, backslash-normalized, control-character, and malformed targets. Audit Host-derived redirects in `src/server/handler/server.go` and route them through the project’s trusted origin/URL utility rather than request Host where required. Extend handler tests.
4. Correct trusted-proxy behavior in `src/server/service/urlvar/urlvar.go` according to AI.md: retain explicit configured proxy CIDRs and only retain listen-subnet convenience behavior if the spec explicitly requires an opt-in/private-only mode. Update tests for spoofed private peers, explicit proxies, loopback, IPv4, and IPv6.
5. Protect debug routes in `src/server/debug.go` as required by AI.md (debug-mode gate plus operator-only/loopback or configured authorization), without breaking documented local debug use. Harden SMTP diagnostic probing in `src/server/service/email/email.go` against unintended destinations and certificate/MITM behavior while preserving configured diagnostics.
6. Resolve favorites visitor ownership in `src/server/handler/favorites.go` using the project’s existing secret/cookie primitives or document the deliberately bearer-based anonymous model only if AI.md explicitly mandates it; handle CSPRNG failures and add tests. Fix logging security fallback to a guaranteed sink and sanitize all CEF header fields, including event/version; add regression tests.
7. Address reliability findings: use timeout/context-aware database queries for first-run checks, preserve/return instrumentation errors where AI.md requires it, and confirm fail-closed CSPRNG behavior. Add focused tests without changing public semantics unnecessarily.
8. Resolve CLI/documentation/spec findings against AI.md: make `--lang` functional or remove the no-op behavior, document `CACHE_URL`, `VIDVEIL_PORT`, `VIDVEIL_LANG`, flags, aliases, and maintenance commands, and reconcile the six-target Makefile rule with the separately required i18n validation (prefer the exact AI.md-authorized target arrangement). Do not invent unimplemented maintenance security behavior; implement it only where AI.md supplies complete rules.
9. Run formatting and project-prescribed Docker-based verification, including `make test`/required target checks, Go tests, vet/build, i18n validation, and targeted security regressions. Review the final diff against AI.md’s compliance checklist and update `AUDIT.AI.md` checkboxes with precise resolved/deferred status and test evidence.

# Critical files

- `src/server/service/maintenance/maintenance.go` and maintenance tests
- `src/server/handler/handlers.go`, `src/server/handler/server.go`, and redirect tests
- `src/server/service/urlvar/urlvar.go` and trusted-proxy tests
- `src/server/debug.go`
- `src/server/service/email/email.go`
- `src/server/handler/favorites.go`
- `src/server/service/logging/logging.go` and logging tests
- `src/main.go`, `src/server/service/database/database.go`
- `src/client/cmd/root.go`
- `Makefile`, `docs/configuration.md`, README/help/completion documentation
- `AUDIT.AI.md`

# Verification

Use only the project-prescribed Docker/Makefile workflows. Run `gofmt`, targeted tests for each changed package, full `make test`, `go vet`/build through Docker, i18n validation, and `git diff --check`. Exercise malicious restore archives, unsafe redirects, spoofed proxy headers, debug access, visitor-cookie ownership, CEF output, email CRLF handling, and CSPRNG/error paths. Confirm no unresolved actionable audit checkbox remains unless AI.md explicitly requires deferral or the finding is disproven, and document any such exception in `AUDIT.AI.md`.
