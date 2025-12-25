// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// PornerBrosEngine searches PornerBros
type PornerBrosEngine struct{ *BaseEngine }

// NewPornerBrosEngine creates a new PornerBros engine
func NewPornerBrosEngine(cfg *config.Config, torClient *tor.Client) *PornerBrosEngine {
	return &PornerBrosEngine{NewBaseEngine("pornerbros", "PornerBros", "https://www.pornerbros.com", 4, cfg, torClient)}
}

// Search performs a search on PornerBros
func (e *PornerBrosEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card.sub")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornerBrosEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
