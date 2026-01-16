# Testing Rules

@AI.md PART 29: TESTING & DEVELOPMENT

## Test Scripts
- `./tests/run_tests.sh` - Auto-detects incus/docker
- `./tests/docker.sh` - Docker alpine (quick)
- `./tests/incus.sh` - Incus debian (PREFERRED)

## Tests MUST Include
- Admin setup (setup token → create admin → API token)
- Binary rename test (verify --help shows actual name)
- CLI full functionality (with API token)
- Agent full functionality (with API token)
- API endpoint tests (.txt extension, Accept headers)

## Debug Container Tools
`apk add --no-cache curl bash file jq`
