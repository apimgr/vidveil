// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// LoveHomePornEngine searches LoveHomePorn
type LoveHomePornEngine struct{ *BaseEngine }

// NewLoveHomePornEngine creates a new LoveHomePorn engine
func NewLoveHomePornEngine(cfg *config.Config, torClient *tor.Client) *LoveHomePornEngine {
	return &LoveHomePornEngine{NewBaseEngine("lovehomeporn", "LoveHomePorn", "https://lovehomeporn.com", 3, cfg, torClient)}
}

// Search performs a search on LoveHomePorn
func (e *LoveHomePornEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *LoveHomePornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
