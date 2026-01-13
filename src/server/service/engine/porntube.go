// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornTubeEngine searches PornTube
type PornTubeEngine struct{ *BaseEngine }

// NewPornTubeEngine creates a new PornTube engine
func NewPornTubeEngine(cfg *config.Config, torClient *tor.Client) *PornTubeEngine {
	return &PornTubeEngine{NewBaseEngine("porntube", "PornTube", "https://www.porntube.com", 3, cfg, torClient)}
}

// Search performs a search on PornTube
func (e *PornTubeEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video_container")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornTubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
