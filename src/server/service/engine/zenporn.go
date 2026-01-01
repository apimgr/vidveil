// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// ZenPornEngine searches ZenPorn
type ZenPornEngine struct{ *BaseEngine }

// NewZenPornEngine creates a new ZenPorn engine
func NewZenPornEngine(cfg *config.Config, torClient *tor.Client) *ZenPornEngine {
	return &ZenPornEngine{NewBaseEngine("zenporn", "ZenPorn", "https://zenporn.com", 3, cfg, torClient)}
}

// Search performs a search on ZenPorn
func (e *ZenPornEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "article.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *ZenPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
