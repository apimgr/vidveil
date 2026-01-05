// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// SearchSuggestions contains common adult content search terms
// Organized by category for easier maintenance
var SearchSuggestions = []string{
	// Categories/Genres
	"amateur", "anal", "asian", "babe", "bbw", "big ass", "big tits", "bisexual",
	"blonde", "blowjob", "bondage", "brunette", "bukkake", "cartoon", "casting",
	"celebrity", "college", "compilation", "cosplay", "creampie", "cuckold",
	"cumshot", "deepthroat", "dildo", "double penetration", "ebony", "facial",
	"feet", "femdom", "fetish", "fisting", "foursome", "gangbang", "gay",
	"girlfriend", "glamour", "gonzo", "granny", "group", "hairy", "handjob",
	"hardcore", "hentai", "hidden cam", "homemade", "indian", "interracial",
	"japanese", "korean", "latina", "lesbian", "lingerie", "massage", "masturbation",
	"mature", "milf", "missionary", "mom", "natural tits", "nurse", "office",
	"old young", "oral", "orgasm", "orgy", "outdoor", "pantyhose", "parody",
	"petite", "pissing", "point of view", "pornstar", "public", "reality",
	"redhead", "rough", "russian", "schoolgirl", "secretary", "shaved", "shemale",
	"shower", "skinny", "sleeping", "small tits", "smoking", "softcore", "solo",
	"spanking", "squirt", "stepmom", "stepdad", "stepsister", "stepbrother",
	"stockings", "strapon", "striptease", "swallow", "swinger", "tattoo", "teacher",
	"teen", "threesome", "titjob", "toys", "trans", "uncensored", "uniform",
	"vintage", "voyeur", "webcam", "wife", "yoga",

	// Popular performers (generic terms)
	"hot blonde", "hot brunette", "hot milf", "hot teen", "sexy girl",
	"busty babe", "thick ass", "natural beauty", "fit body", "curvy",

	// Actions/Positions
	"cowgirl", "doggy style", "missionary", "reverse cowgirl", "spooning",
	"standing", "69", "on top", "from behind", "bent over",

	// Qualities
	"hd", "4k", "full video", "new", "trending", "popular", "best",
	"top rated", "most viewed", "latest", "exclusive", "premium",

	// Common search phrases
	"first time", "behind the scenes", "try not to cum", "homemade porn",
	"real couple", "amateur couple", "cheating wife", "hot wife",
	"step family", "caught cheating", "surprise", "real orgasm",
}

// SearchSuggestion represents a search term suggestion
type SearchSuggestion struct {
	Term  string `json:"term"`
	Score int    `json:"-"`
}

// AutocompleteSuggestions returns search term suggestions for a prefix
// Returns up to maxResults suggestions, sorted by relevance
func AutocompleteSuggestions(prefix string, maxResults int) []SearchSuggestion {
	if prefix == "" || maxResults <= 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 2 {
		return nil
	}

	var suggestions []SearchSuggestion

	for _, term := range SearchSuggestions {
		termLower := strings.ToLower(term)

		score := 0
		if strings.HasPrefix(termLower, prefix) {
			// Exact prefix match scores highest
			// Shorter terms rank higher (more specific)
			score = 100 - len(term)
		} else if strings.Contains(termLower, prefix) {
			// Contains match scores lower
			score = 50 - len(term)
		}

		if score > 0 {
			suggestions = append(suggestions, SearchSuggestion{
				Term:  term,
				Score: score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	// Limit results
	if len(suggestions) > maxResults {
		suggestions = suggestions[:maxResults]
	}

	return suggestions
}

// GetPopularSearches returns a list of popular search terms
// Used for initial suggestions before user types
func GetPopularSearches(count int) []string {
	popular := []string{
		"teen", "milf", "lesbian", "anal", "amateur", "big tits",
		"blonde", "asian", "threesome", "creampie", "blowjob", "latina",
		"ebony", "hardcore", "mature", "stepmom", "japanese", "massage",
	}

	if count > len(popular) {
		count = len(popular)
	}
	return popular[:count]
}
