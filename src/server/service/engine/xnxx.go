// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// XNXXEngine implements the XNXX search engine
type XNXXEngine struct {
	*BaseEngine
	parser *parser.XNXXParser
}

// NewXNXXEngine creates a new XNXX engine
func NewXNXXEngine(appConfig *config.AppConfig) *XNXXEngine {
	e := &XNXXEngine{
		BaseEngine: NewBaseEngine("xnxx", "XNXX", "https://www.xnxx.com", 1, appConfig),
		parser:     parser.NewXNXXParser(),
	}
	// Set capabilities per IDEA.md
	e.SetCapabilities(Capabilities{
		HasPreview:    false, // XNXX doesn't have preview URLs in search results
		HasDownload:   true,
		HasDuration:   true,
		HasViews:      true,
		HasRating:     false,
		HasQuality:    true,
		HasUploadDate: false,
		PreviewSource: "",
		APIType:       "html",
	})
	return e
}

// Search performs a search on XNXX
func (e *XNXXEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
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

	var results []model.VideoResult

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

// convertToResult converts VideoItem to model.VideoResult
func (e *XNXXEngine) convertToResult(item *parser.VideoItem) model.VideoResult {
	// Use video page URL as download URL (works with yt-dlp)
	downloadURL := item.DownloadURL
	if downloadURL == "" {
		downloadURL = item.URL
	}
	return model.VideoResult{
		ID:              GenerateResultID(item.URL, e.Name()),
		URL:             item.URL,
		Title:           item.Title,
		Thumbnail:       item.Thumbnail,
		PreviewURL:      item.PreviewURL,
		DownloadURL:     downloadURL,
		Duration:        item.Duration,
		DurationSeconds: item.DurationSeconds,
		Views:           item.Views,
		ViewsCount:      item.ViewsCount,
		Description:     item.Quality,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
		Tags:            item.Tags,
		Performer:       item.Uploader,
	}
}

// SupportsFeature checks supported features
func (e *XNXXEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
