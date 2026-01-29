// SPDX-License-Identifier: MIT
package engine

import (
	"sort"
	"strings"
)

// Performers contains popular adult performer names for autocomplete
// Names are stored without special characters for easier matching
// Updated periodically based on search trends
var Performers = []string{
	// Top-tier performers (most searched)
	"mia khalifa", "lana rhoades", "riley reid", "abella danger", "angela white",
	"adriana chechik", "emily willis", "gabbie carter", "eva elfie", "autumn falls",
	"elsa jean", "mia malkova", "kendra lust", "brandi love", "lisa ann",
	"nicole aniston", "alexis texas", "madison ivy", "asa akira", "sasha grey",
	"jenna jameson", "christy mack", "dani daniels", "lexi belle", "kagney linn karter",
	"phoenix marie", "keisha grey", "valentina nappi", "gianna michaels", "sara jay",
	"alexis fawx", "julia ann", "syren de mer", "cory chase", "cherie deville",
	"india summer", "reagan foxx", "dee williams", "kit mercer", "london river",

	// Popular current performers
	"lena paul", "violet myers", "skylar vox", "natasha nice", "kenzie reeves",
	"gina valentina", "karlee grey", "jill kassidy", "gia derza", "maya bijou",
	"jane wilde", "vina sky", "kira noir", "ana foxxx", "daya knight",
	"scarlit scandal", "jenna foxx", "september reign", "misty stone", "chanell heart",
	"kendra sunderland", "blair williams", "jessa rhodes", "carter cruise", "aj applegate",
	"kelsi monroe", "anikka albrite", "remy lacroix", "tori black", "kayden kross",
	"jesse jane", "stoya", "leah gotti", "august ames", "peta jensen",
	"anissa kate", "aletta ocean", "jasmine jae", "danny d", "jordi",

	// MILF/Mature performers
	"ava addams", "ariella ferrera", "nikki benz", "bridgette b", "jewels jade",
	"diamond foxxx", "veronica avluv", "dana dearmond", "tanya tate", "danica dillon",
	"mercedes carrera", "kianna dior", "jessica jaymes", "shay fox", "alura jenson",
	"karen fisher", "samantha 38g", "kayla kleevage", "claudia marie", "minka",
	"dee williams", "rachael cavalli", "joslyn james", "holly halston", "charlee chase",

	// Teen/Young performers
	"kenzie madison", "lulu chu", "chloe cherry", "haley reed", "harmony wonder",
	"alex coal", "nia nacci", "kylie rocket", "lacy lennon", "paige owens",
	"emma hix", "athena faris", "angel smalls", "piper perri", "riley star",
	"kristen scott", "alex grey", "lily rader", "naomi swann", "aria lee",
	"brooklyn gray", "jazmin luv", "xxlayna marie", "maya woulfe", "lily larimar",

	// Asian performers
	"marica hase", "london keyes", "alina li", "cindy starfall", "jade kush",
	"ayumi anime", "kendra spade", "ember snow", "morgan lee", "kalina ryu",
	"mia li", "sharon lee", "rae lil black", "polly pons", "may thai",
	"asia akira", "hitomi tanaka", "julia", "eimi fukada", "yua mikami",

	// Latina performers
	"luna star", "rose monroe", "kitty caprice", "juliana vega", "diamond kitty",
	"lela star", "esperanza gomez", "franceska jaimes", "canela skin", "katana kombat",
	"serena santos", "victoria june", "monica asis", "sophia leone", "aaliyah hadid",

	// Ebony performers
	"jenna foxx", "chanell heart", "diamond jackson", "anya ivy", "teanna trump",
	"moriah mills", "brittney white", "skyler nicole", "nadia jay", "yasmine de leon",
	"layton benton", "harley dean", "skin diamond", "jezabel vessir", "raven redmond",

	// European performers
	"anna de ville", "alysa gap", "nataly gold", "francesca le", "proxy paige",
	"casey calvert", "bonnie rotten", "skin diamond", "katrina jade", "jynx maze",
	"holly michaels", "lily love", "casey cumz", "raven bay", "marley brinx",
	"stella cox", "sienna day", "ella hughes", "carmel anderson", "rebecca more",

	// Male performers (for searches)
	"johnny sins", "manuel ferrara", "xander corvus", "keiran lee", "ramon nomar",
	"mick blue", "markus dupree", "dredd", "jax slayher", "isiah maxwell",
	"rob piper", "ricky johnson", "small hands", "tyler nixon", "seth gamble",
	"ryan mclane", "chad white", "damon dice", "lucas frost", "codey steele",
}

// PerformerSuggestion represents a performer autocomplete suggestion
type PerformerSuggestion struct {
	Name  string `json:"name"`
	Score int    `json:"-"`
}

// AutocompletePerformers returns performer suggestions for a partial name
func AutocompletePerformers(prefix string, maxResults int) []PerformerSuggestion {
	if prefix == "" || maxResults <= 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 2 {
		return nil
	}

	var suggestions []PerformerSuggestion

	for _, name := range Performers {
		nameLower := strings.ToLower(name)

		score := 0
		if strings.HasPrefix(nameLower, prefix) {
			// Exact prefix match - highest score
			score = 100 - len(name)
		} else {
			// Check each word in the name
			words := strings.Fields(nameLower)
			for _, word := range words {
				if strings.HasPrefix(word, prefix) {
					score = 80 - len(name)
					break
				}
			}
			// Contains match - lower score
			if score == 0 && strings.Contains(nameLower, prefix) {
				score = 50 - len(name)
			}
		}

		if score > 0 {
			suggestions = append(suggestions, PerformerSuggestion{
				Name:  name,
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

// GetPopularPerformers returns a list of popular performers
func GetPopularPerformers(count int) []string {
	// Return the top performers (first in list = most popular)
	if count > len(Performers) {
		count = len(Performers)
	}
	if count > 20 {
		count = 20 // Cap at 20 for popular list
	}
	return Performers[:count]
}
