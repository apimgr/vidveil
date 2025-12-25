# Backup & Restore

## Creating Backups

### Via Admin Panel

1. Go to `/admin/system/backup`
2. Click "Create Backup"

### Via CLI

```bash
vidveil --maintenance backup
```

## Restoring

### Via CLI

```bash
vidveil --maintenance restore backup.tar.gz
```

## Backup Contents

- Configuration files
- Database (server.db, users.db)
- Custom templates
- SSL certificates (optional)
