// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// PornboxEngine searches Pornbox
type PornboxEngine struct{ *BaseEngine }

// NewPornboxEngine creates a new Pornbox engine
func NewPornboxEngine(cfg *config.Config, torClient *tor.Client) *PornboxEngine {
	return &PornboxEngine{NewBaseEngine("pornbox", "Pornbox", "https://pornbox.com", 4, cfg, torClient)}
}

// Search performs a search on Pornbox
func (e *PornboxEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.item, article.video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornboxEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
