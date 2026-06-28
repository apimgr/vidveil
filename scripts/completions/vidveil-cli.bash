# Bash completion for vidveil-cli
# See AI.md PART 8 for CLI client specification

_vidveil_cli() {
    local cur prev opts commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="search engines bangs login probe"
    opts="--shell --config --server --token --token-file --output --color --lang --timeout --debug --update -h --help -v --version"

    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
        return 0
    fi

    case "${prev}" in
        --config|--token-file)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
        --server)
            COMPREPLY=( $(compgen -W "http://localhost:80 https://localhost:443" -- "${cur}") )
            return 0
            ;;
        --output)
            COMPREPLY=( $(compgen -W "json yaml csv table plain" -- "${cur}") )
            return 0
            ;;
        --color)
            COMPREPLY=( $(compgen -W "always never auto" -- "${cur}") )
            return 0
            ;;
        --timeout)
            COMPREPLY=( $(compgen -W "10 30 60" -- "${cur}") )
            return 0
            ;;
        --shell)
            COMPREPLY=( $(compgen -W "completions init --help" -- "${cur}") )
            return 0
            ;;
        completions|init)
            COMPREPLY=( $(compgen -W "bash zsh fish sh dash ksh powershell pwsh" -- "${cur}") )
            return 0
            ;;
        *)
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
    return 0
}

complete -F _vidveil_cli vidveil-cli
