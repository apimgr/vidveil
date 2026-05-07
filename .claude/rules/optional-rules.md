# Optional Rules (PART 34-36)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Multi-User, Organizations, Custom Domains (all OPTIONAL - NOT IMPLEMENTED in vidveil)

## CRITICAL - NEVER DO

- Add multi-user, organization, or custom-domain code to vidveil - PARTS 34-36 are NOT implemented
- Add references to users/orgs/custom_domains tables in code
- Add `users.enabled: false` style toggles - the code should be written as if the feature never existed
- Flip OPTIONAL -> REQUIRED in AI.md without also updating IDEA.md

## CRITICAL - ALWAYS DO

- If a future PART 34/35/36 implementation happens, change the PART title to NON-NEGOTIABLE in AI.md and update IDEA.md to document the feature as implemented
- Treat the entire flipped PART as non-negotiable from that point forward

---
For complete details, see AI.md PART 34-36
