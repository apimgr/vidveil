# Docker Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Put `Dockerfile` in the repo root
- Ignore multi-stage build requirements
- Skip the required init/process handling
- Expose the wrong container port defaults

## CRITICAL - ALWAYS DO

- Keep Docker assets under `docker/`
- Use the spec-compliant multi-stage build layout
- Keep runtime images minimal and secure
- Use the documented volume and port behavior

## Docker Basics

- Docker build files live in `docker/`
- Container port behavior follows the spec defaults
- Runtime paths use `/config/vidveil/` and `/data/vidveil/`

For complete details, see AI.md PART 27
