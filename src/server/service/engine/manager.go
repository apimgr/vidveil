// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// EngineManager manages all search engines
// Per PART 32: Supports Tor outbound network for anonymized engine queries
type EngineManager struct {
	engines     map[string]SearchEngine
	appConfig   *config.AppConfig
	torProvider TorClientProvider // Per PART 32: Tor client provider for outbound
	mu          sync.RWMutex
}

// NewEngineManager creates a new engine manager
func NewEngineManager(appConfig *config.AppConfig) *EngineManager {
	return &EngineManager{
		engines:   make(map[string]SearchEngine),
		appConfig: appConfig,
	}
}

// SetTorProvider sets the Tor client provider for all engines
// Per PART 32: When set and UseNetwork is enabled, engine queries are anonymized through Tor
func (m *EngineManager) SetTorProvider(provider TorClientProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.torProvider = provider

	// Update all existing engines with the Tor provider
	for _, engine := range m.engines {
		// Use the TorConfigurableEngine interface
		if torEngine, ok := engine.(TorConfigurableEngine); ok {
			torEngine.SetTorProvider(provider)
		}
	}
}

// InitializeEngines sets up all available engines
func (m *EngineManager) InitializeEngines() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Tier 1 - Major Sites (always enabled by default)
	m.engines["pornhub"] = NewPornHubEngine(m.appConfig)
	m.engines["xvideos"] = NewXVideosEngine(m.appConfig)
	m.engines["xnxx"] = NewXNXXEngine(m.appConfig)
	m.engines["redtube"] = NewRedTubeEngine(m.appConfig)
	m.engines["xhamster"] = NewXHamsterEngine(m.appConfig)

	// Tier 2 - Popular Sites (enabled by default)
	m.engines["eporner"] = NewEpornerEngine(m.appConfig)
	m.engines["youporn"] = NewYouPornEngine(m.appConfig)
	m.engines["pornmd"] = NewPornMDEngine(m.appConfig)

	// Tier 3 - Additional Sites (disabled by default, enable via config)
	m.engines["4tube"] = NewFourTubeEngine(m.appConfig)
	m.engines["fux"] = NewFuxEngine(m.appConfig)
	m.engines["porntube"] = NewPornTubeEngine(m.appConfig)
	m.engines["youjizz"] = NewYouJizzEngine(m.appConfig)
	m.engines["sunporno"] = NewSunPornoEngine(m.appConfig)
	m.engines["txxx"] = NewTxxxEngine(m.appConfig)
	m.engines["nuvid"] = NewNuvidEngine(m.appConfig)
	m.engines["tnaflix"] = NewTNAFlixEngine(m.appConfig)
	m.engines["drtuber"] = NewDrTuberEngine(m.appConfig)
	m.engines["empflix"] = NewEMPFlixEngine(m.appConfig)
	m.engines["hellporno"] = NewHellPornoEngine(m.appConfig)
	m.engines["alphaporno"] = NewAlphaPornoEngine(m.appConfig)
	m.engines["pornflip"] = NewPornFlipEngine(m.appConfig)
	m.engines["gotporn"] = NewGotPornEngine(m.appConfig)
	m.engines["xxxymovies"] = NewXXXYMoviesEngine(m.appConfig)
	m.engines["lovehomeporn"] = NewLoveHomePornEngine(m.appConfig)

	// Tier 4 - Additional yt-dlp supported sites
	m.engines["pornerbros"] = NewPornerBrosEngine(m.appConfig)
	m.engines["nonktube"] = NewNonkTubeEngine(m.appConfig)
	m.engines["nubilesporn"] = NewNubilesPornEngine(m.appConfig)
	m.engines["pornbox"] = NewPornboxEngine(m.appConfig)
	m.engines["porntop"] = NewPornTopEngine(m.appConfig)
	m.engines["pornotube"] = NewPornotubeEngine(m.appConfig)
	// vporn removed - site inaccessible (geo-blocked/Cloudflare)
	m.engines["pornhd"] = NewPornHDEngine(m.appConfig)
	m.engines["xbabe"] = NewXBabeEngine(m.appConfig)
	m.engines["pornone"] = NewPornOneEngine(m.appConfig)
	m.engines["pornhat"] = NewPornHatEngine(m.appConfig)
	m.engines["porntrex"] = NewPornTrexEngine(m.appConfig)
	m.engines["hqporner"] = NewHqpornerEngine(m.appConfig)
	m.engines["vjav"] = NewVJAVEngine(m.appConfig)
	m.engines["flyflv"] = NewFlyflvEngine(m.appConfig)
	m.engines["tube8"] = NewTube8Engine(m.appConfig)

	// Tier 5 - New engines
	m.engines["anyporn"] = NewAnyPornEngine(m.appConfig)
	m.engines["tubegalore"] = NewTubeGaloreEngine(m.appConfig)
	m.engines["motherless"] = NewMotherlessEngine(m.appConfig)

	// Tier 6 - Additional engines
	m.engines["3movs"] = NewThreeMovsEngine(m.appConfig)

	// Apply configuration
	m.applyConfig()
}

// applyConfig applies engine-specific configuration
func (m *EngineManager) applyConfig() {
	// All engines are enabled by default
	// DefaultEngines config can limit which engines to use
	defaultEngines := m.appConfig.Search.DefaultEngines

	// If default_engines is specified, only enable those
	if len(defaultEngines) > 0 {
		enabledSet := make(map[string]bool)
		for _, name := range defaultEngines {
			enabledSet[name] = true
		}

		for name, engine := range m.engines {
			if configurable, ok := engine.(ConfigurableSearchEngine); ok {
				configurable.SetEnabled(enabledSet[name])
			}
		}
	}

}

// Search performs a search across enabled engines
func (m *EngineManager) Search(ctx context.Context, query string, page int, engineNames []string) *model.SearchResponse {
	startTime := time.Now()

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Determine which engines to use
	enginesToUse := m.getEnginesToUse(engineNames)

	// Search in parallel
	var wg sync.WaitGroup
	resultsChan := make(chan engineResult, len(enginesToUse))

	for _, engine := range enginesToUse {
		wg.Add(1)
		go func(e SearchEngine) {
			defer wg.Done()
			engineStart := time.Now()
			results, err := e.Search(ctx, query, page)
			resultsChan <- engineResult{
				engine:         e.Name(),
				results:        results,
				err:            err,
				responseTimeMS: time.Since(engineStart).Milliseconds(),
			}
		}(engine)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results with deduplication
	var allResults []model.VideoResult
	var enginesUsed []string
	var enginesFailed []string
	// Track seen URLs and titles for deduplication
	seenURLs := make(map[string]bool)
	seenTitles := make(map[string]bool)
	// Track per-engine stats
	engineStats := make(map[string]model.EngineStatInfo)

	minDuration := m.appConfig.Search.MinDurationSeconds

	for result := range resultsChan {
		if result.err != nil {
			enginesFailed = append(enginesFailed, result.engine)
			engineStats[result.engine] = model.EngineStatInfo{
				ResponseTimeMS: result.responseTimeMS,
				ResultCount:    0,
				Error:          result.err.Error(),
			}
		} else {
			enginesUsed = append(enginesUsed, result.engine)
			resultCount := 0
			// Filter results by thumbnail validity, minimum duration, term matching, and deduplicate
			for _, r := range result.results {
				// Skip results with empty/invalid thumbnails
				if !isValidThumbnail(r.Thumbnail) {
					continue
				}
				// Skip if duration is known and below minimum
				if minDuration > 0 && r.DurationSeconds > 0 && r.DurationSeconds < minDuration {
					continue
				}
				// AND-based term filter: result must match ALL search terms (using synonyms)
				if !resultMatchesAllTerms(r, query) {
					continue
				}
				// Deduplicate by normalized URL and title
				normalizedURL := normalizeURL(r.URL)
				normalizedTitle := normalizeTitle(r.Title)
				// Check URL first
				if seenURLs[normalizedURL] {
					continue
				}
				// Check normalized title (for cross-engine duplicates)
				if normalizedTitle != "" && seenTitles[normalizedTitle] {
					continue
				}
				// Mark as seen
				seenURLs[normalizedURL] = true
				if normalizedTitle != "" {
					seenTitles[normalizedTitle] = true
				}
				allResults = append(allResults, r)
				resultCount++
			}
			engineStats[result.engine] = model.EngineStatInfo{
				ResponseTimeMS: result.responseTimeMS,
				ResultCount:    resultCount,
			}
		}
	}

	// Sort results by relevance and filter by minimum score
	// Default minimum score of 10.0 ensures at least one query word matches
	minScore := m.appConfig.Search.MinRelevanceScore
	allResults = sortAndFilterByRelevance(allResults, query, minScore)

	// Build response
	elapsed := time.Since(startTime)

	return &model.SearchResponse{
		Ok: true,
		Data: model.SearchData{
			Query:         query,
			Results:       allResults,
			EnginesUsed:   enginesUsed,
			EnginesFailed: enginesFailed,
			SearchTimeMS:  elapsed.Milliseconds(),
			EngineStats:   engineStats,
		},
		Pagination: model.PaginationData{
			Page:  page,
			Limit: m.appConfig.Search.ResultsPerPage,
			Total: len(allResults),
			Pages: (len(allResults) + m.appConfig.Search.ResultsPerPage - 1) / m.appConfig.Search.ResultsPerPage,
		},
	}
}

// scoredResult holds a result with its relevance score for sorting
type scoredResult struct {
	result model.VideoResult
	score  float64
}

// sortAndFilterByRelevance sorts results by relevance score and filters by minimum score
// Returns filtered results that meet the minimum relevance threshold
func sortAndFilterByRelevance(results []model.VideoResult, query string, minScore float64) []model.VideoResult {
	return sortAndFilterByRelevanceWithOperators(results, query, minScore, nil, nil, nil)
}

// sortAndFilterByRelevanceWithOperators sorts results by relevance and applies search operators
// exactPhrases requires results to contain all specified phrases
// exclusions removes results containing any excluded word
// performers filters by performer name (OR match)
func sortAndFilterByRelevanceWithOperators(results []model.VideoResult, query string, minScore float64, exactPhrases []string, exclusions []string, performers []string) []model.VideoResult {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// First, apply search operators (exclusions, exact phrases, performers)
	if len(exactPhrases) > 0 || len(exclusions) > 0 || len(performers) > 0 {
		var operatorFiltered []model.VideoResult
		for _, r := range results {
			titleLower := strings.ToLower(r.Title)

			// Check exclusions - skip if any excluded word is found
			excluded := false
			for _, ex := range exclusions {
				if strings.Contains(titleLower, ex) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}

			// Check exact phrases - require all phrases to be present
			hasAllPhrases := true
			for _, phrase := range exactPhrases {
				if !strings.Contains(titleLower, strings.ToLower(phrase)) {
					hasAllPhrases = false
					break
				}
			}
			if !hasAllPhrases {
				continue
			}

			// Check performer filter - at least one performer must match (OR)
			if len(performers) > 0 {
				performerLower := strings.ToLower(r.Performer)
				matchesPerformer := false
				for _, p := range performers {
					if strings.Contains(performerLower, p) {
						matchesPerformer = true
						break
					}
				}
				if !matchesPerformer {
					continue
				}
			}

			operatorFiltered = append(operatorFiltered, r)
		}
		results = operatorFiltered
	}

	// If no query words, return operator-filtered results without scoring
	if len(queryWords) == 0 {
		return results
	}

	// Calculate scores and create scored results
	scored := make([]scoredResult, len(results))
	for i, r := range results {
		scored[i] = scoredResult{
			result: r,
			score:  calculateRelevanceScore(r, queryLower, queryWords),
		}
	}

	// Sort by score descending
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Filter by minimum score and extract results
	filtered := make([]model.VideoResult, 0, len(scored))
	for _, sr := range scored {
		if minScore <= 0 || sr.score >= minScore {
			filtered = append(filtered, sr.result)
		}
	}

	return filtered
}

// calculateRelevanceScore computes a relevance score for a result
func calculateRelevanceScore(r model.VideoResult, queryLower string, queryWords []string) float64 {
	titleLower := strings.ToLower(r.Title)
	score := 0.0

	// Exact match bonus (highest priority)
	if strings.Contains(titleLower, queryLower) {
		score += 100.0
	}

	// Word match scoring
	matchedWords := 0
	for _, word := range queryWords {
		if len(word) < 2 {
			continue
		}
		if strings.Contains(titleLower, word) {
			matchedWords++
			// Bonus for word at start of title
			if strings.HasPrefix(titleLower, word) {
				score += 5.0
			}
		}
	}

	// Percentage of query words matched
	if len(queryWords) > 0 {
		matchRatio := float64(matchedWords) / float64(len(queryWords))
		score += matchRatio * 50.0
	}

	// Quality bonus (HD/4K content ranked higher)
	quality := strings.ToUpper(r.Quality)
	if strings.Contains(quality, "4K") || strings.Contains(quality, "2160") {
		score += 10.0
	} else if strings.Contains(quality, "1080") || strings.Contains(quality, "HD") {
		score += 5.0
	} else if strings.Contains(quality, "720") {
		score += 2.0
	}

	// Views bonus (logarithmic scale to prevent domination)
	if r.ViewsCount > 0 {
		// log10(1000) = 3, log10(1000000) = 6
		viewScore := 0.0
		if r.ViewsCount >= 1000000 {
			viewScore = 6.0
		} else if r.ViewsCount >= 100000 {
			viewScore = 5.0
		} else if r.ViewsCount >= 10000 {
			viewScore = 4.0
		} else if r.ViewsCount >= 1000 {
			viewScore = 3.0
		} else if r.ViewsCount >= 100 {
			viewScore = 2.0
		} else {
			viewScore = 1.0
		}
		score += viewScore
	}

	// Duration preference (mid-length videos often preferred)
	if r.DurationSeconds > 0 {
		// Prefer 5-30 minute videos
		if r.DurationSeconds >= 300 && r.DurationSeconds <= 1800 {
			score += 2.0
		}
	}

	// Shorter titles often more relevant (less filler)
	if len(r.Title) > 0 && len(r.Title) < 60 {
		score += 1.0
	}

	return score
}

// QualityLevel represents video quality as a numeric value for comparison
// Higher values = better quality
const (
	QualityUnknown = 0
	Quality240p    = 240
	Quality360p    = 360
	Quality480p    = 480
	Quality720p    = 720
	Quality1080p   = 1080
	Quality1440p   = 1440
	Quality4K      = 2160
)

// ParseQualityLevel converts a quality string to a numeric level
// Returns QualityUnknown (0) if quality cannot be determined
func ParseQualityLevel(quality string) int {
	if quality == "" {
		return QualityUnknown
	}

	q := strings.ToUpper(quality)

	// Check for explicit resolution numbers
	if strings.Contains(q, "4K") || strings.Contains(q, "2160") || strings.Contains(q, "UHD") {
		return Quality4K
	}
	if strings.Contains(q, "1440") || strings.Contains(q, "2K") || strings.Contains(q, "QHD") {
		return Quality1440p
	}
	if strings.Contains(q, "1080") || strings.Contains(q, "FHD") {
		return Quality1080p
	}
	if strings.Contains(q, "720") {
		return Quality720p
	}
	if strings.Contains(q, "480") || strings.Contains(q, "SD") {
		return Quality480p
	}
	if strings.Contains(q, "360") {
		return Quality360p
	}
	if strings.Contains(q, "240") {
		return Quality240p
	}

	// "HD" without specific resolution typically means 720p+
	if strings.Contains(q, "HD") {
		return Quality720p
	}

	return QualityUnknown
}

// meetsMinQuality checks if a result meets the minimum quality requirement
// Unknown quality (0) passes the filter to avoid excluding videos without quality info
func meetsMinQuality(resultQuality string, minQuality int) bool {
	if minQuality <= 0 {
		return true // No minimum set
	}

	level := ParseQualityLevel(resultQuality)
	if level == QualityUnknown {
		return true // Unknown quality passes (we don't want to filter videos without quality info)
	}

	return level >= minQuality
}

// getEnginesToUse returns the engines to use for search
func (m *EngineManager) getEnginesToUse(engineNames []string) []SearchEngine {
	var engines []SearchEngine

	if len(engineNames) == 0 {
		// Use all enabled engines
		for _, engine := range m.engines {
			if engine.IsAvailable() {
				engines = append(engines, engine)
			}
		}
	} else {
		// Use specified engines
		for _, name := range engineNames {
			if engine, ok := m.engines[name]; ok && engine.IsAvailable() {
				engines = append(engines, engine)
			}
		}
	}

	return engines
}

// GetEngine returns a specific engine by name
func (m *EngineManager) GetEngine(name string) (SearchEngine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engine, ok := m.engines[name]
	return engine, ok
}

// ListEngines returns information about all engines
func (m *EngineManager) ListEngines() []model.EngineInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []model.EngineInfo
	for _, engine := range m.engines {
		infos = append(infos, model.EngineInfo{
			Name:        engine.Name(),
			DisplayName: engine.DisplayName(),
			Enabled:     engine.IsAvailable(),
			Available:   engine.IsAvailable(),
			Tier:        engine.Tier(),
			Features:    getFeatures(engine),
		})
	}
	return infos
}

// EnabledCount returns the number of enabled engines
func (m *EngineManager) EnabledCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, engine := range m.engines {
		if engine.IsAvailable() {
			count++
		}
	}
	return count
}

// engineResult holds the result from a single engine search
type engineResult struct {
	engine         string
	results        []model.VideoResult
	err            error
	responseTimeMS int64
}

// isValidThumbnail checks if a thumbnail URL is valid and usable
// Discards empty, placeholder, or invalid thumbnails per IDEA.md
func isValidThumbnail(thumbnail string) bool {
	if thumbnail == "" {
		return false
	}
	// Check for common placeholder patterns
	lower := strings.ToLower(thumbnail)
	if strings.Contains(lower, "placeholder") ||
		strings.Contains(lower, "no-image") ||
		strings.Contains(lower, "noimage") ||
		strings.Contains(lower, "default_thumb") ||
		strings.Contains(lower, "blank.") ||
		strings.Contains(lower, "missing") {
		return false
	}
	// Must be a valid URL
	if !strings.HasPrefix(thumbnail, "http://") && !strings.HasPrefix(thumbnail, "https://") {
		return false
	}
	return true
}

// normalizeURL normalizes a URL for deduplication purposes
// Handles: http/https, www/non-www, trailing slashes, query params
func normalizeURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	// Lowercase the URL
	normalized := strings.ToLower(urlStr)

	// Remove protocol prefix for comparison
	normalized = strings.TrimPrefix(normalized, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")

	// Remove www. prefix
	normalized = strings.TrimPrefix(normalized, "www.")

	// Remove trailing slash
	normalized = strings.TrimSuffix(normalized, "/")

	// Remove common tracking parameters but keep essential ones
	// Split at ? to handle query params
	if idx := strings.Index(normalized, "?"); idx != -1 {
		basePath := normalized[:idx]
		// For video sites, the path usually contains the video ID
		// Remove query params for deduplication
		normalized = basePath
	}

	// Remove fragment
	if idx := strings.Index(normalized, "#"); idx != -1 {
		normalized = normalized[:idx]
	}

	return normalized
}

// normalizeTitle normalizes a title for fuzzy deduplication
// Removes special characters, normalizes whitespace, sorts words alphabetically
// This catches cross-engine duplicates with identical or near-identical titles
func normalizeTitle(title string) string {
	if title == "" {
		return ""
	}

	// Lowercase
	normalized := strings.ToLower(title)

	// Remove special characters (keep only alphanumeric and spaces)
	var result strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		}
	}
	normalized = result.String()

	// Split into words and filter out short words (likely noise)
	words := strings.Fields(normalized)
	var significantWords []string
	for _, w := range words {
		if len(w) > 2 { // Keep words with > 2 chars
			significantWords = append(significantWords, w)
		}
	}

	// Title too short after filtering - return empty to skip title dedup
	if len(significantWords) < 3 {
		return ""
	}

	// Sort words for position-independent matching
	sort.Strings(significantWords)

	return strings.Join(significantWords, " ")
}

// resultMatchesAllTerms checks if a video result matches ALL search terms using synonym expansion
// This implements AND logic: "pregnant teen lesbian" only returns results containing ALL three terms
// Each term can match via any of its synonyms (e.g., "teen" matches "18", "young", "barely legal", etc.)
func resultMatchesAllTerms(r model.VideoResult, query string) bool {
	// Expand query terms using taxonomy
	expandedTerms := ExpandSearchTerms(query)
	if len(expandedTerms) == 0 {
		return true // No terms to match
	}

	// Build combined text from title, tags, and performer
	combinedText := strings.ToLower(r.Title)
	if len(r.Tags) > 0 {
		combinedText += " " + strings.ToLower(strings.Join(r.Tags, " "))
	}
	if r.Performer != "" {
		combinedText += " " + strings.ToLower(r.Performer)
	}

	// Use taxonomy's MatchesAllTerms to check AND logic with synonyms
	return MatchesAllTerms(combinedText, expandedTerms)
}

// StreamResult represents a single result sent via SSE
type StreamResult struct {
	Result model.VideoResult `json:"result,omitempty"`
	Engine string        `json:"engine"`
	Done   bool          `json:"done"`
	Error  string        `json:"error,omitempty"`
}

// SearchStream performs a search across enabled engines and streams results via channel
// Results are deduplicated by URL across all engines
func (m *EngineManager) SearchStream(ctx context.Context, query string, page int, engineNames []string) <-chan StreamResult {
	return m.SearchStreamWithOperators(ctx, query, page, engineNames, nil, nil, nil, false, 0)
}

// SearchStreamWithOperators performs a streaming search with optional search operators
// exactPhrases requires results to contain all specified phrases
// exclusions removes results containing any excluded word
// performers filters by performer name (OR match)
// showAI overrides server AI filter setting (true = show AI content)
// minQuality filters by minimum quality level (0 = no filter, 360 = 360p+, etc.)
func (m *EngineManager) SearchStreamWithOperators(ctx context.Context, query string, page int, engineNames []string, exactPhrases []string, exclusions []string, performers []string, showAI bool, minQuality int) <-chan StreamResult {
	resultsChan := make(chan StreamResult, 100)

	go func() {
		defer close(resultsChan)

		m.mu.RLock()
		enginesToUse := m.getEnginesToUse(engineNames)
		m.mu.RUnlock()

		var wg sync.WaitGroup
		minDuration := m.appConfig.Search.MinDurationSeconds

		// Shared deduplication maps with mutex for concurrent access
		// Check both URL and normalized title to catch cross-engine duplicates
		var seenMu sync.Mutex
		seenURLs := make(map[string]bool)
		seenTitles := make(map[string]bool)

		for _, engine := range enginesToUse {
			wg.Add(1)
			go func(e SearchEngine) {
				defer wg.Done()

				results, err := e.Search(ctx, query, page)
				if err != nil {
					select {
					case resultsChan <- StreamResult{Engine: e.Name(), Error: err.Error()}:
					case <-ctx.Done():
					}
					return
				}

				// Stream each result individually with thumbnail validation and deduplication
				for _, r := range results {
					// Skip results with empty/invalid thumbnails
					if !isValidThumbnail(r.Thumbnail) {
						continue
					}
					// Skip if duration is known and below minimum
					if minDuration > 0 && r.DurationSeconds > 0 && r.DurationSeconds < minDuration {
						continue
					}

					// Apply search operators
					titleLower := strings.ToLower(r.Title)

					// Check exclusions - skip if any excluded word is found
					excluded := false
					for _, ex := range exclusions {
						if strings.Contains(titleLower, ex) {
							excluded = true
							break
						}
					}
					if excluded {
						continue
					}

					// Check exact phrases - require all phrases to be present
					hasAllPhrases := true
					for _, phrase := range exactPhrases {
						if !strings.Contains(titleLower, strings.ToLower(phrase)) {
							hasAllPhrases = false
							break
						}
					}
					if !hasAllPhrases {
						continue
					}

					// Check performer filter - at least one performer must match (OR)
					if len(performers) > 0 {
						performerLower := strings.ToLower(r.Performer)
						matchesPerformer := false
						for _, p := range performers {
							if strings.Contains(performerLower, p) {
								matchesPerformer = true
								break
							}
						}
						if !matchesPerformer {
							continue
						}
					}

					// AI content filter - skip AI-generated/deepfake content unless user overrides
					// showAI=true means user wants to see AI content (overrides server default)
					if m.appConfig.Search.AIFilter.Enabled && !showAI {
						if isAIGeneratedContent(titleLower, r.Tags, m.appConfig.Search.AIFilter.Keywords) {
							continue
						}
					}

					// Quality filter - skip if below minimum quality
					// Unknown quality passes to avoid filtering videos without quality info
					if !meetsMinQuality(r.Quality, minQuality) {
						continue
					}

					// AND-based term filter: result must match ALL search terms (using synonyms)
					if !resultMatchesAllTerms(r, query) {
						continue
					}

					// Deduplicate by normalized URL and title
					// URL handles: http/https, www, trailing slash differences
					// Title handles: cross-engine duplicates with matching content
					normalizedURL := normalizeURL(r.URL)
					normalizedTitle := normalizeTitle(r.Title)

					seenMu.Lock()
					// Check URL first
					if seenURLs[normalizedURL] {
						seenMu.Unlock()
						continue
					}
					// Check normalized title (for cross-engine duplicates)
					if normalizedTitle != "" && seenTitles[normalizedTitle] {
						seenMu.Unlock()
						continue
					}
					// Mark as seen
					seenURLs[normalizedURL] = true
					if normalizedTitle != "" {
						seenTitles[normalizedTitle] = true
					}
					seenMu.Unlock()

					select {
					case resultsChan <- StreamResult{Result: r, Engine: e.Name()}:
					case <-ctx.Done():
						return
					}
				}

				// Signal engine completion
				select {
				case resultsChan <- StreamResult{Engine: e.Name(), Done: true}:
				case <-ctx.Done():
				}
			}(engine)
		}

		wg.Wait()
	}()

	return resultsChan
}

// getFeatures returns the features supported by an engine
func getFeatures(engine SearchEngine) []string {
	var features []string
	if engine.SupportsFeature(FeaturePagination) {
		features = append(features, "pagination")
	}
	if engine.SupportsFeature(FeatureSorting) {
		features = append(features, "sorting")
	}
	if engine.SupportsFeature(FeatureFiltering) {
		features = append(features, "filtering")
	}
	if engine.SupportsFeature(FeatureThumbnailPreview) {
		features = append(features, "thumbnail_preview")
	}
	return features
}

// isAIGeneratedContent checks if a video result appears to be AI-generated content
// by matching against keywords in title and tags
func isAIGeneratedContent(titleLower string, tags []string, keywords []string) bool {
	// Check title for AI keywords
	for _, keyword := range keywords {
		if strings.Contains(titleLower, strings.ToLower(keyword)) {
			return true
		}
	}

	// Check tags for AI keywords
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		for _, keyword := range keywords {
			if strings.Contains(tagLower, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}
