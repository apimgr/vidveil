#compdef vidveil-cli
# Zsh completion for vidveil-cli
# See AI.md PART 8 for CLI client specification

_vidveil_cli() {
    local -a commands
    commands=(
        'search:Search for videos'
        'engines:List available search engines'
        'bangs:List bang shortcuts'
        'login:Save API token for future use'
        'probe:Test engine availability'
    )

    local -a opts
    opts=(
        '--shell[Shell integration command]:shell command:(completions init --help)'
        '--config[Config file]:file:_files'
        '--server[Server address]:url:'
        '--token[API token]:token:'
        '--token-file[Token file]:file:_files'
        '--output[Output format]:format:(json yaml csv table plain)'
        '--color[Color output]:color:(always never auto)'
        '--lang[Language]:lang:'
        '--timeout[Request timeout]:seconds:(10 30 60)'
        '--debug[Enable debug output]'
        '--update[Update the binary]:action:(check yes)'
        '-h[Show help]'
        '--help[Show help]'
        '-v[Show version]'
        '--version[Show version]'
    )

    if (( CURRENT == 2 )); then
        _describe 'commands' commands
    else
        _arguments -s $opts
    fi
}

_vidveil_cli "$@"
