// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - Search Command
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/apimgr/vidveil/src/client/api"
)

var (
	searchLimit   int
	searchPage    int
	searchEngines string
	searchSafe    bool
)

func runSearch(args []string) error {
	// Parse search-specific flags
	var query []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit", "-l":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &searchLimit)
				i++
			}
		case "--page", "-p":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &searchPage)
				i++
			}
		case "--engines", "-e":
			if i+1 < len(args) {
				searchEngines = args[i+1]
				i++
			}
		case "--safe":
			searchSafe = true
		case "--help", "-h":
			searchHelp()
			return nil
		default:
			// Skip if it starts with - (unknown flag)
			if !strings.HasPrefix(args[i], "-") {
				query = append(query, args[i])
			}
		}
	}

	if len(query) == 0 {
		return fmt.Errorf("search query required")
	}

	queryStr := strings.Join(query, " ")

	// Parse engines
	var engines []string
	if searchEngines != "" {
		engines = strings.Split(searchEngines, ",")
	}

	// Perform search
	resp, err := client.Search(queryStr, searchPage, searchLimit, engines, searchSafe)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("search failed: %s", resp.Error)
	}

	// Output results
	switch cfg.Output.Format {
	case "json":
		return outputJSON(resp)
	case "plain":
		return outputPlain(resp)
	default:
		return outputTable(resp)
	}
}

func searchHelp() {
	fmt.Printf(`Search for videos

Usage:
  %s search [flags] <query>
  %s <query>              (shortcut)

Flags:
  -l, --limit int       Number of results (default: server default)
  -p, --page int        Page number (default: 1)
  -e, --engines string  Comma-separated list of engines
      --safe            Enable safe search
  -h, --help            Show help

Examples:
  %s search "amateur"
  %s search --limit 20 "test query"
  %s search --engines pornhub,xvideos "query"
  %s "quick search"
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

func outputJSON(resp interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func outputPlain(resp *api.SearchResponse) error {
	for _, r := range resp.Results {
		fmt.Printf("%s\n", r.Title)
		fmt.Printf("  %s\n", r.URL)
		if r.Duration != "" {
			fmt.Printf("  Duration: %s", r.Duration)
		}
		if r.Views != "" {
			fmt.Printf("  Views: %s", r.Views)
		}
		fmt.Println()
		fmt.Println()
	}
	fmt.Printf("Found %d results for \"%s\"\n", resp.Count, resp.Query)
	return nil
}

func outputTable(resp *api.SearchResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintf(w, "TITLE\tDURATION\tENGINE\tURL\n")
	fmt.Fprintf(w, "-----\t--------\t------\t---\n")

	for _, r := range resp.Results {
		title := truncate(r.Title, 50)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", title, r.Duration, r.Engine, r.URL)
	}

	w.Flush()

	fmt.Printf("\nFound %d results for \"%s\"\n", resp.Count, resp.Query)
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
