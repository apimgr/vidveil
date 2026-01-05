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
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// Manager manages all search engines
type Manager struct {
	engines   map[string]Engine
	cfg       *config.Config
	torClient *tor.Client
	mu        sync.RWMutex
}

// NewManager creates a new engine manager
func NewManager(cfg *config.Config) *Manager {
	var torClient *tor.Client
	if cfg.Search.Tor.Enabled {
		torClient = tor.NewClient(cfg.Search.Tor.Proxy, cfg.Search.Tor.Timeout)
	}

	return &Manager{
		engines:   make(map[string]Engine),
		cfg:       cfg,
		torClient: torClient,
	}
}

// InitializeEngines sets up all available engines
func (m *Manager) InitializeEngines() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Tier 1 - Major Sites (always enabled by default)
	m.engines["pornhub"] = NewPornHubEngine(m.cfg, m.torClient)
	m.engines["xvideos"] = NewXVideosEngine(m.cfg, m.torClient)
	m.engines["xnxx"] = NewXNXXEngine(m.cfg, m.torClient)
	m.engines["redtube"] = NewRedTubeEngine(m.cfg, m.torClient)
	m.engines["xhamster"] = NewXHamsterEngine(m.cfg, m.torClient)

	// Tier 2 - Popular Sites (enabled by default)
	m.engines["eporner"] = NewEpornerEngine(m.cfg, m.torClient)
	m.engines["youporn"] = NewYouPornEngine(m.cfg, m.torClient)
	m.engines["pornmd"] = NewPornMDEngine(m.cfg, m.torClient)

	// Tier 3 - Additional Sites (disabled by default, enable via config)
	m.engines["4tube"] = NewFourTubeEngine(m.cfg, m.torClient)
	m.engines["fux"] = NewFuxEngine(m.cfg, m.torClient)
	m.engines["porntube"] = NewPornTubeEngine(m.cfg, m.torClient)
	m.engines["youjizz"] = NewYouJizzEngine(m.cfg, m.torClient)
	m.engines["sunporno"] = NewSunPornoEngine(m.cfg, m.torClient)
	m.engines["txxx"] = NewTxxxEngine(m.cfg, m.torClient)
	m.engines["nuvid"] = NewNuvidEngine(m.cfg, m.torClient)
	m.engines["tnaflix"] = NewTNAFlixEngine(m.cfg, m.torClient)
	m.engines["drtuber"] = NewDrTuberEngine(m.cfg, m.torClient)
	m.engines["empflix"] = NewEMPFlixEngine(m.cfg, m.torClient)
	m.engines["hellporno"] = NewHellPornoEngine(m.cfg, m.torClient)
	m.engines["alphaporno"] = NewAlphaPornoEngine(m.cfg, m.torClient)
	m.engines["pornflip"] = NewPornFlipEngine(m.cfg, m.torClient)
	m.engines["zenporn"] = NewZenPornEngine(m.cfg, m.torClient)
	m.engines["gotporn"] = NewGotPornEngine(m.cfg, m.torClient)
	m.engines["xxxymovies"] = NewXXXYMoviesEngine(m.cfg, m.torClient)
	m.engines["lovehomeporn"] = NewLoveHomePornEngine(m.cfg, m.torClient)

	// Tier 4 - Additional yt-dlp supported sites
	m.engines["pornerbros"] = NewPornerBrosEngine(m.cfg, m.torClient)
	m.engines["nonktube"] = NewNonkTubeEngine(m.cfg, m.torClient)
	m.engines["nubilesporn"] = NewNubilesPornEngine(m.cfg, m.torClient)
	m.engines["pornbox"] = NewPornboxEngine(m.cfg, m.torClient)
	m.engines["porntop"] = NewPornTopEngine(m.cfg, m.torClient)
	m.engines["pornotube"] = NewPornotubeEngine(m.cfg, m.torClient)
	m.engines["vporn"] = NewVPornEngine(m.cfg, m.torClient)
	m.engines["pornhd"] = NewPornHDEngine(m.cfg, m.torClient)
	m.engines["xbabe"] = NewXBabeEngine(m.cfg, m.torClient)
	m.engines["pornone"] = NewPornOneEngine(m.cfg, m.torClient)
	m.engines["pornhat"] = NewPornHatEngine(m.cfg, m.torClient)
	m.engines["porntrex"] = NewPornTrexEngine(m.cfg, m.torClient)
	m.engines["hqporner"] = NewHqpornerEngine(m.cfg, m.torClient)
	m.engines["vjav"] = NewVJAVEngine(m.cfg, m.torClient)
	m.engines["flyflv"] = NewFlyflvEngine(m.cfg, m.torClient)
	m.engines["tube8"] = NewTube8Engine(m.cfg, m.torClient)
	m.engines["xtube"] = NewXtubeEngine(m.cfg, m.torClient)

	// Tier 5 - New engines
	m.engines["anyporn"] = NewAnyPornEngine(m.cfg, m.torClient)
	m.engines["superporn"] = NewSuperPornEngine(m.cfg, m.torClient)
	m.engines["tubegalore"] = NewTubeGaloreEngine(m.cfg, m.torClient)
	m.engines["motherless"] = NewMotherlessEngine(m.cfg, m.torClient)

	// Tier 6 - Additional engines
	m.engines["keezmovies"] = NewKeezMoviesEngine(m.cfg, m.torClient)
	m.engines["spankwire"] = NewSpankWireEngine(m.cfg, m.torClient)
	m.engines["extremetube"] = NewExtremeTubeEngine(m.cfg, m.torClient)
	m.engines["3movs"] = NewThreeMovsEngine(m.cfg, m.torClient)
	m.engines["sleazyneasy"] = NewSleazyNeasyEngine(m.cfg, m.torClient)

	// Apply configuration
	m.applyConfig()
}

// applyConfig applies engine-specific configuration
func (m *Manager) applyConfig() {
	// All engines are enabled by default
	// DefaultEngines config can limit which engines to use
	defaultEngines := m.cfg.Search.DefaultEngines

	// If default_engines is specified, only enable those
	if len(defaultEngines) > 0 {
		enabledSet := make(map[string]bool)
		for _, name := range defaultEngines {
			enabledSet[name] = true
		}

		for name, engine := range m.engines {
			if configurable, ok := engine.(ConfigurableEngine); ok {
				configurable.SetEnabled(enabledSet[name])
			}
		}
	}

	// Apply Tor settings if force_all is enabled
	if m.cfg.Search.Tor.ForceAll {
		for _, engine := range m.engines {
			if configurable, ok := engine.(ConfigurableEngine); ok {
				configurable.SetUseTor(true)
			}
		}
	}
}

// Search performs a search across enabled engines
func (m *Manager) Search(ctx context.Context, query string, page int, engineNames []string) *model.SearchResponse {
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
		go func(e Engine) {
			defer wg.Done()
			results, err := e.Search(ctx, query, page)
			resultsChan <- engineResult{
				engine:  e.Name(),
				results: results,
				err:     err,
			}
		}(engine)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var allResults []model.Result
	var enginesUsed []string
	var enginesFailed []string

	minDuration := m.cfg.Search.MinDurationSeconds

	for result := range resultsChan {
		if result.err != nil {
			enginesFailed = append(enginesFailed, result.engine)
		} else {
			enginesUsed = append(enginesUsed, result.engine)
			// Filter results by minimum duration
			for _, r := range result.results {
				// Skip if duration is known and below minimum
				if minDuration > 0 && r.DurationSeconds > 0 && r.DurationSeconds < minDuration {
					continue
				}
				allResults = append(allResults, r)
			}
		}
	}

	// Sort results by relevance to query
	sortByRelevance(allResults, query)

	// Build response
	elapsed := time.Since(startTime)

	return &model.SearchResponse{
		Success: true,
		Data: model.SearchData{
			Query:         query,
			Results:       allResults,
			EnginesUsed:   enginesUsed,
			EnginesFailed: enginesFailed,
			SearchTimeMS:  elapsed.Milliseconds(),
		},
		Pagination: model.PaginationData{
			Page:  page,
			Limit: m.cfg.Search.ResultsPerPage,
			Total: len(allResults),
			Pages: (len(allResults) + m.cfg.Search.ResultsPerPage - 1) / m.cfg.Search.ResultsPerPage,
		},
	}
}

// sortByRelevance sorts results by relevance score
func sortByRelevance(results []model.Result, query string) {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)
	if len(queryWords) == 0 {
		return
	}

	// Pre-calculate scores for all results
	scores := make([]float64, len(results))
	for i, r := range results {
		scores[i] = calculateRelevanceScore(r, queryLower, queryWords)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return scores[i] > scores[j]
	})
}

// calculateRelevanceScore computes a relevance score for a result
func calculateRelevanceScore(r model.Result, queryLower string, queryWords []string) float64 {
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

// getEnginesToUse returns the engines to use for search
func (m *Manager) getEnginesToUse(engineNames []string) []Engine {
	var engines []Engine

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
func (m *Manager) GetEngine(name string) (Engine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engine, ok := m.engines[name]
	return engine, ok
}

// ListEngines returns information about all engines
func (m *Manager) ListEngines() []model.EngineInfo {
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
func (m *Manager) EnabledCount() int {
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
	engine  string
	results []model.Result
	err     error
}

// StreamResult represents a single result sent via SSE
type StreamResult struct {
	Result model.Result `json:"result,omitempty"`
	Engine string        `json:"engine"`
	Done   bool          `json:"done"`
	Error  string        `json:"error,omitempty"`
}

// SearchStream performs a search across enabled engines and streams results via channel
func (m *Manager) SearchStream(ctx context.Context, query string, page int, engineNames []string) <-chan StreamResult {
	resultsChan := make(chan StreamResult, 100)

	go func() {
		defer close(resultsChan)

		m.mu.RLock()
		enginesToUse := m.getEnginesToUse(engineNames)
		m.mu.RUnlock()

		var wg sync.WaitGroup
		minDuration := m.cfg.Search.MinDurationSeconds

		for _, engine := range enginesToUse {
			wg.Add(1)
			go func(e Engine) {
				defer wg.Done()

				results, err := e.Search(ctx, query, page)
				if err != nil {
					select {
					case resultsChan <- StreamResult{Engine: e.Name(), Error: err.Error()}:
					case <-ctx.Done():
					}
					return
				}

				// Stream each result individually
				for _, r := range results {
					// Skip if duration is known and below minimum
					if minDuration > 0 && r.DurationSeconds > 0 && r.DurationSeconds < minDuration {
						continue
					}

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
func getFeatures(engine Engine) []string {
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
