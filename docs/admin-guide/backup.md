# Backup & Restore

## Creating Backups

### Via Admin Panel

1. Go to `/admin/server/system/backup`
2. Click "Create Backup"
3. Optionally enable encryption (password required)

### Via CLI

```bash
# Create unencrypted backup
vidveil --maintenance backup

# Backup is saved to configured backup directory
# Default: /var/lib/apimgr/vidveil/backups/ (root)
# Default: ~/.local/share/Backups/apimgr/vidveil/ (user)
```

## Restoring

### Via Admin Panel

1. Go to `/admin/server/system/backup`
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

Default: Keep last 4 backups. Configure via `backup.max_backups` setting.

## Scheduled Backups

Automatic backups run daily at 02:00. Configure at `/admin/server/scheduler`.
