// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// ThreeMovsEngine searches 3Movs
type ThreeMovsEngine struct{ *BaseEngine }

// NewThreeMovsEngine creates a new 3Movs engine
func NewThreeMovsEngine(cfg *config.Config, torClient *tor.Client) *ThreeMovsEngine {
	return &ThreeMovsEngine{NewBaseEngine("3movs", "3Movs", "https://www.3movs.com", 3, cfg, torClient)}
}

// Search performs a search on 3Movs
func (e *ThreeMovsEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb-item, li.thumb-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *ThreeMovsEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
