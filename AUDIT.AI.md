# Project Audit

Started: 2026-10-02T19:54:29-04:00

## Scope and evidence

This audit is a static review unless explicitly marked otherwise. The following verification was completed inside Docker with `CGO_ENABLED=0`:

- `go test ./...`: PASS
- `go vet ./...`: PASS

No host Go toolchain was used. Passing tests and vet do not eliminate the static-review findings below.

The four files changed during this audit window are listed separately from pre-existing working-tree changes:

### Audit-window files

- `src/server/csrf.go`
- `src/server/template/nojs/home.tmpl`
- `src/server/template/page/index.tmpl`
- `src/server/template/partial/public/header.tmpl`

The `csrf.go` change is intentional fail-closed behavior: `csrfGenToken` returns an empty string when the CSPRNG fails instead of emitting a weak fallback token. The middleware rejects protected requests with an empty token. Existing CSRF tests cover normal, default-length, and negative-length generation, but there is no forced-CSPRNG-failure test.

### Pre-existing working-tree changes

These tracked changes were present outside the four audit-window files:

- `LICENSE.md`
- `README.md`
- `TODO.AI.md`
- `src/common/i18n/locales/ar.json`
- `src/common/i18n/locales/de.json`
- `src/common/i18n/locales/es.json`
- `src/common/i18n/locales/fr.json`
- `src/common/i18n/locales/ja.json`
- `src/common/i18n/locales/zh.json`
- `src/config/config.go`
- `src/main.go`
- `src/server/handler/server.go`
- `src/server/service/logging/logging.go`
- `src/server/service/urlvar/urlvar.go`

Pre-existing untracked files were also observed: `AGENTS.md`, `src/server/service/logging/logging_security_cef_test.go`, and `src/server/service/urlvar/urlvar_trusted_proxy_test.go`.

## Findings ranked by severity

### HIGH

- [x] `src/server/service/email/email.go:423-457`, reached by the public security-report flow at `src/server/handler/server.go:693-694` and `:711-742`: `sendEmail` previously interpolated caller/config-derived values directly into RFC 5322 headers without rejecting CR/LF. Fixed with final `sanitizeEmailHeader` application to From, To, Reply-To, and Subject, covering direct `SendRaw` subjects as well as template substitutions. Verified with Dockerized `gofmt` and `go test ./src/server/service/email` (PASS).
- [x] `src/server/service/maintenance/maintenance.go` restore/archive handling (reported around lines 530, 887, and 960): archive member names previously received only prefix checks before `filepath.Clean`/`filepath.Join`, so crafted names such as `config/../../...` could escape the restore root and overwrite arbitrary files during an authorized restore. Fixed by adding `validateArchiveEntryName` (`:934`, rejects absolute paths, `..` traversal, and control characters) and `resolveRestoreTarget` (`:962`, containment check via `filepath.Rel` before every write). Both are wired into the live paths: `:1027` validates each tar header before extraction, and `:898` resolves each entry through `resolveRestoreTarget` before writing. Covered by `restore_traversal_test.go` (`TestValidateArchiveEntryName`, `TestResolveRestoreTarget`, `TestLoadRestoreArchiveRejectsTraversal`, `TestRestoreWithPasswordRejectsTraversal`). Verified with Dockerized `go build ./...`, `go vet ./...`, and `go test ./...` (all PASS).
- [x] `src/server/handler/handlers.go` age-verification and content-restricted redirect handlers (reported around lines 738, 746, 768, 909, 918, and 943): checking only `strings.HasPrefix(redirect, "/")` permitted protocol-relative or backslash-normalized external redirects such as `//evil.com` or `/\evil.com`. Fixed by routing every user-supplied `?redirect=`/`return_to` value through `safeLocalRedirect` (`:1350`), which rejects non-`/`-prefixed targets, the `//` and `/\` protocol-relative forms, and control characters that could truncate a `Location` header; it falls back to `/` rather than emitting an off-site redirect. A separate `safeReturnPath` (`:1372`) validates Referer-derived and same-origin form targets, rejecting cross-host and protocol-relative values and refusing to bounce back into the preferences endpoints themselves. Covered by `redirect_guard_test.go`. Verified with Dockerized `go test ./...` (PASS).

### MEDIUM

- [ ] `src/server/service/urlvar/urlvar.go:93-125`: every RFC1918/private, link-local, and loopback immediate peer is treated as a trusted proxy, not only configured reverse proxies. A client able to connect from a private network can spoof `X-Forwarded-*`/`X-Real-IP`, poisoning generated URLs and client-IP-dependent allowlists or rate limits. Severity is conditional on private-network exposure; trust only explicit proxy CIDRs or a narrowly configured proxy subnet. **Reclassified 2026-10-05 as spec-inherited, not a code defect:** the behavior is mandated by AI.md, which documents loopback/RFC1918/link-local as "always trusted (no config required)" (AI.md `:16022`, `:16000` "Additional IPs/CIDRs to trust (private ranges always trusted)", `:12484` naming the same canonical list). The implementation matches the spec, including the same-subnet-as-listen check and `server.trusted_proxies.additional`. Closing this requires a spec change (narrow the always-trusted set to explicit proxy CIDRs), not a code change, so it is deliberately left open as a spec/security decision rather than silently "fixed".
- [ ] `src/server/handler/server.go:1069-1089`: possible Host-header redirect poisoning where a redirect is constructed from request Host data. Reachability depends on routing/configuration and requires deployment-specific confirmation; validate Host against configured origins before redirecting. Host values used in logging were empirically checked safe against CR/LF injection; this finding concerns redirect construction only. Static reading only; not fixed in this audit.
- [ ] Debug endpoint at `src/server/debug.go:16-48` exposes request and diagnostic details when debug mode is enabled. Ensure it is operator-only and never reachable on an internet-facing deployment. Static reading only; not fixed in this audit.
- [ ] SMTP probe at `src/server/service/email/email.go:601` can initiate an outbound SMTP probe based on configured values. Restrict probe destinations and ensure diagnostics do not expose credentials or internal network details. Static reading only; not fixed in this audit.
- [ ] `src/server/handler/favorites.go` visitor identity: `visitor_id` is an unsigned, bearer cookie. A client can copy or set another visitor's identifier and read or mutate that visitor's favorites. Use a signed/encrypted or otherwise server-verifiable ownership token, or document and accept the intentional anonymous bearer model. Static reading only; not fixed in this audit.
- [x] `src/server/service/logging/logging.go:1227-1233`: the security fallback called `l.log` with output key `security`; because that output is absent by definition in the fallback branch, `l.log` returned early and silently dropped the security event. Fixed at `Security()` (`:1254-1268`): the fallback now emits to the always-configured `server` and `app` outputs with the masked remote address and sanitized detail fields, so the event reaches a real sink instead of being discarded. Covered by `logging_security_cef_test.go`. Verified with Dockerized `go test ./...` (PASS).
- [x] `src/server/service/logging/logging.go:1273-1274`: CEF header fields used raw `event` twice (signature and name), so pipe/control characters could corrupt the CEF header despite extension escaping. Fixed by adding `cefHeaderEscape` (`:485`), which escapes `\` as `\\` and `|` as `\|` and strips DEL/control characters, and applying it to both header occurrences at `:1308-1309`. Extension values remain escaped separately by `cefEscape`. Covered by `logging_security_cef_test.go` (`TestCEFEscapeNeutralizesPipes`, `TestCEFExtensionsUsesSanitizedValues`). Verified with Dockerized `go test ./...` (PASS).

### LOW

- [x] `src/client/cmd/root.go:489-497`: `--lang` set `VIDVEIL_LANG`, but no corresponding `Accept-Language` header was sent, making the option a confirmed no-op for client requests. Fixed end to end: `initAPIClient` (`root.go:981-993`) now calls `apiClient.SetLanguage(os.Getenv("VIDVEIL_LANG"))`, `SetLanguage` (`src/client/api/client.go:146-150`) validates through `normalizeAcceptLanguage` (`:155`, conservative BCP 47 subset, rejects CR/LF and other control characters), and the value is set on every request at `client.go:177`. Documented at `docs/cli.md:55` and `:74`. Verified with Dockerized `go test ./...` (PASS).
- [ ] `src/main.go:2697-2705` `isDBFirstRun` uses a bare `SELECT COUNT(*) FROM settings` without an explicit context/timeout at the call site. An unavailable or blocked database query can delay startup; use the database timeout-aware query path. **Re-verified 2026-10-05: still open.** The call site (`main.go:862`) passes `migrationMgr.GetDB()`, which returns the raw `*sql.DB` (`migrations.go:231`), so the query bypasses `AppDatabase.QueryRow`'s 5s timeout context (`database.go:349-355`) entirely. Fix requires threading a context with a timeout, or routing through the `AppDatabase` wrapper.
- [ ] `src/server/service/database/database.go:345-363` ignores the return value from `instrumentQuery`. The query result is later surfaced through `sql.Row`, but instrumentation errors are silently discarded, contrary to the project's error-handling rule. **Re-verified 2026-10-05: still open.** `QueryRowContext` (`:358-369`) discards it via `_ = instrumentQuery(...)`. Note the closure returns nil unconditionally, so the discarded value is always nil today; the finding stands as an error-handling/convention violation rather than a live lost-error path. `Exec` (`:285-295`) and `ExecContext` (`:298-306`) both propagate it correctly, so the fix is to make `QueryRowContext` consistent.
- [x] `src/server/handler/favorites.go:55-60` ignored `rand.Read` failure while generating `visitor_id`, which could have issued a predictable all-zero identifier. Fixed: `newVisitorID` (`favorites.go:65-71`) returns `""` on CSPRNG failure, matching `csrfGenToken`'s fail-closed behavior, and the cookie is not set for an empty identifier. Verified with Dockerized `go test ./src/server/handler` (PASS).
- [ ] Docker/CLI/logging convention findings remain from the audit review: verify all Docker build/run paths use the project's required CGO-disabled and reproducible flags; verify CLI help documents every implemented flag/subcommand; and ensure logging conventions do not emit sensitive request/configuration values. These are static convention findings and were not changed in this audit.
- [x] `src/config/env.go:34-47` honors `CACHE_URL`, but `docs/configuration.md` omitted it. Fixed: `docs/configuration.md:54` now documents the variable, including that it sets `server.cache.url`, promotes the cache type from `memory` to `valkey`, and is ignored when `VIDVEIL_SERVER_CACHE_URL` or `VIDVEIL_CACHE_URL` is set (the precedence enforced at `env.go:39-43`).
- [ ] Makefile lines 89 and 251-255 add an `i18n-validate` target even though AI.md PART 25 (lines 32350-32363) says the Makefile has six core targets and says not to add more. This conflicts with AI.md's later i18n validation requirement (AI.md lines 41114-41122 and TODO.AI.md's i18n-validation work). Treat as a LOW spec inconsistency/compliance finding: reconcile the authoritative target list and validation requirement rather than silently assuming either one wins.
- [ ] CLI help/documentation diverges from implementation for `--update`, accepted color values, and environment-variable aliases. **Partially resolved 2026-10-05.** `--lang` is no longer divergent: the flag is implemented end to end (`src/client/api/client.go:146`, `:177`) and documented at `docs/cli.md:55` and `:74`. Also verified as consistent: the accepted `--color` values (`always`, `never`, `auto` at `docs/cli.md:52`) and the environment-variable alias table with its canonical-first precedence rule (`docs/cli.md:64-76`). **Still open:** `--update [check|yes|branch {stable|beta|daily}]` is implemented (`src/main.go:347-351`, plus the `--maintenance update` alias at `:352-361`) and appears in `README.md:295`, but is absent from `docs/cli.md` entirely, and `--maintenance secret|token|data` remains unimplemented (see the separate deferred note in TODO.AI.md). Ranked LOW; documentation/completeness, not security.
- [ ] `src/main.go:1811`: spec-mandated maintenance secret/token/data commands are unimplemented. This is a deferred design issue because implementing the required behavior needs the missing business/security design; it is not treated as a silently fixed stub.
- [ ] Host handling is inconsistent across server paths: some code uses configured host/base URL while other paths use request Host. Normalize origin construction through one trusted configuration path. Static reading only; not fixed in this audit.
- [x] `VIDVEIL_PORT` and `VIDVEIL_LANG` were read configuration/environment variables omitted from the documented environment-variable configuration tables. Fixed: `docs/configuration.md:49` documents `VIDVEIL_PORT` (with the full port precedence chain) and `:62` documents `VIDVEIL_LANG` (including that invalid or empty values send no header); `docs/cli.md:74` documents `VIDVEIL_LANG` for the CLI.
- [ ] `src/server/server.go:201-208` permits wildcard CORS. Because credentials are not enabled, this is not a direct authentication bypass; retain as LOW hardening if sensitive read APIs are intended to be same-origin. Static reading only; not fixed in this audit.
- [ ] `src/server/csrf.go` bypasses CSRF checks for requests carrying `Authorization: Bearer` or `X-API-Token`. This is appropriate for token-authenticated APIs, but browser-session endpoints must not accept attacker-supplied token headers as an authentication substitute. Static reading only; not fixed in this audit.

## Audit subagent findings

All findings above originated from static source/configuration review. No subagent built or tested the project. The Docker test and vet results listed under Scope and evidence were run separately and verified successfully.

## Completed

- [x] `src/server/csrf.go`: removed the weak CSPRNG fallback and made failure fail closed.
- [x] `src/server/template/nojs/home.tmpl`: use the configured title.
- [x] `src/server/template/page/index.tmpl`: use the configured title.
- [x] `src/server/template/partial/public/header.tmpl`: use the localized application name fallback.
- [x] Docker `go test ./...` verification.
- [x] Docker `go vet ./...` verification.
