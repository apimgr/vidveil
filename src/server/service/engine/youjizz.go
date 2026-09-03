// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// YouJizzEngine searches YouJizz
type YouJizzEngine struct{ *BaseEngine }

// newYouJizzEngine creates a new YouJizz engine
func newYouJizzEngine(appConfig *config.AppConfig) *YouJizzEngine {
	return &YouJizzEngine{NewBaseEngine("youjizz", "YouJizz", "https://www.youjizz.com", 3, appConfig)}
}

// Search performs a search on YouJizz
func (e *YouJizzEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?q={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, li.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *YouJizzEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
