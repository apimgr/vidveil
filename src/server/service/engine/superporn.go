// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// SuperPornEngine searches SuperPorn
type SuperPornEngine struct{ *BaseEngine }

// NewSuperPornEngine creates a new SuperPorn engine
func NewSuperPornEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *SuperPornEngine {
	return &SuperPornEngine{NewBaseEngine("superporn", "SuperPorn", "https://www.superporn.com", 3, appConfig, torClient)}
}

// Search performs a search on SuperPorn
func (e *SuperPornEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}?p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.thumb-video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *SuperPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
