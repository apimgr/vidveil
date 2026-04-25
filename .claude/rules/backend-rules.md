# Backend Rules (PART 9, 10, 11, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use string-concatenated SQL
- Use bcrypt
- Expose internal details in user-facing errors
- Disable or stub Tor support
- Overwrite Tor config on every startup

## CRITICAL - ALWAYS DO

- Use parameterized queries
- Use Argon2id for passwords and SHA-256 for tokens
- Return spec-compliant error structures
- Apply security headers and validation consistently
- Auto-enable Tor when available and follow the AddOnion architecture

## Data and Error Rules

- Keep error responses consistent and audience-appropriate
- Use SQLite by default unless the configured database says otherwise
- Validate and sanitize all input before processing

## Security Rules

- Use defense in depth: validation, escaping, CSRF, rate limiting, safe defaults
- Never reveal whether auth identities exist
- Keep audit and log detail separated from user-facing messages

## Tor Rules

- Use `t.Control.AddOnion()` to map `.onion:80` to `127.0.0.1:{server_port}`
- Keep `ensureTorrc()` create-once and persistent
- Use `ControlPort 127.0.0.1:auto`
- Persist the Ed25519 key and use v3 hidden services

For complete details, see AI.md PART 9, PART 10, PART 11, PART 32
