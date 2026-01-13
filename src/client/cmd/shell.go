// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - Shell Completion Command
package cmd

import (
	"fmt"
	"os"
)

// RunShellCommand handles shell completion commands
// Per AI.md PART 1: Function names MUST reveal intent - "runShell" is ambiguous
// Per AI.md PART 36: Built-in shell completion support
func RunShellCommand(args []string) error {
	if len(args) == 0 {
		PrintShellCommandHelp()
		return nil
	}

	switch args[0] {
	case "completions":
		if len(args) < 2 {
			return fmt.Errorf("usage: %s shell completions <bash|zsh|fish|powershell>", BinaryName)
		}
		return OutputShellCompletionScript(args[1])
	case "init":
		if len(args) < 2 {
			return fmt.Errorf("usage: %s shell init <bash|zsh|fish|powershell>", BinaryName)
		}
		return OutputShellInitSnippet(args[1])
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
  %s shell <command> <shell>

Commands:
  completions <shell>    Output shell completion script
  init <shell>           Output shell initialization snippet

Supported shells:
  bash       Bash shell
  zsh        Zsh shell
  fish       Fish shell
  powershell PowerShell

Examples:
  # Bash - add to ~/.bashrc
  %s shell completions bash > ~/.local/share/bash-completion/completions/%s
  # Or source directly:
  source <(%s shell completions bash)

  # Zsh - add to ~/.zshrc
  source <(%s shell completions zsh)

  # Fish - add to fish config
  %s shell completions fish > ~/.config/fish/completions/%s.fish

  # PowerShell - add to profile
  %s shell completions powershell >> $PROFILE
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

// OutputShellCompletionScript outputs the completion script for the specified shell
// Per AI.md PART 1: Function names MUST reveal intent - "shellCompletions" is ambiguous
func OutputShellCompletionScript(shellType string) error {
	switch shellType {
	case "bash":
		return OutputBashCompletionScript()
	case "zsh":
		return OutputZshCompletionScript()
	case "fish":
		return OutputFishCompletionScript()
	case "powershell", "pwsh":
		return OutputPowershellCompletionScript()
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, or powershell)", shellType)
	}
}

// OutputShellInitSnippet outputs the initialization snippet for the specified shell
// Per AI.md PART 1: Function names MUST reveal intent - "shellInit" is ambiguous
func OutputShellInitSnippet(shellType string) error {
	switch shellType {
	case "bash":
		fmt.Printf("source <(%s shell completions bash)\n", os.Args[0])
	case "zsh":
		fmt.Printf("source <(%s shell completions zsh)\n", os.Args[0])
	case "fish":
		fmt.Printf("%s shell completions fish | source\n", os.Args[0])
	case "powershell", "pwsh":
		fmt.Printf("%s shell completions powershell | Out-String | Invoke-Expression\n", os.Args[0])
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, or powershell)", shellType)
	}
	return nil
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

    commands="search login shell help version"
    opts="--server --token --token-file --config --output --no-color --timeout --debug -h --help -v --version"

    case "${prev}" in
        --server|--token|--token-file|--config)
            return 0
            ;;
        --output)
            COMPREPLY=( $(compgen -W "json table plain" -- "${cur}") )
            return 0
            ;;
        --timeout)
            return 0
            ;;
        shell)
            COMPREPLY=( $(compgen -W "completions init" -- "${cur}") )
            return 0
            ;;
        completions|init)
            COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- "${cur}") )
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
# Add to ~/.zshrc: source <(%s shell completions zsh)

__%s() {
    local -a commands
    commands=(
        'search:Search for videos'
        'login:Save API token for future use'
        'shell:Shell completion commands'
        'help:Show help'
        'version:Show version'
    )

    local -a opts
    opts=(
        '--server[Server address]:url:'
        '--token[API token]:token:'
        '--token-file[Token file]:file:_files'
        '--config[Config file]:file:_files'
        '--output[Output format]:format:(json table plain)'
        '--no-color[Disable colored output]'
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
                shell)
                    local -a shell_cmds
                    shell_cmds=(
                        'completions:Output completion script'
                        'init:Output init snippet'
                    )
                    _describe -t shell_cmds 'shell command' shell_cmds
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
complete -c %s -n "__fish_use_subcommand" -a "login" -d "Save API token"
complete -c %s -n "__fish_use_subcommand" -a "shell" -d "Shell completion commands"
complete -c %s -n "__fish_use_subcommand" -a "help" -d "Show help"
complete -c %s -n "__fish_use_subcommand" -a "version" -d "Show version"

# Global flags
complete -c %s -l server -d "Server address" -r
complete -c %s -l token -d "API token" -r
complete -c %s -l token-file -d "Token file" -r -F
complete -c %s -l config -d "Config file" -r -F
complete -c %s -l output -d "Output format" -r -a "json table plain"
complete -c %s -l no-color -d "Disable colored output"
complete -c %s -l timeout -d "Request timeout" -r
complete -c %s -l debug -d "Enable debug output"
complete -c %s -s h -l help -d "Show help"
complete -c %s -s v -l version -d "Show version"

# Shell subcommands
complete -c %s -n "__fish_seen_subcommand_from shell" -a "completions" -d "Output completion script"
complete -c %s -n "__fish_seen_subcommand_from shell" -a "init" -d "Output init snippet"

# Shell types
complete -c %s -n "__fish_seen_subcommand_from completions init" -a "bash zsh fish powershell"
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName,
		BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName,
		BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
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
        [CompletionResult]::new('login', 'login', 'ParameterValue', 'Save API token')
        [CompletionResult]::new('shell', 'shell', 'ParameterValue', 'Shell completion commands')
        [CompletionResult]::new('help', 'help', 'ParameterValue', 'Show help')
        [CompletionResult]::new('version', 'version', 'ParameterValue', 'Show version')
    )

    $flags = @(
        [CompletionResult]::new('--server', '--server', 'ParameterName', 'Server address')
        [CompletionResult]::new('--token', '--token', 'ParameterName', 'API token')
        [CompletionResult]::new('--token-file', '--token-file', 'ParameterName', 'Token file')
        [CompletionResult]::new('--config', '--config', 'ParameterName', 'Config file')
        [CompletionResult]::new('--output', '--output', 'ParameterName', 'Output format')
        [CompletionResult]::new('--no-color', '--no-color', 'ParameterName', 'Disable colors')
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
    } elseif ($elements[1].Value -eq 'shell' -and $elements.Count -eq 2) {
        @(
            [CompletionResult]::new('completions', 'completions', 'ParameterValue', 'Output completion script')
            [CompletionResult]::new('init', 'init', 'ParameterValue', 'Output init snippet')
        ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    } elseif ($elements[1].Value -eq 'shell' -and $elements.Count -eq 3) {
        @(
            [CompletionResult]::new('bash', 'bash', 'ParameterValue', 'Bash shell')
            [CompletionResult]::new('zsh', 'zsh', 'ParameterValue', 'Zsh shell')
            [CompletionResult]::new('fish', 'fish', 'ParameterValue', 'Fish shell')
            [CompletionResult]::new('powershell', 'powershell', 'ParameterValue', 'PowerShell')
        ) | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    } else {
        $flags | Where-Object { $_.CompletionText -like "$wordToComplete*" }
    }
}
`, BinaryName, BinaryName)
	return nil
}
