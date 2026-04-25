# Service Rules (PART 24, 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Assume privilege escalation works the same on every OS
- Install service files into the wrong platform location
- Ignore service-manager detection requirements

## CRITICAL - ALWAYS DO

- Detect elevation and escalation capability per platform
- Use the correct service template and install path for the current OS
- Keep service installation and management aligned with the supported init system

## Service Support

- Linux uses systemd where applicable
- macOS uses launchd
- BSD uses rc.d or the platform-native service path
- Service generation must match the actual app paths and binary names

For complete details, see AI.md PART 24, PART 25
