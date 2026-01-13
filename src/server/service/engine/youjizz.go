// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"strconv"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// YouJizzEngine searches YouJizz
type YouJizzEngine struct{ *BaseEngine }

// NewYouJizzEngine creates a new YouJizz engine
func NewYouJizzEngine(cfg *config.Config, torClient *tor.Client) *YouJizzEngine {
	return &YouJizzEngine{NewBaseEngine("youjizz", "YouJizz", "https://www.youjizz.com", 3, cfg, torClient)}
}

// Search performs a search on YouJizz
func (e *YouJizzEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	// YouJizz uses dashes in search queries
	q := strings.ReplaceAll(query, " ", "-")
	searchURL := e.baseURL + "/search/" + q + "-" + strconv.Itoa(page) + ".html"
	return genericSearch(ctx, e.BaseEngine, searchURL, "div.video-item, li.video-item")
}

// SupportsFeature returns whether the engine supports a feature
func (e *YouJizzEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
