// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Engines Command
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Engine display constants
// Per AI.md PART 1: No magic strings - use named constants
const (
	EngineStatusEnabled  = "enabled"
	EngineStatusDisabled = "disabled"
	EngineDataNotAvail   = "-"
	EngineDataYes        = "yes"
)

// Engines command flags
// Per AI.md PART 1: Variable names MUST reveal intent
var (
	enginesShowEnabledOnly  bool
	enginesShowDisabledOnly bool
	enginesShowAllDetails   bool
)

// EngineInfo represents engine information from the server
// Per AI.md PART 1: Type names MUST be specific
type EngineInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Bang        string `json:"bang"`
	Tier        int    `json:"tier"`
	Enabled     bool   `json:"enabled"`
	Method      string `json:"method"`
	HasPreview  bool   `json:"has_preview"`
	HasDownload bool   `json:"has_download"`
}

// EnginesListResponse represents the API response for engines list
// Per AI.md PART 1: Type names MUST be specific
type EnginesListResponse struct {
	Ok      bool         `json:"ok"`
	Engines []EngineInfo `json:"engines"`
	Count   int          `json:"count"`
	Error   string       `json:"error,omitempty"`
}

// RunEnginesCommand runs the engines command
// Per AI.md PART 1: Function names MUST reveal intent
func RunEnginesCommand(args []string) error {
	// Reset flags
	enginesShowEnabledOnly = false
	enginesShowDisabledOnly = false
	enginesShowAllDetails = false

	// Parse engines-specific flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--enabled":
			enginesShowEnabledOnly = true
		case "--disabled":
			enginesShowDisabledOnly = true
		case "--all":
			enginesShowAllDetails = true
		case "--help", "-h":
			PrintEnginesCommandHelp()
			return nil
		}
	}

	// Fetch engines from server
	enginesData, err := FetchEnginesList()
	if err != nil {
		return fmt.Errorf("failed to fetch engines: %w", err)
	}

	// Filter based on flags
	var filteredEngines []EngineInfo
	for _, engine := range enginesData.Engines {
		if enginesShowEnabledOnly && !engine.Enabled {
			continue
		}
		if enginesShowDisabledOnly && engine.Enabled {
			continue
		}
		filteredEngines = append(filteredEngines, engine)
	}

	// Output results
	switch cliConfig.Output.Format {
	case "json":
		return OutputEnginesAsJSON(filteredEngines)
	case "plain":
		return OutputEnginesAsPlain(filteredEngines)
	default:
		return OutputEnginesAsTable(filteredEngines, enginesShowAllDetails)
	}
}

// FetchEnginesList fetches the list of engines from the server
// Per AI.md PART 1: Function names MUST reveal intent
func FetchEnginesList() (*EnginesListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/engines", apiClient.GetBaseURL())
	responseBytes, err := apiClient.FetchURLResponseBytes(url)
	if err != nil {
		return nil, err
	}

	var response EnginesListResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if !response.Ok {
		return nil, fmt.Errorf("server error: %s", response.Error)
	}

	return &response, nil
}

// PrintEnginesCommandHelp prints help for the engines command
// Per AI.md PART 1: Function names MUST reveal intent
func PrintEnginesCommandHelp() {
	fmt.Printf(`List available search engines

Usage:
  %s engines [flags]

Flags:
      --enabled    Show only enabled engines
      --disabled   Show only disabled engines
      --all        Show all details
  -h, --help       Show help

Examples:
  %s engines
  %s engines --enabled
  %s engines --all --output json
`, BinaryName, BinaryName, BinaryName, BinaryName)
}

// OutputEnginesAsJSON outputs engines as JSON
// Per AI.md PART 1: Function names MUST reveal intent
func OutputEnginesAsJSON(engines []EngineInfo) error {
	jsonEncoder := json.NewEncoder(os.Stdout)
	jsonEncoder.SetIndent("", "  ")
	return jsonEncoder.Encode(engines)
}

// OutputEnginesAsPlain outputs engines as plain text
// Per AI.md PART 1: Function names MUST reveal intent
func OutputEnginesAsPlain(engines []EngineInfo) error {
	for _, engine := range engines {
		status := EngineStatusDisabled
		if engine.Enabled {
			status = EngineStatusEnabled
		}
		fmt.Printf("%s (!%s) - %s [%s]\n", engine.DisplayName, engine.Bang, engine.Method, status)
	}
	fmt.Printf("\nTotal: %d engines\n", len(engines))
	return nil
}

// OutputEnginesAsTable outputs engines as a table
// Per AI.md PART 1: Function names MUST reveal intent
func OutputEnginesAsTable(engines []EngineInfo, showDetails bool) error {
	tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if showDetails {
		fmt.Fprintf(tableWriter, "NAME\tBANG\tTIER\tSTATUS\tMETHOD\tPREVIEW\tDOWNLOAD\n")
		fmt.Fprintf(tableWriter, "----\t----\t----\t------\t------\t-------\t--------\n")
	} else {
		fmt.Fprintf(tableWriter, "NAME\tBANG\tSTATUS\n")
		fmt.Fprintf(tableWriter, "----\t----\t------\n")
	}

	enabledCount := 0
	for _, engine := range engines {
		status := EngineStatusDisabled
		if engine.Enabled {
			status = EngineStatusEnabled
			enabledCount++
		}

		if showDetails {
			preview := EngineDataNotAvail
			download := EngineDataNotAvail
			if engine.HasPreview {
				preview = EngineDataYes
			}
			if engine.HasDownload {
				download = EngineDataYes
			}
			fmt.Fprintf(tableWriter, "%s\t!%s\t%d\t%s\t%s\t%s\t%s\n",
				engine.DisplayName, engine.Bang, engine.Tier, status, engine.Method, preview, download)
		} else {
			fmt.Fprintf(tableWriter, "%s\t!%s\t%s\n",
				engine.DisplayName, engine.Bang, status)
		}
	}

	tableWriter.Flush()

	fmt.Printf("\nTotal: %d engines (%d enabled, %d disabled)\n",
		len(engines), enabledCount, len(engines)-enabledCount)

	return nil
}

// RunBangsCommand runs the bangs command
// Per AI.md PART 1: Function names MUST reveal intent
func RunBangsCommand(args []string) error {
	// Parse bangs-specific flags
	var searchFilter string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--search":
			if i+1 < len(args) {
				searchFilter = args[i+1]
				i++
			}
		case "--help", "-h":
			PrintBangsCommandHelp()
			return nil
		}
	}

	// Fetch engines from server (bangs are derived from engines)
	enginesData, err := FetchEnginesList()
	if err != nil {
		return fmt.Errorf("failed to fetch bangs: %w", err)
	}

	// Extract and filter bangs
	var bangs []BangInfo
	for _, engine := range enginesData.Engines {
		if !engine.Enabled {
			continue
		}
		if searchFilter != "" {
			// Filter by search term
			lowerFilter := strings.ToLower(searchFilter)
			if !strings.Contains(strings.ToLower(engine.Bang), lowerFilter) &&
				!strings.Contains(strings.ToLower(engine.DisplayName), lowerFilter) &&
				!strings.Contains(strings.ToLower(engine.Name), lowerFilter) {
				continue
			}
		}
		bangs = append(bangs, BangInfo{
			Bang:        engine.Bang,
			EngineName:  engine.Name,
			DisplayName: engine.DisplayName,
		})
	}

	// Output results
	switch cliConfig.Output.Format {
	case "json":
		return OutputBangsAsJSON(bangs)
	case "plain":
		return OutputBangsAsPlain(bangs)
	default:
		return OutputBangsAsTable(bangs)
	}
}

// BangInfo represents a bang shortcut
// Per AI.md PART 1: Type names MUST be specific
type BangInfo struct {
	Bang        string `json:"bang"`
	EngineName  string `json:"engine_name"`
	DisplayName string `json:"display_name"`
}

// PrintBangsCommandHelp prints help for the bangs command
// Per AI.md PART 1: Function names MUST reveal intent
func PrintBangsCommandHelp() {
	fmt.Printf(`List bang shortcuts for quick engine selection

Usage:
  %s bangs [flags]

Flags:
      --search string   Filter bangs by name
  -h, --help            Show help

Bang Syntax:
  Use !<bang> before your search query to search a specific engine.
  Multiple bangs can be combined.

Examples:
  %s bangs
  %s bangs --search porn

  # Using bangs in search:
  %s search "!ph amateur"        # Search PornHub only
  %s search "!ph !xv amateur"    # Search PornHub and XVideos
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

// OutputBangsAsJSON outputs bangs as JSON
// Per AI.md PART 1: Function names MUST reveal intent
func OutputBangsAsJSON(bangs []BangInfo) error {
	jsonEncoder := json.NewEncoder(os.Stdout)
	jsonEncoder.SetIndent("", "  ")
	return jsonEncoder.Encode(bangs)
}

// OutputBangsAsPlain outputs bangs as plain text
// Per AI.md PART 1: Function names MUST reveal intent
func OutputBangsAsPlain(bangs []BangInfo) error {
	for _, bang := range bangs {
		fmt.Printf("!%s - %s\n", bang.Bang, bang.DisplayName)
	}
	fmt.Printf("\nTotal: %d bangs available\n", len(bangs))
	return nil
}

// OutputBangsAsTable outputs bangs as a table
// Per AI.md PART 1: Function names MUST reveal intent
func OutputBangsAsTable(bangs []BangInfo) error {
	tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tableWriter, "BANG\tENGINE\n")
	fmt.Fprintf(tableWriter, "----\t------\n")

	for _, bang := range bangs {
		fmt.Fprintf(tableWriter, "!%s\t%s\n", bang.Bang, bang.DisplayName)
	}

	tableWriter.Flush()
	fmt.Printf("\nTotal: %d bangs available\n", len(bangs))
	fmt.Printf("\nUsage: %s search \"!<bang> <query>\"\n", BinaryName)

	return nil
}
