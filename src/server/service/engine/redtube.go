// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// RedTubeEngine implements the RedTube search engine using HTML scraping
// Switched from API to HTML to extract preview URLs (data-mediabook)
type RedTubeEngine struct {
	*BaseEngine
	parser *parser.RedTubeParser
}

// NewRedTubeEngine creates a new RedTube engine
func NewRedTubeEngine(cfg *config.Config, torClient *tor.Client) *RedTubeEngine {
	e := &RedTubeEngine{
		BaseEngine: NewBaseEngine("redtube", "RedTube", "https://www.redtube.com", 1, cfg, torClient),
		parser:     parser.NewRedTubeParser(),
	}
	// Set capabilities per IDEA.md
	e.SetCapabilities(Capabilities{
		HasPreview:    true,
		HasDownload:   false,
		HasDuration:   true,
		HasViews:      true,
		HasRating:     true,
		HasQuality:    true,
		HasUploadDate: false,
		PreviewSource: "data-mediabook",
		APIType:       "html",
	})
	return e
}

// Search performs a search on RedTube using HTML scraping
func (e *RedTubeEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	// RedTube search URL
	searchURL := fmt.Sprintf("%s/?search=%s&page=%d",
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

	var results []model.Result

	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			results = append(results, e.convertToResult(item))
		}
	})

	return results, nil
}

// convertToResult converts VideoItem to model.Result
func (e *RedTubeEngine) convertToResult(item *parser.VideoItem) model.Result {
	result := model.Result{
		ID:              GenerateResultID(item.URL, e.Name()),
		URL:             item.URL,
		Title:           item.Title,
		Thumbnail:       item.Thumbnail,
		PreviewURL:      item.PreviewURL,
		Duration:        item.Duration,
		DurationSeconds: item.DurationSeconds,
		Views:           item.Views,
		ViewsCount:      item.ViewsCount,
		Quality:         item.Quality,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
	}

	// Parse rating if available
	if item.Rating != "" {
		if r, err := strconv.ParseFloat(item.Rating, 64); err == nil {
			result.Rating = r
		}
	}

	return result
}

// SupportsFeature checks if RedTube supports a specific feature
func (e *RedTubeEngine) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeaturePagination:
		return true
	case FeatureSorting:
		return true
	case FeatureThumbnailPreview:
		return true
	default:
		return false
	}
}
