// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Shell Completion Command
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// RunShellCommand handles shell completion commands
// Per AI.md PART 1: Function names MUST reveal intent - "runShell" is ambiguous
// Per AI.md PART 33: Built-in shell completion support
func RunShellCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s --shell [completions|init|--help] [shell]", BinaryName)
	}

	shellType := ""
	if len(args) > 1 {
		shellType = args[1]
	}

	switch args[0] {
	case "completions":
		return OutputShellCompletionScript(shellType)
	case "init":
		return OutputShellInitSnippet(shellType)
	case "-h", "--help":
		PrintShellCommandHelp()
		return nil
	default:
		return fmt.Errorf("unknown shell command: %s", args[0])
	}
}

// PrintShellCommandHelp prints help for the shell command
// Per AI.md PART 1: Function names MUST reveal intent - "shellHelp" is ambiguous
func PrintShellCommandHelp() {
	fmt.Printf(`Shell completion commands

Usage:
  %s --shell [completions|init|--help] [shell]

Commands:
  completions [shell]    Output shell completion script
  init [shell]           Output shell initialization snippet

Supported shells:
  bash       Bash shell
  zsh        Zsh shell
  fish       Fish shell
  sh         POSIX shell
  dash       POSIX shell
  ksh        Korn shell
  powershell PowerShell
  pwsh       PowerShell Core

Examples:
  # Auto-detect from $SHELL
  eval "$(%s --shell init)"

  # Bash - add to ~/.bashrc
  %s --shell completions bash > ~/.local/share/bash-completion/completions/%s
  # Or source directly:
  source <(%s --shell completions bash)

  # Zsh - add to ~/.zshrc
  source <(%s --shell completions zsh)

  # Fish - add to fish config
  %s --shell completions fish > ~/.config/fish/completions/%s.fish

  # PowerShell - add to profile
  %s --shell completions powershell >> $PROFILE
 `, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

// OutputShellCompletionScript outputs the completion script for the specified shell
// Per AI.md PART 1: Function names MUST reveal intent - "shellCompletions" is ambiguous
func OutputShellCompletionScript(shellType string) error {
	if shellType == "" {
		shellType = DetectCurrentShellType()
	}

	switch shellType {
	case "bash":
		return OutputBashCompletionScript()
	case "zsh":
		return OutputZshCompletionScript()
	case "fish":
		return OutputFishCompletionScript()
	case "sh", "dash", "ksh":
		return OutputBashCompletionScript()
	case "powershell", "pwsh":
		return OutputPowershellCompletionScript()
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, sh, dash, ksh, powershell, or pwsh)", shellType)
	}
}

// OutputShellInitSnippet outputs the initialization snippet for the specified shell
// Per AI.md PART 1: Function names MUST reveal intent - "shellInit" is ambiguous
func OutputShellInitSnippet(shellType string) error {
	if shellType == "" {
		shellType = DetectCurrentShellType()
	}

	binaryName := filepath.Base(os.Args[0])

	switch shellType {
	case "bash":
		fmt.Printf("source <(%s --shell completions bash)\n", binaryName)
	case "zsh":
		fmt.Printf("source <(%s --shell completions zsh)\n", binaryName)
	case "fish":
		fmt.Printf("%s --shell completions fish | source\n", binaryName)
	case "sh", "dash", "ksh":
		fmt.Printf("eval \"$(%s --shell completions %s)\"\n", binaryName, shellType)
	case "powershell", "pwsh":
		fmt.Printf("Invoke-Expression (& %s --shell completions powershell)\n", binaryName)
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, sh, dash, ksh, powershell, or pwsh)", shellType)
	}
	return nil
}

// DetectCurrentShellType returns the current shell name from $SHELL or a safe default.
func DetectCurrentShellType() string {
	shellEnv := os.Getenv("SHELL")
	if shellEnv == "" {
		return "bash"
	}

	return filepath.Base(shellEnv)
}

// OutputBashCompletionScript outputs bash completion script
// Per AI.md PART 1: Function names MUST reveal intent - "bashCompletions" is ambiguous
func OutputBashCompletionScript() error {
	fmt.Printf(`# Bash completion for %s
# Add to ~/.bashrc or ~/.local/share/bash-completion/completions/%s

_%s_completions() {
    local cur prev opts commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="search engines bangs probe login"
    opts="--shell --server --token --token-file --config --output --color --timeout --debug -h --help -v --version"

    case "${prev}" in
        --server|--token|--token-file|--config)
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
    esac

    if [[ "${cur}" == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
    elif [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    fi
}

complete -F _%s_completions %s
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
	return nil
}

// OutputZshCompletionScript outputs zsh completion script
// Per AI.md PART 1: Function names MUST reveal intent - "zshCompletions" is ambiguous
func OutputZshCompletionScript() error {
	fmt.Printf(`#compdef %s

# Zsh completion for %s
# Add to ~/.zshrc: source <(%s --shell completions zsh)

__%s() {
    local -a commands
    commands=(
        'search:Search for videos'
        'engines:List available search engines'
        'bangs:List bang shortcuts'
        'probe:Test engine availability'
        'login:Save API token for future use'
    )

    local -a opts
    opts=(
        '--shell[Shell integration command]:shell command:(completions init --help)'
        '--server[Server address]:url:'
        '--token[API token]:token:'
        '--token-file[Token file]:file:_files'
        '--config[Config file]:file:_files'
        '--output[Output format]:format:(json yaml csv table plain)'
        '--color[Color output]:color:(always never auto)'
        '--timeout[Request timeout in seconds]:seconds:'
        '--debug[Enable debug output]'
        '-h[Show help]'
        '--help[Show help]'
        '-v[Show version]'
        '--version[Show version]'
    )

    _arguments -s \
        $opts \
        '1: :->command' \
        '*::arg:->args'

    case "$state" in
        command)
            _describe -t commands 'command' commands
            ;;
        args)
            case "$words[1]" in
                --shell)
                    _values 'shell type' bash zsh fish sh dash ksh powershell pwsh
                    ;;
            esac
            ;;
    esac
}

compdef __%s %s
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
	return nil
}

// OutputFishCompletionScript outputs fish completion script
// Per AI.md PART 1: Function names MUST reveal intent - "fishCompletions" is ambiguous
func OutputFishCompletionScript() error {
	fmt.Printf(`# Fish completion for %s
# Save to ~/.config/fish/completions/%s.fish

# Disable file completion by default
complete -c %s -f

# Commands
complete -c %s -n "__fish_use_subcommand" -a "search" -d "Search for videos"
complete -c %s -n "__fish_use_subcommand" -a "engines" -d "List available search engines"
complete -c %s -n "__fish_use_subcommand" -a "bangs" -d "List bang shortcuts"
complete -c %s -n "__fish_use_subcommand" -a "probe" -d "Test engine availability"
complete -c %s -n "__fish_use_subcommand" -a "login" -d "Save API token for future use"

# Global flags
complete -c %s -l shell -d "Shell integration command" -r -a "completions init --help"
complete -c %s -l server -d "Server address" -r
complete -c %s -l token -d "API token" -r
complete -c %s -l token-file -d "Token file" -r -F
complete -c %s -l config -d "Config file" -r -F
complete -c %s -l output -d "Output format" -r -a "json yaml csv table plain"
complete -c %s -l color -d "Color output" -r -a "always never auto"
complete -c %s -l timeout -d "Request timeout" -r
complete -c %s -l debug -d "Enable debug output"
complete -c %s -s h -l help -d "Show help"
complete -c %s -s v -l version -d "Show version"

# Shell types
complete -c %s -n "__fish_seen_argument -l shell; and __fish_prev_arg_in completions init" -a "bash zsh fish sh dash ksh powershell pwsh"
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName,
		BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName,
		BinaryName, BinaryName, BinaryName, BinaryName)
	return nil
}

// OutputPowershellCompletionScript outputs PowerShell completion script
// Per AI.md PART 1: Function names MUST reveal intent - "powershellCompletions" is ambiguous
func OutputPowershellCompletionScript() error {
	fmt.Printf(`# PowerShell completion for %s
# Add to your PowerShell profile

Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @(
        [CompletionResult]::new('search', 'search', 'ParameterValue', 'Search for videos')
        [CompletionResult]::new('engines', 'engines', 'ParameterValue', 'List available search engines')
        [CompletionResult]::new('bangs', 'bangs', 'ParameterValue', 'List bang shortcuts')
        [CompletionResult]::new('probe', 'probe', 'ParameterValue', 'Test engine availability')
        [CompletionResult]::new('login', 'login', 'ParameterValue', 'Save API token for future use')
    )

    $flags = @(
        [CompletionResult]::new('--shell', '--shell', 'ParameterName', 'Shell integration command')
        [CompletionResult]::new('--server', '--server', 'ParameterName', 'Server address')
        [CompletionResult]::new('--token', '--token', 'ParameterName', 'API token')
        [CompletionResult]::new('--token-file', '--token-file', 'ParameterName', 'Token file')
        [CompletionResult]::new('--config', '--config', 'ParameterName', 'Config file')
        [CompletionResult]::new('--output', '--output', 'ParameterName', 'Output format')
        [CompletionResult]::new('--color', '--color', 'ParameterName', 'Color output (always/never/auto)')
        [CompletionResult]::new('--timeout', '--timeout', 'ParameterName', 'Timeout')
        [CompletionResult]::new('--debug', '--debug', 'ParameterName', 'Debug mode')
        [CompletionResult]::new('-h', '-h', 'ParameterName', 'Help')
        [CompletionResult]::new('--help', '--help', 'ParameterName', 'Help')
        [CompletionResult]::new('-v', '-v', 'ParameterName', 'Version')
        [CompletionResult]::new('--version', '--version', 'ParameterName', 'Version')
    )

    $elements = $commandAst.CommandElements

    if ($elements.Count -eq 1) {
        $commands + $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    } elseif ($elements.Count -ge 2 -and $elements[$elements.Count - 2].Value -eq '--shell') {
        @(
            [CompletionResult]::new('completions', 'completions', 'ParameterValue', 'Output completion script')
            [CompletionResult]::new('init', 'init', 'ParameterValue', 'Output init snippet')
            [CompletionResult]::new('--help', '--help', 'ParameterName', 'Show shell integration help')
        ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    } elseif ($elements.Count -ge 3 -and $elements[$elements.Count - 3].Value -eq '--shell' -and @('completions', 'init') -contains $elements[$elements.Count - 2].Value) {
        @(
            [CompletionResult]::new('bash', 'bash', 'ParameterValue', 'Bash shell')
            [CompletionResult]::new('zsh', 'zsh', 'ParameterValue', 'Zsh shell')
            [CompletionResult]::new('fish', 'fish', 'ParameterValue', 'Fish shell')
            [CompletionResult]::new('sh', 'sh', 'ParameterValue', 'POSIX shell')
            [CompletionResult]::new('dash', 'dash', 'ParameterValue', 'POSIX shell')
            [CompletionResult]::new('ksh', 'ksh', 'ParameterValue', 'Korn shell')
            [CompletionResult]::new('powershell', 'powershell', 'ParameterValue', 'PowerShell')
            [CompletionResult]::new('pwsh', 'pwsh', 'ParameterValue', 'PowerShell Core')
        ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    } else {
        $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    }
}
`, BinaryName, BinaryName)
	return nil
}
