// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornOneEngine searches PornOne
type PornOneEngine struct{ *BaseEngine }

// NewPornOneEngine creates a new PornOne engine
func NewPornOneEngine(cfg *config.Config, torClient *tor.Client) *PornOneEngine {
	return &PornOneEngine{NewBaseEngine("pornone", "PornOne", "https://pornone.com", 4, cfg, torClient)}
}

// Search performs a search on PornOne
func (e *PornOneEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb, article.video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornOneEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
