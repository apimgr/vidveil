# Configuration

## Configuration File

Default locations:

- Linux (root): `/etc/apimgr/vidveil/server.yml`
- Linux (user): `~/.config/apimgr/vidveil/server.yml`
- macOS (root): `/Library/Application Support/apimgr/vidveil/server.yml`
- macOS (user): `~/Library/Application Support/apimgr/vidveil/server.yml`
- BSD (root): `/usr/local/etc/apimgr/vidveil/server.yml`
- Windows: `%AppData%\apimgr\vidveil\server.yml`

## Override Order

1. CLI flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values

## Core Settings

### Server

```yaml
server:
  # Listen address
  address: "::"
  
  # Listen port (random 64xxx port)
  port: 64580
  
  # Application mode: production or development
  mode: production
  
  # Enable debug mode
  debug: false
```

### Database

```yaml
database:
  # Database type: sqlite, postgres, mysql
  type: sqlite
  
  # SQLite path
  path: /var/lib/apimgr/vidveil/db/vidveil.db
```

### Search Engines

```yaml
engines:
  # Enable/disable engines globally
  enabled:
    pornhub: true
    xhamster: true
    redtube: true
    # ... all 54+ engines
  
  # Per-engine timeout (seconds)
  timeout: 5
  
  # Enable result caching
  cache:
    enabled: true
    ttl: 900  # 15 minutes
```

### Privacy

```yaml
privacy:
  # Thumbnail proxy (prevents tracking)
  thumbnail_proxy: true
  
  # Rate limiting
  rate_limit:
    search: 30    # per minute per IP
    api: 60       # per minute per IP
    proxy: 300    # per minute per IP
```

## Environment Variables

All settings can be overridden with `VIDVEIL_` prefix:

```bash
VIDVEIL_SERVER_PORT=8080
VIDVEIL_SERVER_MODE=development
VIDVEIL_DATABASE_TYPE=postgres
```

## Admin Panel

All settings are configurable via the web admin panel at `/admin`
