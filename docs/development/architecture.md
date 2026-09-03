# Architecture Overview

VidVeil is a server-rendered Go web application with a companion CLI client.

## Major Components

- **Public Web UI**: Go templates rendered by the server
- **Admin Panel**: server management, engine control, security, Tor, backup, and updates
- **REST / GraphQL / OpenAPI**: machine-facing interfaces exposed by the same server
- **Search Engine Services**: engine adapters, result merging, filtering, and SSE streaming
- **CLI Client**: `vidveil-cli` for terminal and scripting workflows

## Repository Structure

```text
vidveil/
├── src/
│   ├── main.go
│   ├── admin/
│   ├── client/
│   ├── common/
│   ├── config/
│   ├── graphql/
│   ├── mode/
│   ├── paths/
│   ├── scheduler/
│   ├── server/
│   ├── service/
│   ├── ssl/
│   └── swagger/
├── docker/
├── docs/
├── tests/
├── binaries/
├── AI.md
├── README.md
├── Makefile
└── go.mod
```

## Request Flow

1. Browser, CLI, or API client sends a request.
2. The server routes it through middleware and handlers.
3. Search, admin, Tor, scheduler, and related services perform the work.
4. The response is rendered as HTML, JSON, SSE, or plain text depending on the route and request headers.

## Build and Packaging

- Go compilation is run through Docker-backed Makefile targets
- Runtime container assets live under `docker/`
- Cross-platform binaries are produced into `binaries/`
