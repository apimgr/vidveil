# TODO.AI.md

Findings from the full Phase 2 beta-test pass (commit 0d33f978f556), logged for
triage/fix. Remove each item individually once resolved and committed.

## Low

- [ ] `vidveil-cli` fails with an opaque low-level error when run as root on
  the same host as a server that was also started as root: `creating client
  directories: path /root/.config/apimgr/vidveil owned by uid=899 gid=899,
  want uid=0 gid=0`. Root cause: server (run via `sudo vidveil`) drops
  privileges to the `vidveil` system user (uid 899) and creates
  `~root/.config/apimgr/vidveil/` owned by that uid; CLI run as
  `sudo vidveil-cli` resolves the same default path but expects root
  ownership. Either avoid the path collision, or make the CLI's error
  message explain the conflict and suggest `--config <name>` as a
  workaround.
