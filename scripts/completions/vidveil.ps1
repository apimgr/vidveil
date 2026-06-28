# PowerShell completion for vidveil
# See AI.md PART 7 for CLI specification
# Add to your PowerShell profile:
#   . /path/to/vidveil.ps1

Register-ArgumentCompleter -Native -CommandName vidveil -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $flags = @(
        [CompletionResult]::new('--help', '--help', 'ParameterName', 'Show help')
        [CompletionResult]::new('--version', '--version', 'ParameterName', 'Show version')
        [CompletionResult]::new('--mode', '--mode', 'ParameterName', 'Application mode (production/development/testing)')
        [CompletionResult]::new('--config', '--config', 'ParameterName', 'Config directory')
        [CompletionResult]::new('--data', '--data', 'ParameterName', 'Data directory')
        [CompletionResult]::new('--cache', '--cache', 'ParameterName', 'Cache directory')
        [CompletionResult]::new('--log', '--log', 'ParameterName', 'Log directory')
        [CompletionResult]::new('--pid', '--pid', 'ParameterName', 'PID file')
        [CompletionResult]::new('--address', '--address', 'ParameterName', 'Listen address')
        [CompletionResult]::new('--port', '--port', 'ParameterName', 'Listen port')
        [CompletionResult]::new('--baseurl', '--baseurl', 'ParameterName', 'Base URL')
        [CompletionResult]::new('--lang', '--lang', 'ParameterName', 'Default language')
        [CompletionResult]::new('--color', '--color', 'ParameterName', 'Color output (always/never/auto)')
        [CompletionResult]::new('--debug', '--debug', 'ParameterName', 'Enable debug mode')
        [CompletionResult]::new('--status', '--status', 'ParameterName', 'Show status and health')
        [CompletionResult]::new('--service', '--service', 'ParameterName', 'Service management')
        [CompletionResult]::new('--daemon', '--daemon', 'ParameterName', 'Daemonize (detach from terminal)')
        [CompletionResult]::new('--maintenance', '--maintenance', 'ParameterName', 'Maintenance operations')
        [CompletionResult]::new('--backup', '--backup', 'ParameterName', 'Backup data directory')
        [CompletionResult]::new('--update', '--update', 'ParameterName', 'Update management')
        [CompletionResult]::new('--shell', '--shell', 'ParameterName', 'Shell integration command')
    )

    $elements = $commandAst.CommandElements
    $prevArg = if ($elements.Count -ge 2) { $elements[$elements.Count - 2].Value } else { '' }

    switch ($prevArg) {
        '--mode' {
            @(
                [CompletionResult]::new('production', 'production', 'ParameterValue', 'Production mode')
                [CompletionResult]::new('development', 'development', 'ParameterValue', 'Development mode')
                [CompletionResult]::new('testing', 'testing', 'ParameterValue', 'Testing mode')
            ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            return
        }
        '--color' {
            @(
                [CompletionResult]::new('always', 'always', 'ParameterValue', 'Always use color')
                [CompletionResult]::new('never', 'never', 'ParameterValue', 'Never use color')
                [CompletionResult]::new('auto', 'auto', 'ParameterValue', 'Auto-detect color support')
            ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            return
        }
        '--service' {
            @(
                [CompletionResult]::new('start', 'start', 'ParameterValue', 'Start service')
                [CompletionResult]::new('restart', 'restart', 'ParameterValue', 'Restart service')
                [CompletionResult]::new('stop', 'stop', 'ParameterValue', 'Stop service')
                [CompletionResult]::new('reload', 'reload', 'ParameterValue', 'Reload service')
                [CompletionResult]::new('--install', '--install', 'ParameterName', 'Install as system service')
                [CompletionResult]::new('--uninstall', '--uninstall', 'ParameterName', 'Uninstall service')
            ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            return
        }
        '--update' {
            @(
                [CompletionResult]::new('check', 'check', 'ParameterValue', 'Check for updates')
                [CompletionResult]::new('yes', 'yes', 'ParameterValue', 'Apply update')
                [CompletionResult]::new('--branch', '--branch', 'ParameterName', 'Specify branch')
            ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            return
        }
        '--shell' {
            @(
                [CompletionResult]::new('completions', 'completions', 'ParameterValue', 'Output completion script')
                [CompletionResult]::new('init', 'init', 'ParameterValue', 'Output init snippet')
                [CompletionResult]::new('--help', '--help', 'ParameterName', 'Show shell integration help')
            ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            return
        }
        { @('completions', 'init') -contains $_ } {
            @(
                [CompletionResult]::new('bash', 'bash', 'ParameterValue', 'Bash shell')
                [CompletionResult]::new('zsh', 'zsh', 'ParameterValue', 'Zsh shell')
                [CompletionResult]::new('fish', 'fish', 'ParameterValue', 'Fish shell')
                [CompletionResult]::new('sh', 'sh', 'ParameterValue', 'POSIX shell')
                [CompletionResult]::new('dash', 'dash', 'ParameterValue', 'POSIX shell (dash)')
                [CompletionResult]::new('ksh', 'ksh', 'ParameterValue', 'Korn shell')
                [CompletionResult]::new('powershell', 'powershell', 'ParameterValue', 'PowerShell')
                [CompletionResult]::new('pwsh', 'pwsh', 'ParameterValue', 'PowerShell Core')
            ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
            return
        }
    }

    $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
}
