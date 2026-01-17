// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// NonkTubeEngine searches NonkTube
type NonkTubeEngine struct{ *BaseEngine }

// NewNonkTubeEngine creates a new NonkTube engine
func NewNonkTubeEngine(appConfig *config.AppConfig) *NonkTubeEngine {
	return &NonkTubeEngine{NewBaseEngine("nonktube", "NonkTube", "https://www.nonktube.com", 4, appConfig)}
}

// Search performs a search on NonkTube
func (e *NonkTubeEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *NonkTubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
