// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// TxxxEngine searches Txxx
type TxxxEngine struct{ *BaseEngine }

// NewTxxxEngine creates a new Txxx engine
func NewTxxxEngine(cfg *config.Config, torClient *tor.Client) *TxxxEngine {
	return &TxxxEngine{NewBaseEngine("txxx", "Txxx", "https://www.txxx.com", 3, cfg, torClient)}
}

// Search performs a search on Txxx
func (e *TxxxEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.thumb-item, div.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *TxxxEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
