// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// HqpornerEngine searches Hqporner
type HqpornerEngine struct{ *BaseEngine }

// NewHqpornerEngine creates a new Hqporner engine
func NewHqpornerEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *HqpornerEngine {
	return &HqpornerEngine{NewBaseEngine("hqporner", "Hqporner", "https://hqporner.com", 4, appConfig, torClient)}
}

// Search performs a search on Hqporner
func (e *HqpornerEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.box, div.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *HqpornerEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
