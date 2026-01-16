// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// NuvidEngine searches Nuvid
type NuvidEngine struct{ *BaseEngine }

// NewNuvidEngine creates a new Nuvid engine
func NewNuvidEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *NuvidEngine {
	return &NuvidEngine{NewBaseEngine("nuvid", "Nuvid", "https://www.nuvid.com", 3, appConfig, torClient)}
}

// Search performs a search on Nuvid
func (e *NuvidEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "a.th.video-thumb")
}

// SupportsFeature returns whether the engine supports a feature
func (e *NuvidEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
