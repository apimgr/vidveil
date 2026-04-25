# Frontend Rules (PART 16, 17)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use React, Vue, or client-side rendering frameworks
- Require JavaScript for core behavior
- Break mobile layouts with long strings
- Mix admin and public UI concerns carelessly
- Skip server-side validation because the UI already checks something

## CRITICAL - ALWAYS DO

- Use server-side Go templates
- Build mobile-first layouts
- Make core features work without JavaScript
- Use progressive enhancement only
- Keep the admin panel fully wired to the real settings and routes

## Web UI Rules

- HTML pages are the source of truth for browser UX
- CSS must handle long strings safely on mobile
- Use semantic templates and accessible interactions

## Admin Rules

- Keep admin routes isolated and clearly structured
- Ensure forms, auth, and settings pages map to real server behavior
- Treat admin UI as a first-class product surface, not an afterthought

For complete details, see AI.md PART 16, PART 17
