// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// SpankWireEngine searches SpankWire
type SpankWireEngine struct{ *BaseEngine }

// NewSpankWireEngine creates a new SpankWire engine
func NewSpankWireEngine(appConfig *config.AppConfig) *SpankWireEngine {
	return &SpankWireEngine{NewBaseEngine("spankwire", "SpankWire", "https://www.spankwire.com", 3, appConfig)}
}

// Search performs a search on SpankWire
func (e *SpankWireEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/videos/{query}?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "li.video-item, div.video-item, div.videoblock")
}

// SupportsFeature returns whether the engine supports a feature
func (e *SpankWireEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
