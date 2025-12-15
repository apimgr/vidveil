// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// FourTubeEngine searches 4Tube
type FourTubeEngine struct{ *BaseEngine }

// NewFourTubeEngine creates a new 4Tube engine
func NewFourTubeEngine(cfg *config.Config, torClient *tor.Client) *FourTubeEngine {
	return &FourTubeEngine{NewBaseEngine("4tube", "4Tube", "https://www.4tube.com", 3, cfg, torClient)}
}

// Search performs a search on 4Tube
func (e *FourTubeEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card")
}

// SupportsFeature returns whether the engine supports a feature
func (e *FourTubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
