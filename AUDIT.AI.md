# Project Audit

Started: 2026-06-29T11:00:00Z

## Summary

Comprehensive audit of vidveil against AI.md (44k-line spec). 6 parallel agents audited CLI flags, API endpoints, CI/CD, security, client binary, and Docker. Main coordinator checked remaining areas.

---

## Pass 1: Security (PART 11)

- [x] SHA-256 for API tokens: `src/server/service/auth/tokens.go:82` uses `sha256.Sum256()` - COMPLIANT
- [x] Argon2id for backup encryption: `src/server/service/maintenance/maintenance.go:237` uses `argon2.IDKey()` - COMPLIANT
- [x] No bcrypt usage: grep found no bcrypt imports - COMPLIANT
- [x] Constant-time comparison: `src/server/handler/metrics.go:265` uses `subtle.ConstantTimeCompare` for the only security-sensitive token comparison. VidVeil is stateless (no user accounts), so auth surface is minimal.

---

## Pass 2: Code Quality

- [x] No strconv.ParseBool usage (uses config.ParseBool per spec)
- [x] Error wrapping with fmt.Errorf and %w pattern used
- [x] Context usage correct (Background for top-level, WithTimeout for shutdown)

---

## Pass 3: Logic (CLI Flags - PART 8)

- [x] All required CLI flags implemented: --help, --version, --status, --config, --port, --mode, --service, --maintenance, --update, --shell, --debug, --color, --lang
- [x] NO_COLOR support implemented per spec
- [x] Binary name detection via `filepath.Base(os.Args[0])` correctly implemented
- [x] No hardcoded "Vidveil" strings in user-facing messages (imports/comments are acceptable)

---

## Pass 4: Documentation (PART 29, 30)

- [x] mkdocs.yml exists at project root
- [x] .readthedocs.yaml exists at project root
- [x] docs/ directory with Markdown files exists
- [x] i18n implemented: 7 locales (ar, de, en, es, fr, ja, zh) - spec requires 7, all present
- [x] RTL support for Arabic/Hebrew implemented in i18n.go
- [x] WCAG 2.1 AA accessibility: aria-labels, role attributes, semantic HTML present in templates

---

## Pass 5: Spec Compliance

### Docker (PART 26) - COMPLIANT
- [x] Multi-stage build (builder → alpine runtime)
- [x] tini as init process
- [x] STOPSIGNAL SIGRTMIN+3
- [x] Health check via --status
- [x] No LABEL blocks (OCI annotations used)
- [x] Port 80 internal
- [x] CGO_ENABLED=0

### CI/CD (PART 27-28) - COMPLIANT
- [x] All GitHub Actions pinned to full SHA
- [x] No Makefile in CI
- [x] truffleHog for secret scanning (not gitleaks)
- [x] No build-toolchain.yml (correct for Go)
- [x] release.yml builds all 8 platforms
- [x] casjaysdev/go:latest container used

### API Endpoints (PART 13-14) - COMPLIANT
- [x] /health and /healthz endpoints
- [x] /version endpoint
- [x] /api/v1/ versioned prefix
- [x] Content negotiation (Accept headers)
- [x] .txt extension support
- [x] Health response matches spec structure

### Built-in Features (PART 17-22)
- [x] Scheduler implemented (src/server/service/scheduler/)
- [x] GeoIP implemented (src/server/service/geoip/)
- [x] Metrics implemented (src/server/service/metrics/)
- [x] Email implemented (src/server/service/email/)
- [x] Backup/Restore implemented (src/server/service/maintenance/)
- [x] Tor hidden service implemented (src/server/service/tor/)
- [x] SSL/Let's Encrypt implemented (src/server/service/ssl/)

### Frontend (PART 16) - COMPLIANT
- [x] Server-side Go templates (html/template)
- [x] All assets embedded in binary (//go:embed)
- [x] Mobile-first responsive CSS
- [x] Dark mode default with dark/light/auto via prefers-color-scheme
- [x] CSS custom properties for theming
- [x] word-break CSS for long strings

---

## Pass 6: Code Flow Trace

### Client Binary (PART 32) - VIOLATIONS FOUND

- [x] vidveil-cli binary exists (src/client/main.go)
- [x] TUI mode implemented (bubbletea)
- [x] CLI mode implemented
- [x] Browser launch mode (src/client/browser/open.go)
- [ ] **VIOLATION: Missing native GUI mode** - Spec requires "Full GUI app (GTK/Cocoa/Win32)" for DisplayModeGUI. Only TUI is implemented. This is a MAJOR missing feature requiring significant effort.
- [x] --update flag IS wired (line 481-495 in root.go) - COMPLIANT
- [x] Browser launch via 'o' key in TUI mode (tui.go:274-282) - COMPLIANT (spec doesn't require --browser flag)

---

## Violations Summary (Priority Order)

### CRITICAL (Requires User Decision)

1. **PART 32: Client native GUI mode missing**
   - Location: src/client/
   - Issue: AI.md PART 32 requires "Full GUI app (GTK/Cocoa/Win32)" for DisplayModeGUI. Only bubbletea TUI is implemented.
   - Impact: Client doesn't provide native GUI for desktop environments without a terminal
   - Effort: MAJOR - requires fyne or similar cross-platform GUI toolkit, significant new code
   - Options:
     a) Implement fyne-based native GUI (weeks of work, adds build complexity)
     b) Add exception to IDEA.md `### Security decisions & exceptions` documenting that VidVeil intentionally provides TUI-only client (search tool, not productivity app)
     c) For VidVeil's use case (adult video search), browser-based web UI is primary interface; TUI client is power-user tool
   - **ASK USER: Should VidVeil implement native GUI per spec, or document a IDEA.md exception?**

### RESOLVED (Verified as Compliant)

2. **PART 32: Client --update** - COMPLIANT
   - Verified: `--update` wired at root.go:481-495
   
3. **PART 32: Browser launch** - COMPLIANT
   - Verified: Browser launch via 'o' key in TUI (tui.go:274-282)
   - Spec doesn't require --browser CLI flag

### LOW (Verified)

4. **PART 11: Constant-time comparison** - COMPLIANT
   - Location: src/server/handler/metrics.go:265
   - Verified: The only security-sensitive token comparison (metrics endpoint) uses `subtle.ConstantTimeCompare`
   - Note: VidVeil is stateless (no user accounts per PART 34/35), so auth surface is minimal. Server token is only used for metrics protection.

---

## Completed (No Action Needed)

- Docker setup fully compliant with PART 26
- CI/CD workflows fully compliant with PART 27-28
- API endpoints fully compliant with PART 13-14
- Built-in features (scheduler, GeoIP, metrics, email, backup, Tor, SSL) all implemented
- Frontend fully compliant with PART 16
- i18n/a11y fully compliant with PART 30
- CLI flags (server binary) fully compliant with PART 8
- Security (Argon2id, SHA-256) mostly compliant with PART 11
