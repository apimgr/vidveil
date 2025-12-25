// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// PornHatEngine searches PornHat
type PornHatEngine struct{ *BaseEngine }

// NewPornHatEngine creates a new PornHat engine
func NewPornHatEngine(cfg *config.Config, torClient *tor.Client) *PornHatEngine {
	return &PornHatEngine{NewBaseEngine("pornhat", "PornHat", "https://www.pornhat.com", 4, cfg, torClient)}
}

// Search performs a search on PornHat
func (e *PornHatEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornHatEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
