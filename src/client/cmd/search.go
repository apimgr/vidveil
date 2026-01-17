// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Search Command
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/apimgr/vidveil/src/client/api"
)

// Search command flags
// Per AI.md PART 1: Variable names MUST reveal intent
var (
	searchResultLimit   int
	searchPageNumber    int
	searchEngineFilter  string
	searchSafeModeEnabled bool
)

// RunSearchCommand runs the search command per AI.md PART 33
// Per AI.md PART 1: Function names MUST reveal intent - "runSearch" is ambiguous
// No short flags except -h
func RunSearchCommand(args []string) error {
	// Parse search-specific flags
	var queryParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &searchResultLimit)
				i++
			}
		case "--page":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &searchPageNumber)
				i++
			}
		case "--engines":
			if i+1 < len(args) {
				searchEngineFilter = args[i+1]
				i++
			}
		case "--safe":
			searchSafeModeEnabled = true
		case "--help", "-h":
			PrintSearchCommandHelp()
			return nil
		default:
			// Skip if it starts with - (unknown flag)
			if !strings.HasPrefix(args[i], "-") {
				queryParts = append(queryParts, args[i])
			}
		}
	}

	if len(queryParts) == 0 {
		return fmt.Errorf("search query required")
	}

	searchQueryString := strings.Join(queryParts, " ")

	// Parse engines
	var engineList []string
	if searchEngineFilter != "" {
		engineList = strings.Split(searchEngineFilter, ",")
	}

	// Perform search
	searchResponse, err := apiClient.Search(searchQueryString, searchPageNumber, searchResultLimit, engineList, searchSafeModeEnabled)
	if err != nil {
		return err
	}

	if !searchResponse.Ok {
		return fmt.Errorf("search failed: %s", searchResponse.Error)
	}

	// Output results
	switch cliConfig.Output.Format {
	case "json":
		return OutputSearchResultsAsJSON(searchResponse)
	case "plain":
		return OutputSearchResultsAsPlain(searchResponse)
	default:
		return OutputSearchResultsAsTable(searchResponse)
	}
}

// PrintSearchCommandHelp prints search command help per AI.md PART 33
// Per AI.md PART 1: Function names MUST reveal intent - "searchHelp" is ambiguous
func PrintSearchCommandHelp() {
	fmt.Printf(`Search for videos

Usage:
  %s search [flags] <query>
  %s <query>              (shortcut)

Flags:
      --limit int       Number of results (default: server default)
      --page int        Page number (default: 1)
      --engines string  Comma-separated list of engines
      --safe            Enable safe search
  -h, --help            Show help

Examples:
  %s search "amateur"
  %s search --limit 20 "test query"
  %s search --engines pornhub,xvideos "query"
  %s "quick search"
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

// OutputSearchResultsAsJSON outputs search results as JSON
// Per AI.md PART 1: Function names MUST reveal intent - "outputJSON" is ambiguous
func OutputSearchResultsAsJSON(responseData interface{}) error {
	jsonEncoder := json.NewEncoder(os.Stdout)
	jsonEncoder.SetIndent("", "  ")
	return jsonEncoder.Encode(responseData)
}

// OutputSearchResultsAsPlain outputs search results as plain text
// Per AI.md PART 1: Function names MUST reveal intent - "outputPlain" is ambiguous
func OutputSearchResultsAsPlain(searchResponse *api.SearchResponse) error {
	for _, result := range searchResponse.Results {
		fmt.Printf("%s\n", result.Title)
		fmt.Printf("  %s\n", result.URL)
		if result.Duration != "" {
			fmt.Printf("  Duration: %s", result.Duration)
		}
		if result.Views != "" {
			fmt.Printf("  Views: %s", result.Views)
		}
		fmt.Println()
		fmt.Println()
	}
	fmt.Printf("Found %d results for \"%s\"\n", searchResponse.Count, searchResponse.Query)
	return nil
}

// OutputSearchResultsAsTable outputs search results as a table
// Per AI.md PART 1: Function names MUST reveal intent - "outputTable" is ambiguous
func OutputSearchResultsAsTable(searchResponse *api.SearchResponse) error {
	tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintf(tableWriter, "TITLE\tDURATION\tENGINE\tURL\n")
	fmt.Fprintf(tableWriter, "-----\t--------\t------\t---\n")

	for _, result := range searchResponse.Results {
		truncatedTitle := TruncateSearchResultText(result.Title, 50)
		fmt.Fprintf(tableWriter, "%s\t%s\t%s\t%s\n", truncatedTitle, result.Duration, result.Engine, result.URL)
	}

	tableWriter.Flush()

	fmt.Printf("\nFound %d results for \"%s\"\n", searchResponse.Count, searchResponse.Query)
	return nil
}

// TruncateSearchResultText truncates text for display
// Per AI.md PART 1: Function names MUST reveal intent - "truncate" is ambiguous
func TruncateSearchResultText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength-3] + "..."
}
