// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// VJAVEngine searches VJAV
type VJAVEngine struct{ *BaseEngine }

// NewVJAVEngine creates a new VJAV engine
func NewVJAVEngine(cfg *config.Config, torClient *tor.Client) *VJAVEngine {
	return &VJAVEngine{NewBaseEngine("vjav", "VJAV", "https://vjav.com", 4, cfg, torClient)}
}

// Search performs a search on VJAV
func (e *VJAVEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, article.video, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *VJAVEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
