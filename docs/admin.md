# Admin Panel

## Access

Admin panel is available at `/admin` (configurable via `server.admin.path`).

## First Run Setup

On first run, VidVeil displays a setup token in the console:

```
Setup Token: abc123...
Setup URL: http://localhost:64580/admin
```

Navigate to `/admin` and enter the setup token to create your admin account.

## Route Structure

Per AI.md PART 17, admin routes follow a strict hierarchy:

```
/admin/                      # Dashboard
/admin/profile               # Your profile settings
/admin/server/               # ALL server management
/admin/server/settings       # Server settings
/admin/server/branding       # Branding configuration
/admin/server/ssl            # SSL/TLS settings
/admin/server/scheduler      # Scheduled tasks
/admin/server/email          # Email configuration
/admin/server/logs           # Server logs
/admin/server/database       # Database settings
/admin/server/security/      # Security settings
/admin/server/security/auth  # Authentication config
/admin/server/security/tokens # API tokens
/admin/server/network/       # Network settings
/admin/server/network/tor    # Tor configuration
/admin/server/network/geoip  # GeoIP settings
/admin/server/system/        # System management
/admin/server/system/backup  # Backup & restore
/admin/server/system/maintenance # Maintenance mode
/admin/server/system/updates # Update management
/admin/server/users/         # User management
/admin/server/users/admins   # Admin accounts
/admin/server/engines        # Search engines
/admin/server/help           # Help & documentation
```

## Features

### Dashboard (`/admin`)

- Search statistics
- Engine status overview
- System metrics
- Quick actions

### Server Settings (`/admin/server/settings`)

- Listen address and port
- Application mode (prod/dev/debug)
- Debug settings
- Admin path configuration

### Engine Management (`/admin/server/engines`)

- Enable/disable search engines
- Configure timeouts
- View engine status and health

### Security (`/admin/server/security/*`)

- **Authentication** (`/auth`): Login settings, session config
- **API Tokens** (`/tokens`): Generate and manage API tokens
- **Rate Limiting** (`/ratelimit`): Configure request limits
- **Firewall** (`/firewall`): IP blocking rules

### Network (`/admin/server/network/*`)

- **Tor** (`/tor`): Hidden service configuration
- **GeoIP** (`/geoip`): Geographic IP lookup settings
- **Blocklists** (`/blocklists`): IP and term blocklists

### System (`/admin/server/system/*`)

- **Backup** (`/backup`): Manual and scheduled backups with AES-256-GCM encryption
- **Maintenance** (`/maintenance`): Maintenance mode control
- **Updates** (`/updates`): Update management
- **Info** (`/info`): System information

### SSL/TLS (`/admin/server/ssl`)

- Let's Encrypt integration (HTTP-01, TLS-ALPN-01, DNS-01)
- Manual certificate upload
- Auto-renewal configuration

### Logs (`/admin/server/logs`)

- Access logs
- Error logs
- Audit logs
- Search and filter

### Users (`/admin/server/users/*`)

- Admin user management
- 2FA setup (TOTP, WebAuthn)
- Session management

## API Access

Admin API follows the same hierarchy under `/api/v1/admin/`:

```bash
# Get server settings
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:64580/api/v1/admin/server/settings

# List engines
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:64580/api/v1/admin/server/engines

# Create backup
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:64580/api/v1/admin/server/backup
```

## Security

- Session-based authentication for web UI
- Bearer token authentication for API
- 2FA support (TOTP, WebAuthn)
- Automatic session timeout (30 days default)
- Full audit logging
- Admin panel completely isolated from public site
