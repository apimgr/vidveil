// SPDX-License-Identifier: MIT
package engine

import (
	"context"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// PornMDEngine searches PornMD (meta-search)
type PornMDEngine struct {
	*BaseEngine
	parser *parser.PornMDParser
}

// NewPornMDEngine creates a new PornMD engine
func NewPornMDEngine(appConfig *config.AppConfig, torClient *tor.TorClient) *PornMDEngine {
	e := &PornMDEngine{
		BaseEngine: NewBaseEngine("pornmd", "PornMD", "https://www.pornmd.com", 2, appConfig, torClient),
		parser:     parser.NewPornMDParser(),
	}
	// Set capabilities per IDEA.md
	e.SetCapabilities(Capabilities{
		HasPreview:    false,
		HasDownload:   true,
		HasDuration:   true,
		HasViews:      false, // PornMD doesn't show view counts on search results
		HasRating:     true,  // PornMD shows rating percentage
		HasQuality:    false, // Quality info not consistently available
		HasUploadDate: false,
		PreviewSource: "",
		APIType:       "html",
	})
	return e
}

// Search performs a search on PornMD
func (e *PornMDEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/straight/{query}?page={page}", query, page)
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

func (e *PornMDEngine) convertToResult(item *parser.VideoItem) model.VideoResult {
	desc := item.Quality
	if item.Description != "" {
		if desc != "" {
			desc += " | " + item.Description
		} else {
			desc = item.Description
		}
	}
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
		Description:     desc,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
	}
}

// SupportsFeature returns whether the engine supports a feature
func (e *PornMDEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
