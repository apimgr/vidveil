# Fish completion for vidveil-cli
# See AI.md PART 8 for CLI client specification

# Disable file completion by default
complete -c vidveil-cli -f

# Commands
complete -c vidveil-cli -n '__fish_use_subcommand' -a search -d 'Search for videos'
complete -c vidveil-cli -n '__fish_use_subcommand' -a engines -d 'List available search engines'
complete -c vidveil-cli -n '__fish_use_subcommand' -a bangs -d 'List bang shortcuts'
complete -c vidveil-cli -n '__fish_use_subcommand' -a login -d 'Save API token for future use'
complete -c vidveil-cli -n '__fish_use_subcommand' -a probe -d 'Test engine availability'

# Global options
complete -c vidveil-cli -l shell -d 'Shell integration command' -xa 'completions init --help'
complete -c vidveil-cli -l config -d 'Config file' -r -F
complete -c vidveil-cli -l server -d 'Server address' -r
complete -c vidveil-cli -l token -d 'API token' -r
complete -c vidveil-cli -l token-file -d 'Token file' -r -F
complete -c vidveil-cli -l output -d 'Output format' -xa 'json yaml csv table plain'
complete -c vidveil-cli -l color -d 'Color output' -xa 'always never auto'
complete -c vidveil-cli -l lang -d 'Language' -r
complete -c vidveil-cli -l timeout -d 'Request timeout (seconds)' -xa '10 30 60'
complete -c vidveil-cli -l debug -d 'Enable debug output'
complete -c vidveil-cli -l update -d 'Update the binary' -xa 'check yes'
complete -c vidveil-cli -s h -l help -d 'Show help'
complete -c vidveil-cli -s v -l version -d 'Show version'

# Shell type completions after --shell completions or --shell init
complete -c vidveil-cli -n '__fish_seen_argument -l shell; and __fish_prev_arg_in completions init' -a 'bash zsh fish sh dash ksh powershell pwsh'
