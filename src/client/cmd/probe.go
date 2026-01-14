// SPDX-License-Identifier: MIT
// IDEA.md: CLI Probe Tool - Tests engine availability and capability
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Probe command flags
// Per AI.md PART 1: Variable names MUST reveal intent
var (
	probeAllEngines   bool
	probeEngineFilter string
	probeTestQuery    string
	probeVerboseMode  bool
)

// EngineProbeResult represents the result of probing an engine
type EngineProbeResult struct {
	Name         string                 `json:"name"`
	DisplayName  string                 `json:"display_name"`
	Tier         int                    `json:"tier"`
	Available    bool                   `json:"available"`
	Enabled      bool                   `json:"enabled"`
	Error        string                 `json:"error,omitempty"`
	ResultCount  int                    `json:"result_count"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
	FieldStats   map[string]int         `json:"field_stats,omitempty"`
}

// RunProbeCommand runs the probe command per IDEA.md
// Per AI.md PART 1: Function names MUST reveal intent
func RunProbeCommand(args []string) error {
	// Reset flags for each call
	probeAllEngines = false
	probeEngineFilter = ""
	probeTestQuery = "test"
	probeVerboseMode = false

	// Parse probe-specific flags
	// Per AI.md: Short flags only for -h (help) and -v (version)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			probeAllEngines = true
		case "--engines":
			if i+1 < len(args) {
				probeEngineFilter = args[i+1]
				i++
			}
		case "--query":
			if i+1 < len(args) {
				probeTestQuery = args[i+1]
				i++
			}
		case "--verbose":
			probeVerboseMode = true
		case "--help", "-h":
			PrintProbeCommandHelp()
			return nil
		}
	}

	if !probeAllEngines && probeEngineFilter == "" {
		return fmt.Errorf("specify --all to probe all engines or --engines=name to probe specific engines")
	}

	// Get list of engines to probe
	var engineNames []string
	if probeAllEngines {
		// Fetch engine list from server
		engines, err := fetchEngineList()
		if err != nil {
			return fmt.Errorf("failed to fetch engine list: %w", err)
		}
		engineNames = engines
	} else {
		engineNames = strings.Split(probeEngineFilter, ",")
		for i := range engineNames {
			engineNames[i] = strings.TrimSpace(engineNames[i])
		}
	}

	fmt.Printf("Probing %d engine(s) with query: %q\n\n", len(engineNames), probeTestQuery)

	// Probe each engine
	results := make([]EngineProbeResult, 0, len(engineNames))
	for _, name := range engineNames {
		result := probeEngineByName(name, probeTestQuery)
		results = append(results, result)
	}

	// Output results
	switch cliConfig.Output.Format {
	case "json":
		return OutputProbeResultsAsJSON(results)
	default:
		return OutputProbeResultsAsTable(results)
	}
}

// fetchEngineList gets list of available engines from server
func fetchEngineList() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/engines", apiClient.GetBaseURL())
	resp, err := apiClient.FetchURLResponseBytes(url)
	if err != nil {
		return nil, err
	}

	var data struct {
		Ok      bool `json:"ok"`
		Engines []struct {
			Name string `json:"name"`
		} `json:"engines"`
	}

	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}

	names := make([]string, len(data.Engines))
	for i, e := range data.Engines {
		names[i] = e.Name
	}
	return names, nil
}

// probeEngineByName probes a single engine
func probeEngineByName(name, query string) EngineProbeResult {
	result := EngineProbeResult{
		Name:      name,
		Available: false,
	}

	// Search with this engine only
	searchResp, err := apiClient.Search(query, 1, 20, []string{name}, false)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Available = searchResp.Ok
	result.ResultCount = searchResp.Count

	// Count field stats
	fieldStats := map[string]int{
		"has_thumbnail":    0,
		"has_preview_url":  0,
		"has_download_url": 0,
		"has_duration":     0,
		"has_views":        0,
	}

	for _, r := range searchResp.Results {
		if r.Thumbnail != "" {
			fieldStats["has_thumbnail"]++
		}
		if r.Duration != "" {
			fieldStats["has_duration"]++
		}
		if r.Views != "" {
			fieldStats["has_views"]++
		}
		// Note: preview_url and download_url would need to be added to SearchResult
		// For now, we track what's available
	}

	result.FieldStats = fieldStats

	// Try to get engine info for capabilities
	if infoURL := fmt.Sprintf("%s/api/v1/engines/%s", apiClient.GetBaseURL(), name); infoURL != "" {
		if resp, err := apiClient.FetchURLResponseBytes(infoURL); err == nil {
			var info struct {
				Ok     bool                   `json:"ok"`
				Engine map[string]interface{} `json:"engine"`
			}
			if json.Unmarshal(resp, &info) == nil && info.Ok && info.Engine != nil {
				if dn, ok := info.Engine["display_name"].(string); ok {
					result.DisplayName = dn
				}
				if tier, ok := info.Engine["tier"].(float64); ok {
					result.Tier = int(tier)
				}
				if caps, ok := info.Engine["capabilities"].(map[string]interface{}); ok {
					result.Capabilities = caps
				}
				if enabled, ok := info.Engine["enabled"].(bool); ok {
					result.Enabled = enabled
				}
			}
		}
	}

	if result.DisplayName == "" {
		result.DisplayName = name
	}

	return result
}

// PrintProbeCommandHelp prints probe command help
// Per AI.md PART 1: Function names MUST reveal intent
// Per AI.md: Short flags only for -h (help)
func PrintProbeCommandHelp() {
	fmt.Printf(`Probe engine availability and capabilities

Usage:
  %s probe [flags]

Flags:
      --all              Probe all available engines
      --engines string   Comma-separated list of engines to probe
      --query string     Test query to use (default: "test")
      --verbose          Show detailed output
  -h, --help             Show help

Examples:
  %s probe --all
  %s probe --engines pornhub
  %s probe --engines pornhub,xvideos --query "amateur"
`, BinaryName, BinaryName, BinaryName, BinaryName)
}

// OutputProbeResultsAsJSON outputs probe results as JSON
func OutputProbeResultsAsJSON(results []EngineProbeResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

// OutputProbeResultsAsTable outputs probe results as a table
func OutputProbeResultsAsTable(results []EngineProbeResult) error {
	tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintf(tableWriter, "ENGINE\tTIER\tSTATUS\tRESULTS\tPREVIEW\tDOWNLOAD\n")
	fmt.Fprintf(tableWriter, "------\t----\t------\t-------\t-------\t--------\n")

	available := 0
	for _, r := range results {
		status := "OK"
		if !r.Available {
			status = "FAIL"
		} else {
			available++
		}

		preview := "-"
		download := "-"
		if r.Capabilities != nil {
			if hp, ok := r.Capabilities["has_preview"].(bool); ok && hp {
				preview = "YES"
			}
			if hd, ok := r.Capabilities["has_download"].(bool); ok && hd {
				download = "YES"
			}
		}

		fmt.Fprintf(tableWriter, "%s\t%d\t%s\t%d\t%s\t%s\n",
			r.DisplayName, r.Tier, status, r.ResultCount, preview, download)
	}

	tableWriter.Flush()

	fmt.Printf("\nProbed %d engines: %d available, %d failed\n", len(results), available, len(results)-available)
	return nil
}
