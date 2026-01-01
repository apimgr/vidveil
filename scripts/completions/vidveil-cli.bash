# Bash completion for vidveil-cli
# See AI.md PART 34 for CLI client specification

_vidveil_cli() {
    local cur prev opts commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="search config tui help version"
    opts="--config --server --token --output --no-color --timeout --tui"

    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
        return 0
    fi

    case "${prev}" in
        --config)
            COMPREPLY=( $(compgen -f -- ${cur}) )
            return 0
            ;;
        --server)
            COMPREPLY=( $(compgen -W "http://localhost:80 https://localhost:443" -- ${cur}) )
            return 0
            ;;
        --output)
            COMPREPLY=( $(compgen -W "json yaml table" -- ${cur}) )
            return 0
            ;;
        --timeout)
            COMPREPLY=( $(compgen -W "10 30 60" -- ${cur}) )
            return 0
            ;;
        *)
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _vidveil_cli vidveil-cli
