// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornerBrosEngine searches PornerBros
type PornerBrosEngine struct{ *BaseEngine }

// NewPornerBrosEngine creates a new PornerBros engine
func NewPornerBrosEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *PornerBrosEngine {
	return &PornerBrosEngine{NewBaseEngine("pornerbros", "PornerBros", "https://www.pornerbros.com", 4, appConfig, torClient)}
}

// Search performs a search on PornerBros
func (e *PornerBrosEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card.sub")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornerBrosEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
