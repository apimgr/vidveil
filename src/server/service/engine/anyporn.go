// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// AnyPornEngine searches AnyPorn
type AnyPornEngine struct{ *BaseEngine }

// NewAnyPornEngine creates a new AnyPorn engine
func NewAnyPornEngine(cfg *config.Config, torClient *tor.Client) *AnyPornEngine {
	return &AnyPornEngine{NewBaseEngine("anyporn", "AnyPorn", "https://www.anyporn.com", 3, cfg, torClient)}
}

// Search performs a search on AnyPorn
func (e *AnyPornEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *AnyPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
