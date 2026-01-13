// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// DrTuberEngine searches DrTuber
type DrTuberEngine struct{ *BaseEngine }

// NewDrTuberEngine creates a new DrTuber engine
func NewDrTuberEngine(cfg *config.Config, torClient *tor.Client) *DrTuberEngine {
	return &DrTuberEngine{NewBaseEngine("drtuber", "DrTuber", "https://www.drtuber.com", 3, cfg, torClient)}
}

// Search performs a search on DrTuber
func (e *DrTuberEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/videos?search_type=videos&search_id={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "a.th.ch-video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *DrTuberEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
