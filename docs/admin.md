# Admin Panel

## Access

Admin panel is available at `/admin`

## First Run Setup

On first run, VidVeil displays a setup token:

```
Setup URL: http://localhost:64580/admin/setup?token=abc123...
```

Visit this URL to create your admin account.

## Features

### Dashboard

- Search statistics
- Engine status
- System metrics

### Server Settings

- Listen address and port
- Application mode
- Debug settings

### Engine Management

- Enable/disable search engines
- Configure timeouts
- View engine status

### Blocklist

- Add blocked terms/patterns
- Support for regex patterns
- View blocked content

### Security

- Rate limiting configuration
- Security headers
- CVE vulnerability tracking

### SSL/TLS

- Let's Encrypt integration
- Manual certificate upload
- Auto-renewal configuration

### Backup & Restore

- Manual backup creation
- Scheduled backups (02:00 daily)
- Restore from backup
- AES-256-GCM encryption

### Logs

- Access logs
- Error logs
- Audit logs
- Search and filter

### Users

- Admin user management
- 2FA (TOTP, WebAuthn)
- Session management

## API Access

Admin functions are also available via REST API:

```bash
# List engines
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:64580/api/v1/admin/engines

# Update engine status
curl -X PUT \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}' \
  http://localhost:64580/api/v1/admin/engines/pornhub
```

## Security

- Session-based authentication for web UI
- Bearer token authentication for API
- 2FA support (TOTP, WebAuthn)
- Automatic session timeout
- Audit logging
