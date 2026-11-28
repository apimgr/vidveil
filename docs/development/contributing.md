# Contributing

## Getting Started

1. Fork the repository
2. Create a feature branch
3. Make your changes using the supported containerized workflow
4. Run the relevant checks:
   - `make build` for debug server and CLI binaries across all 8 OS/architecture pairs
   - `make test` for unit tests
   - `./tests/run_tests.sh` for containerized integration tests
5. Submit a pull request

## CI/CD providers

The repository includes workflows for all five providers required by AI.md:
GitHub Actions (`.github/workflows/`), GitLab CI (`.gitlab-ci.yml`), Gitea
Actions (`.gitea/workflows/`), Forgejo Actions (`.forgejo/workflows/`), and
Jenkins (`Jenkinsfile`). Only the provider configured for a repository or mirror runs
its workflow. CI commands are explicit and do not call Makefile targets.

## Code Style

- Follow Go conventions
- Comments above code, not inline
- Run `gofmt` before committing
- Keep docs aligned with implemented behavior and current paths/URLs

## Pull Requests

- Clear description
- Reference any related issues
- Include tests for new features
- Do not introduce local-machine-only build or test steps; use the existing Makefile and test scripts
