// SPDX-License-Identifier: MIT
package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// TubeGaloreEngine searches TubeGalore
type TubeGaloreEngine struct{ *BaseEngine }

// NewTubeGaloreEngine creates a new TubeGalore engine
func NewTubeGaloreEngine(appConfig *config.AppConfig) *TubeGaloreEngine {
	e := &TubeGaloreEngine{NewBaseEngine("tubegalore", "TubeGalore", "https://www.tubegalore.com", 3, appConfig)}
	// TubeGalore uses ttcache.com CDN; preview URL is constructed from data-public-id
	e.SetCapabilities(Capabilities{
		HasPreview:    true,
		HasDuration:   true,
		HasRating:     true,
		PreviewSource: "ttcache-constructed",
		APIType:       "html",
	})
	return e
}

// Search performs a search on TubeGalore
func (e *TubeGaloreEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	searchURL := e.BuildSearchURL("/search/{query}?page={page}", query, page)
	return searchTTCache(ctx, e.BaseEngine, searchURL)
}

// SupportsFeature returns whether the engine supports a feature
func (e *TubeGaloreEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}

// searchTTCache is shared scraper logic for sites running on the ttcache.com CDN platform
// (TubeGalore, PornHD). Cards expose data-public-id which maps to preview URLs.
func searchTTCache(ctx context.Context, e *BaseEngine, url string) ([]model.VideoResult, error) {
	resp, err := e.MakeRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	DebugLogEngineResponse(e.Name(), url, body)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var results []model.VideoResult
	doc.Find("div[data-public-id]").Each(func(i int, s *goquery.Selection) {
		videoID, exists := s.Attr("data-public-id")
		if !exists || videoID == "" {
			return
		}

		link := s.Find("a").First()
		href, _ := link.Attr("href")
		if href == "" {
			return
		}
		if !strings.HasPrefix(href, "http") {
			href = e.baseURL + href
		}

		title := parser.ExtractAttr(link, "title")
		if title == "" {
			title = parser.CleanText(s.Find(".item-title, .title, h3, h4").First().Text())
		}
		if title == "" {
			return
		}

		// Thumbnail is in the img src (regular src, no lazy loading)
		img := s.Find("img").First()
		thumbnail := parser.ExtractAttr(img, "src", "data-src", "data-original")
		if thumbnail != "" && !strings.HasPrefix(thumbnail, "http") {
			if strings.HasPrefix(thumbnail, "//") {
				thumbnail = "https:" + thumbnail
			} else {
				thumbnail = e.baseURL + thumbnail
			}
		}

		// Preview URL is constructed from the video ID using the ttcache CDN pattern
		previewURL := fmt.Sprintf("https://c1.ttcache.com/thumbnail/%s/288x162/preview.mp4", videoID)

		// Duration: check text selectors then fall back to data attributes on the container
		durationText := parser.CleanText(s.Find(".item-duration, .video-duration, .duration, .time, .length, .thumb-icon.video-duration").First().Text())
		if durationText == "" {
			// Some ttcache sites put seconds in data-seconds or data-duration on the card
			durationText = parser.ExtractAttr(s, "data-duration", "data-seconds", "data-length")
		}
		duration, durationSeconds := parser.ParseDuration(durationText)

		// Rating badge (e.g. "51%") — Rating field is float64, skip for now
		_ = parser.CleanText(s.Find(".rating-badge, .rating, .score").First().Text())

		results = append(results, model.VideoResult{
			ID:              GenerateResultID(href, e.Name()),
			URL:             href,
			Title:           title,
			Thumbnail:       thumbnail,
			PreviewURL:      previewURL,
			Duration:        duration,
			DurationSeconds: durationSeconds,
			Source:          e.Name(),
			SourceDisplay:   e.DisplayName(),
		})
	})

	DebugLogEngineParseResult(e.Name(), len(results), map[string]int{
		"preview": len(results), // every result gets a constructed preview
		"thumb":   len(results),
	})

	return results, nil
}
