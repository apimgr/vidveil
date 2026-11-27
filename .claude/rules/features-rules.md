# Feature Rules (PART 17–22)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Replace the built-in scheduler with external cron or silently drop scheduled maintenance.
- Use unverified GeoIP/blocklist data, overwrite a working database with a failed download, or treat location as authoritative identity.
- Expose sensitive operational metrics publicly or omit required privacy-preserving metric behavior.
- Store backup passwords in plaintext or use bcrypt; follow the encrypted-at-rest backup-secret and Argon2id archive-key contract.
- Install updates without the prescribed source, checksum, platform, and atomic replacement checks.

## CRITICAL - ALWAYS DO
- Implement the email/notification, scheduler, GeoIP, metrics, backup/restore, and update behavior defined in each PART.
- Make feature configuration and failure handling match the documented defaults and operator controls.
- Bound background work, retry/backoff, data retention, and resource use as specified.
- Preserve the last usable GeoIP data on refresh failures and report health accurately.
- Test restore/update failure paths and preserve recoverable state on errors.

## KEY RULES
- Backup archive encryption and backup-password storage are separate contracts; read PART 21 before changing either.
- Scheduled tasks, update status, and public metrics must respect their data-classification and privacy rules.

For complete details, see AI.md PART 17–22.
