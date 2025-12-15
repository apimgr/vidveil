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

// YouPornEngine searches YouPorn
type YouPornEngine struct {
	*BaseEngine
	parser *parsers.YouPornParser
}

// NewYouPornEngine creates a new YouPorn engine
func NewYouPornEngine(cfg *config.Config, torClient *tor.Client) *YouPornEngine {
	return &YouPornEngine{
		BaseEngine: NewBaseEngine("youporn", "YouPorn", "https://www.youporn.com", 1, cfg, torClient),
		parser:     parsers.NewYouPornParser(),
	}
}

// Search performs a search on YouPorn
func (e *YouPornEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/search/?query={query}&page={page}", query, page)
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

func (e *YouPornEngine) convertToResult(item *parsers.VideoItem) models.Result {
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
func (e *YouPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
