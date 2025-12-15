// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// HellPornoEngine searches HellPorno
type HellPornoEngine struct{ *BaseEngine }

// NewHellPornoEngine creates a new HellPorno engine
func NewHellPornoEngine(cfg *config.Config, torClient *tor.Client) *HellPornoEngine {
	return &HellPornoEngine{NewBaseEngine("hellporno", "HellPorno", "https://hellporno.com", 3, cfg, torClient)}
}

// Search performs a search on HellPorno
func (e *HellPornoEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/?q={query}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *HellPornoEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
