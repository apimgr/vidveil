// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// SearchSuggestions contains comprehensive adult content search terms (800+ terms)
// Built into the binary at compile time for privacy and performance
// Organized by category for easier maintenance
var SearchSuggestions = []string{
	// Popular General Terms
	"amateur", "teen", "milf", "mature", "asian", "ebony", "latina", "blonde",
	"brunette", "redhead", "big tits", "big ass", "big dick", "small tits",
	"petite", "bbw", "chubby", "skinny", "fit", "athletic", "big boobs",
	"natural tits", "fake tits", "busty", "curvy", "thick", "slim", "young",
	"old", "granny", "gilf", "mom", "step mom", "step sister", "step daughter",
	"stepdad", "dad", "daddy", "daughter", "sister", "brother", "son", "family",
	"taboo", "forbidden", "homemade", "amateur couple", "real couple",
	"verified amateur", "verified couple",

	// Ethnicity & Race
	"arab", "indian", "pakistani", "turkish", "japanese", "chinese", "korean",
	"thai", "vietnamese", "filipina", "indonesian", "malaysian", "singaporean",
	"taiwanese", "russian", "ukrainian", "polish", "czech", "german", "french",
	"italian", "spanish", "british", "irish", "swedish", "norwegian", "danish",
	"finnish", "dutch", "belgian", "brazilian", "colombian", "mexican",
	"argentinian", "venezuelan", "puerto rican", "cuban", "dominican", "african",
	"caribbean",

	// Body Types & Features
	"muscular", "toned", "ripped", "bodybuilder", "fitness model", "yoga pants",
	"leggings", "tall", "short", "midget", "dwarf", "giant", "amazon",
	"voluptuous", "plump", "round", "soft", "jiggly", "bouncing", "huge tits",
	"massive tits", "small boobs", "flat chest", "perky tits", "saggy tits",
	"fake boobs", "implants", "natural boobs", "round ass", "bubble butt",
	"fat ass", "huge ass", "phat ass", "pawg", "thick thighs", "skinny legs",
	"long legs", "hairy", "shaved", "trimmed", "bald pussy", "hairy pussy",
	"bush", "landing strip", "smooth", "tattoo", "tattooed", "pierced",
	"piercings", "pregnant", "lactating", "muscular woman",

	// Hair Color & Style
	"blonde hair", "brown hair", "black hair", "red hair", "ginger",
	"strawberry blonde", "platinum blonde", "dirty blonde", "dyed hair",
	"colored hair", "pink hair", "blue hair", "purple hair", "green hair",
	"rainbow hair", "long hair", "short hair", "ponytail", "pigtails", "braids",
	"bun", "curly hair", "straight hair", "wavy hair", "bald",

	// Age Categories
	"18 years old", "19 years old", "20s", "30s", "40s", "50s", "60s", "70s",
	"college", "university", "student", "schoolgirl", "cheerleader",
	"young adult", "middle aged", "older woman", "older man", "age gap",
	"age difference", "barely legal",

	// Sexual Acts
	"blowjob", "deepthroat", "gagging", "sloppy blowjob", "face fuck", "oral",
	"cunnilingus", "pussy licking", "eating pussy", "pussy eating", "69",
	"rimming", "rimjob", "ass licking", "fingering", "fisting", "anal fisting",
	"vaginal fisting", "handjob", "footjob", "titjob", "boobjob", "titty fuck",
	"tit fuck", "masturbation", "solo", "solo female", "solo male",
	"mutual masturbation", "jerk off", "jerking", "stroking", "rubbing",
	"dildo", "vibrator", "toy", "sex toy", "fucking", "sex", "hardcore",
	"rough", "rough sex", "hard fuck", "pounding", "drilling", "missionary",
	"doggy style", "doggystyle", "from behind", "cowgirl", "reverse cowgirl",
	"standing", "standing sex", "sitting", "lap dance", "grinding", "twerking",
	"riding", "ride", "bounce", "bouncing", "anal", "anal sex", "ass fuck",
	"butt fuck", "anal creampie", "double penetration", "dp", "double anal",
	"triple penetration", "gangbang", "gang bang", "reverse gangbang", "orgy",
	"group sex", "threesome", "foursome", "ffm", "mmf", "mff", "fmf", "mmff",
	"lesbian", "lesbian sex", "girl on girl", "scissoring", "tribbing",
	"pussy rubbing", "fingering lesbian", "strap on", "strapon", "pegging",
	"dildo fucking",

	// Fetishes & Kinks
	"bdsm", "bondage", "tied up", "rope", "shibari", "handcuffs", "chains",
	"collar", "leash", "slave", "master", "mistress", "dom", "domination",
	"submission", "submissive", "dominant", "femdom", "maledom", "spanking",
	"whipping", "flogging", "caning", "punishment", "discipline", "pain",
	"masochism", "sadism", "humiliation", "degradation", "worship",
	"foot worship", "foot fetish", "feet", "toes", "soles", "foot licking",
	"shoe fetish", "high heels", "heels", "stockings", "pantyhose", "nylon",
	"fishnets", "lingerie", "panties", "bra", "underwear", "thong", "g-string",
	"bodysuit", "latex", "leather", "pvc", "rubber", "spandex", "lycra",
	"satin", "silk", "uniform", "cosplay", "costume", "roleplay", "nurse",
	"doctor", "teacher", "secretary", "maid", "police", "military", "stripper",
	"pornstar", "hooker", "prostitute", "escort", "voyeur", "exhibitionist",
	"public", "outdoor", "beach", "car", "shower", "bath", "pool", "hotel",
	"office", "classroom", "library", "gym", "yoga", "massage", "oil", "oiled",
	"wet", "messy", "food", "whipped cream", "chocolate", "syrup", "squirting",
	"squirt", "female ejaculation", "pissing", "peeing", "pee", "watersports",
	"golden shower", "spit", "spitting", "drool", "drooling",

	// Scenarios & Situations
	"casting", "casting couch", "fake agent", "fake taxi", "fake driving",
	"fake cop", "pickup", "picked up", "stranger", "public pickup",
	"beach pickup", "street pickup", "one night stand", "hookup", "tinder",
	"dating app", "blind date", "first date", "first time", "virgin",
	"losing virginity", "defloration", "corruption", "seduction", "seduce",
	"temptation", "cheating", "affair", "infidelity", "cuckold", "hotwife",
	"wife sharing", "swinger", "swingers", "swapping", "swap", "party",
	"sex party", "college party", "frat party", "drunk", "tipsy", "intoxicated",
	"sleepover", "camping", "vacation", "holiday", "hotel room", "motel",
	"airbnb", "neighbors", "roommate", "landlord", "rent", "tenant", "boss",
	"employee", "coworker", "interview", "job interview", "audition",
	"photoshoot", "model", "modeling", "babysitter", "nanny", "tutor", "coach",
	"personal trainer", "massage therapist", "therapist", "counselor",
	"dentist", "gynecologist", "doctor patient", "nurse patient",
	"teacher student", "professor student", "blackmail", "extortion", "revenge",
	"caught", "surprise", "unexpected", "accidental", "mistake",

	// Positions & Actions
	"bent over", "legs up", "legs spread", "spread eagle", "splits", "flexible",
	"contortion", "upside down", "headstand", "handstand", "acrobatic",
	"yoga pose", "against wall", "on table", "on desk", "on couch", "on bed",
	"on floor", "in chair", "face down", "face up", "on knees", "kneeling",
	"squatting", "lying down", "side by side", "spooning", "behind", "in front",
	"above", "below", "eye contact", "looking at camera", "looking away",
	"moaning", "screaming", "dirty talk", "begging", "crying", "laughing",
	"smiling", "serious", "intense", "passionate", "sensual", "romantic",
	"loving", "aggressive",

	// Production & Quality
	"hd", "1080p", "4k", "uhd", "60fps", "high quality", "professional",
	"amateur video", "homemade video", "pov", "point of view", "first person",
	"gonzo", "reality", "real", "authentic", "verified", "exclusive", "premium",
	"vip", "compilation", "best of", "top rated", "most viewed", "trending",
	"popular", "new", "recent", "latest", "classic", "vintage", "retro", "80s",
	"90s", "2000s", "behind the scenes", "bts", "bloopers", "outtakes",
	"uncut", "raw", "uncensored",

	// Relationship Types
	"couple", "married couple", "husband wife", "boyfriend girlfriend", "bf gf",
	"ex girlfriend", "ex boyfriend", "fuckbuddy", "friends with benefits", "fwb",
	"casual", "dating", "relationship", "lovers", "partners", "swingers couple",
	"polyamory", "cuckold couple", "hotwife couple", "amateur couple sex",
	"couple swap", "couple exchange", "group of couples", "multiple couples",
	"stranger couple", "unknown couple", "shy couple", "first time couple",
	"nervous couple", "experienced couple", "kinky couple", "vanilla couple",
	"adventurous couple",

	// Settings & Locations
	"bedroom", "bathroom", "kitchen", "living room", "garage", "basement",
	"attic", "balcony", "patio", "garden", "backyard", "pool area", "hot tub",
	"sauna", "locker room", "shower room", "changing room", "fitting room",
	"dressing room", "backstage", "stage", "club", "nightclub", "bar", "pub",
	"restaurant", "cafe", "cinema", "theater", "car interior", "van", "truck",
	"bus", "train", "plane", "boat", "yacht", "tent", "camper", "rv", "cabin",
	"cottage", "mansion", "penthouse", "apartment", "dorm room", "hotel suite",
	"motel room", "beach house", "vacation rental",

	// Clothing & Accessories
	"naked", "nude", "topless", "bottomless", "fully clothed", "partially clothed",
	"clothed sex", "dressed", "undressed", "undressing", "stripping",
	"striptease", "taking off", "removing", "revealing", "flashing", "upskirt",
	"downblouse", "cleavage", "mini skirt", "short skirt", "tight dress",
	"bodycon dress", "cocktail dress", "evening gown", "sundress", "tank top",
	"crop top", "t-shirt", "shirt", "blouse", "sweater", "cardigan", "jacket",
	"coat", "jeans", "shorts", "skirt", "dress", "robe", "bathrobe", "towel",
	"bikini", "swimsuit", "one piece", "two piece", "thong bikini",
	"micro bikini", "see through", "transparent", "sheer", "mesh", "lace",
	"velvet", "cotton", "wool",

	// Specific Acts & Details
	"creampie", "cum inside", "internal cumshot", "breeding", "impregnation",
	"facial", "cum on face", "bukakke", "cum on tits", "cum on ass",
	"cum on stomach", "cum on back", "cum on feet", "cumshot", "cum shot",
	"cumming", "orgasm", "climax", "coming", "multiple orgasms",
	"shaking orgasm", "screaming orgasm", "intense orgasm", "real orgasm",
	"fake orgasm", "premature", "edging", "denial", "tease", "teasing",
	"dirty talking", "moaning loud", "loud moans", "whispering", "quiet",
	"silent", "muted", "gagged", "ball gag", "tape gag", "panty gag", "choking",
	"breath play", "asphyxiation", "slapping", "face slapping", "ass slapping",
	"spanking ass", "tit slapping", "pussy slapping", "cock slapping",
	"dick slapping", "spitting on", "spit on face", "spit on pussy",
	"spit on cock", "drooling on", "slobbering", "messy oral", "sloppy", "wet",
	"soaking", "drenched", "sweaty", "steamy", "hot", "gentle", "soft", "tender",
	"slow", "fast", "hard", "deep", "shallow",

	// Popular Niches
	"gonzo porn", "reality porn", "casting porn", "pov porn", "virtual reality",
	"vr porn", "360 degree", "interactive", "jerk off instruction", "joi",
	"cum countdown", "asmr", "erotic audio", "phone sex", "sexting", "cam girl",
	"webcam", "live cam", "chaturbate", "onlyfans", "premium snapchat",
	"patreon", "custom video", "personalized", "fan request", "user submitted",
	"viewer request", "interactive toy", "lovense", "ohmibod", "tip controlled",
	"donation controlled", "public show", "private show", "exclusive content",
	"members only", "subscription", "paysite",

	// Combinations & Modifiers
	"interracial", "bbc", "big black cock", "wmaf", "bmwf", "amwf",
	"age gap relationship", "size difference", "height difference",
	"muscle worship", "bicep worship", "abs worship", "pussy worship",
	"ass worship", "tit worship", "cock worship", "dick worship",
	"worship session", "marathon sex", "long session", "extended", "all night",
	"quick", "quickie", "fast fuck", "wham bam", "slow fuck", "slow sex",
	"sensual sex", "romantic sex",

	// Additional Common Terms
	"babe", "bisexual", "bukkake", "cartoon", "celebrity", "compilation",
	"cuckold", "cumshot", "dildo", "facial", "femdom", "fetish", "glamour",
	"granny", "group", "handjob", "hentai", "hidden cam", "interracial",
	"lingerie", "massage", "masturbation", "nurse", "orgasm", "orgy",
	"parody", "pissing", "pornstar", "reality", "rough", "secretary",
	"shemale", "sleeping", "smoking", "softcore", "solo", "swallow", "swinger",
	"tattoo", "toys", "trans", "uncensored", "vintage", "voyeur", "webcam",
	"wife",

	// Popular Descriptive Terms
	"hot blonde", "hot brunette", "hot milf", "hot teen", "sexy girl",
	"busty babe", "thick ass", "natural beauty", "fit body", "curvy body",
	"on top", "try not to cum", "homemade porn", "cheating wife", "hot wife",
	"step family", "caught cheating", "real orgasm",
}

// SearchSuggestion represents a search term suggestion
type SearchSuggestion struct {
	Term  string `json:"term"`
	Score int    `json:"-"`
}

var (
	// customTerms holds additional terms from config file
	customTerms []string
)

// SetCustomTerms sets additional search terms from config
// These are ADDED to the built-in SearchSuggestions
func SetCustomTerms(terms []string) {
	customTerms = terms
}

// getAllSuggestions returns built-in suggestions + custom terms from config
func getAllSuggestions() []string {
	if len(customTerms) == 0 {
		return SearchSuggestions
	}
	// Merge built-in and custom terms
	all := make([]string, 0, len(SearchSuggestions)+len(customTerms))
	all = append(all, SearchSuggestions...)
	all = append(all, customTerms...)
	return all
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
	allTerms := getAllSuggestions()

	for _, term := range allTerms {
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

// GetRelatedSearches returns search terms related to the given query
// Finds suggestions that share words with or are conceptually related to the query
func GetRelatedSearches(query string, maxResults int) []string {
	if query == "" || maxResults <= 0 {
		return nil
	}

	query = strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(query)
	if len(queryWords) == 0 {
		return nil
	}

	allTerms := getAllSuggestions()
	type scoredTerm struct {
		term  string
		score int
	}

	var related []scoredTerm
	seenTerms := make(map[string]bool)

	for _, term := range allTerms {
		termLower := strings.ToLower(term)

		// Skip if exact match with query
		if termLower == query {
			continue
		}

		// Skip if we've seen this term
		if seenTerms[termLower] {
			continue
		}

		score := 0
		termWords := strings.Fields(termLower)

		// Score based on shared words
		for _, qw := range queryWords {
			if len(qw) < 3 {
				continue
			}
			for _, tw := range termWords {
				if tw == qw {
					// Exact word match
					score += 20
				} else if strings.HasPrefix(tw, qw) || strings.HasPrefix(qw, tw) {
					// Prefix match
					score += 10
				} else if strings.Contains(tw, qw) || strings.Contains(qw, tw) {
					// Contains match
					score += 5
				}
			}
		}

		// Also check if query is a substring or vice versa
		if strings.Contains(termLower, query) || strings.Contains(query, termLower) {
			score += 15
		}

		if score > 0 {
			seenTerms[termLower] = true
			related = append(related, scoredTerm{term: term, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(related, func(i, j int) bool {
		return related[i].score > related[j].score
	})

	// Limit and extract terms
	if len(related) > maxResults {
		related = related[:maxResults]
	}

	result := make([]string, len(related))
	for i, r := range related {
		result[i] = r.term
	}

	return result
}
