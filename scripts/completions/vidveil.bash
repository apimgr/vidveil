# Bash completion for vidveil
# See AI.md PART 7 for CLI specification

_vidveil() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    opts="--help --version --mode --config --data --log --pid --address --port --debug --status --service --daemon --maintenance --update"

    case "${prev}" in
        --mode)
            COMPREPLY=( $(compgen -W "production development" -- ${cur}) )
            return 0
            ;;
        --service)
            COMPREPLY=( $(compgen -W "start restart stop reload --install --uninstall --disable --help" -- ${cur}) )
            return 0
            ;;
        --maintenance)
            COMPREPLY=( $(compgen -W "backup restore update mode setup" -- ${cur}) )
            return 0
            ;;
        --update)
            COMPREPLY=( $(compgen -W "check yes --branch" -- ${cur}) )
            return 0
            ;;
        --config|--data|--log|--pid)
            COMPREPLY=( $(compgen -d -- ${cur}) )
            return 0
            ;;
        *)
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _vidveil vidveil
