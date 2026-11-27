# Binary Rules (PART 7, 8, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Enable CGO or ship a dynamically linked application binary.
- Drop required server CLI flags/subcommands, change their names, or add short flags other than `-h` and `-v`.
- Embed mutable external databases or platform data that AI.md requires to be downloaded/managed at runtime.
- Make the companion CLI a thin alias when PART 32 requires full CLI/TUI/GUI behavior.

## CRITICAL - ALWAYS DO
- Build static server and companion client binaries for all eight required OS/architecture combinations.
- Embed application assets and follow PART 7’s display, theme, terminal, and banner package requirements.
- Follow server startup order, graceful shutdown, PID handling, directory validation, output-stream, and exit-code contracts.
- Support NO_COLOR and the specified CLI display/mode selection and resize behavior.
- Keep runtime update sources, checksum verification, and replacement behavior aligned with AI.md.

## KEY RULES
- Server is `{project_name}`; the required client is `{project_name}-cli`.
- See PART 8 for the non-negotiable server command interface and PART 32 for client/TUI/GUI behavior.

For complete details, see AI.md PART 7, 8, 32.
