#compdef vidveil
# Zsh completion for vidveil
# See AI.md PART 7 for CLI specification

_vidveil() {
    local -a opts
    opts=(
        '(- *)'{-h,--help}'[Show help]'
        '(- *)'{-v,--version}'[Show version]'
        '--mode[Application mode]:mode:(production development testing)'
        '--config[Config directory]:directory:_files -/'
        '--data[Data directory]:directory:_files -/'
        '--cache[Cache directory]:directory:_files -/'
        '--log[Log directory]:directory:_files -/'
        '--pid[PID file]:file:_files'
        '--address[Listen address]:address:'
        '--port[Listen port]:port:'
        '--baseurl[Base URL]:url:'
        '--lang[Default language]:lang:'
        '--color[Color output]:color:(always never auto)'
        '--debug[Enable debug mode]'
        '--status[Show status and health]'
        '--service[Service management]:action:(start restart stop reload --install --uninstall --disable --help)'
        '--daemon[Daemonize (detach from terminal)]'
        '--maintenance[Maintenance operations]:operation:(backup restore update mode setup)'
        '--backup[Backup data directory]'
        '--update[Update management]:action:(check yes --branch)'
        '--shell[Shell integration command]:shell command:(completions init --help)'
    )
    _arguments -s $opts
}

_vidveil "$@"
