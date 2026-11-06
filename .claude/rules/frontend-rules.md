# Frontend Rules (PART 16)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

<!-- Rule summary checked against AI.md on 2026-11-06; AI.md remains authoritative. -->

## CRITICAL - NEVER DO

- Never build API-only — a full-featured browser frontend is mandatory alongside JSON API and CLI
- Never break core functionality without JS — nav, forms, content display, and CRUD MUST work with JS disabled; JS enhances only, never enables
- Never use inline `onclick`/inline event handlers — bind via `data-action` attributes in `app.js` (CSP blocks inline handlers)
- Never use inline CSS or inline `<script>` blocks — external `/static/css/`, `/static/js/` files only
- Never use a JS framework (React/Vue/etc.) — pure vanilla JS only; HTML5 first, CSS second, JavaScript last resort
- Never add JavaScript for anything HTML5+CSS already does (form submit, field validation, show/hide via `<details>`/`:target`, `<dialog>` modals, radio+`:checked` tabs, styling/animation, navigation, date/number formatting) — JS is a necessity-gated LAST RESORT; every `<script>` must name a capability impossible in HTML5+CSS, else delete it; the default answer to "add JS?" is NO
- Never let a service worker `respondWith()` promise resolve to `undefined` or reject — the browser renders `net::ERR_FAILED` (dead tab); every branch must end in a guaranteed real `Response` (synthesized 503 offline page for navigations, 504 for subresources)
- Never intercept non-GET, cross-origin, or `/api/*` requests in the service worker — fall through untouched, never call `respondWith()` for them
- Never use `alert()`/`confirm()`/`prompt()` — use custom modals/toasts
- Never write desktop-first CSS (`max-width` media queries) — mobile-first only (`min-width`)
- Never hardcode colors in CSS — use the `:root` CSS custom properties (theme variables)
- Never let content author custom footer/branding HTML render unsanitized — sanitize via `bluemonday` (strict allowlist, no `<script>`, no event handlers, no `style` attr, no `javascript:` URLs)
- Never skip the CSRF token on a mutating browser request that lacks a Bearer/API-token header — no Origin-based bypass exists, ever
- Never allow a `GET`/`HEAD`/`OPTIONS` request to mutate state — CSRF assumes these are side-effect-free
- Never invent alternate reserved-name slugs for vanity URLs — check the reserved list (see below) before allowing registration
- Never use `SameSite=Lax` (or unset) for the `csrf_token` cookie — must be `SameSite=Strict`
- Never send `Access-Control-Allow-Headers: *` or allow credentials with a wildcard origin — list every supported auth header by name, credentials only with an explicit origin
- Never build a PWA without `manifest.json` + service worker + HTTPS — all three required for install criteria
- Never let a single-submit form button be clickable twice — disable + show loading text after first submit
- Never let a `<dialog>` modal skip focus trap, `aria-modal`, `role="dialog"`, or Escape/backdrop-click close
- Never use generic placeholder text on `/server/about` or `/server/help` — content MUST come from `IDEA.md`

## CRITICAL - ALWAYS DO

- Always pair every API route `/api/{api_version}/{resource}` with a frontend route `/{resource}` (HTML)
- Always support three response modes: HTML forms (with `_method` hidden field for PUT/PATCH/DELETE), JSON API, and text/form-encoded CLI requests — same CRUD operation, three channels
- Always detect client type via `detectClientType()`: `Accept` header first, then User-Agent (browser list vs CLI-tool list), default `text` for empty/unknown UA, `html` as the safest generic fallback
- Always use Go `html/template` for HTML rendering — never string-concatenate HTML
- Always layer in order HTML5 → CSS → (only then) a thin JS enhancement over an already-working path; legitimate JS names a concrete impossible-without-it capability (live search-as-you-type `fetch`, canvas charts, drag-and-drop reorder, WebSocket streams, clipboard copy)
- Always terminate every request in a rendered response — panic/`recover` middleware and template-render failures must fall back to a minimal hardcoded error response (correct status, short body, content-negotiated HTML/JSON), never a blank body, dropped connection, or leaked stack trace (backend mirror of the SW guaranteed-`Response` rule)
- Always apply mobile-first responsive CSS: base = mobile, `@media (min-width: 768px)` tablet+, `@media (min-width: 1024px)` desktop+, optional `1280px` large desktop
- Always meet WCAG 2.1 AA: keyboard nav, visible focus rings, ARIA labels, 4.5:1 contrast (3:1 large text), alt text, `<label>` on every input, `aria-live` error announcements, skip-to-content link, proper heading hierarchy, `prefers-reduced-motion` support
- Always persist theme preference in a `theme` cookie (server-readable, renders class on `<html>`); default to dark when unset
- Always keep both light and dark themes readable with WCAG AA contrast — theme applies project-wide (web, Swagger, GraphiQL, CLI, TUI, GUI)
- Always double-submit CSRF: `csrf_token` cookie (`SameSite=Strict`, `Secure`, NOT `HttpOnly`) must match the `X-CSRF-Token` header or `csrf_token` form field on every mutating request without a Bearer header; compare in constant time
- Always sanitize any operator/user-supplied HTML (footer custom branding, markdown-rendered content) through a strict allowlist sanitizer before rendering
- Always source `/server/about`, `/server/help`, and other standard-page content from `IDEA.md` — never invent generic copy
- Always prefer `Accept: text/plain` responses over HTML parsing in frontend test scripts — HTML parsing is fragile

## Key Rules Summary

### Frontend Route Structure

Every API resource gets a matching frontend page: `/api/{api_version}/{resource}` (JSON) ↔ `/{resource}` (HTML). `/server/` routes mirror this same pairing (see Standard Pages below). Build the no-JS server-rendered form → POST → redirect path FIRST and verify it works with JavaScript disabled; JS may only enhance what already works.

### Middleware Execution Order (10 steps)

`URLNormalize → RequestID → PathSecurity → SecurityHeaders → Allowlist → Blocklist → RateLimit → GeoIP → Auth → Logging`

URL normalization strips trailing slashes (except `/` and file-like paths with a `.` in the last segment) and 301-redirects to canonical form.

### Vanity URLs (optional)

Reserved names blocked from slug registration: `api, server, static, assets, healthz, metrics, webhook, webhooks, search, explore, discover, trending, help, support, docs, documentation, about, contact, terms, privacy, legal, security, graphql, swagger, rest, rpc, ws, websocket, cdn, media, uploads, files, images, .well-known, robots.txt, sitemap.xml, favicon.ico`.

### Mobile-First Breakpoints

| Breakpoint | Query |
|------------|-------|
| Mobile (base) | no media query |
| Tablet+ | `@media (min-width: 768px)` |
| Desktop+ | `@media (min-width: 1024px)` |
| Large desktop (optional) | `@media (min-width: 1280px)` |

Layout width: ≥768px = 90% (5% margins); <768px = 98% (1% margins). Footer always centered, always at bottom.

### Theme System (dark default, NON-NEGOTIABLE project-wide)

- Three modes: **Dark** (default), **Light**, **Auto** (`prefers-color-scheme`)
- Applies everywhere: HTML pages, Swagger UI, GraphiQL, CLI colors, TUI (lipgloss), native GUI (`gogpu/ui` + `gogpu/gogpu`), ReadTheDocs
- Literal hex palette defined ONCE in Go (`src/common/theme/colors.go`, `ThemePalette` struct: Background/Foreground/Primary/Secondary/Accent/Success/Warning/Error/Info/Surface/SurfaceAlt/Border/Muted) and consumed directly by Web CSS, Swagger, GraphiQL
- CLI/TUI do NOT consume the literal hex values — they map the same semantic roles to a `TerminalPalette` of ANSI 16-color indices instead, gated on the same `ColorEnabled(nil)`/`NO_COLOR` precedence as PART 8; native GUI does not paint hex either — `gogpu/ui` applies the OS theme and supports Linux/macOS/Windows, while BSD falls back to TUI/CLI
- Theme detection flow: server reads `theme` cookie → renders `theme-light`/`theme-dark` class on `<html>`; if unset/auto, render `theme-auto` and let pure CSS `prefers-color-scheme` handle it (no JS needed); default dark if all detection fails
- Switching requires no page reload; no-JS visitors get a `<noscript>` form that POSTs to the theme endpoint
- CSS variables (dark defaults, `:root`): `--color-bg`, `--color-bg-secondary`, `--color-bg-card`, `--color-bg-hover`, `--color-bg-active`, `--color-code-bg`, `--color-text`, `--color-muted`, `--color-border`, `--color-border-hover`, `--color-success`/`-bg`, `--color-error`/`-bg`, `--color-warning`/`-bg`, `--color-primary`/`-bg`. `html.theme-light` overrides the same set with light values

### UI Components

| Component | Rule |
|-----------|------|
| Buttons | Types: Primary/Secondary/Danger/Icon; 5 states; single-submit-only (disable + loading text, e.g. Save→Saving…, Delete→Deleting…); bind via `data-action`, never inline `onclick` |
| Toggle switches | CSS-only, hidden-checkbox + slider pattern |
| Modals | Native `<dialog>` + `showModal()`; `<form method="dialog">` for CSP-safe close/cancel; focus trap, initial focus, return focus on close, `role="dialog"`, `aria-modal="true"`, `aria-labelledby`, Escape key, backdrop click, body scroll lock |
| Toast vs Modal | Toast = non-blocking confirmation (e.g. "Settings saved"), top-right, max 5 stacked, `aria-live`; auto-dismiss Success 3s/Info 3s/Warning 5s/Error no auto-dismiss, pause-on-hover, Escape dismisses topmost. Modal = requires a decision/input (e.g. "Are you sure? delete", "Enter new password") |
| Site banner | `web.announcements` config renders as first element inside `<body>` before `<main>`; dismissal via `dismissed_announcements` cookie with a zero-JS POST fallback; `role="status"` (info/success) or `role="alert"` (warning/error); stacking order: cookie consent → announcements → PWA update banner |

### PWA Requirements

| Feature | Requirement |
|---------|-------------|
| Manifest | `/manifest.json` — `name`, `short_name`, `start_url`, `scope`, `display`, `icons` required |
| Icons | 72–512px full set + maskable variants (192, 512) with safe zone = inner 80% |
| Service worker | `/sw.js` — install (pre-cache), activate (clean old caches), fetch: only same-origin GET intercepted; navigations network-first → cached page → synthesized 503 offline `Response`; static cache-first → network → guaranteed 504; everything else network-first → cache → guaranteed 504; `/api/*` always network-only (never intercepted); every `respondWith()` branch ends in a real `Response` — never `undefined`, never a rejection (`net::ERR_FAILED` otherwise) |
| Cache versioning | `{app_name}-cache-v{major}.{minor}.{patch}` |
| Offline | `/offline.html` fallback page required |
| HTTPS | Required for service workers — non-negotiable |
| Updates | Detect new SW version via `updatefound`, prompt user, `skipWaiting()`/`clients.claim()` |
| Install prompt | `beforeinstallprompt` (unsupported on iOS — custom install-instructions modal instead) |
| Token storage | `owner_token` cookie (HttpOnly, Secure, SameSite=Strict) is primary; localStorage copy optional/JS convenience only, never load-bearing; IndexedDB for offline data |
| iOS | Needs `apple-mobile-web-app-capable`, `apple-touch-icon`, custom install modal, aware of storage eviction |
| Lighthouse | Must pass all PWA audit categories before deployment (manifest valid, SW registered+controlling, HTTPS, icons ≥192/512, maskable icon present, viewport meta, no horizontal scroll at mobile width) |

### Accessibility (WCAG 2.1 AA)

Keyboard navigation, visible focus indicators, ARIA labels/roles, 4.5:1 contrast (3:1 large text), alt text on all images, `<label>` on all inputs, `aria-live` error announcements, skip-to-content link, proper `h1→h2→h3` hierarchy, `prefers-reduced-motion` respected. Both themes MUST independently pass all of the above — no information conveyed by color alone.

### CORS

Default policy: `allowed_origins: ["*"]`. `Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`; `Access-Control-Allow-Headers` lists every supported auth header by name (never `*` — invalid with credentials and doesn't cover `Authorization`); `Access-Control-Allow-Credentials: true` only when the resolved origin list is explicit, never with `*`. Resolution order: explicit `server.cors.allowed_origins` (a single `""` entry disables CORS entirely) → `DOMAIN` env hostnames → reverse-proxy-learned `X-Forwarded-Host` (trusted proxies only) → fallback `*`. Same resolved list feeds CSP `connect-src {learned_origins}`. Native API credentials use `Authorization: Bearer` only; `?token=` is ignored and never treated as authentication.

### CSRF Protection (stateless double-submit cookie)

- No sessions — token never stored server-side
- Cookie `csrf_token`: `SameSite=Strict`, `Secure` (per `csrf.secure: auto`), `HttpOnly=false` (JS must read it); compare cookie vs header/field in constant time
- Validate when: method is `POST`/`PUT`/`PATCH`/`DELETE` **and** no `Authorization: Bearer …`/`X-API-Token` header present
- Bypass when: Bearer/API-token header present, method is `GET`/`HEAD`/`OPTIONS`, WebSocket upgrade, or path is in `server.csrf.exempt_paths` (glob patterns, e.g. webhook receivers)
- No Origin-based bypass, ever — `Origin`/`Referer` can be absent or spoofed
- Server-rendered forms auto-inject a hidden `<input name="csrf_token">`; token regenerated when the cookie is absent or on revocation
- Failure → `403 FORBIDDEN`, body `{"ok": false, "error": "CSRF_FAILED", "message": "CSRF token validation failed"}`; log to `security.log` as `security.csrf_failure`
- Does NOT stop: XSS-stolen tokens (defense is CSP + escaping) or stolen Bearer tokens (defense is rotation/scoping/rate limiting)

### Owner Token and Preference Handling

- Owner and linked API tokens are masked by default and revealable on demand from the token section of `/server/preferences`; reveal is POST, CSRF-protected, audited, and `Cache-Control: no-store`. Never include token values in lists, logs, exports, or errors.
- Preferences remain cookie-only and server-stateless. Export/import includes `theme`, `lang`, and valid `{project_name}_pref_*` cookies only; never include consent, build-stamp, CSRF, session, or owner-token cookies.
- Preference import is additive, validates every value, ignores unknown/invalid fields, and uses same-origin local redirects only. Its web route works without JavaScript; API mirrors return JSON.

### Untrusted Content Handling

Any operator/user-supplied HTML (footer custom branding, markdown-rendered content) is sanitized via `bluemonday` strict policy before rendering: allow only `p, br, span, div, strong, b, em, i, u, s, small, h1-h6, ul, ol, li, a[href,title,target,rel], img[src,alt,title,width,height], class, id`; force `rel="noopener noreferrer"` on links; allow only `https`/`data` URL schemes on images; strip `<script>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, `<input>`, `<style>`, `<link>`, `<meta>`, `<base>`, all event-handler attributes, `javascript:` URLs, and the `style` attribute. `web.footer.branding`: unset/empty = default built-in branding; `" "` (single space) = disable branding entirely; any other string = sanitized custom HTML.

### Standard Pages (`/server/*`)

**ALL applications MUST have these pages; content is defined per-application and MUST come from `IDEA.md` — never generic placeholders.**

| Frontend Route | API Route | Notes |
|-----------------|-----------|-------|
| `/server` → 301 → `/server/about` | — | redirect only |
| `/server/about` | `/api/{api_version}/server/about` | name/version/description/features/links from `IDEA.md` |
| `/server/privacy` | `/api/{api_version}/server/privacy` | auto-generated from `server.privacy` config; CCPA "Do Not Sell" section only when `server.privacy.data.sold = true` |
| `/server/contact` | `/api/{api_version}/server/contact` (POST) | form (name/email/subject/message/captcha) + captcha (`recaptcha`/`hcaptcha`/`simple`); posts to `server.contact.general.email`, falls back to `admin.email`; page also links to `/server/security` for vulnerability reports and states the abuse-report email |
| `/server/help` | `/api/{api_version}/server/help` | getting-started/features/API docs links/Tor access (if enabled)/FAQ/troubleshooting, all project-specific |
| `/server/terms` | `/api/{api_version}/server/terms` | markdown-supported, default template provided |
| `/server/healthz` | `/api/{api_version}/server/healthz` | HTML view; `/healthz` root alias optional via `server.healthz.root.enabled` |
| `/server/docs/swagger` | `/api/{api_version}/server/swagger` (alias `/api/swagger`) | Swagger UI |
| `/server/docs/graphql` | `/api/{api_version}/server/graphql` (alias `/api/graphql`) | GraphiQL UI |

All `/server/` pages follow standard frontend rules: full HTML header/footer, theme support, responsive layout, SEO meta tags.

### Project Variables

`project_name=vidveil`, `project_org=apimgr`, `internal_name=vidveil`, `internal_org=apimgr`, `official_site=https://x.scour.li`

For complete details, see AI.md PART 16
