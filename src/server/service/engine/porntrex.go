// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// PornTrexEngine searches PornTrex
type PornTrexEngine struct{ *BaseEngine }

// newPornTrexEngine creates a new PornTrex engine
func newPornTrexEngine(appConfig *config.AppConfig) *PornTrexEngine {
	return &PornTrexEngine{NewBaseEngine("porntrex", "PornTrex", "https://www.porntrex.com", 4, appConfig)}
}

// Search performs a search on PornTrex
func (e *PornTrexEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornTrexEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
