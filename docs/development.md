# Development

## Building

**ALWAYS use Docker for builds per PART 0:**

```bash
# Build using Docker
make build

# Build for all platforms
make release
```

## Testing

### Docker Tests

```bash
./tests/docker.sh
```

### Incus Tests (Preferred for systemd)

```bash
./tests/incus.sh
```

### Run All Tests

```bash
./tests/run_tests.sh
```

## Project Structure

```
vidveil/
├── src/
│   ├── server/         # Server application
│   │   ├── handler/    # HTTP handlers
│   │   ├── model/      # Data models
│   │   ├── service/    # Business logic
│   │   │   └── engine/ # Search engines (54+)
│   │   ├── static/     # CSS, JS, images
│   │   └── template/   # HTML templates
│   ├── client/         # CLI client (optional)
│   │   ├── cmd/        # Commands
│   │   ├── api/        # API client
│   │   └── tui/        # Terminal UI
│   └── config/         # Configuration
├── docker/             # Docker files
├── docs/               # ReadTheDocs documentation
├── tests/              # Test scripts
├── AI.md               # Project specification
├── Makefile            # Build targets
└── go.mod              # Go dependencies
```

## Adding a Search Engine

1. Create `src/server/service/engine/{name}.go`
2. Implement the `Engine` interface
3. Add bang shortcuts to `bangs.go`
4. Register in `manager.go`
5. Update PART 37 in AI.md

## Code Guidelines

- CGO_ENABLED=0 (pure Go only)
- No inline CSS in templates
- No JavaScript alerts
- Use 2-space JSON indent
- All YAML comments above settings
- Follow PART 16 theme system

## Documentation

Update when adding features:

- AI.md PART 37 (business logic)
- README.md
- docs/ (ReadTheDocs)
- OpenAPI spec
- GraphQL schema

## Contributing

1. Fork the repository
2. Create feature branch
3. Make changes per AI.md spec
4. Test with Docker/Incus
5. Submit pull request
