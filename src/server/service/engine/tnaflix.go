// SPDX-License-Identifier: MIT
package engine

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// TNAFlixEngine searches TNAFlix
type TNAFlixEngine struct{ *BaseEngine }

// NewTNAFlixEngine creates a new TNAFlix engine
func NewTNAFlixEngine(appConfig *config.AppConfig) *TNAFlixEngine {
	e := &TNAFlixEngine{NewBaseEngine("tnaflix", "TNAFlix", "https://www.tnaflix.com", 3, appConfig)}
	// TNAFlix has preview URLs in data attributes
	e.SetCapabilities(Capabilities{
		HasPreview:    true,
		HasDownload:   false,
		HasDuration:   true,
		HasViews:      true,
		HasRating:     false,
		HasQuality:    false,
		PreviewSource: "data-preview",
		APIType:       "html",
	})
	return e
}

// Search performs a search on TNAFlix with preview URL extraction
func (e *TNAFlixEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search.php?what={query}&page={page}", query, page)

	resp, err := e.MakeRequest(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	DebugLogEngineResponse(e.Name(), searchURL, body)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var results []model.VideoResult
	doc.Find("div.col-xs-6.col-md-4").Each(func(i int, s *goquery.Selection) {
		// Find link
		link := s.Find("a").First()
		href := parser.ExtractAttr(link, "href")
		if href == "" {
			return
		}
		if !strings.HasPrefix(href, "http") {
			href = e.baseURL + href
		}

		// Get title
		title := parser.ExtractAttr(link, "title")
		if title == "" {
			title = parser.CleanText(s.Find(".title, .video-title, h4").First().Text())
		}
		if title == "" {
			return
		}

		// Get thumbnail
		img := s.Find("img").First()
		thumbnail := parser.ExtractAttr(img, "data-src", "data-original", "src")
		if thumbnail != "" && !strings.HasPrefix(thumbnail, "http") {
			if strings.HasPrefix(thumbnail, "//") {
				thumbnail = "https:" + thumbnail
			} else {
				thumbnail = e.baseURL + thumbnail
			}
		}

		// Get preview URL - TNAFlix may use data-preview, data-video-preview, or rollover attributes
		previewURL := parser.ExtractAttr(s, "data-preview", "data-video-preview", "data-rollover")
		if previewURL == "" {
			previewURL = parser.ExtractAttr(img, "data-preview", "data-video-preview", "data-rollover")
		}
		if previewURL != "" && !strings.HasPrefix(previewURL, "http") {
			if strings.HasPrefix(previewURL, "//") {
				previewURL = "https:" + previewURL
			}
		}

		// Get duration
		durationText := parser.CleanText(s.Find(".duration, .time, .length").First().Text())
		duration, durationSeconds := parser.ParseDuration(durationText)

		// Get views
		viewsText := parser.CleanText(s.Find(".views, .cnt").First().Text())
		views, viewsCount := parser.ParseViews(viewsText)

		results = append(results, model.VideoResult{
			ID:              GenerateResultID(href, e.Name()),
			URL:             href,
			Title:           title,
			Thumbnail:       thumbnail,
			PreviewURL:      previewURL,
			Duration:        duration,
			DurationSeconds: durationSeconds,
			Views:           views,
			ViewsCount:      viewsCount,
			Source:          e.Name(),
			SourceDisplay:   e.DisplayName(),
		})
	})

	DebugLogEngineParseResult(e.Name(), len(results), nil)
	return results, nil
}

// SupportsFeature returns whether the engine supports a feature
func (e *TNAFlixEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
