# Project Audit

Started: 2026-08-21

Scope: theming / CSS / JS only (AI.md PART 16, `.claude/rules/frontend-rules.md`).
Backend, routes, auth, and DB were explicitly out of scope for this pass.

## Pass 5: Spec Compliance (PART 16)

- [ ] No site-wide theme toggle anywhere. AI.md 22230-22248 requires a header
      toggle (right side, last item, Dark/Light/Auto, Enter/Space cycles) plus a
      `<noscript>` form POSTing to the theme endpoint. Today the only way to change
      theme is the `<select>` on `/server/preferences`. `grep -c noscript
      src/server/template/` is 0. Fix: add the `.theme-toggle` button to
      `partial/public/header.tmpl` with a `data-action="cycle-theme"` handler in
      `app.js`, and a `<noscript>` three-option form POSTing to
      `/server/preferences/save` (route already exists and already validates
      `theme`). Requires new i18n keys in all 7 locale files.

- [ ] Hardcoded user-facing English strings in `src/server/static/js/app.js`:
      `showConfirm()` builds `'Confirm Action'` / `'Cancel'` / `'Confirm'` /
      `aria-label="Close"` (lines ~663-670), and the preferences/history IIFEs use
      `i18n.x || 'English fallback'` throughout. testing-rules.md: "Never hardcode
      user-facing strings anywhere ... every string MUST use a translation key, no
      exceptions." Fix: add a global `#app-i18n` JSON data island to
      `partial/public/head.tmpl` (loaded on every page) and read from it, dropping
      every English fallback literal.

- [ ] `src/server/handler/handlers.go:2288` and `:3781`: `"safeHTML": func(s string)
      template.HTML { return template.HTML(s) }` is a raw, unsanitized passthrough.
      `bluemonday` is not in `go.mod` at all. Currently it is only applied to
      project-owned translation output (`page/favorites.tmpl:35`), so there is no
      live injection path, but PART 16 requires operator/user-supplied HTML to go
      through a strict bluemonday allowlist. Fix: add bluemonday and route
      `safeHTML` through a strict policy, or rename the helper to make its
      trusted-input-only contract explicit and assert it at the call sites.
      (Backend change — deliberately deferred, out of this pass's scope.)

## Completed

- static/js/app.js (~line 2859-2861): removed the dead `window.toggleNav` /
  `window.closeNav` global exports and their stale "for onclick handlers"
  comment — nav toggle/close is bound via `data-action="toggle-nav"` /
  `data-action="close-nav"` in `header.tmpl`, dispatched through the
  CSP-safe `data-action` handler in `app.js`; no inline `onclick` exists
  anywhere in the codebase and nothing referenced the `window.*` globals.
- template/page/preferences.tmpl: JS-only form (no `method`/`action`/CSRF) silently
  discarded all preferences with JS disabled — added
  `method="post" action="/server/preferences/save"`, the CSRF and `return_to`
  hidden fields, and server-rendered `selected` on the theme `<select>`.
- template/component/modal.tmpl: `<dialog>` was missing `role="dialog"`,
  `aria-modal="true"`, `aria-labelledby` — all three added.
- static/js/app.js `showConfirm()`: added `role="dialog"` and backdrop-click close.
- static/css/common.css: five hardcoded literals in the `@media print` block
  (`#666`, `#ccc`, `#f0f0f0`) replaced with new `--color-print-muted` /
  `--color-print-border` / `--color-print-surface` tokens defined once in `:root`.
- static/js/sw.js: added terminal `.catch` guards on the navigation and
  non-navigation fallback chains so a rejecting `caches.match` can never reject
  `respondWith` (`net::ERR_FAILED`); documented that `CACHE_NAME` is a placeholder
  rewritten by `server.go` to `vidveil-cache-v{version}-{commit}`.
- template/partial/public/head.tmpl:5: `<meta name="theme-color" content="#282a36">`
  hardcoded the Dracula background hex. Added `ThemeColor` (map key in
  `renderTemplate()`, field in `HealthzHTMLData`) resolved via a new
  `getThemeColor()` helper reading `theme.Dark.Background` /
  `theme.Light.Background` from `src/common/theme/colors.go`, and switched the
  template to `{{.ThemeColor}}`. `static/js/app.js` `updateMetaThemeColor()`
  also hardcoded `'#ffffff'`/`'#282a36'` for the runtime (auto-mode) update —
  replaced with reading the resolved `--color-bg` CSS custom property off
  `<html>` (already set by the caller before this runs), so the hex now exists
  in exactly one place (the Go palette, mirrored once into `common.css`).
- template/page/preferences.tmpl:239: engine toggle `<label>`s were rendered
  with the `hidden` attribute and only revealed by JS
  (`initEngineTiers()`/`app.js`), so engine selection was unavailable with
  JS disabled — a "JS enhances only, never enables" violation. Removed
  `hidden` from the server-rendered markup; the base `.engine-toggle` /
  `.engine-tiers` CSS in `common.css` already renders a usable flat
  checkbox grid with no JS, and `initEngineTiers()` still progressively
  enhances it into collapsible tier groups when JS is available (it
  rebuilds the container and re-appends the same nodes, so no duplicate
  reveal logic was needed).
