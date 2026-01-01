#compdef vidveil
# Zsh completion for vidveil
# See AI.md PART 7 for CLI specification

_vidveil() {
    local -a opts
    opts=(
        '(- *)'{-h,--help}'[Show help]'
        '(- *)'{-v,--version}'[Show version]'
        '--mode[Application mode]:mode:(production development)'
        '--config[Config directory]:directory:_files -/'
        '--data[Data directory]:directory:_files -/'
        '--log[Log directory]:directory:_files -/'
        '--pid[PID file]:file:_files'
        '--address[Listen address]:address:'
        '--port[Listen port]:port:'
        '--debug[Enable debug mode]'
        '--status[Show status and health]'
        '--service[Service management]:action:(start restart stop reload --install --uninstall --disable --help)'
        '--daemon[Daemonize (detach from terminal)]'
        '--maintenance[Maintenance operations]:operation:(backup restore update mode setup)'
        '--update[Update management]:action:(check yes --branch)'
    )
    _arguments -s $opts
}

_vidveil "$@"
