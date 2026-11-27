# Frontend Rules (PART 16)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Use client-side rendering (React, Vue, Angular, etc.), client-side routing, or JavaScript for core functionality.
- Put business logic in JavaScript or require JavaScript for core functionality.
- Use desktop-first CSS, inline CSS/JavaScript, JavaScript alerts, placeholder pages, or empty handlers.
- Let long strings break mobile layouts.

## CRITICAL - ALWAYS DO
- Use server-side Go templates and progressive enhancement; every core flow must work without JavaScript.
- Use mobile-first responsive CSS, WCAG 2.1 AA accessibility, and touch targets of at least 44×44px.
- Apply `word-break: break-all`, `overflow-wrap: break-word`, and monospace styling to specified long strings; use the separate scroll-box treatment for onion/I2P copy targets.
- Source about/help content from IDEA.md; source privacy, terms, and contact content/settings exactly as PART 16 specifies.
- Keep every server page useful, complete, secure, localized, and consistent with real routes and behavior.

## KEY RULES
- Server owns validation, rendering, formatting, and business logic. JavaScript is limited to browser-only enhancements.
- Follow the page, theme, CSP, responsive breakpoint, form, and no-JS requirements in PART 16.

For complete details, see AI.md PART 16.
