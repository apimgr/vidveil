// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// NonkTubeEngine searches NonkTube
type NonkTubeEngine struct{ *BaseEngine }

// NewNonkTubeEngine creates a new NonkTube engine
func NewNonkTubeEngine(cfg *config.Config, torClient *tor.Client) *NonkTubeEngine {
	return &NonkTubeEngine{NewBaseEngine("nonktube", "NonkTube", "https://www.nonktube.com", 4, cfg, torClient)}
}

// Search performs a search on NonkTube
func (e *NonkTubeEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *NonkTubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
