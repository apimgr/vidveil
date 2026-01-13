// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornotubeEngine searches Pornotube
type PornotubeEngine struct{ *BaseEngine }

// NewPornotubeEngine creates a new Pornotube engine
func NewPornotubeEngine(cfg *config.Config, torClient *tor.Client) *PornotubeEngine {
	return &PornotubeEngine{NewBaseEngine("pornotube", "Pornotube", "https://pornotube.com", 4, cfg, torClient)}
}

// Search performs a search on Pornotube
func (e *PornotubeEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb, article.video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornotubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
