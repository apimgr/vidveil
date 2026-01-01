#compdef vidveil-cli
# Zsh completion for vidveil-cli
# See AI.md PART 34 for CLI client specification

_vidveil_cli() {
    local -a commands
    commands=(
        'search:Search for content'
        'config:Manage configuration'
        'tui:Launch interactive TUI'
        'help:Show help'
        'version:Show version'
    )

    local -a opts
    opts=(
        '--config[Config file]:file:_files'
        '--server[Server address]:url:'
        '--token[API token]:token:'
        '--output[Output format]:format:(json yaml table)'
        '--no-color[Disable colored output]'
        '--timeout[Request timeout]:seconds:(10 30 60)'
        '--tui[Launch TUI mode]'
    )

    if (( CURRENT == 2 )); then
        _describe 'commands' commands
    else
        _arguments -s $opts
    fi
}

_vidveil_cli "$@"
