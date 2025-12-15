// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// PornTopEngine searches PornTop
type PornTopEngine struct{ *BaseEngine }

// NewPornTopEngine creates a new PornTop engine
func NewPornTopEngine(cfg *config.Config, torClient *tor.Client) *PornTopEngine {
	return &PornTopEngine{NewBaseEngine("porntop", "PornTop", "https://porntop.com", 4, cfg, torClient)}
}

// Search performs a search on PornTop
func (e *PornTopEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/?s={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornTopEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
