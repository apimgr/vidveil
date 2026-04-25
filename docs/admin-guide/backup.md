# Backup & Restore

## Creating Backups

### Via Admin Panel

1. Go to `https://x.scour.li/admin/server/system/backup`
2. Click "Create Backup"
3. Optionally enable encryption (password required)

### Via CLI

```bash
# Create unencrypted backup
vidveil --maintenance backup

# Backup is saved to configured backup directory
# Default (Linux root): /mnt/Backups/apimgr/vidveil/
# Default (BSD root): /var/backups/apimgr/vidveil/
# Default: ~/.local/share/Backups/apimgr/vidveil/ (user)
```

## Restoring

### Via Admin Panel

1. Go to `https://x.scour.li/admin/server/system/backup`
2. Select backup from list or upload file
3. Click "Restore"

### Via CLI

```bash
vidveil --maintenance restore backup.tar.gz
```

## Backup Contents

Per AI.md PART 22, backups include:

- `manifest.json` - Backup metadata with checksums
- Configuration files (`server.yml`)
- Database files (`server.db`, `users.db`)
- Custom templates (if modified)
- SSL certificates (optional, config setting)
- Data directory (optional, config setting)

## Encryption

Backups support AES-256-GCM encryption with Argon2id key derivation:

- Configure encryption password in server settings
- Encrypted backups have `.enc` extension
- Password required for restore

## Retention

Default: Keep the last 1 daily backup. Configure via `backup.max_backups`, `backup.keep_weekly`, `backup.keep_monthly`, and `backup.keep_yearly`.

## Scheduled Backups

Automatic backups are scheduled for 02:00 daily but disabled by default. Enable and configure them at `https://x.scour.li/admin/server/scheduler`.
