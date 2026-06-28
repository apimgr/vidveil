# PowerShell completion for vidveil-cli
# See AI.md PART 8 for CLI client specification
# Add to your PowerShell profile:
#   . /path/to/vidveil-cli.ps1

Register-ArgumentCompleter -Native -CommandName vidveil-cli -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @(
        [CompletionResult]::new('search', 'search', 'ParameterValue', 'Search for videos')
        [CompletionResult]::new('engines', 'engines', 'ParameterValue', 'List available search engines')
        [CompletionResult]::new('bangs', 'bangs', 'ParameterValue', 'List bang shortcuts')
        [CompletionResult]::new('login', 'login', 'ParameterValue', 'Save API token for future use')
        [CompletionResult]::new('probe', 'probe', 'ParameterValue', 'Test engine availability')
    )

    $flags = @(
        [CompletionResult]::new('--shell', '--shell', 'ParameterName', 'Shell integration command')
        [CompletionResult]::new('--config', '--config', 'ParameterName', 'Config file')
        [CompletionResult]::new('--server', '--server', 'ParameterName', 'Server address')
        [CompletionResult]::new('--token', '--token', 'ParameterName', 'API token')
        [CompletionResult]::new('--token-file', '--token-file', 'ParameterName', 'Token file')
        [CompletionResult]::new('--output', '--output', 'ParameterName', 'Output format')
        [CompletionResult]::new('--color', '--color', 'ParameterName', 'Color output (always/never/auto)')
        [CompletionResult]::new('--lang', '--lang', 'ParameterName', 'Language')
        [CompletionResult]::new('--timeout', '--timeout', 'ParameterName', 'Request timeout in seconds')
        [CompletionResult]::new('--debug', '--debug', 'ParameterName', 'Enable debug output')
        [CompletionResult]::new('--update', '--update', 'ParameterName', 'Update the binary')
        [CompletionResult]::new('-h', '-h', 'ParameterName', 'Show help')
        [CompletionResult]::new('--help', '--help', 'ParameterName', 'Show help')
        [CompletionResult]::new('-v', '-v', 'ParameterName', 'Show version')
        [CompletionResult]::new('--version', '--version', 'ParameterName', 'Show version')
    )

    $elements = $commandAst.CommandElements
    $prevArg = if ($elements.Count -ge 2) { $elements[$elements.Count - 2].Value } else { '' }

    switch ($prevArg) {
        '--output' {
            @(
                [CompletionResult]::new('json', 'json', 'ParameterValue', 'JSON output')
                [CompletionResult]::new('yaml', 'yaml', 'ParameterValue', 'YAML output')
                [CompletionResult]::new('csv', 'csv', 'ParameterValue', 'CSV output')
                [CompletionResult]::new('table', 'table', 'ParameterValue', 'Table output')
                [CompletionResult]::new('plain', 'plain', 'ParameterValue', 'Plain text output')
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
        '--update' {
            @(
                [CompletionResult]::new('check', 'check', 'ParameterValue', 'Check for updates')
                [CompletionResult]::new('yes', 'yes', 'ParameterValue', 'Apply update')
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

    if ($elements.Count -eq 1) {
        $commands + $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    } else {
        $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    }
}
