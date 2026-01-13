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

// PornHubEngine implements the PornHub search engine using HTML scraping
// Switched from API to HTML to extract preview URLs (data-mediabook)
type PornHubEngine struct {
	*BaseEngine
	parser *parser.PornHubParser
}

// NewPornHubEngine creates a new PornHub engine
func NewPornHubEngine(cfg *config.Config, torClient *tor.Client) *PornHubEngine {
	e := &PornHubEngine{
		BaseEngine: NewBaseEngine("pornhub", "PornHub", "https://www.pornhub.com", 1, cfg, torClient),
		parser:     parser.NewPornHubParser(),
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

// Search performs a search on PornHub using HTML scraping
func (e *PornHubEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	// PornHub search URL
	searchURL := fmt.Sprintf("%s/video/search?search=%s&page=%d",
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

	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			results = append(results, e.convertToResult(item))
		}
	})

	return results, nil
}

// convertToResult converts VideoItem to model.VideoResult
func (e *PornHubEngine) convertToResult(item *parser.VideoItem) model.VideoResult {
	result := model.VideoResult{
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

// SupportsFeature checks if PornHub supports a specific feature
func (e *PornHubEngine) SupportsFeature(feature Feature) bool {
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

// formatViews formats view count to human readable string
func formatViews(views int64) string {
	if views >= 1000000 {
		return fmt.Sprintf("%.1fM views", float64(views)/1000000)
	}
	if views >= 1000 {
		return fmt.Sprintf("%.1fK views", float64(views)/1000)
	}
	return strconv.FormatInt(views, 10) + " views"
}
