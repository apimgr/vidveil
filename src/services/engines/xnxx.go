// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/parsers"
	"github.com/apimgr/vidveil/src/services/tor"
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
func (e *XNXXEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
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

	var results []models.Result

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

// convertToResult converts VideoItem to models.Result
func (e *XNXXEngine) convertToResult(item *parsers.VideoItem) models.Result {
	return models.Result{
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
