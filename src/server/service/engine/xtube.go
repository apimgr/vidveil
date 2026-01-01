// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// XtubeEngine searches Xtube
type XtubeEngine struct{ *BaseEngine }

// NewXtubeEngine creates a new Xtube engine
func NewXtubeEngine(cfg *config.Config, torClient *tor.Client) *XtubeEngine {
	return &XtubeEngine{NewBaseEngine("xtube", "Xtube", "https://www.xtube.com", 4, cfg, torClient)}
}

// Search performs a search on Xtube
func (e *XtubeEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *XtubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
