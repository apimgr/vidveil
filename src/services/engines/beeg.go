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

// BeegEngine searches Beeg
type BeegEngine struct {
	*BaseEngine
	parser *parsers.BeegParser
}

// NewBeegEngine creates a new Beeg engine
func NewBeegEngine(cfg *config.Config, torClient *tor.Client) *BeegEngine {
	return &BeegEngine{
		BaseEngine: NewBaseEngine("beeg", "Beeg", "https://beeg.com", 2, cfg, torClient),
		parser:     parsers.NewBeegParser(),
	}
}

// Search performs a search on Beeg
func (e *BeegEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
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
	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			results = append(results, e.convertToResult(item))
		}
	})
	return results, nil
}

func (e *BeegEngine) convertToResult(item *parsers.VideoItem) models.Result {
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

// SupportsFeature returns whether the engine supports a feature
func (e *BeegEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
