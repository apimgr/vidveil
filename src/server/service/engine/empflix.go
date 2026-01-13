// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// EMPFlixEngine searches EMPFlix
type EMPFlixEngine struct{ *BaseEngine }

// NewEMPFlixEngine creates a new EMPFlix engine
func NewEMPFlixEngine(cfg *config.Config, torClient *tor.Client) *EMPFlixEngine {
	return &EMPFlixEngine{NewBaseEngine("empflix", "EMPFlix", "https://www.empflix.com", 3, cfg, torClient)}
}

// Search performs a search on EMPFlix
func (e *EMPFlixEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search.php?what={query}&page={page}", query, page)
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.item-video, div.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *EMPFlixEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
