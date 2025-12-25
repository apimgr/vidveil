// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/parsers"
	"github.com/apimgr/vidveil/src/service/tor"
)

// XNXXEngine implements the XNXX search engine
type XNXXEngine struct {
	*BaseEngine
	parser *parsers.XNXXParser
}

// NewXNXXEngine creates a new XNXX engine
func NewXNXXEngine(cfg *config.Config, torClient *tor.Client) *XNXXEngine {
	return &XNXXEngine{
		BaseEngine: NewBaseEngine("xnxx", "XNXX", "https://www.xnxx.com", 1, cfg, torClient),
		parser:     parsers.NewXNXXParser(),
	}
}

// Search performs a search on XNXX
func (e *XNXXEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}/{page}", query, page)

	resp, err := e.MakeRequest(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []model.Result

	// Use parser's item selector
	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			// Skip gold/premium content
			if strings.Contains(item.URL, "xnxx.gold") || strings.Contains(item.URL, "/gold/") {
				return
			}
			result := e.convertToResult(item)
			results = append(results, result)
		}
	})

	return results, nil
}

// convertToResult converts VideoItem to model.Result
func (e *XNXXEngine) convertToResult(item *parsers.VideoItem) model.Result {
	return model.Result{
		ID:              GenerateResultID(item.URL, e.Name()),
		URL:             item.URL,
		Title:           item.Title,
		Thumbnail:       item.Thumbnail,
		PreviewURL:      item.PreviewURL,
		Duration:        item.Duration,
		DurationSeconds: item.DurationSeconds,
		Views:           item.Views,
		ViewsCount:      item.ViewsCount,
		Description:     item.Quality,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
	}
}

// SupportsFeature checks supported features
func (e *XNXXEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
