// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// XVideosEngine implements the XVideos search engine
type XVideosEngine struct {
	*BaseEngine
	parser *parser.XVideosParser
}

// NewXVideosEngine creates a new XVideos engine
func NewXVideosEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *XVideosEngine {
	e := &XVideosEngine{
		BaseEngine: NewBaseEngine("xvideos", "XVideos", "https://www.xvideos.com", 1, appConfig, torClient),
		parser:     parser.NewXVideosParser(),
	}
	// Set capabilities per IDEA.md
	e.SetCapabilities(Capabilities{
		HasPreview:    true,
		HasDownload:   true,
		HasDuration:   true,
		HasViews:      true,
		HasRating:     false,
		HasQuality:    true,
		HasUploadDate: false,
		PreviewSource: "data-pvv",
		APIType:       "html",
	})
	return e
}

// Search performs a search on XVideos
func (e *XVideosEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	// XVideos uses 0-based pagination
	searchURL := fmt.Sprintf("%s/?k=%s&p=%d", e.baseURL, query, page-1)

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
func (e *XVideosEngine) convertToResult(item *parser.VideoItem) model.VideoResult {
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
		Description:     item.Quality,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
	}
}

// SupportsFeature checks if XVideos supports a specific feature
func (e *XVideosEngine) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeaturePagination:
		return true
	case FeatureSorting:
		return true
	default:
		return false
	}
}
