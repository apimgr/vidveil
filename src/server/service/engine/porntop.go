// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// PornTopEngine searches PornTop
type PornTopEngine struct{ *BaseEngine }

// NewPornTopEngine creates a new PornTop engine
func NewPornTopEngine(appConfig *config.AppConfig) *PornTopEngine {
	return &PornTopEngine{NewBaseEngine("porntop", "PornTop", "https://porntop.com", 4, appConfig)}
}

// Search performs a search on PornTop
func (e *PornTopEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/?s={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornTopEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
