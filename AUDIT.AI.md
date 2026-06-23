# AUDIT.AI.md — Spec Compliance Audit (2026-06-23)

Directive: "If it's in the spec it must be implemented. If it's NOT in the spec it must be removed."
Spec source of truth: AI.md (READ-ONLY). Audited PARTs: 1, 8, 13, 14, 16, 17-22, 23, 24, 26, 27, 28, 29, 31, 32.

## VIOLATIONS TO FIX

### Missing (in spec, not implemented)

- [x] PART 20 (Metrics): `/metrics` is now wired to `promhttp.Handler()` from the default Prometheus registry.
  All promauto-registered labeled metrics are served: HTTP request/latency/size metrics via
  `InstrumentMiddleware`, rate-limit counters incremented in ratelimit middleware, app-info gauge set via
  `Init()` in `main.init()`. FIXED: commit b65fa38.

- [x] PART 20 (Metrics): `vidveil_rate_limit_hits_total{endpoint_class,ip}` and
  `vidveil_rate_limit_blocked_total{ip}` added to `src/server/service/metrics/metrics.go` and incremented
  in `ratelimit.Middleware()` when a request is blocked. FIXED: commit b65fa38.

- [ ] PART 26 (Docker): IDEA.md does not document the no-`USER` exception. The runtime stage of
  `docker/Dockerfile` has no non-root `USER` directive. vidveil binds port 80 and performs its own privilege
  drop (PART 23), so running as root in the container is intentional — but PART 26 requires this exception to
  be DOCUMENTED in IDEA.md. FIX: add a one-line note to IDEA.md (requires user confirmation before editing
  `## Project variables`; this is a new note, not a variable change). → IDEA.md

- [ ] CI multi-provider (AI.md:710-724, 1971-1977, 3036): spec lists `.gitea/workflows/{ci,release}.yml`,
  `.forgejo/workflows/{ci,release}.yml`, and `.gitlab-ci.yml` as multi-provider CI requirements. Repo has
  EMPTY `.gitea/` and `.forgejo/` dirs and NO `.gitlab-ci.yml`. NOTE: this conflicts with the project's own
  `.claude/rules/cicd-rules.md` which describes GitHub + Jenkins only. Flagged below under NOTES — resolve
  intent with user before populating or removing. → .gitea/, .forgejo/, .gitlab-ci.yml

### Extra (not in spec, must remove) — FIXED

- [x] `src/server/template/layout/admin.tmpl` — defines `{{define "admin"}}` layout; never parsed/executed
  anywhere in src/ (only self-reference). Orphan from the removed admin web UI. Spec explicitly states "there
  is no admin web UI" (AI.md:5276, 22733). FIXED: deleted.
- [x] `src/server/static/css/admin.css` — zero references in any .go/.tmpl; embedded via `static/css/*` glob
  but dead. Admin-UI orphan. FIXED: deleted.
- [x] `src/server/static/js/admin.js` — zero references in any .go/.tmpl; embedded via `static/js/*` glob but
  dead. Admin-UI orphan. FIXED: deleted.

### Wrong values — FIXED

- [x] PART 26 (Docker): `docker/Dockerfile` builder stage used `FROM golang:alpine AS builder`. PART 26 +
  project rules require the Go build image `casjaysdev/go:latest`. FIXED: changed FROM line.

- [x] PART 1 (Security): `src/server/handler/metrics.go` compared the metrics-endpoint auth token with `==`
  (timing-unsafe). PART 1 / backend rules require `crypto/subtle.ConstantTimeCompare`. FIXED by PART-1 audit
  pass (constant-time compare added).

## COMPLIANT (verified)

- PART 1: Argon2id for config/backup passwords; SHA-256 for API tokens; parameterized queries; CGO_ENABLED=0;
  no premium tiers; rate limits present in config. Token comparisons now constant-time. ✓
- PART 8: All 22 server CLI flags present in src/main.go (--help/-h, --version/-v, --status, --shell, --mode,
  --config, --data, --cache, --log, --backup, --pid, --address, --port, --baseurl, --daemon, --debug, --color,
  --lang, --service, --update, --maintenance). ✓
- PART 13: /server/healthz (+.json/.txt), root /healthz gated on config, /api/healthz alias,
  /api/v1/server/healthz, --status exits 0/1, /api/v1/version. ✓
- PART 14: all API routes under /api/v1; unversioned aliases served directly (not redirected); content
  negotiation (JSON/text-plain/.txt) in handler/response.go; custom 404; /metrics route present. ✓
- PART 16: server-side Go html/template only (no React/Vue/Angular/Svelte); dark mode default +
  prefers-color-scheme auto; CSS custom properties; no-JS fallback templates (template/nojs/*). ✓
- PART 17: Email — stdlib net/smtp, SMTP autodetect w/ EHLO, SMTP_* env overrides, TLS modes, embedded +
  custom + default templates. ✓
- PART 18: Scheduler — internal robfig/cron/v3, DB persistence, catch-up, history, all builtin tasks with
  canonical IDs + legacy migration. ✓
- PART 19: GeoIP — maxminddb-golang, ip-location-db CDN, ASN/country/city, allow/deny modes, allowlist +
  RFC1918 + Tor-exit handling, content restriction warn/soft/hard block + ack cookie. (Hard block returns 403;
  AI.md does not mandate 451 — not a violation.) ✓
- PART 21: Backup/Restore — tar.gz of config/data/ssl + manifest, AES-256-GCM + Argon2id key derivation,
  hourly + timestamped names, wired via --maintenance backup/restore + scheduler. ✓
- PART 22: Update — --update [check|yes|branch <stable|beta|daily>] + --maintenance update alias, GitHub
  releases + checksum verify. ✓
- PART 23/24: Service — src/server/service/system/service.go; detection systemd→OpenRC→SysVinit→runit;
  install for systemd/runit/openrc/sysvinit/darwin/bsd; findAvailableUID(200,899); privilege drop after bind. ✓
- PART 26: Dockerfile not in root; multi-stage; tini ENTRYPOINT; STOPSIGNAL SIGRTMIN+3; EXPOSE 80; HEALTHCHECK
  via --status; OCI annotations (no LABEL block); rootfs/entrypoint.sh traps signals + exec. (FROM + USER-doc
  fixes above.) ✓
- PART 27: GitHub ci.yml/release.yml/docker.yml/daily.yml — all third-party actions pinned to full commit SHA;
  truffleHog (not gitleaks); concurrency cancel-in-progress. Jenkinsfile present. ✓
- PART 28: tests/run_tests.sh, tests/docker.sh, tests/incus.sh all present. ✓
- PART 29: mkdocs.yml + .readthedocs.yaml at root; populated docs/. ✓
- PART 31: Tor — src/server/service/tor/service.go, cretz/bine, exec.LookPath("tor") autodetect, ADD_ONion,
  HiddenServiceVersion 3, default ports never used, binary owns all config. ✓
- PART 32: src/client/ with cmd/, tui/, api/, browser/, paths/; BinaryName "vidveil-cli"; bubbletea+lipgloss;
  auto-detect TUI/CLI mode; no forbidden UI-mode flags. ✓

## NOTES

- AI.md vs project CLAUDE.md conflict (FLAG — do not auto-resolve): AI.md PART 27 text mandates
  `docker/Dockerfile.build`, a `:build` toolchain image, an `ensure-build-image` gate, and
  `build-toolchain.yml`. The project's `.claude/rules/cicd-rules.md` and `.claude/rules/project-rules.md`
  EXPLICITLY FORBID these for Go projects ("use casjaysdev/go:latest directly; never create build-toolchain.yml
  or docker/Dockerfile.build for Go projects"). The repo follows the CLAUDE.md rule. This contradiction must be
  resolved by the project owner, not auto-fixed either direction. It also bears on whether the multi-provider
  CI dirs (.gitea/.forgejo/.gitlab-ci.yml) should be populated.

- Engine registry lives at `src/server/service/engine/engine.go` (IDEA.md references `src/server/engine/`);
  path differs but is consistent with Go service layout — not a violation.

- The orphaned `src/server/service/metrics` package is NOT recommended for deletion — wiring it into /metrics
  is the correct fix for the PART 20 Missing findings above.
