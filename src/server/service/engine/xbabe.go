// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// XBabeEngine searches XBabe
type XBabeEngine struct{ *BaseEngine }

// NewXBabeEngine creates a new XBabe engine
func NewXBabeEngine(cfg *config.Config, torClient *tor.Client) *XBabeEngine {
	return &XBabeEngine{NewBaseEngine("xbabe", "XBabe", "https://xbabe.com", 4, cfg, torClient)}
}

// Search performs a search on XBabe
func (e *XBabeEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/?s={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *XBabeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
