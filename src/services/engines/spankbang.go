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

// SpankBangEngine implements the SpankBang search engine
type SpankBangEngine struct {
	*BaseEngine
	parser *parsers.SpankBangParser
}

// NewSpankBangEngine creates a new SpankBang engine
func NewSpankBangEngine(cfg *config.Config, torClient *tor.Client) *SpankBangEngine {
	return &SpankBangEngine{
		BaseEngine: NewBaseEngine("spankbang", "SpankBang", "https://spankbang.com", 1, cfg, torClient),
		parser:     parsers.NewSpankBangParser(),
	}
}

// Search performs a search on SpankBang
func (e *SpankBangEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/s/{query}/{page}/", query, page)

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
func (e *SpankBangEngine) convertToResult(item *parsers.VideoItem) models.Result {
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
func (e *SpankBangEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
