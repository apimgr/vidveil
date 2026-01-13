// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// KeezMoviesEngine searches KeezMovies
type KeezMoviesEngine struct{ *BaseEngine }

// NewKeezMoviesEngine creates a new KeezMovies engine
func NewKeezMoviesEngine(cfg *config.Config, torClient *tor.Client) *KeezMoviesEngine {
	return &KeezMoviesEngine{NewBaseEngine("keezmovies", "KeezMovies", "https://www.keezmovies.com", 3, cfg, torClient)}
}

// Search performs a search on KeezMovies
func (e *KeezMoviesEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "li.video-item, div.video-item, div.videoblock")
}

// SupportsFeature returns whether the engine supports a feature
func (e *KeezMoviesEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
