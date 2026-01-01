# Fish completion for vidveil
# See AI.md PART 7 for CLI specification

# Options
complete -c vidveil -s h -l help -d 'Show help'
complete -c vidveil -s v -l version -d 'Show version'
complete -c vidveil -l mode -d 'Application mode' -xa 'production development'
complete -c vidveil -l config -d 'Config directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l data -d 'Data directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l log -d 'Log directory' -r -f -a '(__fish_complete_directories)'
complete -c vidveil -l pid -d 'PID file' -r -F
complete -c vidveil -l address -d 'Listen address' -r
complete -c vidveil -l port -d 'Listen port' -r
complete -c vidveil -l debug -d 'Enable debug mode'
complete -c vidveil -l status -d 'Show status and health'
complete -c vidveil -l service -d 'Service management' -xa 'start restart stop reload --install --uninstall --disable --help'
complete -c vidveil -l daemon -d 'Daemonize (detach from terminal)'
complete -c vidveil -l maintenance -d 'Maintenance operations' -xa 'backup restore update mode setup'
complete -c vidveil -l update -d 'Update management' -xa 'check yes --branch'
