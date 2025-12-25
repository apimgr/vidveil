// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// SpankWireEngine searches SpankWire
type SpankWireEngine struct{ *BaseEngine }

// NewSpankWireEngine creates a new SpankWire engine
func NewSpankWireEngine(cfg *config.Config, torClient *tor.Client) *SpankWireEngine {
	return &SpankWireEngine{NewBaseEngine("spankwire", "SpankWire", "https://www.spankwire.com", 3, cfg, torClient)}
}

// Search performs a search on SpankWire
func (e *SpankWireEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/videos/{query}?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "li.video-item, div.video-item, div.videoblock")
}

// SupportsFeature returns whether the engine supports a feature
func (e *SpankWireEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
