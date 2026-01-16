// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// FuxEngine searches Fux
type FuxEngine struct{ *BaseEngine }

// NewFuxEngine creates a new Fux engine
func NewFuxEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *FuxEngine {
	return &FuxEngine{NewBaseEngine("fux", "Fux", "https://www.fux.com", 3, appConfig, torClient)}
}

// Search performs a search on Fux
func (e *FuxEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card")
}

// SupportsFeature returns whether the engine supports a feature
func (e *FuxEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
