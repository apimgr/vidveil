// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// FlyflvEngine searches Flyflv
type FlyflvEngine struct{ *BaseEngine }

// NewFlyflvEngine creates a new Flyflv engine
func NewFlyflvEngine(appConfig *config.AppConfig) *FlyflvEngine {
	return &FlyflvEngine{NewBaseEngine("flyflv", "Flyflv", "https://www.flyflv.com", 4, appConfig)}
}

// Search performs a search on Flyflv
func (e *FlyflvEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *FlyflvEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
