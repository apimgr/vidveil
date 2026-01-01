// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// TNAFlixEngine searches TNAFlix
type TNAFlixEngine struct{ *BaseEngine }

// NewTNAFlixEngine creates a new TNAFlix engine
func NewTNAFlixEngine(cfg *config.Config, torClient *tor.Client) *TNAFlixEngine {
	return &TNAFlixEngine{NewBaseEngine("tnaflix", "TNAFlix", "https://www.tnaflix.com", 3, cfg, torClient)}
}

// Search performs a search on TNAFlix
func (e *TNAFlixEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search.php?what={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.col-xs-6.col-md-4")
}

// SupportsFeature returns whether the engine supports a feature
func (e *TNAFlixEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
