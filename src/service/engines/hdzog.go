// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// HDZogEngine searches HDZog
type HDZogEngine struct{ *BaseEngine }

// NewHDZogEngine creates a new HDZog engine
func NewHDZogEngine(cfg *config.Config, torClient *tor.Client) *HDZogEngine {
	return &HDZogEngine{NewBaseEngine("hdzog", "HDZog", "https://www.hdzog.com", 3, cfg, torClient)}
}

// Search performs a search on HDZog
func (e *HDZogEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video, div.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *HDZogEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
