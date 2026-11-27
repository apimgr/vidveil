// SPDX-License-Identifier: MIT
// AI.md PART 32: VidVeil favorite commands
package cmd

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/apimgr/vidveil/src/client/api"
	"gopkg.in/yaml.v3"
)

// RunFavoritesCommand lists and manages favorites owned by the configured API token.
func RunFavoritesCommand(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printFavoritesHelp()
		return nil
	}
	if apiClient == nil {
		return fmt.Errorf("server is not configured")
	}
	switch args[0] {
	case "list":
		favorites, err := apiClient.ListFavorites()
		if err != nil {
			return err
		}
		return printFavorites(favorites)
	case "add":
		flagSet := flag.NewFlagSet("favorites add", flag.ContinueOnError)
		flagSet.SetOutput(io.Discard)
		urlValue := flagSet.String("url", "", "Result URL")
		titleValue := flagSet.String("title", "", "Result title")
		thumbnailValue := flagSet.String("thumbnail", "", "Thumbnail URL")
		sourceValue := flagSet.String("source", "", "Search engine source")
		if err := flagSet.Parse(args[1:]); err != nil {
			return NewExitError(ExitUsage, err)
		}
		if *urlValue == "" || *titleValue == "" || flagSet.NArg() != 0 {
			return NewExitError(ExitUsage, fmt.Errorf("favorites add requires --url and --title"))
		}
		if err := apiClient.AddFavorite(api.Favorite{URL: *urlValue, Title: *titleValue, Thumbnail: *thumbnailValue, Source: *sourceValue}); err != nil {
			return err
		}
		fmt.Println("Favorite added")
		return nil
	case "remove":
		if len(args) != 2 {
			return NewExitError(ExitUsage, fmt.Errorf("favorites remove requires an ID"))
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || id <= 0 {
			return NewExitError(ExitUsage, fmt.Errorf("favorite ID must be a positive integer"))
		}
		if err := apiClient.RemoveFavorite(id); err != nil {
			return err
		}
		fmt.Println("Favorite removed")
		return nil
	case "clear":
		flagSet := flag.NewFlagSet("favorites clear", flag.ContinueOnError)
		flagSet.SetOutput(io.Discard)
		confirm := flagSet.Bool("yes", false, "Confirm clearing all favorites")
		if err := flagSet.Parse(args[1:]); err != nil {
			return NewExitError(ExitUsage, err)
		}
		if !*confirm || flagSet.NArg() != 0 {
			return NewExitError(ExitUsage, fmt.Errorf("favorites clear requires --yes"))
		}
		if err := apiClient.ClearFavorites(); err != nil {
			return err
		}
		fmt.Println("Favorites cleared")
		return nil
	default:
		return NewExitError(ExitUsage, fmt.Errorf("unknown favorites action %q", args[0]))
	}
}

func printFavoritesHelp() {
	fmt.Printf(`Manage private favorites

Usage:
  %s favorites list
  %s favorites add --url URL --title TITLE [--thumbnail URL] [--source NAME]
  %s favorites remove ID
  %s favorites clear --yes
`, BinaryName, BinaryName, BinaryName, BinaryName)
}

func printFavorites(favorites []api.Favorite) error {
	format := "table"
	if cliConfig != nil && cliConfig.Output.Format != "" {
		format = cliConfig.Output.Format
	}
	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(favorites)
	case "yaml":
		encoded, err := yaml.Marshal(favorites)
		if err != nil {
			return fmt.Errorf("encoding favorites: %w", err)
		}
		_, err = os.Stdout.Write(encoded)
		return err
	case "csv":
		writer := csv.NewWriter(os.Stdout)
		if err := writer.Write([]string{"id", "url", "title", "thumbnail", "source", "created_at"}); err != nil {
			return err
		}
		for _, favorite := range favorites {
			if err := writer.Write([]string{strconv.FormatInt(favorite.ID, 10), favorite.URL, favorite.Title, favorite.Thumbnail, favorite.Source, favorite.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case "plain":
		for _, favorite := range favorites {
			fmt.Printf("%s\t%s\n", favorite.Title, favorite.URL)
		}
		return nil
	default:
		writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(writer, "ID\tTITLE\tURL\tSOURCE"); err != nil {
			return err
		}
		for _, favorite := range favorites {
			if _, err := fmt.Fprintf(writer, "%d\t%s\t%s\t%s\n", favorite.ID, favorite.Title, favorite.URL, favorite.Source); err != nil {
				return err
			}
		}
		return writer.Flush()
	}
}
