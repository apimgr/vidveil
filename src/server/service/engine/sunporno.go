// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// SunPornoEngine searches SunPorno
type SunPornoEngine struct{ *BaseEngine }

// NewSunPornoEngine creates a new SunPorno engine
func NewSunPornoEngine(appConfig *config.AppConfig) *SunPornoEngine {
	return &SunPornoEngine{NewBaseEngine("sunporno", "SunPorno", "https://www.sunporno.com", 3, appConfig)}
}

// Search performs a search on SunPorno
func (e *SunPornoEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/videos?q={query}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "a.item.drclass")
}

// SupportsFeature returns whether the engine supports a feature
func (e *SunPornoEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
