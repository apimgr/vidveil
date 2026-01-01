# Fish completion for vidveil-cli
# See AI.md PART 34 for CLI client specification

# Commands
complete -c vidveil-cli -n '__fish_use_subcommand' -a search -d 'Search for content'
complete -c vidveil-cli -n '__fish_use_subcommand' -a config -d 'Manage configuration'
complete -c vidveil-cli -n '__fish_use_subcommand' -a tui -d 'Launch interactive TUI'
complete -c vidveil-cli -n '__fish_use_subcommand' -a help -d 'Show help'
complete -c vidveil-cli -n '__fish_use_subcommand' -a version -d 'Show version'

# Global options
complete -c vidveil-cli -l config -d 'Config file' -r -F
complete -c vidveil-cli -l server -d 'Server address' -r
complete -c vidveil-cli -l token -d 'API token' -r
complete -c vidveil-cli -l output -d 'Output format' -xa 'json yaml table'
complete -c vidveil-cli -l no-color -d 'Disable colored output'
complete -c vidveil-cli -l timeout -d 'Request timeout (seconds)' -xa '10 30 60'
complete -c vidveil-cli -l tui -d 'Launch TUI mode'
