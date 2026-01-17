// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// VPornEngine searches VPorn
type VPornEngine struct{ *BaseEngine }

// NewVPornEngine creates a new VPorn engine
func NewVPornEngine(appConfig *config.AppConfig) *VPornEngine {
	return &VPornEngine{NewBaseEngine("vporn", "VPorn", "https://www.vporn.com", 4, appConfig)}
}

// Search performs a search on VPorn
func (e *VPornEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *VPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
