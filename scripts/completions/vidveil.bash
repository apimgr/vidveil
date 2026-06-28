# Bash completion for vidveil
# See AI.md PART 7 for CLI specification

_vidveil() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    opts="--help --version --mode --config --data --cache --log --pid --address --port --baseurl --lang --color --debug --status --service --daemon --maintenance --backup --update --shell"

    case "${prev}" in
        --mode)
            COMPREPLY=( $(compgen -W "production development testing" -- "${cur}") )
            return 0
            ;;
        --service)
            COMPREPLY=( $(compgen -W "start restart stop reload --install --uninstall --disable --help" -- "${cur}") )
            return 0
            ;;
        --maintenance)
            COMPREPLY=( $(compgen -W "backup restore update mode setup" -- "${cur}") )
            return 0
            ;;
        --update)
            COMPREPLY=( $(compgen -W "check yes --branch" -- "${cur}") )
            return 0
            ;;
        --color)
            COMPREPLY=( $(compgen -W "always never auto" -- "${cur}") )
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
        --config|--data|--cache|--log|--pid)
            COMPREPLY=( $(compgen -d -- "${cur}") )
            return 0
            ;;
        *)
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
    return 0
}

complete -F _vidveil vidveil
