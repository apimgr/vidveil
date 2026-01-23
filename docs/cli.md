# CLI Reference

The `vidveil-cli` command-line client provides a powerful interface for searching videos and managing your VidVeil server connection.

## Installation

The CLI client is built alongside the server. Download the appropriate binary for your platform:

- `vidveil-cli` (Linux/macOS)
- `vidveil-cli.exe` (Windows)

Or build from source:

```bash
make local  # Builds both server and CLI client
```

## Quick Start

```bash
# Launch interactive TUI (first run shows setup wizard)
vidveil-cli

# Quick search
vidveil-cli "search term"

# Explicit search command
vidveil-cli search "amateur"
```

## Automatic Mode Detection

The CLI automatically selects the appropriate interface:

| Condition | Mode |
|-----------|------|
| Interactive terminal + no command | TUI (interactive) |
| Interactive terminal + command | CLI output |
| Piped/redirected output | Plain text (no TUI) |
| First run without server configured | Setup wizard |

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path (default: `~/.config/apimgr/vidveil/cli.yml`) |
| `--server <url>` | Server address |
| `--token <token>` | API token for authentication |
| `--token-file <path>` | Read token from file |
| `--output <format>` | Output format: `json`, `table`, `plain` (default: `table`) |
| `--no-color` | Disable colored output |
| `--timeout <seconds>` | Request timeout (default: 30) |
| `--debug` | Enable debug output |
| `-h, --help` | Show help |
| `-v, --version` | Show version |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VIDVEIL_SERVER` | Server address |
| `VIDVEIL_TOKEN` | API token |

## Commands

### search

Search for videos across enabled engines.

```bash
vidveil-cli search [flags] <query>
vidveil-cli <query>  # shortcut
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--limit <n>` | Number of results |
| `--page <n>` | Page number (default: 1) |
| `--engines <list>` | Comma-separated list of engines |
| `--safe` | Enable safe search |

**Examples:**

```bash
# Basic search
vidveil-cli search "amateur"

# Limit results
vidveil-cli search --limit 20 "test query"

# Search specific engines
vidveil-cli search --engines pornhub,xvideos "query"

# Output as JSON
vidveil-cli --output json "query"

# Using bang shortcuts in query
vidveil-cli search "!ph amateur"        # PornHub only
vidveil-cli search "!ph !xv amateur"    # PornHub + XVideos
```

### engines

List available search engines.

```bash
vidveil-cli engines [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--enabled` | Show only enabled engines |
| `--disabled` | Show only disabled engines |
| `--all` | Show all details (tier, method, preview, download) |

**Examples:**

```bash
# List all engines
vidveil-cli engines

# List enabled only
vidveil-cli engines --enabled

# Full details as JSON
vidveil-cli engines --all --output json
```

### bangs

List bang shortcuts for quick engine selection.

```bash
vidveil-cli bangs [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--search <term>` | Filter bangs by name |

**Examples:**

```bash
# List all bangs
vidveil-cli bangs

# Filter bangs
vidveil-cli bangs --search porn
```

### probe

Test engine availability and response times.

```bash
vidveil-cli probe [flags]
```

### login

Save API token to configuration file.

```bash
vidveil-cli login
```

Prompts for server address and API token, then saves to config file.

### shell

Generate shell completions.

```bash
vidveil-cli shell completions <shell>
```

Supported shells: `bash`, `zsh`, `fish`, `powershell`

**Examples:**

```bash
# Bash completions
vidveil-cli shell completions bash > /etc/bash_completion.d/vidveil-cli

# Zsh completions
vidveil-cli shell completions zsh > ~/.zsh/completions/_vidveil-cli
```

## Configuration File

The CLI stores configuration in `~/.config/apimgr/vidveil/cli.yml`:

```yaml
server:
  address: "https://your-server.example.com"
  token: "your-api-token"
  timeout: 30

output:
  format: table    # json, table, plain
  color: auto      # auto, always, never

tui:
  theme: default
  show_hints: true
```

### Configuration Priority

Settings are resolved in this order (highest to lowest):

1. Command-line flags (`--server`, `--token`, etc.)
2. Environment variables (`VIDVEIL_SERVER`, `VIDVEIL_TOKEN`)
3. Config file (`cli.yml`)
4. Token file (`~/.config/apimgr/vidveil/token`)
5. Built-in defaults

## Token Storage

Tokens can be stored in multiple locations:

| Location | Method |
|----------|--------|
| Config file | `server.token` in `cli.yml` |
| Token file | `~/.config/apimgr/vidveil/token` |
| Environment | `VIDVEIL_TOKEN` |
| Command flag | `--token` or `--token-file` |

## Output Formats

### Table (default)

```
TITLE                                              DURATION  ENGINE    URL
-----                                              --------  ------    ---
Video Title One                                    10:30     pornhub   https://...
Video Title Two                                    05:45     xvideos   https://...

Found 25 results for "query"
```

### JSON

```json
{
  "ok": true,
  "query": "query",
  "count": 25,
  "results": [
    {
      "title": "Video Title One",
      "url": "https://...",
      "duration": "10:30",
      "engine": "pornhub"
    }
  ]
}
```

### Plain

```
Video Title One
  https://...
  Duration: 10:30  Views: 1.2M

Video Title Two
  https://...
  Duration: 05:45  Views: 500K

Found 25 results for "query"
```

## Bang Syntax

Bangs are shortcuts for targeting specific search engines:

| Bang | Engine |
|------|--------|
| `!ph` | PornHub |
| `!xv` | XVideos |
| `!xn` | xnxx |
| `!rt` | RedTube |
| `!yp` | YouPorn |

Use `vidveil-cli bangs` to see all available bangs.

**Usage:**

```bash
# Single engine
vidveil-cli search "!ph amateur"

# Multiple engines
vidveil-cli search "!ph !xv !rt amateur"
```

## Interactive TUI Mode

When launched without arguments in an interactive terminal, the CLI opens a full-screen TUI interface with:

- Real-time search with SSE streaming results
- Keyboard navigation
- Engine filtering
- Result preview
- Bang autocomplete

### TUI Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Execute search |
| `Tab` | Cycle through panels |
| `↑/↓` | Navigate results |
| `Enter` (on result) | Open in browser |
| `q` or `Ctrl+C` | Quit |
| `?` | Show help |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |

## Examples

### Typical Workflow

```bash
# First run - setup wizard
vidveil-cli

# Quick searches
vidveil-cli "amateur"
vidveil-cli "!ph amateur"

# Check available engines
vidveil-cli engines --enabled

# Get JSON for scripting
vidveil-cli --output json "query" | jq '.results[].url'

# Pipe to other tools
vidveil-cli --output plain "query" | head -20
```

### Scripting

```bash
#!/bin/bash
# Search and download thumbnails

results=$(vidveil-cli --output json "query")
echo "$results" | jq -r '.results[].thumbnail' | while read url; do
    curl -sO "$url"
done
```
