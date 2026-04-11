// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// PornHDEngine searches PornHD
type PornHDEngine struct{ *BaseEngine }

// NewPornHDEngine creates a new PornHD engine
func NewPornHDEngine(appConfig *config.AppConfig) *PornHDEngine {
	e := &PornHDEngine{NewBaseEngine("pornhd", "PornHD", "https://www.pornhd.com", 4, appConfig)}
	// PornHD runs on the same ttcache.com CDN platform as TubeGalore
	e.SetCapabilities(Capabilities{
		HasPreview:    true,
		HasDuration:   true,
		HasRating:     true,
		PreviewSource: "ttcache-constructed",
		APIType:       "html",
	})
	return e
}

// Search performs a search on PornHD
func (e *PornHDEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search?search={query}&page={page}", query, page)
	return searchTTCache(ctx, e.BaseEngine, searchURL)
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornHDEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
