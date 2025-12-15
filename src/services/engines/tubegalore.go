// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
)

// TubeGaloreEngine searches TubeGalore
type TubeGaloreEngine struct{ *BaseEngine }

// NewTubeGaloreEngine creates a new TubeGalore engine
func NewTubeGaloreEngine(cfg *config.Config, torClient *tor.Client) *TubeGaloreEngine {
	return &TubeGaloreEngine{NewBaseEngine("tubegalore", "TubeGalore", "https://www.tubegalore.com", 3, cfg, torClient)}
}

// Search performs a search on TubeGalore
func (e *TubeGaloreEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/?q={query}&p={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.card")
}

// SupportsFeature returns whether the engine supports a feature
func (e *TubeGaloreEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
