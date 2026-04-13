// SPDX-License-Identifier: MIT
package engine

import (
	"strings"

	"github.com/apimgr/vidveil/src/server/model"
)

// Category represents a content category with synonyms and related terms
type Category struct {
	// Primary name
	Name     string
	// Terms that mean the same thing
	Synonyms []string
	// Related but different terms
	Related  []string
}

// Categories defines the content taxonomy for search term normalization
// Each category has synonyms (equivalent terms) and related terms (similar concepts)
var Categories = map[string]*Category{
	// Age-related
	"teen": {
		Name:     "teen",
		Synonyms: []string{"teen", "18", "19", "eighteen", "nineteen", "barely legal", "young", "18yo", "19yo", "18 years old", "19 years old", "lolita", "young girl", "young woman"},
		Related:  []string{"college", "student", "petite", "amateur"},
	},
	"milf": {
		Name:     "milf",
		Synonyms: []string{"milf", "mom", "mother", "mommy", "cougar", "mature"},
		Related:  []string{"stepmom", "step mom", "housewife", "30s", "40s"},
	},
	"mature": {
		Name:     "mature",
		Synonyms: []string{"mature", "older", "granny", "gilf", "grandma", "old"},
		Related:  []string{"50s", "60s", "experienced", "milf"},
	},

	// Body types
	"bbw": {
		Name:     "bbw",
		Synonyms: []string{"bbw", "chubby", "fat", "plump", "thick", "curvy", "plus size", "heavy"},
		Related:  []string{"pawg", "voluptuous", "big ass", "big tits"},
	},
	"petite": {
		Name:     "petite",
		Synonyms: []string{"petite", "tiny", "small", "skinny", "slim", "thin"},
		Related:  []string{"teen", "flat chest", "small tits", "spinner"},
	},
	"busty": {
		Name:     "busty",
		Synonyms: []string{"busty", "big tits", "big boobs", "huge tits", "large breasts", "big breasts", "natural tits"},
		Related:  []string{"milf", "titjob", "titty fuck"},
	},

	// Ethnicity
	"asian": {
		Name:     "asian",
		Synonyms: []string{"asian", "oriental"},
		Related:  []string{"japanese", "chinese", "korean", "thai", "filipina", "vietnamese"},
	},
	"latina": {
		Name:     "latina",
		Synonyms: []string{"latina", "latino", "hispanic", "spanish"},
		Related:  []string{"mexican", "brazilian", "colombian", "puerto rican"},
	},
	"ebony": {
		Name:     "ebony",
		Synonyms: []string{"ebony", "black", "african", "dark skin"},
		Related:  []string{"bbc", "interracial"},
	},

	// Sexual orientation/acts
	"lesbian": {
		Name:     "lesbian",
		Synonyms: []string{"lesbian", "lesbo", "girl on girl", "girls", "lez", "lesbians"},
		Related:  []string{"scissoring", "tribbing", "strapon", "fingering"},
	},
	"gay": {
		Name:     "gay",
		Synonyms: []string{"gay", "homosexual", "guy on guy", "men"},
		Related:  []string{"twink", "bear", "daddy"},
	},
	"anal": {
		Name:     "anal",
		Synonyms: []string{"anal", "ass fuck", "butt fuck", "ass sex", "backdoor"},
		Related:  []string{"anal creampie", "gape", "dp", "atm"},
	},
	"blowjob": {
		Name:     "blowjob",
		Synonyms: []string{"blowjob", "bj", "oral", "sucking", "head", "fellatio", "cock sucking"},
		Related:  []string{"deepthroat", "face fuck", "gagging", "cum in mouth"},
	},
	"creampie": {
		Name:     "creampie",
		Synonyms: []string{"creampie", "cream pie", "cum inside", "internal cumshot", "internal"},
		Related:  []string{"breeding", "impregnation", "pregnant"},
	},

	// Scenarios
	"amateur": {
		Name:     "amateur",
		Synonyms: []string{"amateur", "homemade", "real", "authentic", "verified", "genuine"},
		Related:  []string{"pov", "couple", "first time"},
	},
	"pov": {
		Name:     "pov",
		Synonyms: []string{"pov", "point of view", "first person", "gonzo"},
		Related:  []string{"amateur", "blowjob", "virtual"},
	},
	"threesome": {
		Name:     "threesome",
		Synonyms: []string{"threesome", "3some", "three way", "threeway", "trio"},
		Related:  []string{"ffm", "mmf", "group", "orgy"},
	},
	"gangbang": {
		Name:     "gangbang",
		Synonyms: []string{"gangbang", "gang bang", "gb"},
		Related:  []string{"group", "orgy", "bukakke", "dp"},
	},

	// Physical states
	"pregnant": {
		Name:     "pregnant",
		Synonyms: []string{"pregnant", "preggo", "preggy", "expecting", "knocked up", "with child"},
		Related:  []string{"lactating", "breeding", "creampie"},
	},
	"lactating": {
		Name:     "lactating",
		Synonyms: []string{"lactating", "lactation", "breastfeeding", "breast milk", "milking", "milk", "milky", "nursing", "tits milk", "breast feeding"},
		Related:  []string{"pregnant", "busty", "big tits", "natural tits"},
	},
	"hairy": {
		Name:     "hairy",
		Synonyms: []string{"hairy", "bush", "unshaved", "natural", "furry", "hairy pussy"},
		Related:  []string{"vintage", "retro"},
	},

	// Production style
	"hd": {
		Name:     "hd",
		Synonyms: []string{"hd", "1080p", "high definition", "full hd", "fhd"},
		Related:  []string{"4k", "uhd", "high quality"},
	},
	"4k": {
		Name:     "4k",
		Synonyms: []string{"4k", "uhd", "ultra hd", "2160p"},
		Related:  []string{"hd", "high quality", "vr"},
	},

	// Relationships
	"stepmom": {
		Name:     "stepmom",
		Synonyms: []string{"stepmom", "step mom", "step mother", "stepmother"},
		Related:  []string{"milf", "taboo", "family"},
	},
	"stepsister": {
		Name:     "stepsister",
		Synonyms: []string{"stepsister", "step sister", "step sis", "stepsis"},
		Related:  []string{"teen", "taboo", "family"},
	},

	// Fetishes
	"bdsm": {
		Name:     "bdsm",
		Synonyms: []string{"bdsm", "bondage", "domination", "submission", "s&m", "sm"},
		Related:  []string{"tied up", "spanking", "femdom", "slave"},
	},
	"feet": {
		Name:     "feet",
		Synonyms: []string{"feet", "foot", "toes", "soles", "foot fetish", "footjob"},
		Related:  []string{"high heels", "stockings", "worship"},
	},
}

// categoryLookup maps all synonyms back to their category
var categoryLookup map[string]string

func init() {
	categoryLookup = make(map[string]string)
	for name, cat := range Categories {
		categoryLookup[strings.ToLower(name)] = name
		for _, syn := range cat.Synonyms {
			categoryLookup[strings.ToLower(syn)] = name
		}
	}
}

// NormalizeTerm returns the canonical category name for a term, or the term itself if not found
func NormalizeTerm(term string) string {
	term = strings.ToLower(strings.TrimSpace(term))
	if cat, ok := categoryLookup[term]; ok {
		return cat
	}
	return term
}

// GetSynonyms returns all synonyms for a term (including the term itself)
func GetSynonyms(term string) []string {
	term = strings.ToLower(strings.TrimSpace(term))

	// Check if term maps to a category
	if catName, ok := categoryLookup[term]; ok {
		if cat, exists := Categories[catName]; exists {
			return cat.Synonyms
		}
	}

	// Return just the term if no category found
	return []string{term}
}

// GetRelatedTerms returns related terms for a search term
func GetRelatedTerms(term string) []string {
	term = strings.ToLower(strings.TrimSpace(term))

	// Check if term maps to a category
	if catName, ok := categoryLookup[term]; ok {
		if cat, exists := Categories[catName]; exists {
			return cat.Related
		}
	}

	return nil
}

// ExpandSearchTerms takes a search query and returns expanded terms including synonyms
// Used for matching results against search criteria
func ExpandSearchTerms(query string) map[string][]string {
	words := strings.Fields(strings.ToLower(query))
	expanded := make(map[string][]string)

	for _, word := range words {
		// Normalize to canonical form
		normalized := NormalizeTerm(word)
		// Get all synonyms
		synonyms := GetSynonyms(word)
		expanded[normalized] = synonyms
	}

	return expanded
}

// MatchesAllTerms checks if text matches ALL search terms (using synonyms)
// Returns true only if every term (or its synonyms) is found in the text
func MatchesAllTerms(text string, expandedTerms map[string][]string) bool {
	textLower := strings.ToLower(text)

	for _, synonyms := range expandedTerms {
		found := false
		for _, syn := range synonyms {
			if strings.Contains(textLower, syn) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// MatchesAnyTerm checks if text matches ANY of the search terms (using synonyms)
func MatchesAnyTerm(text string, expandedTerms map[string][]string) bool {
	textLower := strings.ToLower(text)

	for _, synonyms := range expandedTerms {
		for _, syn := range synonyms {
			if strings.Contains(textLower, syn) {
				return true
			}
		}
	}

	return false
}

// GenerateSmartRelated generates related searches based on the actual query
// Creates combinations and variations that are semantically related
func GenerateSmartRelated(query string, maxResults int) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	words := strings.Fields(query)

	if len(words) == 0 {
		return nil
	}

	// Build a set of query words for fast lookup
	queryWordSet := make(map[string]bool, len(words))
	for _, w := range words {
		queryWordSet[w] = true
	}

	var related []string
	seen := make(map[string]bool)
	// Never include the original query itself
	seen[query] = true

	// Helper: only add multi-word terms or meaningful single terms not in query
	addUnique := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" || seen[term] {
			return
		}
		seen[term] = true
		related = append(related, term)
	}

	// 1. Swap synonyms — keeps the full query structure, just swaps one word
	for i, word := range words {
		synonyms := GetSynonyms(word)
		for _, syn := range synonyms {
			if syn != word {
				newWords := make([]string, len(words))
				copy(newWords, words)
				newWords[i] = syn
				addUnique(strings.Join(newWords, " "))
			}
		}
	}

	// 2. Related terms combined with query words — always anchor to context
	for _, word := range words {
		relatedTerms := GetRelatedTerms(word)
		for _, rt := range relatedTerms {
			rtLower := strings.ToLower(rt)
			// Combine with other query words to keep context
			for _, other := range words {
				if other != word {
					addUnique(rtLower + " " + other)
					addUnique(other + " " + rtLower)
				}
			}
			// Single-word queries: the related term alone is acceptable
			if len(words) == 1 {
				addUnique(rtLower)
			}
		}
	}

	// 3. Quality/style modifiers appended to full query (skip if already present)
	qualityMods := []string{"hd", "4k", "amateur", "homemade", "pov"}
	for _, mod := range qualityMods {
		if !queryWordSet[mod] {
			addUnique(query + " " + mod)
		}
	}

	// 4. Sub-combinations for 3+ word queries — word pairs (no singles)
	if len(words) >= 3 {
		for i := 0; i < len(words)-1; i++ {
			for j := i + 1; j < len(words); j++ {
				addUnique(words[i] + " " + words[j])
			}
		}
	}

	if len(related) > maxResults {
		related = related[:maxResults]
	}

	return related
}

// QueryIntent holds the detected semantic intent of a search query.
// Used to filter results that contradict the intended content type.
type QueryIntent struct {
	// IsFemaleOnly means the query implies female-only (lesbian/sapphic) content.
	// Results featuring biological males should be rejected.
	IsFemaleOnly bool
	// HasAgeTypes lists age-type terms detected in the query (e.g. "teen", "milf").
	HasAgeTypes []string
}

// femaleOnlyTerms trigger IsFemaleOnly when found in the query.
var femaleOnlyTerms = []string{
	"lesbian", "lesbians", "lesbo", "lesbos",
	"girl on girl", "girls only", "female only",
	"sapphic", "lez", "lezz", "lezzies",
	"women only", "all female",
}

// malePresenceWords are tokens in a result title that strongly indicate
// a biological male participant, contradicting female-only queries.
// "strapon", "dildo", "strap-on" are intentionally excluded — they appear in
// lesbian content and must NOT trigger this filter.
var malePresenceWords = []string{
	"cock", "cocks", "dick", "dicks", "penis",
	"bbc",
	"stepbro", "stepfather", "stepdad",
	"blowbang", "facial", "cumshot", "cum shot",
	"handjob", "hand job",
	"he fucks", "guy fucks", "man fucks",
	"boyfriend fucks", "hubby fucks",
}

// containsWholeWord reports whether s contains word as a standalone word token.
func containsWholeWord(s, word string) bool {
	wlen := len(word)
	for i := 0; i <= len(s)-wlen; i++ {
		if s[i:i+wlen] != word {
			continue
		}
		before := i == 0 || !isWordChar(rune(s[i-1]))
		after := i+wlen >= len(s) || !isWordChar(rune(s[i+wlen]))
		if before && after {
			return true
		}
	}
	return false
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// DetectQueryIntent analyses the search query and returns its semantic intent.
func DetectQueryIntent(query string) QueryIntent {
	lower := strings.ToLower(query)
	intent := QueryIntent{}

	for _, term := range femaleOnlyTerms {
		if strings.Contains(lower, term) {
			intent.IsFemaleOnly = true
			break
		}
	}

	ageGroups := map[string][]string{
		"teen":   {"teen", "teens", "18yo", "19yo", "young girl", "young adult"},
		"milf":   {"milf", "milfs", "cougar", "mommy", "mature woman"},
		"mature": {"mature", "granny", "gilf", "older woman"},
	}
	for age, synonyms := range ageGroups {
		for _, syn := range synonyms {
			if strings.Contains(lower, syn) {
				intent.HasAgeTypes = append(intent.HasAgeTypes, age)
				break
			}
		}
	}

	return intent
}

// ResultMatchesIntent returns false if a result semantically contradicts the query intent.
// toyWords indicate a title is describing a sex toy rather than a biological male,
// used to allow "bbc dildo", "artificial cock", etc. in female-only queries.
var toyWords = []string{"dildo", "strapon", "strap-on", "vibrator", "toy", "artificial", "fake"}

// ambiguousMaleWords are checked only when no toy word is present in the title.
// e.g. "bbc dildo" or "strapon dick" refer to toys, not biological males.
var ambiguousMaleWords = []string{"cock", "cocks", "dick", "dicks", "bbc"}

// Conservative: only rejects when there is clear, unambiguous evidence of contradiction.
func ResultMatchesIntent(r model.VideoResult, intent QueryIntent) bool {
	if !intent.IsFemaleOnly {
		return true
	}

	titleLower := strings.ToLower(r.Title)

	// If title mentions a sex toy, skip ambiguous words ("bbc dildo", "artificial cock")
	hasToy := false
	for _, tw := range toyWords {
		if strings.Contains(titleLower, tw) {
			hasToy = true
			break
		}
	}

	for _, word := range malePresenceWords {
		if hasToy {
			isAmbiguous := false
			for _, aw := range ambiguousMaleWords {
				if word == aw {
					isAmbiguous = true
					break
				}
			}
			if isAmbiguous {
				continue
			}
		}
		if containsWholeWord(titleLower, word) {
			return false
		}
	}

	return true
}
