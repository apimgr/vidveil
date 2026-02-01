// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"fmt"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// MotherlessEngine searches Motherless
type MotherlessEngine struct {
	*BaseEngine
	parser *parser.MotherlessParser
}

// NewMotherlessEngine creates a new Motherless engine
func NewMotherlessEngine(appConfig *config.AppConfig) *MotherlessEngine {
	return &MotherlessEngine{
		BaseEngine: NewBaseEngine("motherless", "Motherless", "https://motherless.com", 3, appConfig),
		parser:     parser.NewMotherlessParser(),
	}
}

// Search performs a search on Motherless
func (e *MotherlessEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := fmt.Sprintf("%s/term/videos/%s?page=%d",
		e.baseURL, url.QueryEscape(query), page)

	resp, err := e.MakeRequest(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []model.VideoResult
	fieldStats := map[string]int{
		"preview": 0,
		"thumb":   0,
		"quality": 0,
		"views":   0,
	}

	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			result := e.convertToResult(item)
			results = append(results, result)

			// Track field extraction stats
			if item.PreviewURL != "" {
				fieldStats["preview"]++
			}
			if item.Thumbnail != "" {
				fieldStats["thumb"]++
			}
			if item.Quality != "" {
				fieldStats["quality"]++
			}
			if item.ViewsCount > 0 {
				fieldStats["views"]++
			}
		}
	})

	// Log extraction stats for debugging
	DebugLogEngineParseResult(e.Name(), len(results), fieldStats)

	return results, nil
}

// convertToResult converts VideoItem to model.VideoResult
func (e *MotherlessEngine) convertToResult(item *parser.VideoItem) model.VideoResult {
	return model.VideoResult{
		ID:              GenerateResultID(item.URL, e.Name()),
		URL:             item.URL,
		Title:           item.Title,
		Thumbnail:       item.Thumbnail,
		PreviewURL:      item.PreviewURL,
		DownloadURL:     item.DownloadURL,
		Duration:        item.Duration,
		DurationSeconds: item.DurationSeconds,
		Views:           item.Views,
		ViewsCount:      item.ViewsCount,
		Quality:         item.Quality,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
		Tags:            item.Tags,
		Performer:       item.Uploader,
	}
}

// SupportsFeature returns whether the engine supports a feature
// Note: Motherless does NOT provide video previews in search results
func (e *MotherlessEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
