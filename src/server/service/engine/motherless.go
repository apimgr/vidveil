// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// MotherlessEngine searches Motherless
type MotherlessEngine struct{ *BaseEngine }

// NewMotherlessEngine creates a new Motherless engine
func NewMotherlessEngine(appConfig *config.AppConfig) *MotherlessEngine {
	return &MotherlessEngine{NewBaseEngine("motherless", "Motherless", "https://motherless.com", 3, appConfig)}
}

// Search performs a search on Motherless
func (e *MotherlessEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/term/videos/{query}?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.thumb-container.video")
}

// SupportsFeature returns whether the engine supports a feature
func (e *MotherlessEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
