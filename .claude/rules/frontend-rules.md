# Frontend Rules (PART 16, 17)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Web Frontend, Admin Panel

## CRITICAL - NEVER DO

- Client-side rendering (React, Vue, Angular)
- Require JavaScript for core functionality
- Client-side routing (SPA)
- Business logic in JavaScript
- Let long strings (IPv6, .onion, tokens) break mobile layout
- Desktop-first CSS - mobile-first only
- Inline CSS or JavaScript
- JavaScript alerts - use toast notifications
- Generic placeholder content in /server/about or /server/help

## CRITICAL - ALWAYS DO

- Server-side rendering (Go templates)
- Progressive enhancement (works without JS)
- Mobile-first responsive CSS
- CSS `word-break: break-all` for IPv6/.onion/tokens/hashes
- WCAG 2.1 AA accessibility
- Touch targets minimum 44x44px
- All admin settings have a UI panel
- Source /server/about and /server/help content from IDEA.md

---
For complete details, see AI.md PART 16, 17
