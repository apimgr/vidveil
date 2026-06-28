# Fish completion for vidveil
# See AI.md PART 7 for CLI specification

# Options
complete -c vidveil -s h -l help -d 'Show help'
complete -c vidveil -s v -l version -d 'Show version'
complete -c vidveil -l mode -d 'Application mode' -xa 'production development testing'
complete -c vidveil -l config -d 'Config directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l data -d 'Data directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l cache -d 'Cache directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l log -d 'Log directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l pid -d 'PID file' -r -F
complete -c vidveil -l address -d 'Listen address' -r
complete -c vidveil -l port -d 'Listen port' -r
complete -c vidveil -l baseurl -d 'Base URL' -r
complete -c vidveil -l lang -d 'Default language' -r
complete -c vidveil -l color -d 'Color output' -xa 'always never auto'
complete -c vidveil -l debug -d 'Enable debug mode'
complete -c vidveil -l status -d 'Show status and health'
complete -c vidveil -l service -d 'Service management' -xa 'start restart stop reload --install --uninstall --disable --help'
complete -c vidveil -l daemon -d 'Daemonize (detach from terminal)'
complete -c vidveil -l maintenance -d 'Maintenance operations' -xa 'backup restore update mode setup'
complete -c vidveil -l backup -d 'Backup data directory'
complete -c vidveil -l update -d 'Update management' -xa 'check yes --branch'
complete -c vidveil -l shell -d 'Shell integration command' -xa 'completions init --help'

# Shell type completions after --shell completions or --shell init
complete -c vidveil -n '__fish_seen_argument -l shell; and __fish_prev_arg_in completions init' -a 'bash zsh fish sh dash ksh powershell pwsh'
