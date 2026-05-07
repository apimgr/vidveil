# Repo-local Task History

This file was migrated from session-local SQL/checkpoint tracking so the ongoing spec-compliance audit survives the dev-server migration. AI.md (the canonical spec) is the source of truth - this file is a working task list, not the spec.

## Current Status

- AI.md was re-copied from `~/Templates/go/TEMPLATE.md` on 2026-05-06 and substituted with project-specific values (project_name=vidveil, project_org=apimgr, internal_name=vidveil, plist_name=io.github.apimgr.vidveil). 2412 placeholder replacements applied. Current md5 after substitution is `ab2546e6b41c6e844c3206c2cdbeb600`. AI.md is UNTRACKED in git intentionally.
- IDEA.md migrated to the AI.md PART 0 three-section format on 2026-05-06: `## Project description`, `## Project variables`, `## Business logic` (with all six required subsections plus reference detail). Backup at `IDEA.md.preMigration.bak`. official_site sourced from `site.txt` (https://x.scour.li).
- Repo-root AI loader memory file regenerated on 2026-05-06 from the AI.md PART 0 template, pointing into AI.md and the per-tool rules cheatsheets.
- 14 per-tool rule cheatsheet files regenerated on 2026-05-06 in the per-tool rules dir (gitignored, local only): ai-rules, project-rules, config-rules, binary-rules, backend-rules, api-rules, frontend-rules, features-rules, service-rules, makefile-rules, docker-rules, cicd-rules, testing-rules, optional-rules.
- AUDIT.AI.md deleted on 2026-05-06 as stale relative to the new template. A fresh post-template-refresh audit may be opened later as a separate AUDIT.AI.md when work resumes.
- `PLAN.AI.md` is present and not modified this session.
- COMMIT/NEVER/MUST rules saved to harness memory on 2026-05-06.

## Active / Pending

- [ ] client-elevated-user-rule - AI.md PART 33 says `vidveil-cli` always runs as a normal user and never as root/administrator, but the current client has no elevated-user startup guard yet. Do not implement this until the exact runtime behavior is pinned down against the existing cross-platform privilege helpers and the containerized validation flow.
- [ ] license-md-full-regen - LICENSE.md was manually trimmed (removed lib/pq and rewrote the count line) but the listed Go dependencies are still incomplete vs go.mod. Full regeneration via `go-licenses` MUST run inside `golang:alpine` (NEVER on host); operator action required before next release.
- [ ] historical-copilot-attribution - Multiple existing commit bodies contain `Co-authored-by: Copilot <...>`. AI.md PART 0 forbids AI attribution in commits. Cleaning historical commits requires a force-push to main (a destructive op) and is not actionable without explicit user authorization.

## Completed Migrated Tasks

- [x] bootstrap-commit-mess - Sync commit message - Reconcile .git/COMMIT_MESS with the actual uncommitted bootstrap changes after creating the missing rule files.
- [x] bootstrap-review - Review bootstrap state - Confirm current AI.md/TODO.AI.md/.cl-aude state and the required rule-file outputs for this repo bootstrap.
- [x] bootstrap-rules - Create rule cheatsheets - Create the full per-tool rules/*.md cheatsheet set from the current AI.md PART mappings and repo-specific values.
- [x] audit-baseline - Run baseline checks - Run the existing repository build and test commands to establish the current baseline before making audit fixes.
- [x] audit-code-review - Inspect code compliance - Audit code, docs, and infrastructure against AI.md and identify concrete mismatches that require fixes.
- [x] audit-final-verify - Verify audit state - Re-run the existing build/test workflow and confirm the audited project state after fixes.
- [x] audit-fixes - Apply audit fixes - Fix all concrete spec mismatches found during the audit and keep documentation and infra in sync.
- [x] audit-spec-read - Read audit spec - Read AI.md PART 0 and PART 1 audit and compliance sections plus any directly relevant rule cheatsheets before auditing.
- [x] audit-followup - Run follow-up audit sweep - Re-read AI.md PART 0 and PART 1 doc-related requirements, inspect remaining modified files and untracked rule cheatsheets, fix any remaining spec drift, then refresh COMMIT_MESS to match the final worktree.
- [x] followup-verify - Verify post-audit state - Run the repository build/test commands after the final documentation and spec cleanup, then refresh COMMIT_MESS to match the final worktree.
- [x] final-drift-sweep - Run final drift sweep - Search the repository for any remaining spec drift after the documentation URL normalization, fix concrete issues found, then refresh COMMIT_MESS if needed.
- [x] docs-drift-sweep-2 - Sweep remaining docs drift - Search README.md, docs/, mkdocs.yml, and adjacent project docs for remaining relative public/admin/API URLs, stale path examples, or non-standard curl usage after the prior compliance fixes.
- [x] metadata-sweep - Sweep metadata and badges - Inspect workflow files, README badges, docs deployment metadata, and unresolved placeholders outside the already-fixed docs set.
- [x] support-docs-sweep - Sweep support docs - Inspect root markdown/support files outside the already-audited docs set for stale URLs, placeholders, non-standard curl usage, and path drift.
- [x] privacy-copy-verify - Verify privacy copy changes - Run the repository build/test workflow after updating the public footer template and preferences documentation.
- [x] privacy-spec-recheck - Recheck privacy copy against spec.
- [x] user-guide-parity - Check user guide parity.
- [x] cli-guide-parity - Check CLI guide parity against vidveil-cli commands, flags, storage paths, behavior.
- [x] development-docs-parity - Check development docs parity against current implementation.
- [x] build-docs-sweep - Sweep remaining build docs for leftover stale development guidance.
- [x] test-scripts-parity - Check test script parity in tests/docker.sh and tests/incus.sh.
- [x] ci-spec-audit - Audit CI and Docker against AI.md build/CI/Docker rules.
- [x] automation-drift-sweep - Sweep remaining automation drift in Jenkinsfile and docker compose.
- [x] final-repo-drift-sweep - Sweep remaining repo drift across the repository.
- [x] tracked-files-sweep - Sweep tracked files for URLs, artifact names, stale metadata, deprecated endpoints.
- [x] placeholder-sweep - Sweep leftover placeholders such as stale domain examples or outdated endpoint strings.
- [x] debug-docs-sweep - Verify debug docs (localhost:8080, pprof, reverse proxy snippets).
- [x] exact-drift-sweep - Sweep exact remaining drift in flags, ports, URLs, artifact names, placeholder domains.
- [x] command-docs-sweep - Sweep command docs for nonexistent flags, wrong config semantics, stale ports.
- [x] command-surface-sweep - Sweep vidveil command surface against implemented server CLI/help output.
- [x] backup-defaults-sweep - Verify backup defaults (retention count, schedule).
- [x] user-docs-exact-sweep - Sweep user docs exactly for command examples, default values, path claims.
- [x] defaults-sweep - Sweep documented defaults vs actual config and CLI defaults.
- [x] macos-service-path-fix - Fix macOS service naming drift; align launchd plist path/label with com.apimgr.vidveil naming.
- [x] cli-docs-sweep - Verify CLI doc defaults; align docs/cli.md and README CLI wording with PART 33 + actual vidveil-cli behavior.
- [x] exact-sweep-next - Scan next exact drifts after defaults and CLI wording sweeps.
- [x] cli-flags-sweep - Verify documented CLI flags against actual vidveil-cli subcommand parsers.
- [x] probe-details-fix - Make probe output match docs (search_time_ms, response time, --verbose details).
- [x] address-env-sweep - Verify listen env naming; align AI.md, docs, entrypoint, server CLI/env around LISTEN vs ADDRESS.
- [x] remove-no-color-alias - Remove undocumented --no-color client alias to match PART 8 / PART 33.
- [x] sync-shell-completions - Sync client shell completions for bash/zsh/fish/PowerShell.
- [x] cli-equals-flag-support - Support `--flag=value` syntax across global and command-specific client flag parsers.
- [x] client-shell-flag-alignment - Move shell completion to spec-compliant --shell flag handling.
- [x] client-help-output-alignment - Align client --help output with spec-required --shell completions/init/--help lines.
- [x] client-token-env-alias - Accept VIDVEIL_CLI_TOKEN as a fallback alias for VIDVEIL_TOKEN.
- [x] global-flags-after-command - Parse global flags after command names while preserving command-specific flag handling.
- [x] client-config-selection - Implement PART 33 --config resolution rules (names + paths) end-to-end.
- [x] client-token-file-path - Move default CLI token file from CLI data directory to PART 33 config directory path.
- [x] client-server-primary-persistence - Read legacy server.address but write spec-aligned server.primary; accept VIDVEIL_SERVER_PRIMARY with VIDVEIL_SERVER alias; persist --server when none saved.
- [x] client-token-config-key - Treat top-level token in cli.yml as canonical PART 33 token source.
- [x] client-env-precedence - Apply PART 33 env precedence so canonical env vars override config; token-file beats env.
- [x] client-official-site-default - Embed official site via ldflags as the PART 33 compiled default server.
- [x] client-config-autocreate - Auto-create cli.yml on first run with current sane defaults.
- [x] client-setup-detection - Keep compiled official-site default out of first-run setup detection.
- [x] build-official-site-source - Read official site from site.txt with OFFICIALSITE/OFFICIAL_SITE env override.
- [x] client-tui-enabled-config - Honor tui.enabled config setting; default true; suppress autostart when false.
- [x] client-tui-shortcuts-help - Implement `?` shortcuts help in TUI with regression coverage.
- [x] docs-tui-surface-alignment - Update docs/cli.md TUI section to reflect implemented surface only.
- [x] client-tui-config-shape - Replace tui.show_hints with PART 33 tui.mouse and tui.unicode keys.
- [x] client-output-formats - Add yaml and csv output formats alongside json/table/plain.
- [x] client-timeout-duration-config - Read/write server.timeout in PART 33 duration form (e.g. 30s) with int compatibility.
- [x] client-debug-config-key - Add PART 33 top-level debug config key; env/flag still wins.
- [x] client-api-version-config - Honor cli.yml server.api_version through the API client (search/version/health/engines/probe).
- [x] client-admin-path-config - Add PART 33 server.admin_path config key with default `admin`.
- [x] client-auth-config-shape - Add spec-aligned auth.token and auth.token_file with legacy compatibility reads.
- [x] client-output-config-shape - Add output.pager / output.quiet / output.verbose; apply verbose to probe.
- [x] client-logging-cache-config - Add PART 33 logging and cache config sections with documented defaults.
- [x] client-cli-log-init - Initialize cli.log on startup with logging.file override or default path at 0600.
- [x] client-startup-order - Move EnsureClientDirs into ExecuteCLI after ParseCLIGlobalFlags and before config load.
- [x] client-config-permissions - Normalize cli.yml permissions to 0600 on startup, including pre-existing configs.
- [x] client-ownership-verification - Verify standard client directories plus cli.yml and cli.log are owned by current user.
- [x] client-token-file-permissions - Normalize default token file at {config_dir}/token; share write helper for login/setup.
- [x] repo-plan-todo-migration - Move plan and todo into repo as PLAN.AI.md and TODO.AI.md.
- [x] template-refresh-2026-05-06 - Re-copy AI.md from ~/Templates/go/TEMPLATE.md on 2026-05-06; refresh TODO.AI.md against the new template; commit COMMIT/NEVER/MUST rules to memory.
- [x] post-template-refresh-audit - Audit closed 2026-05-06: 13 findings logged in AUDIT.AI.md and fixed in place; AUDIT.AI.md deleted per PART 0 Step 8 when all open items closed.
- [x] docker-overlay-rename - Renamed `docker/file_system/` -> `docker/rootfs/`, fixed Dockerfile COPY + comments to match (PART 3 + PART 27).
- [x] compose-runtime-mounts - Switched the 3 docker-compose*.yml runtime mounts from `./rootfs/{config,data}` to `./volumes/{config,data}` (PART 27).
- [x] dockerignore-volumes - `.dockerignore` now excludes `volumes/` (was `rootfs/`); the build-time overlay at `docker/rootfs/` is no longer accidentally excluded from the build context.
- [x] healthz-canonical-routes - Added `/server/healthz` + `.json` + `.txt` (frontend) and `/api/v1/server/healthz` (API) per PART 13/14; gated `/healthz` root alias on the new `server.healthz.root.enabled` config (default false; never redirects).
- [x] healthz-config-field - Added `HealthzConfig` / `HealthzRootConfig` to ServerConfig (`server.healthz.root.enabled: false`).
- [x] forbidden-bak-deleted - Removed `src/server/handler/admin.go.bak` (124KB local backup; was gitignored).
- [x] i18n-relocation - Moved `src/server/service/i18n/` -> `src/common/i18n/` per PART 3; zero importers, no code rewrites needed.
- [x] license-md-libpq - Removed the `github.com/lib/pq` line from LICENSE.md (not in go.mod) and replaced the stale "(73 packages)" header with a regen-from-container note.
- [x] backup-tmpl-placeholder - `src/server/template/admin/backup.tmpl` placeholder updated from `/var/lib/vidveil/backups` to the spec-correct privileged Linux backup path `/mnt/Backups/apimgr/vidveil` (PART 4).
- [x] form-bool-config-parsebool - `src/server/handler/admin.go` `verify_ssl` form value now flows through `config.ParseBool(r.FormValue(...))` per PART 5 Boolean Handling.
- [x] firsttime-placeholder-substitution - Substituted 2412 first-time-setup placeholders in AI.md (project_name=vidveil, project_org=apimgr, internal_name=vidveil, plist_name=io.github.apimgr.vidveil) on 2026-05-06.
- [x] idea-md-three-section-migration - Migrated IDEA.md to the AI.md PART 0 three-section format on 2026-05-06; backup retained at IDEA.md.preMigration.bak; official_site sourced from site.txt.
- [x] missing-root-ai-loader - Recreated the project-root AI loader memory file on 2026-05-06 from the AI.md PART 0 template.
- [x] recreate-rules-cheatsheets - Regenerated all 14 per-tool rule cheatsheet files on 2026-05-06 (local only; the rules dir is gitignored).
- [x] delete-stale-audit-ai - Deleted AUDIT.AI.md on 2026-05-06 as stale relative to the new template; a fresh AUDIT.AI.md will be opened when post-template-refresh-audit resumes.
- [x] memorize-commit-never-must - Saved the COMMIT, NEVER, and MUST rule sets from AI.md PART 0-5 to harness memory on 2026-05-06.
