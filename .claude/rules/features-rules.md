# Feature Rules (PART 18, 19, 20, 21, 22, 23)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use external cron for scheduled work
- Ship email features that ignore SMTP gating rules
- Skip built-in metrics, backup, or update behavior when the spec requires them
- Treat GeoIP, metrics, or update behavior as optional shortcuts when enabled by spec

## CRITICAL - ALWAYS DO

- Use the built-in scheduler for recurring tasks
- Respect email and notification configuration rules
- Keep GeoIP, metrics, backup, and update flows aligned with the spec
- Expose relevant settings in admin/config surfaces

## Coverage

- PART 18: Email and notifications
- PART 19: Built-in scheduler
- PART 20: GeoIP features
- PART 21: Metrics
- PART 22: Backup and restore
- PART 23: Update command

## Key Rules

- Scheduler work stays inside the app
- Update flows use the documented commands and channels
- Backup, retention, and verification follow the defined safety rules
- Metrics and health signals should be structured and predictable

For complete details, see AI.md PART 18, PART 19, PART 20, PART 21, PART 22, PART 23
