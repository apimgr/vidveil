# VIDVEIL Specification

**Name**: vidveil

---

# CRITICAL RULES - READ FIRST

**STOP. Read this entire section before doing ANYTHING.**

## THIS IS A STRICT SPECIFICATION - NOT GUIDELINES

**EVERY SINGLE ITEM in this specification MUST be followed EXACTLY as written.**

- This is NOT a suggestion document
- This is NOT a "best practices" guide
- This is NOT open to interpretation
- There are NO exceptions unless explicitly stated
- Deviation from ANY part of this spec is a failure
- "I thought it would be better to..." is NOT acceptable
- "The standard way is..." - THIS spec IS the standard
- If the spec says X, you do X - not Y, not "improved X", not "X but slightly different"

**If something seems wrong or suboptimal in this spec, follow it anyway and flag it for review. Do NOT silently "fix" it.**

## Build & Binary Rules

| Rule | Description |
|------|-------------|
| **CGO_ENABLED=0** | ALWAYS. No exceptions. Pure Go only. |
| **Single static binary** | All assets embedded with Go `embed` package |
| **8 platforms required** | linux, darwin, windows, freebsd × amd64, arm64 |
| **Binary naming** | `vidveil-{os}-{arch}` (windows adds `.exe`) |
| **NEVER use -musl suffix** | Alpine builds are NOT musl-specific |
| **Build source** | ALWAYS `src` directory |

## Docker Rules

| Rule | Description |
|------|-------------|
| **Multi-stage Dockerfile** | Builder stage (golang:alpine) + Runtime stage (alpine:latest) |
| **Dockerfile location** | `docker/Dockerfile` - NEVER in project root |
| **Internal port** | ALWAYS `80` |
| **STOPSIGNAL** | `SIGRTMIN+3` |
| **ENTRYPOINT** | `["tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh"]` |
| **NEVER modify ENTRYPOINT/CMD** | All customization via entrypoint.sh |
| **Required packages** | `curl`, `bash`, `tini`, `tor` |
| **Tor** | Auto-enabled if `tor` binary installed (Docker image always has Tor) |

## CI/CD Rules

| Rule | Description |
|------|-------------|
| **NEVER use Makefile in CI** | Workflows have explicit commands with all env vars |
| **GitHub/Gitea/Jenkins must match** | Same platforms, same env vars, same logic |
| **VERSION from tag** | Strip `v` prefix: `v1.2.3` → `1.2.3` |
| **LDFLAGS** | `-s -w -X 'main.Version=...' -X 'main.CommitID=...' -X 'main.BuildDate=...'` |
| **Docker builds on EVERY push** | Any branch push triggers Docker image build |
| **Docker tags** | Any push → `devel`, `{commit}`; beta → adds `beta`; tag → `{version}`, `latest`, `YYMM`, `{commit}` |

## Database Rules

| Rule | Description |
|------|-------------|
| **SQLite default** | `{datadir}/db/server.db` and `{datadir}/db/users.db` |
| **Password hashing** | Argon2id - NEVER bcrypt |
| **Valkey/Redis** | Every app supports it for caching/clustering |

## CLI Rules (NON-NEGOTIABLE)

```
--help                       # Show help
--version                    # Show version
--mode {production|development}
--config {configdir}
--data {datadir}
--log {logdir}
--pid {pidfile}
--address {listen}
--port {port}
--status                     # Show status and health
--service {start,restart,stop,reload,--install,--uninstall,--disable,--help}
--daemon                     # Daemonize (detach from terminal)
--maintenance {backup,restore,update,mode,setup} [optional-file-or-setting]
--update [check|yes|branch {stable|beta|daily}]
```

**These CLI commands are NON-NEGOTIABLE. Do not change, rename, or remove them.**

## Directory Structure

```
src/                        # Go source code (REQUIRED)
src/main.go                 # Server application entry point
src/config/                 # Configuration package
src/server/                 # HTTP server package
docker/                     # Docker files (REQUIRED)
docker/Dockerfile           # Multi-stage Dockerfile
docker/docker-compose.yml   # Production docker-compose
docker/docker-compose.dev.yml   # Development docker-compose
docker/entrypoint.sh        # Container entrypoint script
scripts/                    # Build and helper scripts
tests/                      # Test files
Makefile                    # Build targets
AI.md                       # THIS file - project specification
TODO.AI.md                  # Task tracking (when needed)
README.md                   # User documentation
LICENSE.md                  # License file
```

---

# PART 0: COMMENTS ABOVE CODE (NON-NEGOTIABLE)

**ALL comments MUST be ABOVE the code they describe, NEVER inline/beside.**

## Comment Placement Rules

| Position | Status | Example |
|----------|--------|---------|
| **Above** | REQUIRED | `// This does X` (newline) `code` |
| **Inline** | FORBIDDEN | `code // This does X` |
| **End of line** | FORBIDDEN | `value: 123 # comment` |

## Why This Rule Exists

| Reason | Description |
|--------|-------------|
| **Consistency** | Same style everywhere = easier to read |
| **Diff clarity** | Code changes don't affect comment lines |
| **Line length** | Comments don't make lines too long |
| **Scanning** | Can read code flow without visual interruption |

## Correct Examples

### Go
```go
// Calculate the total price including tax
totalPrice := price * (1 + taxRate)

// Check if user has admin privileges
if user.Role == "admin" {
    // Grant full access to all resources
    grantFullAccess(user)
}
```

### YAML
```yaml
server:
  # Port to listen on (1-65535)
  port: 8080

  # Maximum concurrent connections
  max_connections: 1000
```

## Wrong Examples

### Go (FORBIDDEN)
```go
totalPrice := price * (1 + taxRate)  // Calculate total with tax  WRONG!
if user.Role == "admin" {  // Check admin  WRONG!
```

### YAML (FORBIDDEN)
```yaml
server:
  port: 8080  # Port to listen on  WRONG!
  max_connections: 1000  # Max connections  WRONG!
```

---

# PROJECT-SPECIFIC: VIDVEIL

## Overview

Vidveil is a privacy-respecting adult video meta search engine. It aggregates results from multiple adult video sites without tracking users.

## Key Features

| Feature | Description |
|---------|-------------|
| **Meta Search** | Aggregates results from 50+ adult video sites |
| **Privacy First** | No tracking, no cookies, no personal data storage |
| **Age Verification** | Client-side age gate before accessing content |
| **Tor Support** | Built-in Tor hidden service support |
| **Multiple Formats** | HTML, JSON API, GraphQL, plain text |
| **Bang Syntax** | Direct site searches with `!site query` |

## Supported Search Engines

### Primary Engines (Always Enabled)
- PornHub
- XVideos
- XNXX
- RedTube
- YouPorn
- XHamster
- Eporner
- PornMD (meta-search)

### Additional Engines
- SpankBang, Tube8, KeezMovies, SpankWire, ExtremeTube
- DrTuber, TNAFlix, HQPorner, FlyFlv, TubeGalore
- 4Tube, Nuvid, Txxx, PornHat, HDZog
- VJav, NonkTube, PornTube, AnyPorn, EmpFlix
- SunPorno, HellPorno, ZenPorn, AlphaPorno
- GotPorn, VPorn, LoveHomePorn, NubilesPorn
- And many more...

## Project-Specific Routes

### Public Web Routes

| Route | Method | Description |
|-------|--------|-------------|
| `/` | GET | Homepage with search box |
| `/search` | GET | Search results page |
| `/preferences` | GET | User preferences (stored in cookies) |
| `/about` | GET | About page |
| `/privacy` | GET | Privacy policy |
| `/age-verify` | GET/POST | Age verification gate |

### Public API Routes

| Route | Method | Description |
|-------|--------|-------------|
| `/api/v1/search` | GET | Search API (JSON) |
| `/api/v1/search/stream` | GET | SSE streaming search |
| `/api/v1/search.txt` | GET | Plain text results |
| `/api/v1/engines` | GET | List available engines |
| `/api/v1/engines/{name}` | GET | Engine details |
| `/api/v1/bangs` | GET | List bang commands |
| `/api/v1/autocomplete` | GET | Search autocomplete |
| `/api/v1/stats` | GET | Service statistics |

### Search Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Search query (required) |
| `engines` | string | Comma-separated engine list |
| `page` | int | Page number (default: 1) |
| `safe` | bool | Safe search filter |
| `sort` | string | Sort order (relevance, date, views) |
| `duration` | string | Duration filter (short, medium, long) |
| `quality` | string | Quality filter (hd, 4k) |

### Bang Commands

| Bang | Engine | Example |
|------|--------|---------|
| `!ph` | PornHub | `!ph amateur` |
| `!xv` | XVideos | `!xv homemade` |
| `!xnxx` | XNXX | `!xnxx compilation` |
| `!rt` | RedTube | `!rt massage` |
| `!yp` | YouPorn | `!yp blonde` |
| `!xh` | XHamster | `!xh milf` |
| `!ep` | Eporner | `!ep 4k` |

## Configuration

### Search-Specific Settings

```yaml
search:
  # Default engines to use
  default_engines:
    - pornhub
    - xvideos
    - xnxx
    - redtube
    - youporn
    - xhamster
    - eporner

  # Maximum results per engine
  max_results_per_engine: 20

  # Search timeout per engine
  engine_timeout: 10s

  # Total search timeout
  total_timeout: 30s

  # Enable parallel searching
  parallel: true

  # Maximum concurrent engine requests
  max_concurrent: 10

  # Safe search default
  safe_search: false

  # Cache settings
  cache:
    enabled: true
    ttl: 300

  # Tor settings
  tor:
    enabled: false
    proxy: "socks5://127.0.0.1:9050"
```

### Age Verification Settings

```yaml
age_verification:
  enabled: true
  # Cookie duration in days
  cookie_duration: 30
  cookie_name: "age_verified"
```

## Database Schema

### Engine Stats Table

| Column | Type | Description |
|--------|------|-------------|
| `engine` | String | Engine name |
| `searches` | Integer | Total searches |
| `results` | Integer | Total results returned |
| `errors` | Integer | Total errors |
| `avg_latency` | Float | Average response time (ms) |
| `last_success` | Timestamp | Last successful query |
| `last_error` | Timestamp | Last error |
| `enabled` | Boolean | Engine enabled |

### Search History Table (Optional - if analytics enabled)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `query_hash` | String | SHA-256 of query (privacy) |
| `engines` | JSON | Engines used |
| `result_count` | Integer | Total results |
| `latency` | Float | Search time (ms) |
| `timestamp` | Timestamp | Search time |

**Note: User searches are NEVER logged with identifiable information.**

## Admin Panel Extensions

### Engines Page (`/admin/engines`)

| Element | Description |
|---------|-------------|
| Engine list | All supported engines with status |
| Enable/Disable | Toggle engines on/off |
| Test Engine | Send test query |
| Statistics | Searches, results, errors, latency |
| Reset Stats | Clear engine statistics |

### Search Settings (`/admin/server/settings`)

| Setting | Description |
|---------|-------------|
| Default engines | Select default search engines |
| Timeout | Engine and total timeout settings |
| Cache TTL | Search result cache duration |
| Parallel | Enable/disable parallel searching |
| Safe search | Default safe search setting |

---

# STANDARD SECTIONS

## Server Configuration

All standard server configuration from TEMPLATE.md applies:
- HTTP/HTTPS settings
- Rate limiting
- Session management
- Security headers
- CORS
- Compression
- Caching (Valkey/Redis support)

## Admin Panel

All standard admin panel routes from TEMPLATE.md apply:
- `/admin/` - Dashboard
- `/admin/server/settings` - Server settings
- `/admin/server/ssl` - SSL/TLS configuration
- `/admin/server/scheduler` - Scheduled tasks
- `/admin/server/email` - Email settings
- `/admin/server/logs` - Log viewer
- `/admin/security/*` - Security settings
- `/admin/network/*` - Network settings (Tor, GeoIP)
- `/admin/system/*` - Backup, maintenance, updates

## API Standards

All API standards from TEMPLATE.md apply:
- REST API at `/api/v1/`
- GraphQL at `/graphql`
- OpenAPI/Swagger at `/openapi`
- Content negotiation (JSON, HTML, text)
- `.txt` extension for plain text
- Standard response formats
- Pagination
- Error responses

## Security

All security requirements from TEMPLATE.md apply:
- HTTPS support (Let's Encrypt)
- Security headers (CSP, HSTS, etc.)
- Rate limiting
- CSRF protection
- Session management
- Password hashing (Argon2id)
- API token hashing (SHA-256)
- 2FA support (TOTP)

## Scheduler Tasks

Standard scheduler tasks plus:

| Task | Schedule | Description |
|------|----------|-------------|
| Engine health check | Every 5 minutes | Check engine availability |
| Cache cleanup | Hourly | Clear expired search cache |

---

# COMPLIANCE CHECKLIST

- [ ] CGO_ENABLED=0 for all builds
- [ ] All 8 platform builds working
- [ ] Docker multi-stage build with tini
- [ ] All CLI flags implemented
- [ ] Admin panel fully functional
- [ ] REST, GraphQL, and OpenAPI working
- [ ] Age verification gate functional
- [ ] All search engines implemented
- [ ] Tor support working
- [ ] Security headers in place
- [ ] Rate limiting enabled
- [ ] Scheduler running
- [ ] Backup/restore working

---

**END OF SPECIFICATION**
