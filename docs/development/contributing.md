# Contributing

## Getting Started

1. Fork the repository
2. Create a feature branch
3. Make your changes using the supported containerized workflow
4. Run the relevant checks:
   - `make dev` for a quick local build
   - `make test` for unit tests
   - `./tests/run_tests.sh` for containerized integration tests
5. Submit a pull request

## Code Style

- Follow Go conventions
- Comments above code, not inline
- Run `gofmt` before committing
- Keep docs aligned with implemented behavior and current paths/URLs

## Pull Requests

- Clear description
- Reference any related issues
- Include tests for new features
- Do not introduce host-only build or test steps; use the existing Makefile and test scripts
