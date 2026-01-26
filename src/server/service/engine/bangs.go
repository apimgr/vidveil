// SPDX-License-Identifier: MIT
package engine

import (
	"strings"
)

// BangMapping maps bang shortcuts to engine names
// Supports both full names (!pornhub) and short codes (!ph)
var BangMapping = map[string]string{
	// Tier 1 - Major Sites
	"ph":       "pornhub",
	"pornhub":  "pornhub",
	"xv":       "xvideos",
	"xvideos":  "xvideos",
	"xn":       "xnxx",
	"xnxx":     "xnxx",
	"rt":       "redtube",
	"redtube":  "redtube",
	"xh":       "xhamster",
	"xhamster": "xhamster",

	// Tier 2 - Popular Sites
	"ep":      "eporner",
	"eporner": "eporner",
	"yp":      "youporn",
	"youporn": "youporn",
	"pmd":     "pornmd",
	"pornmd":  "pornmd",

	// Tier 3 - Additional Sites
	"4t":       "4tube",
	"4tube":    "4tube",
	"fux":      "fux",
	"pt":       "porntube",
	"porntube": "porntube",
	"yj":       "youjizz",
	"youjizz":  "youjizz",
	"sp":       "sunporno",
	"sunporno": "sunporno",
	"tx":       "txxx",
	"txxx":     "txxx",
	"nv":       "nuvid",
	"nuvid":    "nuvid",
	"tna":      "tnaflix",
	"tnaflix":  "tnaflix",
	"dt":       "drtuber",
	"drtuber":  "drtuber",
	"emp":      "empflix",
	"empflix":  "empflix",
	"hp":       "hellporno",
	"hellporno": "hellporno",
	"ap":       "alphaporno",
	"alphaporno": "alphaporno",
	"pf":       "pornflip",
	"pornflip": "pornflip",
	"zp":       "zenporn",
	"zenporn":  "zenporn",
	"gp":       "gotporn",
	"gotporn":  "gotporn",
	"hz":       "hdzog",
	"hdzog":    "hdzog",
	"xxxy":     "xxxymovies",
	"xxxymovies": "xxxymovies",
	"lhp":      "lovehomeporn",
	"lovehomeporn": "lovehomeporn",

	// Tier 4 - Additional yt-dlp supported sites
	"pb":        "pornerbros",
	"pornerbros": "pornerbros",
	"nk":        "nonktube",
	"nonktube":  "nonktube",
	"np":        "nubilesporn",
	"nubilesporn": "nubilesporn",
	"pbox":      "pornbox",
	"pornbox":   "pornbox",
	"ptop":      "porntop",
	"porntop":   "porntop",
	"pnt":       "pornotube",
	"pornotube": "pornotube",
	"phd":       "pornhd",
	"pornhd":    "pornhd",
	"xb":        "xbabe",
	"xbabe":     "xbabe",
	"p1":        "pornone",
	"pornone":   "pornone",
	"phat":      "pornhat",
	"pornhat":   "pornhat",
	"ptrex":     "porntrex",
	"porntrex":  "porntrex",
	"hq":        "hqporner",
	"hqporner":  "hqporner",
	"vj":        "vjav",
	"vjav":      "vjav",
	"ff":        "flyflv",
	"flyflv":    "flyflv",
	"t8":        "tube8",
	"tube8":     "tube8",

	// Tier 5 - New engines
	"any":       "anyporn",
	"anyporn":   "anyporn",
	"tg":        "tubegalore",
	"tubegalore": "tubegalore",
	"ml":        "motherless",
	"motherless": "motherless",

	// Tier 6 - Additional engines
	"3m":          "3movs",
	"3movs":       "3movs",
}

// ParsedQuery represents a query after bang parsing
type ParsedQuery struct {
	// The search query without bangs, quotes, exclusions, and performers
	Query string
	// Engine names to search (empty = all)
	Engines []string
	// Whether a bang was detected
	HasBang bool
	// If a bang was not recognized
	InvalidBang string
	// Exact phrases to require (from "quoted text")
	ExactPhrases []string
	// Words to exclude from results (from -word)
	Exclusions []string
	// Performer names to filter by (from @performer)
	Performers []string
	// Whether performer filter was specified
	HasPerformer bool
}

// ParseBangs extracts bang commands from a query
// Supports:
//   - !ph query -> search pornhub for "query"
//   - !rt !ph query -> search redtube and pornhub for "query"
//   - query !ph -> search pornhub for "query"
//   - !pornhub query -> search pornhub for "query"
//   - "exact phrase" -> require exact phrase match
//   - -word -> exclude results containing word
//   - @performer -> filter by performer name
func ParseBangs(query string) ParsedQuery {
	result := ParsedQuery{
		Query:        query,
		Engines:      []string{},
		HasBang:      false,
		ExactPhrases: []string{},
		Exclusions:   []string{},
		Performers:   []string{},
		HasPerformer: false,
	}

	if query == "" {
		return result
	}

	// First, extract quoted phrases
	remaining := query
	for {
		start := strings.Index(remaining, "\"")
		if start == -1 {
			break
		}
		end := strings.Index(remaining[start+1:], "\"")
		if end == -1 {
			break
		}
		// Extract the phrase (without quotes)
		phrase := strings.TrimSpace(remaining[start+1 : start+1+end])
		if phrase != "" {
			result.ExactPhrases = append(result.ExactPhrases, phrase)
		}
		// Remove the quoted phrase from the query
		remaining = remaining[:start] + remaining[start+1+end+1:]
	}

	words := strings.Fields(remaining)
	var queryWords []string
	// Deduplicate engines
	engineSet := make(map[string]bool)
	// Deduplicate performers
	performerSet := make(map[string]bool)

	for _, word := range words {
		if strings.HasPrefix(word, "!") && len(word) > 1 {
			bang := strings.ToLower(word[1:])
			if engineName, ok := BangMapping[bang]; ok {
				result.HasBang = true
				if !engineSet[engineName] {
					engineSet[engineName] = true
					result.Engines = append(result.Engines, engineName)
				}
			} else {
				// Unknown bang - keep it as part of query but note it
				result.InvalidBang = word
				queryWords = append(queryWords, word)
			}
		} else if strings.HasPrefix(word, "@") && len(word) > 1 {
			// Performer filter
			performer := strings.ToLower(word[1:])
			if !performerSet[performer] {
				performerSet[performer] = true
				result.Performers = append(result.Performers, performer)
				result.HasPerformer = true
			}
		} else if strings.HasPrefix(word, "-") && len(word) > 1 {
			// Exclusion term
			exclusion := strings.ToLower(word[1:])
			result.Exclusions = append(result.Exclusions, exclusion)
		} else {
			queryWords = append(queryWords, word)
		}
	}

	result.Query = strings.TrimSpace(strings.Join(queryWords, " "))

	return result
}

// GetEngineBangs returns all bangs for a given engine name
func GetEngineBangs(engineName string) []string {
	var bangs []string
	for bang, engine := range BangMapping {
		if engine == engineName {
			bangs = append(bangs, "!"+bang)
		}
	}
	return bangs
}

// GetAllBangs returns a map of engine names to their bangs
func GetAllBangs() map[string][]string {
	result := make(map[string][]string)
	for bang, engine := range BangMapping {
		result[engine] = append(result[engine], "!"+bang)
	}
	return result
}

// BangInfo holds information about a bang shortcut
type BangInfo struct {
	Bang        string `json:"bang"`
	EngineName  string `json:"engine_name"`
	DisplayName string `json:"display_name"`
	ShortCode   string `json:"short_code"`
}

// EngineDisplayNames maps engine names to display names
var EngineDisplayNames = map[string]string{
	"pornhub":     "PornHub",
	"xvideos":     "XVideos",
	"xnxx":        "XNXX",
	"redtube":     "RedTube",
	"xhamster":    "xHamster",
	"eporner":     "Eporner",
	"youporn":     "YouPorn",
	"pornmd":      "PornMD",
	"4tube":       "4Tube",
	"fux":         "Fux",
	"porntube":    "PornTube",
	"youjizz":     "YouJizz",
	"sunporno":    "SunPorno",
	"txxx":        "TXXX",
	"nuvid":       "Nuvid",
	"tnaflix":     "TNAFlix",
	"drtuber":     "DrTuber",
	"empflix":     "EMPFlix",
	"hellporno":   "HellPorno",
	"alphaporno":  "AlphaPorno",
	"pornflip":    "PornFlip",
	"zenporn":     "ZenPorn",
	"gotporn":     "GotPorn",
	"hdzog":       "HDZog",
	"xxxymovies":  "XXXYMovies",
	"lovehomeporn": "LoveHomePorn",
	"pornerbros":  "PornerBros",
	"nonktube":    "NonkTube",
	"nubilesporn": "NubilesPorn",
	"pornbox":     "PornBox",
	"porntop":     "PornTop",
	"pornotube":   "Pornotube",
	"pornhd":      "PornHD",
	"xbabe":       "XBabe",
	"pornone":     "PornOne",
	"pornhat":     "PornHat",
	"porntrex":    "PornTrex",
	"hqporner":    "HQPorner",
	"vjav":        "VJAV",
	"flyflv":      "FlyFLV",
	"tube8":       "Tube8",
	"anyporn":     "AnyPorn",
	"tubegalore":  "TubeGalore",
	"motherless":  "Motherless",
	"3movs":       "3Movs",
}

// ListBangs returns a sorted list of all available bangs
func ListBangs() []BangInfo {
	seen := make(map[string]bool)
	var result []BangInfo

	// Get unique engine names first
	for _, engine := range BangMapping {
		if !seen[engine] {
			seen[engine] = true
			// Find the short code (shortest bang for this engine)
			shortCode := engine
			for bang, eng := range BangMapping {
				if eng == engine && len(bang) < len(shortCode) {
					shortCode = bang
				}
			}
			displayName := EngineDisplayNames[engine]
			if displayName == "" {
				displayName = engine
			}
			result = append(result, BangInfo{
				Bang:        "!" + engine,
				EngineName:  engine,
				DisplayName: displayName,
				ShortCode:   "!" + shortCode,
			})
		}
	}

	return result
}

// AutocompleteSuggestion represents a single autocomplete suggestion
type AutocompleteSuggestion struct {
	Bang        string `json:"bang"`
	EngineName  string `json:"engine_name"`
	DisplayName string `json:"display_name"`
	ShortCode   string `json:"short_code"`
	// For sorting, not exposed
	Score int `json:"-"`
}

// Autocomplete returns bang suggestions for a partial input
// prefix should be the text after "!" (e.g., "po" for "!po")
func Autocomplete(prefix string) []AutocompleteSuggestion {
	prefix = strings.ToLower(prefix)
	if prefix == "" {
		return nil
	}

	seen := make(map[string]bool)
	var suggestions []AutocompleteSuggestion

	for bang, engine := range BangMapping {
		// Skip if we've already added this engine
		if seen[engine] {
			continue
		}

		// Check if bang or engine name starts with prefix
		bangLower := strings.ToLower(bang)
		engineLower := strings.ToLower(engine)

		score := 0
		// Shorter = better
		if strings.HasPrefix(bangLower, prefix) {
			score = 100 - len(bang)
		} else if strings.HasPrefix(engineLower, prefix) {
			score = 50 - len(engine)
		} else if strings.Contains(bangLower, prefix) || strings.Contains(engineLower, prefix) {
			score = 10
		}

		if score > 0 {
			seen[engine] = true
			// Find shortest bang for this engine
			shortCode := engine
			for b, e := range BangMapping {
				if e == engine && len(b) < len(shortCode) {
					shortCode = b
				}
			}
			displayName := EngineDisplayNames[engine]
			if displayName == "" {
				displayName = engine
			}
			suggestions = append(suggestions, AutocompleteSuggestion{
				Bang:        "!" + engine,
				EngineName:  engine,
				DisplayName: displayName,
				ShortCode:   "!" + shortCode,
				Score:       score,
			})
		}
	}

	// Sort by score (descending)
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Score > suggestions[i].Score {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	// Limit results
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	return suggestions
}
