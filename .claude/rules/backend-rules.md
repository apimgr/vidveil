# Backend Rules (PART 9, 10, 11, 32)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Error Handling & Caching, Database & Cluster, Security & Logging, Tor Hidden Service

## CRITICAL - NEVER DO

- Concatenate user input into SQL - use parameterized queries
- Log passwords, tokens, or session secrets - even hashed
- Store secrets in plaintext config or environment without flagging it
- Skip CSRF tokens on state-changing forms
- Trust engine response bodies - parse, never execute

## CRITICAL - ALWAYS DO

- Argon2id for passwords; SHA-256 for tokens; Argon2id PHC string format
- Built-in scheduler - NEVER external cron/Task Scheduler
- Auto-enable Tor hidden service when Tor binary is present (PART 32)
- Cluster mode: DB is source of truth, server.yml is cache/backup
- Audit log every admin action and config change

---
For complete details, see AI.md PART 9, 10, 11, 32
