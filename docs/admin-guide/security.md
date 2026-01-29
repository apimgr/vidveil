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

## Content Restriction

Configure at `/admin/server/network/geoip`:

Some jurisdictions have laws restricting adult content access. VidVeil supports geographic content restriction with configurable behavior.

### Restriction Modes

| Mode | Description |
|------|-------------|
| `off` | No restriction checking (privacy-first) |
| `warn` | Show dismissable warning banner via header |
| `soft_block` | Interstitial page requiring acknowledgment |
| `hard_block` | Complete access denial |

### Default Restricted Regions

US states with age verification laws:

- Texas, Utah, Louisiana, Arkansas
- Montana, Mississippi, Virginia, North Carolina

### Tor Bypass

By default, users accessing via Tor hidden service or with non-geolocatable IPs bypass restriction checks. This respects VidVeil's privacy-first design.

### Configuration

```yaml
geoip:
  content_restriction:
    mode: warn
    bypass_tor: true
    restricted_regions:
      - "US:Texas"
      - "US:Utah"
    restricted_countries: []
```
