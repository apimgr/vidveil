# Features Rules (PART 18-23)

WARNING: These rules are NON-NEGOTIABLE. Violations are bugs.

Topics: Email & Notifications, Scheduler, GeoIP, Metrics, Backup & Restore, Update Command

## CRITICAL - NEVER DO

- Use external cron/Task Scheduler - internal scheduler only
- Skip GeoIP/blocklist/CVE database refreshes - scheduler handles them
- Expose Prometheus metrics publicly - internal-only with optional bearer token
- Forget to retain `server.scheduler.tasks.backup.retention` (default 4)

## CRITICAL - ALWAYS DO

- Use the built-in scheduler for backup, log rotation, session cleanup, SSL renewal, health check, Tor health, geoip/blocklist/cve updates
- Provide --update with stable/beta/daily branches and --maintenance backup/restore/update/mode/setup
- Country blocking via deny_countries / allow_countries (PART 20)
- Email & notifications via SMTP auto-detection (PART 18)

---
For complete details, see AI.md PART 18-23
