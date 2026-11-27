# Backend Rules (PART 9, 10, 11, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Return raw internal errors, secrets, tokens, paths, or unsafe diagnostics to callers or logs.
- Store owner/API tokens in plaintext; persist only their SHA-256 hashes.
- Use bcrypt/MD5/SHA-1 for passwords; use Argon2id for login password hashes. Backup-password handling follows its separate encryption-at-rest contract.
- Build SQL by concatenating untrusted input or bypass the shared database/store abstractions.
- Run untrusted content through shell commands, hooks, interpreters, or build tools.
- Operate Tor as a relay, exit, or general-purpose proxy.

## CRITICAL - ALWAYS DO
- Return the prescribed structured API errors; sanitize and classify errors before logging or responding.
- Use bounded database pools, query timeouts, idempotent migrations, and transactions for coupled writes.
- Enforce ownership on every private resource operation and apply the authentication/authorization rules from PART 11.
- Protect secrets at rest, redact sensitive fields, audit security-sensitive operations, and apply required security/privacy headers.
- Provide the required app-scoped Tor hidden service, with best-effort lifecycle and non-blocking integration; implement I2P per its explicit opt-in/container rules.

## KEY RULES
- Caching must use the prescribed keys, TTLs, invalidation, headers, and version-change purge behavior.
- Database concurrency and transaction semantics are defined in PART 10; do not assume SQLite behavior.

For complete details, see AI.md PART 9, 10, 11, 31.
