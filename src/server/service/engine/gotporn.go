// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// GotPornEngine searches GotPorn
type GotPornEngine struct{ *BaseEngine }

// NewGotPornEngine creates a new GotPorn engine
func NewGotPornEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *GotPornEngine {
	return &GotPornEngine{NewBaseEngine("gotporn", "GotPorn", "https://www.gotporn.com", 3, appConfig, torClient)}
}

// Search performs a search on GotPorn
func (e *GotPornEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card.sub")
}

// SupportsFeature returns whether the engine supports a feature
func (e *GotPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
