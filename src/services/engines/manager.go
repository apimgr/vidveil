// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
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
	m.engines["xhamster"] = NewXHamsterEngine(m.cfg, m.torClient)
	m.engines["xvideos"] = NewXVideosEngine(m.cfg, m.torClient)
	m.engines["xnxx"] = NewXNXXEngine(m.cfg, m.torClient)
	m.engines["youporn"] = NewYouPornEngine(m.cfg, m.torClient)
	m.engines["redtube"] = NewRedTubeEngine(m.cfg, m.torClient)
	m.engines["spankbang"] = NewSpankBangEngine(m.cfg, m.torClient)

	// Tier 2 - Popular Sites (enabled by default)
	m.engines["eporner"] = NewEpornerEngine(m.cfg, m.torClient)
	m.engines["beeg"] = NewBeegEngine(m.cfg, m.torClient)
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
	m.engines["hdzog"] = NewHDZogEngine(m.cfg, m.torClient)
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
func (m *Manager) Search(ctx context.Context, query string, page int, engineNames []string) *models.SearchResponse {
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
	var allResults []models.Result
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

	return &models.SearchResponse{
		Success: true,
		Data: models.SearchData{
			Query:         query,
			Results:       allResults,
			EnginesUsed:   enginesUsed,
			EnginesFailed: enginesFailed,
			SearchTimeMS:  elapsed.Milliseconds(),
		},
		Pagination: models.PaginationData{
			Page:  page,
			Limit: m.cfg.Search.ResultsPerPage,
			Total: len(allResults),
			Pages: (len(allResults) + m.cfg.Search.ResultsPerPage - 1) / m.cfg.Search.ResultsPerPage,
		},
	}
}

// sortByRelevance sorts results by how many query words appear in the title
func sortByRelevance(results []models.Result, query string) {
	queryWords := strings.Fields(strings.ToLower(query))
	if len(queryWords) == 0 {
		return
	}

	sort.SliceStable(results, func(i, j int) bool {
		titleI := strings.ToLower(results[i].Title)
		titleJ := strings.ToLower(results[j].Title)

		// Count matching words for each result
		scoreI, scoreJ := 0, 0
		for _, word := range queryWords {
			if strings.Contains(titleI, word) {
				scoreI++
			}
			if strings.Contains(titleJ, word) {
				scoreJ++
			}
		}

		// Higher score = more relevant = should come first
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		// Tie-breaker: prefer higher view counts
		return results[i].ViewsCount > results[j].ViewsCount
	})
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
func (m *Manager) ListEngines() []models.EngineInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []models.EngineInfo
	for _, engine := range m.engines {
		infos = append(infos, models.EngineInfo{
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
	results []models.Result
	err     error
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
