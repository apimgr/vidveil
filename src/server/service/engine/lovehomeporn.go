// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// LoveHomePornEngine searches LoveHomePorn
type LoveHomePornEngine struct{ *BaseEngine }

// newLoveHomePornEngine creates a new LoveHomePorn engine
func newLoveHomePornEngine(appConfig *config.AppConfig) *LoveHomePornEngine {
	return &LoveHomePornEngine{NewBaseEngine("lovehomeporn", "LoveHomePorn", "https://lovehomeporn.com", 3, appConfig)}
}

// Search performs a search on LoveHomePorn
func (e *LoveHomePornEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&search_type=videos&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "a.item-thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *LoveHomePornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
