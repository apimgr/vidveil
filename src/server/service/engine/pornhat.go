// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// PornHatEngine searches PornHat
type PornHatEngine struct{ *BaseEngine }

// NewPornHatEngine creates a new PornHat engine
func NewPornHatEngine(appConfig *config.AppConfig) *PornHatEngine {
	e := &PornHatEngine{NewBaseEngine("pornhat", "PornHat", "https://www.pornhat.com", 4, appConfig)}
	// PornHat exposes preview MP4s via data-preview-custom on the <a> link element
	e.SetCapabilities(Capabilities{
		HasPreview:    true,
		HasDuration:   true,
		HasViews:      true,
		PreviewSource: "data-preview-custom",
		APIType:       "html",
	})
	return e
}

// Search performs a search on PornHat
func (e *PornHatEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	// div.item.thumb-bl is the video card container; data-preview-custom is on its inner <a>
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.item.thumb-bl")
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornHatEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
