// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// ExtremeTubeEngine searches ExtremeTube
type ExtremeTubeEngine struct{ *BaseEngine }

// NewExtremeTubeEngine creates a new ExtremeTube engine
func NewExtremeTubeEngine(cfg *config.Config, torClient *tor.Client) *ExtremeTubeEngine {
	return &ExtremeTubeEngine{NewBaseEngine("extremetube", "ExtremeTube", "https://www.extremetube.com", 3, cfg, torClient)}
}

// Search performs a search on ExtremeTube
func (e *ExtremeTubeEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "li.video-item, div.video-item, div.thumb-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *ExtremeTubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
