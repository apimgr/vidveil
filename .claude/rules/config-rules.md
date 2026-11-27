# Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Put YAML comments after a setting; place them above the setting.
- Use `strconv.ParseBool()`; use `config.ParseBool()`.
- Trust request paths, filesystem paths, proxy headers, or configuration input without the PART 5/12 validation and normalization rules.
- Treat debug endpoints as production endpoints; mount them only under the specified debug conditions.
- Store machine-detected values as build-time or config defaults.

## CRITICAL - ALWAYS DO
- Keep operator settings in the prescribed `server.yml`, `yaml.d/`, and `secrets.yml` configuration sources.
- Validate configuration and every input at the authoritative boundary.
- Apply the specified production/development/debug state priority and security behavior.
- Apply base URL prefixes consistently to routes and URL generation.
- Follow the exact maintenance-mode recovery, health, and operator-guidance behavior.

## KEY RULES
- Runtime detection and configuration override precedence are defined in PARTS 5, 6, and 12; follow those sections for each setting.
- Debug pprof/expvar behavior is restricted to the debug mode and exposure rules in PART 6.
- No `.env` file may be required for runtime behavior.

For complete details, see AI.md PART 5, 6, 12.
