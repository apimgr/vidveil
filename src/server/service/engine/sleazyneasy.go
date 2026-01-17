// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// SleazyNeasyEngine searches SleazyNeasy
type SleazyNeasyEngine struct{ *BaseEngine }

// NewSleazyNeasyEngine creates a new SleazyNeasy engine
func NewSleazyNeasyEngine(appConfig *config.AppConfig) *SleazyNeasyEngine {
	return &SleazyNeasyEngine{NewBaseEngine("sleazyneasy", "SleazyNeasy", "https://www.sleazyneasy.com", 3, appConfig)}
}

// Search performs a search on SleazyNeasy
func (e *SleazyNeasyEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, div.thumb-item, article.video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *SleazyNeasyEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
