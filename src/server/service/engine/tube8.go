// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// Tube8Engine searches Tube8
type Tube8Engine struct{ *BaseEngine }

// NewTube8Engine creates a new Tube8 engine
func NewTube8Engine(cfg *config.Config, torClient *tor.Client) *Tube8Engine {
	return &Tube8Engine{NewBaseEngine("tube8", "Tube8", "https://www.tube8.com", 4, cfg, torClient)}
}

// Search performs a search on Tube8
func (e *Tube8Engine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/searches?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-box, div.thumbnail-card")
}

// SupportsFeature returns whether the engine supports a feature
func (e *Tube8Engine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
