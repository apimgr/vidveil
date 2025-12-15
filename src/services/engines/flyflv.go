// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// FlyflvEngine searches Flyflv
type FlyflvEngine struct{ *BaseEngine }

// NewFlyflvEngine creates a new Flyflv engine
func NewFlyflvEngine(cfg *config.Config, torClient *tor.Client) *FlyflvEngine {
	return &FlyflvEngine{NewBaseEngine("flyflv", "Flyflv", "https://www.flyflv.com", 4, cfg, torClient)}
}

// Search performs a search on Flyflv
func (e *FlyflvEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *FlyflvEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
