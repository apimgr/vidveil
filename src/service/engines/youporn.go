// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/parser"
	"github.com/apimgr/vidveil/src/service/tor"
)

// YouPornEngine searches YouPorn
type YouPornEngine struct{ *BaseEngine }

// NewYouPornEngine creates a new YouPorn engine
func NewYouPornEngine(cfg *config.Config, torClient *tor.Client) *YouPornEngine {
	e := &YouPornEngine{NewBaseEngine("youporn", "YouPorn", "https://www.youporn.com", 2, cfg, torClient)}
	// Don't use spoofed TLS - standard client works for YouPorn
	return e
}

// Search performs a search on YouPorn
func (e *YouPornEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	// YouPorn search URL format
	searchURL := fmt.Sprintf("%s/search/?query=%s&page=%d", e.baseURL, url.QueryEscape(query), page)

	resp, err := e.MakeRequest(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []model.Result

	// YouPorn uses .video-box for video containers
	doc.Find(".video-box").Each(func(i int, s *goquery.Selection) {
		// Get the video link
		link := s.Find("a.video-box-image, a.tm_video_link").First()
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}

		// Build full URL
		videoURL := href
		if !strings.HasPrefix(href, "http") {
			videoURL = e.baseURL + href
		}

		// Get title
		title := parser.CleanText(s.Find(".video-title-text").First().Text())
		if title == "" {
			title, _ = link.Attr("title")
		}
		if title == "" {
			return
		}

		// Get thumbnail
		img := s.Find("img.thumb-image").First()
		thumbnail := parser.ExtractAttr(img, "data-src", "data-thumb", "src")
		if thumbnail != "" && strings.HasPrefix(thumbnail, "//") {
			thumbnail = "https:" + thumbnail
		}

		// Get duration
		duration := parser.CleanText(s.Find(".video-duration").First().Text())

		// Get views (if available)
		views := parser.CleanText(s.Find(".video-views").First().Text())

		results = append(results, model.Result{
			ID:            GenerateResultID(videoURL, e.Name()),
			URL:           videoURL,
			Title:         title,
			Thumbnail:     thumbnail,
			Duration:      duration,
			Views:         views,
			Source:        e.Name(),
			SourceDisplay: e.DisplayName(),
		})
	})

	return results, nil
}

// SupportsFeature returns whether the engine supports a feature
func (e *YouPornEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}
