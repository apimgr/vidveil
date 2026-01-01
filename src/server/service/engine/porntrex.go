// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornTrexEngine searches PornTrex
type PornTrexEngine struct{ *BaseEngine }

// NewPornTrexEngine creates a new PornTrex engine
func NewPornTrexEngine(cfg *config.Config, torClient *tor.Client) *PornTrexEngine {
	return &PornTrexEngine{NewBaseEngine("porntrex", "PornTrex", "https://www.porntrex.com", 4, cfg, torClient)}
}

// Search performs a search on PornTrex
func (e *PornTrexEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornTrexEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
