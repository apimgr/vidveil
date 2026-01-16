// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// VJAVEngine searches VJAV
type VJAVEngine struct{ *BaseEngine }

// NewVJAVEngine creates a new VJAV engine
func NewVJAVEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *VJAVEngine {
	return &VJAVEngine{NewBaseEngine("vjav", "VJAV", "https://vjav.com", 4, appConfig, torClient)}
}

// Search performs a search on VJAV
func (e *VJAVEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, article.video, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *VJAVEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
