# TODO.AI.md

## Open Items

### 1. `/search?q=...` intermittently fails with `net::ERR_FAILED` in browser — RESOLVED
Root-caused via live `curl` testing against `https://x.scour.li/search?q=Test`
with a mobile Chrome User-Agent: `AgeVerifyMiddleware` and
`ContentRestrictionMiddleware` (src/server/handler/handlers.go) built the
redirect `Location` header by raw string-concatenating the original
path+query (e.g. `/search?q=Test`) directly as the `redirect` query
parameter's value, producing a malformed URL with an ambiguous nested query
string. Chrome's strict URI parser rejects this outright as
`net::ERR_FAILED`; lenient clients like `curl` (without a query string of
their own) still followed it, which is why earlier reproduction attempts in
this file didn't catch it. Confirmed via `git blame`/`git log -L` that this
defect has existed since the project's initial commit (2025-12-15) — not a
regression from any recent session's changes. Fixed both call sites with
`url.QueryEscape()`; the consuming side (`r.URL.Query().Get("redirect")`)
already auto-decodes, and the existing open-redirect guard
(`strings.HasPrefix(redirect, "/")`) is unaffected. Verified via full
build+vet+test+go-lint pass; committed+pushed as `951a480b3ae7`; CI (CI,
Daily Build, Docker Build) verified green for this commit.

Separately, the user flagged the live healthz response as spec-noncompliant.
Audited `TorInfo` (handlers.go) against AI.md PART 13's canonical struct and
found `Hostname` was tagged `json:"hostname,omitempty"` instead of the spec's
required `json:"hostname"` — causing the `hostname` key to silently vanish
from live JSON instead of serializing as `""`. Fixed, verified, committed+
pushed as `d97f831ee38a`; CI verified green for this commit too.

**Still open, NOT fixable from this repo** — see item 7 below: the live
production server was observed reporting `"mode": "development"` in its
healthz JSON with debug mode also enabled (`/debug/*` endpoints reachable),
even though this repo's own `docker/docker-compose.yml` (production) already
sets `MODE=production` with no `DEBUG` override. The discrepancy must be in
the actual deployment/CD configuration outside this repo's visibility.

### 8. `/server/security` coordinated-disclosure feature (AI.md PART 11) — IMPLEMENTED
Built per AI.md PART 11 "Security Reports — Coordinated Disclosure Pipeline":
- `/server/contact?security_id={id}` mode switch — validates the rotating
  48h HMAC id server-side, renders the security-mode form fields (component,
  endpoint, severity, summary, steps, impact, suggested fix, CVE requested,
  disclosure window, credit preference, GPG key URL/paste, agreement
  checkbox) instead of the standard contact form.
- `handleSecurityReportSubmit`/`apiSecurityReportSubmit` (server.go) — both
  HTML-form and JSON-API submission paths: allocates `sec_`-prefixed
  tracking id, encrypts the report body at rest (PGP if a project keypair
  exists, else AES-256-GCM via `server.security.encryption_key`), sends a
  PGP-encrypted maintainer notification and a PGP-encrypted (or plaintext
  fallback) researcher acknowledgment containing the one-shot status URL,
  logs `security.report_received` with no PII/vuln content.
- `/server/security`, `/server/security/policy`, `/server/security/thanks`,
  `/server/security/report/{tracking_id}` — all four public pages built with
  templates and handlers.
- New `secreport` package: `securityid.go` (rotating id gen/validate),
  `crypto.go` (PGP/AES-256-GCM at-rest encryption), `researcher.go`
  (SSRF-safe researcher GPG key resolution via an explicit keyserver
  allowlist), `store.go` (`security_reports` CRUD/status lookup).
- Test coverage added for the whole `secreport` package (0.0% -> 70.4%):
  `securityid_test.go`, `crypto_test.go`, `researcher_test.go`,
  `store_test.go`. Fixed a real bug caught by the new tests: `GetReportStatus`
  scanned the nullable `maintainer_comments` column into a plain `string`,
  which panicked with "converting NULL to string is unsupported" on any
  report with no maintainer comments yet — fixed to `sql.NullString`.
- Two automated background security-review findings addressed: SSRF in the
  researcher-key URL fetch (fixed via hostname allowlist + redirect
  re-validation) and one-shot token exposure via URL query string (mitigated
  with `Referrer-Policy: no-referrer` on the status page).
- Full project `go build`, `go vet`, `gofmt -l` (session-touched files),
  `go test ./src/... -cover` (all packages ok, zero FAIL), and `go-lint`
  agent pass all clean for this feature's files (one pre-existing,
  out-of-scope `go-lint` finding in `email.go` logged separately below).
- `report_token_expires_at` (dead/unused DB column) removed from the
  `security_reports` DDL — resolved, not deferred. The `--maintenance pgp`
  CLI dispatcher was independently confirmed already fully implemented in
  `main.go` (a prior grep in this session had incorrectly claimed it was
  missing; corrected).
- Beta-tester agent pass (static trace, Docker build did not finish in
  reasonable time so no live HTTP verification was performed) found one
  confirmed spec deviation: the "Affected component" field (AI.md PART 11)
  is spec'd as a dropdown populated from IDEA.md features with an "Other"
  free-text fallback; `server-contact.tmpl` implemented it as a plain
  free-text `<input>` only. FIXED — `server-contact.tmpl`'s component field is
  now a `<select>` populated from a new `securityComponentOptions()` helper
  (VidVeil-specific list drawn from IDEA.md's actual in-scope feature set,
  since VidVeil has no auth/user-accounts to match AI.md's generic example),
  plus an "Other" option paired with an always-visible `component_other`
  free-text input (no-JS-required — not toggled by script). New
  `resolveSecurityComponent()` helper in `server.go` resolves the final value
  server-side (falls back to `component_other` when the dropdown is "other"),
  used by both `handleSecurityReportSubmit` and `apiSecurityReportSubmit`.
  Verified via Docker `go build`/`go vet`/`gofmt -l` (clean) and
  `go test ./src/server/handler/... ./src/server/service/secreport/... -cover`
  (handler 82.1%, secreport 70.4%, zero FAIL).
- Handler-level tests for `handleSecurityReportSubmit`, `apiSecurityReportSubmit`,
  and the security page handlers — added in new
  `server_security_test.go` (real tempdir-sqlite DB via
  `database.SchemaManager`, real `secrets.Manager`, real security_id):
  covers the security-mode `ContactPage` GET rendering the component dropdown,
  successful submission with a dropdown value, the "Other" free-text fallback
  for both the HTML form path and the JSON API path (asserted by querying the
  stored `security_reports.component` row), rejection when "Other" is selected
  with no free text, and `SecurityPage`/`SecurityPolicyPage`/
  `SecurityThanksPage`/`SecurityReportStatusPage` (404 unknown id, 200 valid
  id/token). `src/server/handler` package coverage 82.1% -> 86.6%. Verified
  via Docker `go build`, `go vet`, `gofmt -l` (clean), and
  `go test ./src/server/handler/... ./src/server/service/secreport/... -cover`
  (handler 86.6%, secreport 70.4%, zero FAIL).

### 9. Generic PART 16 privacy config-tree (`server.privacy.*`) intentionally not implemented — verify no gap
AI.md PART 16's `/server/privacy` spec is the generic template model driven by a
`server.privacy.*` config tree (data.sold, CCPA opt-out, retention, third_party
services, GetDataUsageContent/GetConsentMessage helpers). VidVeil has NO such config
tree and IDEA.md non-goals explicitly make it inapplicable: no user accounts, stateless,
nothing persisted per user, nothing sold. The `/server/privacy` page and API were made
accurate to VidVeil's actual reality instead of fabricating that config. No code gap —
this is a deliberate, IDEA.md-documented deviation. Logged only so a future reader
doesn't "fix" the privacy page back toward the generic CCPA/data.sold model.

### 10. `email.go` `getTemplate()` reads operator-custom templates via `os.ReadFile()` at runtime — flagged by go-lint, needs a decision
`go-lint` agent run (during the PART 11 coordinated-disclosure feature
verification) flagged `src/server/service/email/email.go` line 248:
`getTemplate()` calls `os.ReadFile(customPath)` at send-time to let an
operator override a built-in email template without a rebuild, falling back
to the `go:embed`-embedded template and then a Go-literal default. This is
pre-existing code (not touched by this session's diff — confirmed via
`git diff --stat` showing this session's `email.go` change was purely
46 additive lines) and is a deliberate operator-customization mechanism, not
an oversight — but it does technically violate the "assets embedded at
build time" convention. Needs a decision: (a) keep as-is (documented
exception — operator template overrides are inherently a runtime feature),
(b) cache the custom-template read at startup/init instead of per-send, or
(c) remove the runtime customization path entirely. Not yet decided/fixed —
flagging so it isn't lost after compaction.

### 7. Live deployment intentionally running MODE=development/DEBUG=true — not an issue
`https://x.scour.li` was observed reporting `mode: development` with
`/debug/*` reachable. **User confirmed this is intentional** — set deliberately
to debug the app while it was broken (see item 1), not a misconfiguration.
No action needed here; this repo's own `docker/docker-compose.yml`
(production) still correctly defaults to `MODE=production`/no `DEBUG` for
normal deploys, which is unaffected by the user's deliberate live override.
