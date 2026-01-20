# VidVeil CLI Client Implementation

## Completed

- [x] **Setup Wizard** - First-run wizard when no server configured
  - TUI wizard using bubbletea
  - Prompts for server URL and optional token
  - Tests connection before saving
  - Saves to cli.yml and token file

- [x] **Engines Command** - List available engines from server
  - `vidveil-cli engines` - list all engines
  - `vidveil-cli engines --enabled` - list enabled only
  - `vidveil-cli engines --disabled` - list disabled only
  - `vidveil-cli engines --all` - show all details
  - Output in table/json/plain formats

- [x] **Bangs Command** - List bang shortcuts
  - `vidveil-cli bangs` - list all bangs
  - `vidveil-cli bangs --search <term>` - filter bangs
  - Output in table/json/plain formats

- [x] **Open URL in Browser** - TUI feature to open selected result
  - Press `Enter` or `o` to open in browser
  - Cross-platform: xdg-open (Linux), open (macOS), start (Windows)
  - Fallback displays URL if browser unavailable

- [x] **Connection Health Check** - Verify server on startup
  - Checks /healthz endpoint before commands
  - Shows warning if server unreachable
  - Non-blocking (commands still run)

## Pending

- [ ] **SSE Streaming** - Real-time search results (low priority for VidVeil)
  - Accept: text/event-stream header
  - Parse SSE events
  - Display results as they arrive
  - Note: VidVeil aggregates from 50+ sites, SSE would show incremental results

## CLI Structure

```
vidveil-cli [command] [flags]
vidveil-cli <query>              (shortcut for search)

Commands:
  search <query>    Search for videos
  engines           List available search engines
  bangs             List bang shortcuts
  probe             Test engine availability
  login             Save API token to config
  shell             Shell completion commands

Flags:
  --config          Config file path
  --server          Server address
  --token           API token
  --output          Output format (json, table, plain)
  --no-color        Disable colored output
  --timeout         Request timeout in seconds
  --debug           Enable debug output
  -h, --help        Show help
  -v, --version     Show version
```

## TUI Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Search (no results) / Open in browser (with results) |
| `o` | Open selected result in browser |
| `/` | New search (clear results) |
| `j` / `Down` | Move selection down |
| `k` / `Up` | Move selection up |
| `Esc` | Clear search and results |
| `q` / `Ctrl+C` | Quit |
