# Security

## Authentication

Configure at `/admin/server/security/auth`:

- Password requirements
- Session duration (default: 30 days)
- 2FA settings (TOTP, WebAuthn)
- Login attempt limits

## API Tokens

Manage at `/admin/server/security/tokens`:

- Generate new API tokens
- Revoke existing tokens
- Set token expiration
- View token usage

## Rate Limiting

Configure at `/admin/server/security/ratelimit`:

- Requests per minute (default: 60)
- Burst allowance
- Bypass for trusted IPs
- Per-endpoint limits

## Firewall

Configure at `/admin/server/security/firewall`:

- IP whitelist/blacklist
- Country blocking (requires GeoIP)
- CIDR range blocking
- Temporary bans
