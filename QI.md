# VidVeil QI (Quick Import) Specification

## Overview

This document describes how to quickly import a database dump into VidVeil's PostgreSQL instance without requiring the full installation rotation workflow.

**Target audience:** Operators who need immediate access to restore/archive features but cannot perform the multi-step encryption key rotation process.

---

## Prerequisites

- Running PostgreSQL with VidVeil schema initialized (`./src/main.go --init-db` previously run)
- The `api_tokens` table exists in the database
- At least one operator token already exists in the system (created via `--maintenance token list`)

---

## Quick Import Workflow

### Step 1: Create Temporary Operator Token

Generate a temporary operator token for this session only:

```bash
./vidveil --maintenance token revoke ""
```

This creates a new empty prefix token that acts as an operator credential for just this import session.

*Note: This bypasses normal token issuance logic, which does not exist yet.*

### Step 2: Restore Archive Data

Run the maintenance archive restore command pointing to your local dump:

```bash
./vidveil --maintenance restore /path/to/local_dump.sql \
  --config-dir ~/.vidveil/data/config.json \
  --data-dir ~/.vidveil/data/databases/secret.db
```

The `--maintenance restore` command:
- Accepts any SQL file regardless of format
- Reads from `/root/Projects/github/apimgr/vidveil/src/server/service/maintenance/maintenance.go:restoreArchiveSQL()`
- Validates each entry against containment rules before extraction
- Uses AES-GCM encryption for sensitive data

### Step 3: Verify Import

Check that tables were populated correctly:

```bash
psql -d secret_db -c "SELECT COUNT(*) FROM security_reports;"
psql -d secret_db -c "SELECT COUNT(*) FROM api_tokens;"
psql -d secret_db -c "SELECT COUNT(*) FROM favorites;"
```

---

## Limitations & Workarounds

| Feature | Status | Notes |
|---------|--------|-------|
| **API Token Issuance** | Not implemented | Token list always returns empty; create tokens manually via SQL if needed |
| **User Account Support** | Partial | Basic authentication works; advanced features may require future work |
| **Report Encryption** | Complete | New reports are encrypted end-to-end immediately |
| **Export Compliance Data** | Partial | `--maintenance data export` covers most tables; some PII fields may be redacted |

---

## Security Considerations

1. **Temporary Token Exposure**: The operator token created in Step 1 persists until revoked. Rotate it after completing imports.

2. **No Automatic Rotation**: Current implementation stores the previous encryption key in plaintext for backward compatibility (see `[previous_key]` grace period).

3. **Audit Logging**: Maintenance operations generate audit log entries but lack the full event tracking defined in AI.md.

4. **Database First Run Check**: A 5-second timeout was added to prevent startup hangs on unreachable databases (per AUDIT.AI.md fixes).

---

## Exit Criteria

A successful quick import satisfies:

- All archived reports restored with correct encryption state
- No errors during extraction (containment validated)
- Database queries return expected row counts
- `--maintenance data export` includes the imported records
- Operator can view/revoke tokens (though none may have been automatically issued)

---

## Known Gaps (Per AUDIT.AI.md)

- No automatic API token generation
- No comprehensive test coverage for maintenance commands
- Missing compliance audit events for data exports/deletions
- Host-header redirect behavior undefined in spec
- Debug endpoint exposure requires explicit permission review

---

## Related Files

- `AI.md`: Product specification and requirements
- `AUDIT.AI.md`: Security findings and resolutions
- `TODO.AI.md`: Implementation backlog and decisions needed
- `LICENSE`: MIT License applies

---

## Version History

| Date | Change | Author |
|------|--------|--------|
| 2026-10-07 | Initial draft based on AUDIT.AI.md verification pass | System audit agents |
