// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// HqpornerEngine searches Hqporner
type HqpornerEngine struct{ *BaseEngine }

// NewHqpornerEngine creates a new Hqporner engine
func NewHqpornerEngine(cfg *config.Config, torClient *tor.Client) *HqpornerEngine {
	return &HqpornerEngine{NewBaseEngine("hqporner", "Hqporner", "https://hqporner.com", 4, cfg, torClient)}
}

// Search performs a search on Hqporner
func (e *HqpornerEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.box, div.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *HqpornerEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
