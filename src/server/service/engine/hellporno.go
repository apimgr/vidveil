// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// HellPornoEngine searches HellPorno
type HellPornoEngine struct{ *BaseEngine }

// NewHellPornoEngine creates a new HellPorno engine
func NewHellPornoEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *HellPornoEngine {
	return &HellPornoEngine{NewBaseEngine("hellporno", "HellPorno", "https://hellporno.com", 3, appConfig, torClient)}
}

// Search performs a search on HellPorno
func (e *HellPornoEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/?q={query}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *HellPornoEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
