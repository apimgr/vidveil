// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// NubilesPornEngine searches NubilesPorn
type NubilesPornEngine struct{ *BaseEngine }

// NewNubilesPornEngine creates a new NubilesPorn engine
func NewNubilesPornEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *NubilesPornEngine {
	return &NubilesPornEngine{NewBaseEngine("nubilesporn", "NubilesPorn", "https://nubiles-porn.com", 4, appConfig, torClient)}
}

// Search performs a search on NubilesPorn
func (e *NubilesPornEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}/?page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.scene, article.video, div.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *NubilesPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
