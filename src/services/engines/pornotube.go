// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// PornotubeEngine searches Pornotube
type PornotubeEngine struct{ *BaseEngine }

// NewPornotubeEngine creates a new Pornotube engine
func NewPornotubeEngine(cfg *config.Config, torClient *tor.Client) *PornotubeEngine {
	return &PornotubeEngine{NewBaseEngine("pornotube", "Pornotube", "https://pornotube.com", 4, cfg, torClient)}
}

// Search performs a search on Pornotube
func (e *PornotubeEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb, article.video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornotubeEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
