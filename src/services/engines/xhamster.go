// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/parsers"
	"github.com/apimgr/vidveil/src/services/tor"
)

// XHamsterEngine implements the xHamster search engine
type XHamsterEngine struct {
	*BaseEngine
	parser *parsers.XHamsterParser
}

// NewXHamsterEngine creates a new xHamster engine
func NewXHamsterEngine(cfg *config.Config, torClient *tor.Client) *XHamsterEngine {
	return &XHamsterEngine{
		BaseEngine: NewBaseEngine("xhamster", "xHamster", "https://xhamster.com", 1, cfg, torClient),
		parser:     parsers.NewXHamsterParser(),
	}
}

// Search performs a search on xHamster
func (e *XHamsterEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/{query}?page={page}", query, page)

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

	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			results = append(results, e.convertToResult(item))
		}
	})

	return results, nil
}

// convertToResult converts VideoItem to models.Result
func (e *XHamsterEngine) convertToResult(item *parsers.VideoItem) models.Result {
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

// SupportsFeature checks if xHamster supports a specific feature
func (e *XHamsterEngine) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeaturePagination:
		return true
	case FeatureSorting:
		return true
	case FeatureFiltering:
		return true
	default:
		return false
	}
}
