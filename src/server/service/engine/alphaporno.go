// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// AlphaPornoEngine searches AlphaPorno
type AlphaPornoEngine struct{ *BaseEngine }

// NewAlphaPornoEngine creates a new AlphaPorno engine
func NewAlphaPornoEngine(cfg *config.Config, torClient *tor.Client) *AlphaPornoEngine {
	return &AlphaPornoEngine{NewBaseEngine("alphaporno", "AlphaPorno", "https://www.alphaporno.com", 3, cfg, torClient)}
}

// Search performs a search on AlphaPorno
func (e *AlphaPornoEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "li.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *AlphaPornoEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
