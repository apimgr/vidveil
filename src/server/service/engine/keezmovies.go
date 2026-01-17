// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// KeezMoviesEngine searches KeezMovies
type KeezMoviesEngine struct{ *BaseEngine }

// NewKeezMoviesEngine creates a new KeezMovies engine
func NewKeezMoviesEngine(appConfig *config.AppConfig) *KeezMoviesEngine {
	return &KeezMoviesEngine{NewBaseEngine("keezmovies", "KeezMovies", "https://www.keezmovies.com", 3, appConfig)}
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
