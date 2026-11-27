# Service Rules (PART 23, 24)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Run the long-lived server permanently with elevated privileges when the specified privilege drop is available.
- Assume an OS, init system, privilege manager, UID/GID, service user, or service home directory.
- Install, uninstall, disable, or alter an existing service without the prescribed detection and safety checks.
- Generate service definitions that bypass the canonical binary arguments, paths, or signals.

## CRITICAL - ALWAYS DO
- Detect platform and privilege state at runtime and follow PART 23’s per-OS escalation and account-selection rules.
- Bind privileged resources before dropping privileges where required, then run as the dedicated service account.
- Provide and maintain service templates for each required init/service manager in PART 24.
- Set service paths, environment, working directory, permissions, restart behavior, and signal handling exactly as specified.
- Keep service install/uninstall/help output accurate and safe to run repeatedly.

## KEY RULES
- Keep the service account’s home and data directories aligned with the resolved platform-specific paths.
- Service management must degrade safely when the target manager is unavailable.

For complete details, see AI.md PART 23, 24.
