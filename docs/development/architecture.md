# Architecture Overview

This document describes the high-level architecture of VidVeil.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         VidVeil Server                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Public    │  │    Admin    │  │          API            │  │
│  │  Frontend   │  │    Panel    │  │  (REST/GraphQL/OpenAPI) │  │
│  └──────┬──────┘  └──────┬──────┘  └────────────┬────────────┘  │
│         │                │                      │               │
│  ┌──────┴────────────────┴──────────────────────┴─────────────┐ │
│  │                      HTTP Router (Chi)                      │ │
│  │  • Rate Limiting  • CORS  • Sessions  • Auth Middleware    │ │
│  └─────────────────────────────┬───────────────────────────────┘ │
│                                │                                 │
│  ┌─────────────────────────────┴───────────────────────────────┐ │
│  │                        Handlers                              │ │
│  │  • Public  • Admin  • API  • GraphQL  • Metrics             │ │
│  └─────────────────────────────┬───────────────────────────────┘ │
│                                │                                 │
│  ┌─────────────────────────────┴───────────────────────────────┐ │
│  │                        Services                              │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────────┐ │ │
│  │  │ Engines │ │  Admin  │ │  Email  │ │Scheduler│ │  SSL   │ │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └────────┘ │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────────┐ │ │
│  │  │  Cache  │ │ Logging │ │  GeoIP  │ │   Tor   │ │Cluster │ │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └────────┘ │ │
│  └─────────────────────────────┬───────────────────────────────┘ │
│                                │                                 │
│  ┌─────────────────────────────┴───────────────────────────────┐ │
│  │                      Data Layer                              │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │ │
│  │  │   SQLite     │  │ PostgreSQL/  │  │ Valkey/Redis │       │ │
│  │  │  (default)   │  │    MySQL     │  │   (cache)    │       │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘       │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
vidveil/
├── src/
│   ├── main.go                 # Application entry point
│   ├── config/                 # Configuration management
│   │   ├── config.go          # Config struct and loading
│   │   └── paths.go           # Path resolution
│   ├── models/                 # Data models
│   │   ├── video.go           # Video result model
│   │   └── search.go          # Search request/response
│   ├── server/                 # HTTP server
│   │   ├── server.go          # Server setup and routes
│   │   ├── handlers/          # HTTP handlers
│   │   │   ├── handlers.go    # Public handlers
│   │   │   ├── admin.go       # Admin panel handlers
│   │   │   ├── api.go         # REST API handlers
│   │   │   └── graphql.go     # GraphQL handlers
│   │   ├── templates/         # Go HTML templates
│   │   │   ├── layouts/       # Base layouts
│   │   │   ├── partials/      # Reusable partials
│   │   │   └── admin/         # Admin templates
│   │   └── static/            # Static assets
│   │       ├── css/           # Stylesheets
│   │       ├── js/            # JavaScript
│   │       └── img/           # Images
│   └── services/              # Business logic
│       ├── admin/             # Admin authentication
│       ├── backup/            # Backup/restore
│       ├── cache/             # Caching layer
│       ├── cluster/           # Clustering support
│       ├── database/          # Database operations
│       ├── email/             # Email service
│       ├── engines/           # Search engine adapters
│       ├── geoip/             # GeoIP lookups
│       ├── i18n/              # Internationalization
│       ├── logging/           # Structured logging
│       ├── scheduler/         # Task scheduler
│       ├── ssl/               # SSL/TLS management
│       ├── tor/               # Tor hidden service
│       └── validation/        # Input validation
├── docs/                       # Documentation
├── Dockerfile                  # Container build
├── Makefile                    # Build targets
└── go.mod                      # Go modules
```

## Core Components

### 1. Search Engine Manager

The engine manager handles all search engine integrations:

- **Adapter Pattern**: Each engine implements a common interface
- **Parallel Execution**: Searches run concurrently across engines
- **Result Merging**: Results are deduplicated and ranked
- **Caching**: Results cached with configurable TTL

### 2. Admin Service

Handles admin authentication and management:

- **Argon2id Hashing**: Passwords hashed with PHC format
- **TOTP 2FA**: Optional two-factor authentication
- **Recovery Keys**: 8 one-time backup codes
- **Session Management**: Secure session handling

### 3. Scheduler

Built-in task scheduler:

- **Persistent State**: Tasks survive restarts
- **Cluster-Aware**: Distributed locking for HA
- **Built-in Tasks**: SSL renewal, GeoIP updates, cleanup

### 4. Cache Layer

Flexible caching with multiple backends:

- **In-Memory**: Default for single-node
- **Valkey/Redis**: Required for clustering
- **TTL-Based**: Automatic expiration

## Request Flow

```
Client Request
      │
      ▼
┌─────────────────┐
│  Rate Limiter   │──▶ 429 Too Many Requests
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  CORS/Security  │
│    Headers      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Session      │
│   Middleware    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     Router      │──▶ 404 Not Found
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Handler      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Services     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Database/     │
│     Cache       │
└────────┬────────┘
         │
         ▼
    Response
```

## Database Schema

### server.db (Application Data)

```sql
-- Search engines configuration
CREATE TABLE engines (
    name TEXT PRIMARY KEY,
    enabled INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 50,
    config TEXT  -- JSON config
);

-- Scheduled tasks
CREATE TABLE scheduler_tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cron TEXT NOT NULL,
    last_run TEXT,
    next_run TEXT,
    status TEXT DEFAULT 'pending'
);

-- Audit log
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    action TEXT NOT NULL,
    user TEXT,
    resource TEXT,
    details TEXT  -- JSON
);
```

### users.db (User/Admin Data)

```sql
-- Admin accounts
CREATE TABLE admins (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE,
    password_hash TEXT NOT NULL,
    totp_secret TEXT,
    recovery_keys TEXT,  -- JSON array
    api_token TEXT UNIQUE,
    created_at TEXT,
    last_login TEXT
);

-- Sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    admin_id INTEGER,
    data TEXT,  -- JSON
    expires_at TEXT,
    FOREIGN KEY (admin_id) REFERENCES admins(id)
);
```

## Security Model

### Authentication

1. **Admin Panel**: Session-based with optional 2FA
2. **API Access**: Bearer token authentication
3. **Password Policy**: 12+ chars, complexity requirements

### Authorization

- **Server Admin**: Full access to all features
- **Additional Admins**: Configurable permissions
- **API Tokens**: Scoped access control

### Data Protection

- **Passwords**: Argon2id with PHC format
- **Backups**: AES-256-GCM encryption
- **Logs**: PII masking for emails/usernames
- **Sessions**: Secure, HttpOnly cookies

## Clustering

For high availability, VidVeil supports clustering:

```
                    ┌─────────────┐
                    │   Load      │
                    │  Balancer   │
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
   ┌──────────┐     ┌──────────┐     ┌──────────┐
   │  Node 1  │     │  Node 2  │     │  Node 3  │
   │ (Primary)│     │(Secondary)│    │(Secondary)│
   └────┬─────┘     └────┬─────┘     └────┬─────┘
        │                │                │
        └────────────────┼────────────────┘
                         │
                    ┌────┴────┐
                    │ Valkey/ │
                    │  Redis  │
                    └────┬────┘
                         │
              ┌──────────┴──────────┐
              │                     │
         ┌────┴────┐          ┌────┴────┐
         │PostgreSQL│         │  MySQL  │
         └─────────┘          └─────────┘
```

### Cluster Features

- **Config Sync**: Automatic synchronization
- **Session Sharing**: Via Valkey/Redis
- **Distributed Locking**: For scheduler tasks
- **Primary Election**: Automatic failover
