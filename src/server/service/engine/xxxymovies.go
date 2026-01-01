// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// XXXYMoviesEngine searches XXXYMovies
type XXXYMoviesEngine struct{ *BaseEngine }

// NewXXXYMoviesEngine creates a new XXXYMovies engine
func NewXXXYMoviesEngine(cfg *config.Config, torClient *tor.Client) *XXXYMoviesEngine {
	return &XXXYMoviesEngine{NewBaseEngine("xxxymovies", "XXXYMovies", "https://www.xxxymovies.com", 3, cfg, torClient)}
}

// Search performs a search on XXXYMovies
func (e *XXXYMoviesEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *XXXYMoviesEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
