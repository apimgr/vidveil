// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornHDEngine searches PornHD
type PornHDEngine struct{ *BaseEngine }

// NewPornHDEngine creates a new PornHD engine
func NewPornHDEngine(cfg *config.Config, torClient *tor.Client) *PornHDEngine {
	return &PornHDEngine{NewBaseEngine("pornhd", "PornHD", "https://www.pornhd.com", 4, cfg, torClient)}
}

// Search performs a search on PornHD
func (e *PornHDEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search?search={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card.sub")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornHDEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
