// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// PornFlipEngine searches PornFlip
type PornFlipEngine struct{ *BaseEngine }

// NewPornFlipEngine creates a new PornFlip engine
func NewPornFlipEngine(appConfig *config.AppConfig) *PornFlipEngine {
	return &PornFlipEngine{NewBaseEngine("pornflip", "PornFlip", "https://www.pornflip.com", 3, appConfig)}
}

// Search performs a search on PornFlip
func (e *PornFlipEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?search={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornFlipEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
